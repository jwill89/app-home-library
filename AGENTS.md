# Working on Home Library

The one deep guide to this repo, for agents and developers alike: read it before any change. The rules every project shares are shared-tooling's standards, indexed in [shared/standards/README.md](shared/standards/README.md); what is here is Home Library's own, and where this project still does something the standards rule out, the last section says so.

Never run `git commit`, `git tag`, `git push`, or create a repository without the owner's explicit permission for that specific action in the conversation. Prepare changes and wait. A release is the commit CI tags when a push to `main` raises `web/package.json`'s version, deployed with `task deploy`; `task deploy -- -Preview` ships the working tree as it is, to try a build on the droplet before it is committed.

Read PLAN.md before changing anything structural. It holds the product decisions, the data model, and the phase order. A change to any of them updates PLAN.md in the same change, with a dated line in docs/decisions.md.

## Where things are

- `cmd/homelibrary`: the one binary and its subcommands: `serve`, `read-text` (the text service), `migrate`, `backup`, `setup-code`, `import`, `openapi`, `dev-session` and `version`.
- `internal/`: packages by domain. Handlers in `internal/server` and `internal/api` stay thin; the rules live in `internal/catalog` and `internal/lookup`. `internal/db` is sqlc's output beside the hand-written open, search and migration code, with the queries in `internal/db/queries` and the goose migrations in `internal/db/migrations`.
- `web/`: the Vue app, embedded in the binary by `web/embed.go`, with its unit tests beside the code (`src/**/*.test.ts`) and its end-to-end suite in `web/e2e`.
- `deploy/`: the systemd units and the Apache vhost the deploy installs where they differ. The nightly backup's units are written by the deploy itself (shared-tooling's `Install-BackupTimer`).
- `scripts/`: `deploy.ps1` (`task deploy`; `Get-Help .\scripts\deploy.ps1 -Full` gives its whole run) and `pull-backup.ps1`, the daily copy of the droplet's backup to this machine, both on shared-tooling's SharedDeploy module.
- `docs/ops.md`: the host, releasing, the deploy, the backups and how to restore them. `docs/decisions.md`: a dated line for each decision. `CHANGELOG.md`: what each release changes, its section the release's notes.
- The app's one version is `web/package.json`'s. A push to `main` that raises it is tagged `v<version>` by CI's `release` job once every other job passes (`.github/scripts/release.sh`), and the binary reports that tag (`/healthz`, through the build's ldflags).
- `shared/`: the shared-tooling submodule, at the release its `VERSION` names. It moves only with `task shared:update -- vX.Y.Z`.

## Principles

- Other projects in `C:\Web Projects` (Kindred Nights above all) are references for tooling and host mechanics. Nothing is copied for being there; what every project shares comes from `shared/`.
- One way to do each thing: one API client, one form pattern, one cover component, one table. A second way is a refactor of the first.
- Endpoints are resource-shaped. Nothing is disabled or kept for later; dead code goes in the change that makes it dead.
- A lookup fills blanks. It never overwrites a field the owner typed or an import brought; what a source gives otherwise is offered, and taken only when the owner says.

## Go

- Stdlib first. Packages by domain. Handlers stay thin; rules live in `internal/catalog` and `internal/lookup`.
- All SQL through sqlc, but for the FTS5 statements sqlc cannot parse (the search and the index's writes, in `internal/db/search.go`) and the backup's `VACUUM INTO` (see docs/decisions.md). Migrations are append-only.
- Errors wrapped with context, logs through `slog`, no globals except config. Environment variables are prefixed `HL_`.

## Vue and TypeScript

- `<script setup lang="ts">` everywhere, no `any`. The generated API types are the only API types.
- Every list of records is a PrimeVue DataTable.
- The shared pieces are the only way to do their job: `PagePanel` for every box, `PageHeader` for every page title and the trail of pages above it (`PublicPage` frames the pages seen before signing in), `returnTo` and `goBack` for every page left once its work is done, `FormField` for every labelled control, `BookForm` for every book's details, `CopyFields` for every copy's, `LengthField` for every length (with `UnitChoice` for inches or centimetres), `BookCover` for every cover, `CoverPhoto` for every photo (a cover's or a page's), `TagInput` for every list of names, `CoverChoices` for every choice among covers (each named by its source and size), `BarcodeScanner` for every camera scan, `BookPicker` for every choice of a book from the library (`whole` for the book itself, not a copy), `PaperPicker` for every choice of a paper, `LoanDialog` for every loan, `CopyGoneDialog` for every copy marked gone, `SuggestedChanges` for every change a lookup offers (a book's, a series', or papers'), `ReadStatusChoice` for every reading status in a table's row, `SubjectList` for every record's subjects, `CourseDialog` for every course, `CourseCopyDialog` for putting a copy on a course, `LateNote` for every overdue loan, `CitationBox` for every citation, `DocumentForm` for every document's details, `DoiSearchDialog` for every search for a paper's DOI, `LibraryTabs` between books and papers, `ShelfFields` for every shelf's measurements, `lengthIn` for every length shown, `useConfirmAction` for every confirmation, `useRemembered` (and `recall`, `remember`) for every choice kept on the device, `useReturnLoan` for every copy marked back, `useFeed` (or `listen`, for a store) for every change a page follows, the background's work and another device's edits alike (`useQueue` is the lookup's, shown by `QueueProgress`), `refreshRows` for every list it brings up to date (only the records the feed names, fetched by `?ids=`), `useNotice` for toasts, `copyText` for every copy to the clipboard, `librarySearchLink` (with `workWords`) for every search of the owner's library, `isOffline` for every answer that came without a connection, the library store's `setStatus` for every reading status changed, `addDocumentFile` for every file a document takes, `detailLabel` and `detailText` for every detail of a book named or shown, `detailChoices` for every filter by detail, `paperDetailLabel` and `paperDetailText` for every detail of a paper's, `count` for every number of things in words, `listed` for every list of names in a sentence, `seriesLine` for every book's place in its series, `courseLabel` for every course named as a choice, `copyLabel` for every copy named as a choice, `containerLabel` for every name of where a paper appeared. A second way is a refactor of the first.
- Icons are `<FontAwesomeIcon :icon="faName" />`, each definition imported where it is used from the site's kit, `@awesome.me/kit-4f2bd9b614/icons/classic/regular` (the kit Fantasy Dossier and the Gallery share). A style the kit lacks is added to the kit first, since an import from it fails to build.

## UI and accessibility

- WCAG 2.2 AAA is the target, in the light scheme and the dark one alike. A shortfall that cannot be helped is a dated line in docs/decisions.md, never a silent one.
- Contrast: 7:1 for text, 4.5:1 for large text (24px, or 19px bold), 3:1 for control boundaries, focus rings, and meaningful icons. `src/app/theme/contrast.test.ts` computes every pair from the preset; a new color pair is added there.
- Colors only through tokens in `web/src/app/theme/`; `npm run lint` fails on a literal color anywhere else. Token names say what they are for (`ink-muted`, `hairline`), never what they look like.
- Light and dark follow the device, or the owner's choice on it in Settings, and are equals: every page is checked in both. The scheme is one class on the page's root (`scheme-dark`), set only by `app/scheme.ts`; nothing styles by `prefers-color-scheme` itself.
- Every control is at least 44 by 44 CSS pixels, with a pointer or a finger. Focus is always visible. Motion honors `prefers-reduced-motion`.
- Prose (descriptions, notes, help) is at most 70 characters wide with line height 1.5 or more, never justified.
- A skip link leads every page in the app's frame; one `h1` a page, headings in order; landmarks for the bar, the navigation, and the main content. The pages before signing in are a main alone, with nothing to skip.
- A link standing on its own, not in a sentence, takes `link-target` (a table row's, `row-link`) for its 44-pixel reach. A link inside a sentence is underlined (`underline`), since its color alone is not enough beside muted text (axe's `link-in-text-block`).
- Errors say what happened and what to do; a destructive action asks first and says what it removes.
- Check every page at 390px and at 1280px, in both schemes, with its dialogs open, before calling it done.

## Data

- SQLite: WAL, foreign keys on, busy timeout 5000, synchronous NORMAL; one write connection, a read pool; multi-statement writes in a transaction.
- Timestamps are RFC 3339 text in UTC, to the second (`2026-11-07T09:18:09Z`), written by the columns' `strftime('%Y-%m-%dT%H:%M:%SZ', 'now')` defaults. Money is integer cents. Dimensions are integer millimetres.

## Checks

- `task check` passes before a change is done: `task lint` (Go vet, the shared golangci-lint config, the front end's ESLint, Stylelint, token check, types and Prettier, the scripts, Markdown, the shared repo files and workflow refs, and the vulnerability audits), then `task test:coverage` and `task test:e2e`. CI runs the same through shared-tooling's workflows, plus the check that the generated code is current.
- The lint configs are shared-tooling's (`web/eslint.config.js`, `web/stylelint.config.mjs`, `web/prettier.config.js`, `web/tsconfig.json`, and `.golangci.yml`, generated from the template with `.golangci.local.yml` on top by `task shared:golangci:sync`). A rule turned off here says why, at the line it does not fit.
- Coverage floors only rise: the front end's in `web/vite.config.ts`, the Go module's in `.github/workflows/ci.yml` (sqlc's output left out of the count).
- The end-to-end suite runs against the real server built from the tree, on an empty library of its own; `e2e/a11y.e2e.ts` runs every main page through axe at WCAG 2.2 AA, so a new page goes on its list as well as `e2e/pages.e2e.ts`'s.

## Known traps

- The Go packages embed `web/dist`, so the front end builds before any Go build, test, lint or `go run` (`task` does it through `build:web`; CI's Go and generated jobs download the web job's build, and the end-to-end job builds it in its setup).
- A Go package ships inside one of the front end's npm packages (`web/node_modules/flatted`), so `go test ./...` here reaches it; golangci-lint skips it (`.golangci.local.yml`), and CI's Go job never installs node_modules.
- The text service writes the database too, so the deploy stops, starts and rolls back both services together; neither a migration nor a restore runs under it.
- The deploy's start check waits for `/healthz` on the host, and its smoke check needs this build's version from `/healthz` through Cloudflare; a version the ldflags don't set fails it.
- The end-to-end tests share one library and some depend on what another added, so they run in one worker, in order.
- The type checker reads TypeScript's narrowing across an `await` as final, so a flag a Stop button sets during the loop looks always false to `no-unnecessary-condition`; the two loops that check one say so at the line.

## Where this project departs from the standards

Each line is what the project does today where the standards rule it otherwise. The owner decides whether each moves or stays; one that stays gives its reason here.

- Units and vhost: the deploy ships `deploy/`'s systemd units and Apache vhost, but their host values (server names, certificate path, port, paths, service user) are written into the templates, not placeholders filled from the git-ignored deploy config (deploy-and-ops.md).
- Ownership: the binary and `/opt/homelibrary` are root's, so the service runs the binary and cannot change it; the data and the backups are the service's. The standards have the code owned by the service's own user, read-only to it under `ProtectSystem=strict`.
- Backups: the nightly backup compresses the database, but the covers' and documents' files in each day's `files-YYYY-MM-DD/` stay as hard links to the live files. They take no room until the library lets a file go, and photographs and PDFs barely compress; a compressed copy would hold every file twice.
- Icons: each component imports the definitions it uses and passes them to `:icon`, so there is no register in `src/fontawesome.ts` and no test of one; a missing icon fails the type check instead (components.md asks for the register and its test).
- Vitest: the specs are `*.test.ts` files beside the code, on Node, and one `web/tsconfig.json` with Node's types covers the app, the specs, the configs and `e2e/`. The standards put specs in `src/__tests__/` as `*.spec.ts`, on jsdom by default, under their own `tsconfig.vitest.json`.
- Timestamp columns: every one is declared `TEXT` with no CHECK, though its default already writes the standard form. They move to `DATETIME` with `CHECK (col IS strftime('%Y-%m-%dT%H:%M:%SZ', col))`, by database.md's table rebuild.
- Migrations: the goose migrations never set `PRAGMA user_version`. Each is to set it to its own number, with a test that a fresh database's `user_version` equals goose's version.
- Opening the database: `homelibrary serve` and `homelibrary migrate` create the file and its folder when it is missing (`db.Open`); only the commands that go through `existingDB`, such as `backup`, refuse a missing file.
- Schema: there is no committed schema file, and no schema test that fails when the migrations build anything else.
- Queries: the files in `internal/db/queries` use `SELECT *` and `RETURNING *` on 30 lines, where a query names exactly the columns it needs.
- Sign-in: the passkey ceremonies, sign-out and the dev session are plain `net/http` handlers under `/auth/*`, outside the API and its spec, and the front end calls them by hand. They are to be operations under `/api/v1/auth/*`, in the spec and through the typed client (plans/sign-in-on-huma.md).
- Sessions: scs's SQLite store keeps each session token as it is, so the database, its backups and the pulled dev copy hold live sign-ins; only a hash is to be kept. The cookie is `hl_session`, not `__Host-` prefixed.
- Server timeouts: the `http.Server` sets `ReadHeaderTimeout` only, without `ReadTimeout` and `IdleTimeout`.
- Log level: it is read once from `HL_LOG_LEVEL` at start, not held in a `slog.LevelVar`, so Debug cannot be turned on while the service runs.
- Cache headers: the API's answers send `Cache-Control: no-store`, with no `ETag`. The baseline is `private, no-store` for the owner's own answers and `no-cache` with an `ETag` for a public read (api-and-realtime.md).
- Service worker: `web/public/sw.js` is written by hand. It asks the network first for a page and keeps the shell only for opening without a connection, and keeps the content-hashed files the device has loaded; it never touches the API, covers or documents. The standards let an installable app keep its shell only through Workbox's versioned precache with `autoUpdate` (performance.md).
- Page structure: the skip link leads to `<main id="main">`, where accessibility.md names it `main-content`.
- Motion: one global rule in `web/src/app/theme/tailwind.css` cuts every transition and animation to nothing under `prefers-reduced-motion: reduce`, with `!important`. Each animation and transition is to honour reduced motion in its own rule (css.md).
- Front-end folders: the stores, the router and the app's helpers sit together in `web/src/app/`, with no `composables/`, `stores/` or `types/` folders (naming.md).
- Dependency scan: CI has no OSV-Scanner job (`osv.yml`). Every lockfile here carries a moderate minimatch advisory through ESLint and typescript-eslint, which the shared lint tooling brings and `npm audit fix` cannot lift, so the job would fail on every run; it joins once shared-tooling's `lint/web` moves past it.
