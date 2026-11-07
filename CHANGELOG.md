# Changelog

What changes in Home Library, release by release. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). A push to `main` that raises `web/package.json`'s version is tagged and released by CI with its section below as the notes; `task deploy` then ships it (docs/ops.md, Releasing).

## [Unreleased]

## [1.1.1] - 2026-11-07

### Fixed

- `/healthz` answers publicly, as the standards have it: the vhost no longer refuses it, so the deploy's check through Cloudflare can read the version, and an uptime check can poll it. It returns only the status, version, commit, build date and schema number. 1.1.0's deploy was rolled back on this check, so 1.1.1 is the first of the 1.1 line to run on the droplet.

## [1.1.0] - 2026-11-07

### Added

- Suggested changes: what the sources now give otherwise for the books, gathered on a page of its own (`/suggestions`) to review, kept up to date as the change feed names the books that changed.

### Changed

- Every nightly backup of the database is checked and compressed (`homelibrary-YYYY-MM-DD.sqlite.gz`), and so is each deploy's snapshot; older uncompressed copies are compressed by the next nightly run. `scripts/pull-backup.ps1` brings home `homelibrary.sqlite.gz`.
- The deploy runs on shared-tooling's SharedDeploy module over one SSH connection. It checks that Apache takes the vhost before anything stops, and after the swap fetches the site through Cloudflare (`/healthz` must name the new version, `/` its entry script), rolling back on a failure. The switch that skips the pull is `-NoDbPull`.
- A link inside a sentence is underlined, so it shows beside muted text without its color.

## [1.0.0] - 2026-11-05

- The first release: the catalog, covers, loans, courses, labels, documents, share links, imports and exports, and the nightly backup (PLAN.md).

[Unreleased]: https://github.com/jwill89/Home-Library/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/jwill89/Home-Library/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jwill89/Home-Library/releases/tag/v1.0.0
