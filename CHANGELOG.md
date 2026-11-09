# Changelog

What changes in Home Library, release by release. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). A push to `main` that raises `web/package.json`'s version is tagged and released by CI with its section below as the notes; `task deploy` then ships it (docs/ops.md, Releasing).

## [Unreleased]

### Changed

- `task deploy` ships the newest release CI tagged on main, built in a checkout of its own with clean installs from its lockfiles (shared-tooling 2.4.0). A commit after the release, such as a shared-tooling update, no longer stops it with "HEAD is not tagged", and nothing uncommitted reaches the site. The droplet records the release each deploy leaves running, and deploying one already running does nothing; `task deploy -- -Force` deploys it again. `-SkipBuild` now goes with `-Preview` alone.

## [1.1.2] - 2026-11-09

### Changed

- The cover detection runs on OpenCV 5.0 (`@techstark/opencv-js` 5.0.0-release.1), the first OpenCV release since 4.12. The package now hands its module over as a promise, so the detection worker waits on that; taken as it was, every photo would have come back with no edges found and the corners left to place by hand, with nothing to say why. The detection itself moved to `web/src/app/outline.ts`, where a test runs it on the real OpenCV, and an end-to-end test chooses a cover photo in the browser and waits for its corners to be found. The first detection on a device downloads about 230 KB more, compressed.
- Dependencies, within their ranges: Vue 3.5.45, Vite 8.3.7 and Tailwind CSS 4.3.5 on the frontend; the SQLite driver (`modernc.org/sqlite` 1.62.0) and `golang.org/x/image` 0.48.0 on the server.

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
