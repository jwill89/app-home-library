# Changelog

What changes in Home Library, release by release. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). A push to `main` that raises `web/package.json`'s version is tagged and released by CI with its section below as the notes; `task deploy` then ships it (docs/ops.md, Releasing).

## [Unreleased]

## [1.6.0] - 2026-11-17

### Added

- A Stacks tab under Library, where every book stands as a spine on bookcases drawn on the screen: as tall and as thick as the book (a thin one drawn as wide as a finger needs), in its cover's main colour, with its title, author and volume number on it. Arrange the shelves by category, call number, area, title, or author; the device keeps the choice. Find on the shelves marks every book that matches and goes to each in turn. Pull a book off its shelf, with a tap or the keyboard, to see its cover, author, series, call number, category, reading, publisher, format and size, and open its page from there. A visitor by a share link has the Stacks too, without the areas.
- On the Stacks, My bookcases draws your measured bookcases to scale, with their corner braces, as the layout in use fills them or as a fresh suggestion would, each shelf with its area, number of books, how full it is, and its range. Nothing is put to use from there.
- `GET /api/v1/areas?include=books` gives each area the books its rules take, in its order; the library list gives each book its size, its first copy's call number and place in shelf order, its first author as filed, and its cover's main colour.

## [1.5.0] - 2026-11-17

### Added

- A Textbook category, with its own tab on the Library and a place in an area's rules and a share link's. A book is a textbook when you set it on its Edit page, or while a copy of it is on a course and its category isn't set. A Bookshelf shelf named Textbooks imports as textbooks.

### Changed

- An omnibus's series number, such as 1-3, 1–3 or 1, 2 & 3, counts as the volumes it holds: it comes after volume 1 and before volume 2 on the series' page, on the shelves and in an area's order, and its volumes are no longer listed as missing. The Number field gives a phone's full keyboard, so a range can be typed.
- Every list with pages has its page controls above the rows as well as below, and turning the page from below goes back to the top of the list.

### Fixed

- An edition written "2nd ed" was cited as "2nd ed ed.". Every style now says "2nd ed." however the edition is written ("2nd edition", "Second edition", "2d ed", "2"), and cites one that says more, such as "Revised ed., incorporating new material", as written.

## [1.4.0] - 2026-11-15

### Added

- Settings has Server log detail: how much the server writes to its log, from everything to failures alone, until it next restarts (`GET` and `PUT /api/v1/log-level`).

### Changed

- After a release, the app takes the new version on its next load, and the installed app still opens without a connection. It keeps in advance only what the sign-in pages need, and clears the old version's stored files on its next load.
- Every date and time is stored in one checked form, and a date not set is stored as none rather than as empty text; the API sends the same values as before. Needs a step at deploy (docs/ops.md, Upgrading to 1.4.0).
- "Skip to the content" moves to the page's content without changing the address, and moving to another page puts the focus on its content.
- With reduced motion asked for on the device, dialogs, lists of choices and toasts fade instead of growing, and spinners pulse instead of turning; the lookup's progress bar and the switches move at once.
- A list the browser receives whole (the library, papers, loans and the rest) carries an `ETag`, kept in the browser's own cache and checked each time, so an unchanged one comes back as an empty 304; signing out, or a share link ending, lets go of what was kept. Other answers to the owner and to a share link's visitor are never kept by the browser, and a read made before signing in is answered 304 when unchanged.
- The API's description says who may call each operation.
- `homelibrary` refuses a missing database file instead of making an empty one, so a mistyped `HL_DB_PATH` stops it rather than starting a new library.
- `task deploy` gives the server's binary and folder to the service's own account, and closes the backups folder to all but it.

### Fixed

- Saving a book, copy, loan, borrower, course, paper, area, bookcase, shelf or share link that was changed on another device or page since you opened it put the old values back without asking. The app now says so and offers "Load the latest" or "Save mine over it", marking a loan back and taking a lookup's suggested changes included.
- A cover photo, a document's file or an import sent over a slow connection was cut off after five seconds; an upload now has ten minutes.
- A day that isn't on the calendar, such as 30 February, was taken as a loan's, a copy's or a reading's date; it is refused with the field named.

### Security

- The server closes a connection that is slow to send its request or sits idle between requests.

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

### Added

- The first release: the catalog, covers, loans, courses, labels, documents, share links, imports and exports, and the nightly backup.

[Unreleased]: https://github.com/jwill89/app-home-library/compare/v1.6.0...HEAD
[1.6.0]: https://github.com/jwill89/app-home-library/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/jwill89/app-home-library/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/jwill89/app-home-library/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/jwill89/app-home-library/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/jwill89/app-home-library/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/jwill89/app-home-library/compare/v1.1.2...v1.2.0
[1.1.2]: https://github.com/jwill89/app-home-library/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/jwill89/app-home-library/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jwill89/app-home-library/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jwill89/app-home-library/releases/tag/v1.0.0
