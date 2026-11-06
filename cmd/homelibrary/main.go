// Command homelibrary runs the Home Library server and its maintenance commands.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"homelibrary/internal/api"
	"homelibrary/internal/auth"
	"homelibrary/internal/backup"
	"homelibrary/internal/catalog"
	"homelibrary/internal/config"
	"homelibrary/internal/db"
	"homelibrary/internal/extract"
	"homelibrary/internal/files"
	"homelibrary/internal/server"
	"homelibrary/internal/version"
)

const usage = `usage: homelibrary <command>

  serve                     run the server (default)
  read-text                 run the text service: read the words in documents' files
  migrate                   apply pending migrations and print the schema version
  backup                    snapshot the database, covers, and documents; prune old snapshots
  setup-code [name]         print a one-time code that adds a passkey on a device
  import <file> [not-owned] import an export from Bookshelf, Goodreads, LibraryThing, or
                            EndNote (XML); books not marked owned: leave, library, or wishlist
  openapi                   print the OpenAPI document
  dev-session               print a signed-in session cookie (development only)
  version                   print the build information`

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "read-text":
		err = readText()
	case "migrate":
		err = migrate()
	case "backup":
		err = runBackup()
	case "setup-code":
		err = setupCode(os.Args[2:])
	case "import":
		err = importFile(os.Args[2:])
	case "openapi":
		err = openapi()
	case "dev-session":
		err = devSession()
	case "version":
		fmt.Println(version.String())
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "homelibrary:", err)
		os.Exit(1)
	}
}

func serve() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := server.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, app.Close()) }()
	if err := app.Listen(ctx); err != nil {
		return err
	}
	logger.Info("stopped")
	return nil
}

// newLogger is the one shape of log line: JSON on stdout, which journald captures.
func newLogger(cfg config.Config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
}

// existingDB is for the commands that work on the live database: run from a shell without
// the env file they would otherwise make an empty one at the default path and succeed.
func existingDB(cfg config.Config) (*db.DB, error) {
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return nil, fmt.Errorf("no database at %s; set HL_DB_PATH to the live one: %w", cfg.DBPath, err)
	}
	return db.Open(cfg.DBPath)
}

func migrate() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	v, err := db.Migrate(context.Background(), database.Write)
	if err != nil {
		return err
	}
	fmt.Println("schema version", v)
	return nil
}

// runBackup is the nightly timer's job. It never migrates: the live service owns the schema.
func runBackup() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := existingDB(cfg)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	path, err := backup.Run(context.Background(), database.Write, backup.Options{
		Dir: cfg.Backup.Dir, Keep: cfg.Backup.Keep, Logger: newLogger(cfg),
		Files: map[string]string{"media": cfg.MediaDir, "documents": cfg.DocsDir},
	})
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func setupCode(args []string) (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	if _, err := db.Migrate(ctx, database.Write); err != nil {
		return err
	}
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		name = "Owner"
	}
	q := db.New(database.Write)
	owner, err := auth.EnsureOwner(ctx, q, name)
	if err != nil {
		return err
	}
	code, expires, err := auth.NewSetupCode(ctx, q, owner.ID, 30*time.Minute)
	if err != nil {
		return err
	}
	fmt.Printf("Setup code for %s: %s (until %s)\n", owner.Name, code, expires.Local().Format("15:04"))
	for _, o := range cfg.Origins {
		fmt.Printf("  %s/setup\n", o)
	}
	return nil
}

func importFile(args []string) (err error) {
	if len(args) < 1 || len(args) > 2 {
		return errors.New("import needs the path of the export file, and may say where books not marked owned go")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	f, err := os.Open(args[0]) //nolint:gosec // the path the owner typed on the command line, to read
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // opened to read
	ctx := context.Background()
	database, _, svc, err := server.Services(ctx, cfg, newLogger(cfg))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	o := catalog.ImportOptions{NotOwned: "leave"}
	if len(args) > 1 {
		o.NotOwned = args[1]
	}
	if o.NotOwned != "leave" && o.NotOwned != "library" && o.NotOwned != "wishlist" {
		return fmt.Errorf("books not marked owned go to leave, library, or wishlist, not %q", o.NotOwned)
	}
	rep, err := svc.ImportFile(ctx, f, o)
	if err != nil {
		return err
	}
	fmt.Printf("%s, %d rows: %d books added, %d documents added, %d matched, %d imported before, %d wanted, %d not owned, %d without an ISBN; %d queued for lookup\n",
		rep.Source, rep.Rows, rep.Added, rep.Documents, rep.Matched, rep.Already, rep.Wanted, rep.NotOwned, rep.NoISBN, rep.Queued)
	for _, p := range rep.Problems {
		fmt.Println("  ", p)
	}
	return nil
}

func openapi() error {
	spec, err := api.Spec()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(spec, '\n'))
	return err
}

// devSession signs the owner in for a browser that cannot make a passkey, such as an
// automated one. It refuses unless every origin is this machine.
func devSession() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.Local() {
		return errors.New("dev-session runs only when every HL_ORIGINS entry is localhost")
	}
	ctx := context.Background()
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	if _, err := db.Migrate(ctx, database.Write); err != nil {
		return err
	}
	owner, err := auth.EnsureOwner(ctx, db.New(database.Write), "Owner")
	if err != nil {
		return err
	}
	token, err := auth.NewSessionToken(ctx, auth.NewSessionManager(database.Write, database.Read, false), owner.ID)
	if err != nil {
		return err
	}
	fmt.Println("hl_session=" + token)
	return nil
}

// readText is the text service: it reads the words in documents' files one at a time, with
// pdftotext and Tesseract, at the low priority its own unit gives it, so the web server never
// waits on it. It never migrates: the web server owns the schema.
func readText() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg).With("service", "text")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	database, err := existingDB(cfg)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, database.Close()) }()
	store, err := files.NewStore(cfg.DocsDir)
	if err != nil {
		return err
	}
	svc := &catalog.Service{DB: database, Files: store, Logger: logger}
	tools := extract.Tools{}
	logger.Info("reading documents' text")
	for {
		did, err := svc.ReadNextFile(ctx, tools.Text, extract.ErrNoTools)
		wait := 20 * time.Second // nothing waiting: look again shortly
		switch {
		case errors.Is(err, extract.ErrNoTools):
			logger.Error("cannot read text", "err", err)
			wait = 10 * time.Minute
		case err != nil:
			logger.Error("reading text", "err", err)
			wait = time.Minute
		case did:
			wait = time.Second // the next file, after a breath
		}
		select {
		case <-ctx.Done():
			logger.Info("stopped")
			return nil
		case <-time.After(wait):
		}
	}
}
