# Home Library

Never run `git commit`, `git tag`, `git push`, or create a repository without the owner's explicit permission for that specific action in the conversation. Prepare changes and wait. The owner commits nothing until a feature-complete 1.0.0; until then a build reaches the droplet with `task deploy -- -Preview`, which skips the clean-and-tagged check.

Read PLAN.md before changing anything structural. It holds the product decisions, the data model, and the phase order. A change to any of them updates PLAN.md in the same change, with a dated line in docs/decisions.md.

## Principles

- Other projects in `C:\Web Projects` (Kindred Nights above all) are references for tooling and host mechanics. Nothing is copied for being there.
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
- The shared pieces are the only way to do their job: `PagePanel` for every box, `PageHeader` for every page title and the trail of pages above it (`PublicPage` frames the pages seen before signing in), `returnTo` and `goBack` for every page left once its work is done, `FormField` for every labelled control, `BookForm` for every book's details, `CopyFields` for every copy's, `LengthField` for every length (with `UnitChoice` for inches or centimetres), `BookCover` for every cover, `CoverPhoto` for every photo (a cover's or a page's), `TagInput` for every list of names, `CoverChoices` for every choice among covers (each named by its source and size), `BarcodeScanner` for every camera scan, `BookPicker` for every choice of a book from the library (`whole` for the book itself, not a copy), `PaperPicker` for every choice of a paper, `LoanDialog` for every loan, `CopyGoneDialog` for every copy marked gone, `SuggestedChanges` for every change a lookup offers (a book's, a series', or papers'), `ReadStatusChoice` for every reading status in a table's row, `SubjectList` for every record's subjects, `CourseDialog` for every course, `CourseCopyDialog` for putting a copy on a course, `LateNote` for every overdue loan, `CitationBox` for every citation, `DocumentForm` for every document's details, `DoiSearchDialog` for every search for a paper's DOI, `LibraryTabs` between books and papers, `ShelfFields` for every shelf's measurements, `lengthIn` for every length shown, `useConfirmAction` for every confirmation, `useRemembered` (and `recall`, `remember`) for every choice kept on the device, `useReturnLoan` for every copy marked back, `useFeed` (or `listen`, for a store) for every change a page follows, the background's work and another device's edits alike (`useQueue` is the lookup's, shown by `QueueProgress`), `useNotice` for toasts, `copyText` for every copy to the clipboard, `librarySearchLink` (with `workWords`) for every search of the owner's library, `isOffline` for every answer that came without a connection, the library store's `setStatus` for every reading status changed, `addDocumentFile` for every file a document takes, `detailLabel` and `detailText` for every detail of a book named or shown, `paperDetailLabel` and `paperDetailText` for every detail of a paper's, `count` for every number of things in words, `listed` for every list of names in a sentence, `seriesLine` for every book's place in its series, `courseLabel` for every course named as a choice, `copyLabel` for every copy named as a choice, `containerLabel` for every name of where a paper appeared. A second way is a refactor of the first.

## UI and accessibility

- WCAG 2.2 AAA is the target, in the light scheme and the dark one alike. A shortfall that cannot be helped is a dated line in docs/decisions.md, never a silent one.
- Contrast: 7:1 for text, 4.5:1 for large text (24px, or 19px bold), 3:1 for control boundaries, focus rings, and meaningful icons. `src/app/theme/contrast.test.ts` computes every pair from the preset; a new color pair is added there.
- Colors only through tokens in `web/src/app/theme/`; `npm run lint` fails on a literal color anywhere else. Token names say what they are for (`ink-muted`, `hairline`), never what they look like.
- Light and dark follow the device, or the owner's choice on it in Settings, and are equals: every page is checked in both. The scheme is one class on the page's root (`scheme-dark`), set only by `app/scheme.ts`; nothing styles by `prefers-color-scheme` itself.
- Every control is at least 44 by 44 CSS pixels, with a pointer or a finger. Focus is always visible. Motion honors `prefers-reduced-motion`.
- Prose (descriptions, notes, help) is at most 70 characters wide with line height 1.5 or more, never justified.
- A skip link leads every page in the app's frame; one `h1` a page, headings in order; landmarks for the bar, the navigation, and the main content. The pages before signing in are a main alone, with nothing to skip.
- A link standing on its own, not in a sentence, takes `link-target` (a table row's, `row-link`) for its 44-pixel reach.
- Errors say what happened and what to do; a destructive action asks first and says what it removes.
- Check every page at 390px and at 1280px, in both schemes, with its dialogs open, before calling it done.

## Data

- SQLite: WAL, foreign keys on, busy timeout 5000, synchronous NORMAL; one write connection, a read pool; multi-statement writes in a transaction.
- Dates are ISO text. Money is integer cents. Dimensions are integer millimetres.

## General

- `task lint` and `task test` pass before a change is done.
- Comments explain why, in one line where one will do.
- Tables and columns snake_case, JSON camelCase, routes kebab-case.
