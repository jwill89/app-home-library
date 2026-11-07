# Home Library

A private catalog of the house's books, used from a desktop browser and from a phone, at books.jamescwill.com (and books.mathdad.me).

- Add a book by ISBN (typed, or scanned one after another with the phone's camera), by a title or author search, or by hand. The Library of Congress, Open Library, Google Books, and Hardcover are asked together and their answers merged, field by field, with the source of each kept.
- Covers fetched from those sources, or photographed and cropped on the phone.
- Work, edition, and copy: Dewey and LC numbers, format, dimensions, publisher and date, subjects; new or used, where and when it came from, condition; reading status.
- Loans, courses, barcode and spine labels, the school documents with their searchable text and citations, and a shelf planner.
- Read-only share links for visitors; passkey sign-in for the one owner.
- Imports from Bookshelf, Goodreads, and LibraryThing; exports as CSV and JSON; nightly backups with a copy on the owner's PC.

See [PLAN.md](PLAN.md) for the decisions, the data model, and the phases, and [docs/ops.md](docs/ops.md) for the droplet.

## Development

```bash
task setup   # frontend dependencies
task dev     # Vite on :5173, the server on :8130
task check   # what CI runs: lint, the tests with their coverage floors, the end-to-end suite
```

Go 1.27, Node 26, Task, sqlc, PowerShell 7 with PSScriptAnalyzer. A fresh clone runs `git submodule update --init` first, for the shared tooling in `shared/`. Copy `.env.example` to `.env`. [AGENTS.md](AGENTS.md) is the guide to working on it, and [shared/standards/README.md](shared/standards/README.md) the standards it follows.
