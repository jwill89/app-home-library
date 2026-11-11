# shared-tooling rollout

The passages of shared-tooling's cross-project plan (`PLAN.md` at v2.6.0) that name this project, moved here word for word on 2026-11-10 when shared-tooling became project-agnostic. The portfolio-wide rest is in `PLAN.md` beside the project folders.

## Summary

Four follow-up questions are answered below. Vue's Vapor mode is still a release candidate and will help these sites only a little. Only Home Library gains from a real-time change, and it's a server-sent event stream, not WebSockets. Two shared repos are worth building: one tooling submodule for every stack's lint, tests, CI and deploys, and the component kit. Rust isn't a fit for any migration.

## Decisions so far

The standardization work starts from these, so it doesn't reopen them. All were decided on November 5, 2026.

- Home Library moves from polling to an SSE stream.

Decided later the same day, while seeding shared-tooling:

- A failed start restores the database snapshot only where a project opts in: Kindred Nights, Home Library and Senpan do; the Gallery doesn't, since uploads made during the deploy would be lost.
- Home Library adopts the shared deploy module fully, so the module handles several units and services and an Apache vhost.

Decided 2026-11-07, where the projects' own docs disagreed (55 questions, now rules in `standards/`):

- **Accessibility:** the target is WCAG 2.2 AAA, as Kindred Nights, Home Library and Senpan's themes already state it, falling back to AA only where AAA can't be met, with each such place a dated line in the project's decision log. axe in the end-to-end suites enforces the AA floor; AAA contrast is held by each project's own contrast test. (Corrected 2026-11-07: v2.1.0's guide named AA as the target.)
- **First principle:** where a current, accepted RFC covers something (standards-track or a Best Current Practice, not obsoleted), every project follows it, since it is guidance the whole web already knows. A project that departs says why in its AGENTS.md. Timestamps are stored as RFC 3339 text in UTC, to the second (`2026-11-07T09:18:09Z`), in `DATETIME` columns with a `strftime` default and a CHECK that refuses any other form, and leave every API in that form. Date-only columns are `DATE`, holding `2026-11-07`; since the Go driver reads a `DATE` as midnight UTC, each Go project reads them through a small `Date` type that keeps the day, writes it back as text and documents it to huma as `format: date`. A missing date is `NULL`, not `''`. Tested on SQLite 3.54.1 and through modernc v1.61.1 and huma v2.40.0: the date functions read these forms, an index serves range queries, a `DATETIME` column reads into a `time.Time` that JSON writes back unchanged, a `DATE` column round-trips through the `Date` type as `2026-11-07`, and a `time.Time` passed as a parameter is refused by the CHECK. Kindred Nights' `business_date` and `sold_on` and Home Library's ten `*_on` columns (which use `''` for no date) move at their standards passes. Each project's existing timestamp columns move at its standards pass.

- **Code, tests and releases:** Go packages are split by domain, with no package-level globals, config included. Vue templates have no static `style=""`. BEM class names are for reusable components only. Vitest specs live in `src/__tests__/` as `*.spec.ts`, with the config in `vite.config.ts`. A repo carries one app version, and CI tags each release; a hand-made tag may be lightweight. A commit has a subject and, when the diff doesn't say why, a body. The CHANGELOG lists what users notice; git history keeps the rest. An agent may deploy when asked, asking before it connects. `AGENTS.md` is a repo's one deep guide. Font Awesome comes from the free packages where they cover a site's icons, otherwise from the Pro kit package the owner assigns the site; several sites may share a kit (revised twice the same day: Fantasy Dossier, the Gallery and Home Library now share one). A kit holds only the styles its sites import from, and every icon is a `<font-awesome-icon>`, never an `<i class="fa-…">` element or a kit script.

## Project by project

| Project | Stack now | Verdict |
| --- | --- | --- |
| Kindred Nights, Senpan, Home Library | Go + Vue / Vite / TS + SQLite | Keep |

## Where the real gains are

### 1. Stay on Vue; Vapor is a small gain, later

**How much it helps.** Not much that visitors would notice:

- **Where you might feel it:** screens that redraw many items often, such as Home Library's list (thousands of books filtered and sorted in the browser), Senpan's live bingo boards and Kindred's staff consoles, mostly on older phones.

### 5. Small inconsistencies

- **Two OpenAPI approaches:** huma (code-first) in Kindred Nights and Home Library, kin-openapi with a hand-written spec in Senpan. Settled: huma, so Senpan moves to it.

## Real-time: WebSockets, streams or REST

The projects that need WebSockets already use them. Only Home Library would gain from a change, and the right change there is a server-sent event (SSE) stream, not WebSockets.

| Project | Today | Best fit | Why |
| --- | --- | --- | --- |
| Home Library | REST, polling every 2.5–10 s while background work runs | SSE (reuse Kindred's feed) | Scan, queue and text-reading progress only flows to the browser |

For Home Library the switch is a nice-to-have: one owner, polling only while work runs, so the requests saved are few. The gain is progress that shows the moment it happens.

**WebSockets won't shrink Home Library's list.** It sends the whole library in one response (a decision from November 4). WebSockets change how bytes travel, not how many. If the list grows large:

- Add an ETag, so an unchanged list costs an empty 304 response. The Go server sets none today.
- Or keep a copy in the browser (IndexedDB) and fetch only rows changed since the last version, with the SSE feed saying when.
- Check the response is compressed in transit; JSON like this usually shrinks by most of its size.

## Shared packages

The copies have already drifted:

- Six `.golangci.yml` files, all different, though Kindred's and Home Library's differ only in comments, one path and the module name.

## Testing standards

| Stack | Today | Gap | shared-tooling adds |
| --- | --- | --- | --- |
| End-to-end | Playwright in Home Library and TarotGen.io; Puppeteer in the résumé site | Two tools | `test/e2e`: a Playwright base config and conventions; move the résumé site to it |

No test relies on a happy-dom-only API, so the switch is one config line per project plus a test run. The Gallery, Home Library, TarotGen.io and Kindred's markdown test move over; Senpan already uses jsdom. Tests that touch no DOM, like Fantasy Dossier's, stay on Vitest's faster `node` environment.

## Shared standards guide

The material exists but is scattered: about 8,600 lines across 24 files, including AGENTS.md in six repos, CLAUDE.md in three, CONTRIBUTING.md in four, plus plans, decision logs, ops notes and TarotGen.io's 2,037-line audit. Some rules already repeat word for word, such as Home Library's and Kindred's rule that an agent never commits without your permission.

## Project gaps

| Project | Missing | Note |
| --- | --- | --- |
| Home Library | First commit and a remote, changelog | Nothing is committed until 1.0.0 by your choice, so its CI has never run |

## Where to start

<!-- markdownlint-disable MD029 -->

4. **Seed shared-tooling from what you already have; no new rules yet.**
   - The PSScriptAnalyzer file that Home Library, Kindred and Marxbot already share becomes `lint/pwsh`.
5. **Pilot on Kindred Nights, then the Gallery.** Kindred has Go, Vue, C#, PowerShell, Markdown and CI that runs; the Gallery adds PHP. Home Library has no commits or remote yet, so its CI has never run and it can't show whether the shared workflows work. Kindred's pilot is done (2026-11-05): v0.29.4 deployed on the shared module over one held connection. It found three shared bugs, fixed in v1.1.0 and v1.1.1; its plugin publish on the module is still to be run, at its next plugin release. The Gallery's pilot is done (2026-11-06): 7.0.1 deployed on the shared module, with the smoke check and its rollback, and it gained Dependabot. It found one shared bug, Vite's warning about `test/web/vitest.js`, fixed in v1.1.2. Both are on v1.1.2 with CI passing, the PHP workflow included. Both moved to v1.2.0 on 2026-11-06 and took the pieces decided after them:
6. **Roll out one repo at a time**, on `main`, with no separate branches. Everything shared-tooling can do lands in one pass: swap in the shared configs, move its deploy script onto the shared module, turn on its scheduled backup and database pull, and add its basic end-to-end suite, together (decided 2026-11-05, after Kindred's pilot did them on two branches; backups added 2026-11-06). Point its CLAUDE.md at the standards, trim what moved there, and close that project's gaps (see Project gaps). Small fixes ride along: the SQLite switch in Senpan and Modem Monitor, jsdom, and Playwright for the résumé site. After the pilots, the order is Senpan, Marxbot, TarotGen.io, then the rest (decided 2026-11-06).
   - **Fantasy Dossier** (prepared 2026-11-07 on v2.1.4, not yet committed), the first rollout to take the standards from the start:
     - The shared configs in place of the private `eslint-config` submodule, which is removed. With it gone no project uses `jwill89/eslint-config`: the owner archived the repo and deleted every `ESLINT_CONFIG_TOKEN` secret (2026-11-07), the last in Family Card Game.
     - CI on the shared workflows, with a release job; the deploy on SharedDeploy (a swap of `dist` and `.htaccess`, a smoke check through Cloudflare, rollback); a basic end-to-end suite with axe. It has no database, so no backup timer or pull.
     - Vitest specs moved to `src/__tests__/`, with the config in `vite.config.ts`; `noUncheckedIndexedAccess` found 29 places, now handled.
     - Font Awesome moved from the per-style Pro packages to the kit `@awesome.me/kit-4f2bd9b614` (classic solid, classic regular and brands), made on 2026-11-07 and shared since with the Gallery and Home Library.
     - Its e2e suite waits for the route's entrance animation (`settled` in `e2e/fixtures.ts`) before axe runs: axe run while a dossier page was still dropping in through its 3D transform stalled the browser for good, failing CI's first run.
   - **Font Awesome bandwidth** (2026-11-07). The account went over its November bandwidth: CI installed from the Pro registry 50 times in the first week (13 Kindred, 21 Senpan, 15 TarotGen.io, 1 Dossier), since every changed lockfile missed setup-node's exact-key npm cache, and a job that failed after installing saved nothing. Senpan's kit held all 17 styles, a 201 MB download each time, about 4.2 GB of the 4.6 GB.
     - v2.1.5's shared workflows restore npm's download cache from the newest earlier run and save both caches straight after the install, so a package is fetched again only when its own version changes. Home Library's own CI took the same scheme.
     - The owner cut every kit to the styles its sites import (Senpan's to four, 42 MB), and the guide now says a kit holds only those.
     - The Gallery stopped loading its kit from `kit.fontawesome.com` on every page view: it installs the shared kit and renders inline SVG through `<font-awesome-icon>`, with a test that fails on an unregistered icon. Home Library moved from the per-style packages to the same kit.
     - Kits are not cut to chosen icons (Font Awesome's custom subsetting), decided 2026-11-07: it would make each package a few hundred KB, but every new icon would first need adding to the kit and a new package version. A few MB per style, fetched only when a kit's version changes, is the price of adding an icon freely.
   - **Home Library** (prepared 2026-11-07 on v2.1.6, not yet committed; its own suggestions feature was in progress beside it):
     - The shared configs and CI: web, go with a 57% floor, e2e with axe on every main page, pwsh, docs, and its own `generated` job.
     - The deploy on SharedDeploy: both services, with its units and vhost; a compressed snapshot; a smoke check through Cloudflare with `?deploy=` that rolls back; `Install-BackupTimer` running `homelibrary backup`; and the database and files pull. `pull-backup.ps1` moved onto the module too.
     - The nightly backup now compresses the database, and the files snapshot stays as hard links.
     - `noUncheckedIndexedAccess` found 7 places, and the shared ESLint rules about 530 findings (385 `curly`). Axe found two links in text told apart by colour alone.
     - No OSV job yet: a moderate minimatch advisory in the shared lint tooling (through ESLint and typescript-eslint) would fail it, and `npm audit fix` can't lift it.
   - **Release tags from CI** (2026-11-07). The standard has CI tag each release, but Kindred, Marxbot and Home Library still tagged by hand or not at all; Kindred's standards pass had recorded it as a departure instead of adopting it, and its deploy then refused an untagged HEAD. All three now have the Dossier's `release` job and `release.sh`, reading `web/package.json` (Kindred 0.30.1, Home Library 1.1.0) or a `VERSION` file (Marxbot 1.0.0, which the bot embeds and logs), and each deploy fetches the tags and refuses a HEAD whose tag isn't that version. Kindred's hand tags had gone backwards (v0.29.5 and v0.29.6 after v0.30.0). The Gallery's, TarotGen.io's and the Dossier's deploys then took the same check (their CI already tagged releases; the Gallery hadn't recorded the gap as a departure), with `-Preview` for an untagged try. Senpan's waits for its move to one app version, since CI still tags its parts separately.
   - **Newest-release deploys** (2026-11-09). A deploy that required HEAD to carry the tag stopped as soon as a tooling update landed after a release (Home Library's v2.3.2 commit after v1.1.2), and checking the tag out by hand meant matching its submodule too. `task deploy` now ships the newest release CI tagged on main, built in its own checkout with clean installs from its lockfiles, and skips a release the host already runs (a record in `~/.shared-deploy/` on the host, written only by a deploy that passed its checks; `-Force` deploys it again). All six rolled-out projects moved to it with shared-tooling 2.4.0; Senpan takes it with its overhaul.
7. **Then the migrations, in this order:**
   1. Senpan's overhaul, as five releases, each shipped and deployed on its own (decided 2026-11-09). Its API moves to huma in C, replacing its hand-written spec and kin-openapi.
      - **A, infrastructure (4.0.0):** one app version with `v<version>` tags; the SPA embedded in the Go binary, with `/healthz`; Apache serving only the document root's own folders and proxying the rest to the app; shared-tooling 2.4.0 and its newest-release deploy; the binary owned by the service's own account. Deployed 2026-11-09.
      - **B, the data layer (4.1.0):** goose from a baseline at v71, a write pool and a read pool, RFC 3339 timestamps (production's drifted tables rebuilt to match a fresh build, checked on a copy of the live data first), sessions in SQLite under hashed tokens, and draws from crypto/rand (the owner took both, 2026-11-09). sqlc waits for C, domain by domain. Deployed 2026-11-09.
      - **Before C, a small front-end fix** (found 2026-11-09, npm's deprecation warning on glob 11.1.0): Workbox's build (through vite-plugin-pwa) requires glob ^11 and js-beautify (through @vue/test-utils) ^13, both at their newest releases, and the advisory GHSA-3g3m-2fjr-6pw4 is fixed only in glob 14.0.2. Both call only `globSync`, which glob 14 still ships for CommonJS, so an npm override of glob ^14.0.2 for each should hold; check that the PWA build's precache list is unchanged and the tests pass, then drop the advisory's entry from `frontend/osv-scanner.toml`. Done 2026-11-09: the service worker built byte for byte the same, the tests passed, and OSV-Scanner reports nothing.
      - **C, the API to huma (5.0.0):** domain packages with thin handlers, RFC 9457 errors, 422 for validation, camelCase JSON with openapi-typescript, and the plugin's API models moving with it. Decided 2026-11-09: the plugin moves in the same release and deploys the same day, no endpoint keeps snake_case, and the public tea-room API changes too (the Carrd pages' scripts with it); the live feed moves from WebSockets to SSE, as Kindred's, since its traffic is one-way; 502s give way to the standard's 503/500/429, public reads take the caching baseline, and the tea-room API's `*` gives way to an allowlist setting. Its plan, domain by domain, is Senpan's `plans/release-c.md`.
      - **D, front-end cleanup:** specs in `src/__tests__/` with the config in `vite.config.ts`, and the static inline styles into CSS so the shared rule comes back on. It touches no backend, so it can come at any point.
      - **E, nothing served from `/var/www`, the goal (decided 2026-11-09):** the app serves everything, as Home Library and Kindred Nights do, with its vhost proxying `/` to it and its data under `/opt/senpan`. The uploads (`images/`, `fonts/`), the Carrd sites and the plugin repository leave the document root: the app serves `/images/`, `/plugin/` (as Kindred serves its own) and the Carrd sites, the `carrd.senpan.cafe` vhost proxies to it, `-webroot` gives way to a data folder, and the deploy ships the vhost, as Home Library's does. It comes after C, so the file serving is written once, into C's domain packages. The fonts need the most care. They are licensed, so they must not be reachable as plain files at any point in the move. `fonts.senpan.cafe/kit.css` and its token URLs, which external Carrd sites embed, must stay as they are through C and E. The live `fonts.senpan.cafe` vhost is read first, since the repo can't see whether it still names the folder. The folder moves whole, with its hidden `.woff2/` copies, while the service is stopped, and is checked afterwards from an allowed site and from Safari, which sends a `Referer` in place of an `Origin`.
   2. Sign-in inside huma in Home Library, then Kindred Nights (decided 2026-11-09, as Senpan's release C does it). Both sign in and out through plain handlers under `/auth/*`, outside huma and the API document; huma doesn't ask for that, and the standard puts these endpoints inside the API. They become operations under `/api/v1/auth/*`, called through the typed client, as Senpan's are. Kindred's Discord start and callback become operations that answer 302. Home Library's share links and calendar feed stay outside, being addresses already handed out. The standard's line 24 then names `/api/v1/auth/*` and gains a rule for operations a browser navigates to. The plan, project by project, is this repo's `plans/sign-in-on-huma.md`. It also covers what adding Discord sign-in to Senpan or Home Library later would take: it argues for an identities table, not for keeping sign-in outside huma.
      - **Home Library 1.2.0** (built 2026-11-10): the passkey sign-in, setup codes and sign-out are operations under `/api/v1/auth/*`, a passkey is made under `/api/v1/passkeys`, the typed client's 401 handler leaves `/auth/*` answers with their page, and the end-to-end suite runs the passkey ceremony with a virtual authenticator. It ships on the owner's go, with nothing to do outside the app.

   <!-- -->

   4. Home Library's event stream, with its 1.0 work. It's small and touches nothing else.

<!-- markdownlint-enable MD029 -->
