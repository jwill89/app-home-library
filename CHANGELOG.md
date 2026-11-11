# Changelog

What changes in Home Library, release by release. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). A push to `main` that raises `web/package.json`'s version is tagged and released by CI with its section below as the notes; `task deploy` then ships it (docs/ops.md, Releasing).

## [Unreleased]

## [1.2.0] - 2026-11-10

### Changed

- Signing in, setting up a device with a code, and signing out moved from `/auth/` into the API, under `/api/v1/auth/`, and a new passkey is made under `/api/v1/passkeys`. An installed app left open from an earlier release signs in again once it is reloaded.
- A passkey's name longer than 60 characters is refused rather than cut short.
- `task deploy` ships the newest release CI tagged on main, built from a clean checkout of its own, so commits made after the release and uncommitted work never reach the site.
- Deploying a release that is already running does nothing; `-Force` deploys it again.
- `-SkipBuild` goes with `-Preview` alone.

### Security

- The server is built with Go 1.27.4, which fixes a way it could crash while reading a request.
- The server's debug log keeps a short hash of a share or calendar link's token in place of the token.

## [1.1.2] - 2026-11-09

### Changed

- Cover detection runs on OpenCV 5.0. The first detection on a device downloads about 230 KB more.

## [1.1.1] - 2026-11-07

### Fixed

- `/healthz` was refused from outside the droplet, so the deploy's check through Cloudflare could not read the version and rolled 1.1.0 back. It now answers publicly, with only the status, version, commit, build date and schema number.

## [1.1.0] - 2026-11-07

### Added

- A Suggested changes page (`/suggestions`) gathers what the sources give otherwise for the books, to take or keep one by one. It updates itself as books change.

### Changed

- Nightly database backups, and each deploy's snapshot, are checked and compressed. Older copies are compressed by the next nightly run, and `scripts/pull-backup.ps1` brings home the compressed file.
- `task deploy` checks that Apache takes the vhost before anything stops, and after the swap fetches the site through Cloudflare, rolling back on a failure. `-NoDbPull` skips the database pull.
- A link inside a sentence is underlined, so it shows beside muted text without its color.

## [1.0.0] - 2026-11-05

- The first release: the catalog, covers, loans, courses, labels, documents, share links, imports and exports, and the nightly backup.

[Unreleased]: https://github.com/jwill89/Home-Library/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/jwill89/Home-Library/compare/v1.1.2...v1.2.0
[1.1.2]: https://github.com/jwill89/Home-Library/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/jwill89/Home-Library/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jwill89/Home-Library/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jwill89/Home-Library/releases/tag/v1.0.0
