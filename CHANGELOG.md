# Changelog

What changes in Home Library, release by release. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). A push to `main` that raises `web/package.json`'s version is tagged and released by CI with its section below as the notes; `task deploy` then ships it (docs/ops.md, Releasing).

## [Unreleased]

## [1.3.0] - 2026-11-13

### Added

- Areas of shelves on the Shelves page: each takes the books its rules say (categories, a Dewey range, subject words, authors, series, heights), in the order chosen (subject, author, series and volume, title, date, height), and a book goes to the first area that takes it. An area's shelves need not stand side by side.
- Ordering by subject uses the Dewey number, or one estimated from the LC class where a book has none, shown as estimated.
- A bookcase can be added with some shelves another height than the rest, such as a taller bottom shelf: each shelf's height is set from the top shelf down, and starts at the one the shelves share.

### Changed

- Lengths read to three significant figures (1.18 in, 8.35 in) where they read to a tenth, keeping whole, half, and quarter inches that are exact, and a length field shows the same.
- The Shelves page no longer lays books out unasked: the shelves are listed on their own, a layout is suggested only when asked for, and no book has a place (on its page, in Where does this go, or for the shelf check and range labels) until a suggestion is put to use. A layout in use before this release is let go.
- The API's `get-layout` takes `suggest` in place of `proposal`; shelves carry their area, and a copy's place its area.
- `POST /api/v1/bookcases` takes the new bookcase's shelves as a list from the top down, each with its own measurements, in place of a count and one shelf's.

### Removed

- Oversize shelves: an area for tall books, tried before the others, takes their place.

### Fixed

- Adding a second bookcase, or a shelf to a bookcase that already had one, was refused with "unexpected property": the form started from a copy of the last shelf whole, its id and place with its measurements.
- A request the browser gave up on, as a page left mid-request does, could sign the owner out: a sign-in is now forgotten only when its passkey or user is gone, not when the request could not be read through.

## [1.2.1] - 2026-11-11

### Changed

- `task deploy` fills the service's units and the site's vhost from the deploy config and retires any other enabled site serving the site's names; it needs new deploy settings (docs/ops.md, Upgrading to 1.2.1).

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

[Unreleased]: https://github.com/jwill89/app-home-library/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/jwill89/app-home-library/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/jwill89/app-home-library/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/jwill89/app-home-library/compare/v1.1.2...v1.2.0
[1.1.2]: https://github.com/jwill89/app-home-library/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/jwill89/app-home-library/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jwill89/app-home-library/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jwill89/app-home-library/releases/tag/v1.0.0
