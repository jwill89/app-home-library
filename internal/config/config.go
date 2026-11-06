// Package config reads the server's settings from HL_* environment variables.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds every setting the server reads at startup.
type Config struct {
	Addr      string
	Origins   []string // every public origin the site answers on, scheme://host[:port]
	DBPath    string
	MediaDir  string // covers, which visitors see too
	DocsDir   string // documents' files, which never leave through /media
	LogLevel  slog.Level
	GoogleKey string
	// HardcoverToken is a Hardcover account's API token, which turns Hardcover on as a source.
	HardcoverToken string
	Backup         Backup
}

// Backup is where `homelibrary backup` puts the daily snapshot and how many days it keeps.
type Backup struct {
	Dir  string
	Keep int
}

// Load reads the environment, applying defaults for anything unset.
func Load() (Config, error) {
	cfg := Config{
		Addr:           envOr("HL_ADDR", "127.0.0.1:8130"),
		DBPath:         envOr("HL_DB_PATH", "devdata/homelibrary.sqlite"),
		MediaDir:       envOr("HL_MEDIA_DIR", "devdata/media"),
		GoogleKey:      strings.TrimSpace(os.Getenv("HL_GOOGLE_BOOKS_KEY")),
		HardcoverToken: strings.TrimSpace(os.Getenv("HL_HARDCOVER_TOKEN")),
		Backup:         Backup{Dir: envOr("HL_BACKUP_DIR", "devdata/backups")},
	}
	// Documents' files sit beside the covers unless told otherwise: devdata/documents here,
	// /opt/homelibrary/data/documents on the droplet, with nothing to add to its settings.
	cfg.DocsDir = envOr("HL_DOCS_DIR", filepath.Join(filepath.Dir(cfg.MediaDir), "documents"))
	if err := cfg.LogLevel.UnmarshalText([]byte(envOr("HL_LOG_LEVEL", "info"))); err != nil {
		return cfg, fmt.Errorf("HL_LOG_LEVEL: %w", err)
	}
	keep := envOr("HL_BACKUP_KEEP", "28")
	days, err := strconv.Atoi(keep)
	if err != nil || days < 1 {
		return cfg, fmt.Errorf("HL_BACKUP_KEEP must be a number of days, got %q", keep)
	}
	cfg.Backup.Keep = days
	for _, o := range strings.Split(envOr("HL_ORIGINS", "http://localhost:5173"), ",") {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" {
			return cfg, fmt.Errorf("HL_ORIGINS: %q is not an origin like https://books.example.com", o)
		}
		cfg.Origins = append(cfg.Origins, u.Scheme+"://"+strings.ToLower(u.Host))
	}
	if len(cfg.Origins) == 0 {
		return cfg, fmt.Errorf("HL_ORIGINS names no origin")
	}
	return cfg, nil
}

// Secure reports whether every origin is HTTPS, which decides the cookie flag. A mix
// would leave the cookie unsent on one name or exposed on the other.
func (c Config) Secure() bool {
	for _, o := range c.Origins {
		if !strings.HasPrefix(o, "https://") {
			return false
		}
	}
	return true
}

// Local reports whether every origin is this machine, which only development is.
func (c Config) Local() bool {
	for _, o := range c.Origins {
		u, _ := url.Parse(o) // checked in Load
		if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" {
			return false
		}
	}
	return true
}

// Origin is the configured origin for a request's Host header, and false for a name the
// site does not answer on. Everything built from the request's name (the passkey's
// relying party above all) goes through here, never through the header itself.
func (c Config) Origin(host string) (string, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, o := range c.Origins {
		if strings.TrimPrefix(strings.TrimPrefix(o, "https://"), "http://") == host {
			return o, true
		}
	}
	return "", false
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
