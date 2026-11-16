# Operations

How the droplet is set up for Home Library, how a release goes out, where things live, how the data is backed up and restored, and what to check by hand.

The host's details are settings of the git-ignored `scripts/deploy.config.ps1` (copied from `scripts/deploy.config.example.ps1`), and this guide names them by their placeholders: `<service>` is the `ServiceName` setting (the web server's unit; the text service is `<service>-text`), `<service user>` the `ServiceUser`, `<opt dir>` the `RemoteOptDir` (the binary's folder, with `data/` and `backups/` inside it), `<cert>` the `CertName`, and `<env file>` the `EnvFile` (`/etc/<service>/<service>.env` unless set). The app listens on the loopback address at the port the `AppPort` setting names, which is written nowhere else.

## Host layout

| What | Where |
| --- | --- |
| Binary | `<opt dir>/homelibrary` (previous build at `homelibrary.old`), `<service user>`'s, read-only to the running service (`ProtectSystem=strict`) |
| Database | `<opt dir>/data/homelibrary.sqlite` |
| Covers | `<opt dir>/data/media/` |
| Documents' files | `<opt dir>/data/documents/` (served only through the API) |
| Backups and deploy snapshots | `<opt dir>/backups/` (0700, every file 0600) |
| Secrets and settings | `<env file>` (root's, 0600) |
| Service unit | `/etc/systemd/system/<service>.service` (from `deploy/homelibrary.service`) |
| Text service | `/etc/systemd/system/<service>-text.service` (from `deploy/homelibrary-text.service`): reads the words in documents' files |
| Backup timer | `/etc/systemd/system/<service>-backup.{service,timer}` |
| Apache | `/etc/apache2/sites-available/<service>.conf` (from `deploy/apache-vhost.conf`), the site's one vhost file, both ports |
| Logs | journald (see [Logs](#logs)) |

The units and the vhost are the repository's: templates in `deploy/`, filled from the deploy config (the service, its user and folder, the site's names, the certificate and the port), and installed by every deploy where they differ. The deploy enables the vhost and retires any other enabled site serving its names. A change to one is made in `deploy/` and shipped, never on the droplet by hand.

The service runs as its own locked account, `<service user>`, on the loopback address at `AppPort`, writing nothing outside `data/`. Apache terminates TLS for **books.jamescwill.com** and **books.mathdad.me** (one VirtualHost, the second name a `ServerAlias`) with the certificate `<cert>`, which covers both, and proxies both to the service with the name preserved. Each port-443 site sends HSTS. Cloudflare fronts both names, and the vhost takes the visitor's address from `CF-Connecting-IP`, believed only from Cloudflare's ranges in `/etc/apache2/cloudflare-ips.txt`. The owner's pages hold one request open each, the change feed (`/api/v1/changes`, Server-Sent Events), which the app pings every 25 seconds inside Apache's 120-second proxy timeout and Cloudflare's 100; Apache does not compress `text/event-stream`, so nothing holds the events back. Requests for share links (`/s/`) and the calendar (`/calendar/`) are not logged, their addresses being their passwords. `/healthz` answers publicly, through Cloudflare, for the deploy's smoke check and any uptime check, and says nothing private.

## One-time host setup

DNS: an A record for each of `books.jamescwill.com` and `books.mathdad.me` pointing at the droplet, both proxied by Cloudflare.

Fill in `scripts/deploy.config.ps1` from `scripts/deploy.config.example.ps1` on this machine. Then once on the droplet as root, with the four names filled in from the deploy config; it is safe to run again.

```bash
SERVICE='<service>'             # the ServiceName setting
SERVICE_USER='<service user>'   # the ServiceUser setting
OPT='<opt dir>'                 # the RemoteOptDir setting
CERT='<cert>'                   # the CertName setting
PORT='<AppPort>'                # the AppPort setting
ENV_FILE="/etc/$SERVICE/$SERVICE.env"

id "$SERVICE_USER" >/dev/null 2>&1 || useradd --system --home "$OPT" --shell /usr/sbin/nologin "$SERVICE_USER"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 755 "$OPT"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 750 "$OPT/data" "$OPT/data/media" "$OPT/data/documents"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 700 "$OPT/backups"
# The app never makes its database: a new one is an empty file, which the first start migrates.
[ -f "$OPT/data/homelibrary.sqlite" ] || install -o "$SERVICE_USER" -g "$SERVICE_USER" -m 600 /dev/null "$OPT/data/homelibrary.sqlite"
install -d -m 755 "/etc/$SERVICE"
[ -f "$ENV_FILE" ] || ( umask 077; cat > "$ENV_FILE" <<EOF
HL_ADDR=127.0.0.1:$PORT
HL_LOG_LEVEL=info
HL_ORIGINS=https://books.jamescwill.com,https://books.mathdad.me
HL_DB_PATH=$OPT/data/homelibrary.sqlite
HL_MEDIA_DIR=$OPT/data/media
HL_BACKUP_DIR=$OPT/backups
HL_BACKUP_KEEP=28
HL_GOOGLE_BOOKS_KEY=
HL_HARDCOVER_TOKEN=
EOF
)
chown root:root "$ENV_FILE" && chmod 600 "$ENV_FILE"
apt-get install -y sqlite3 poppler-utils tesseract-ocr tesseract-ocr-eng   # the deploy's snapshots; the text service's programs
a2enmod proxy proxy_http headers rewrite ssl remoteip && systemctl reload apache2
certbot certificates --cert-name "$CERT"   # both books.* names must be listed
```

Put the Google Books key in `HL_GOOGLE_BOOKS_KEY` with an editor. It is restricted in Google Cloud to the Books API and the droplet's address, so it works from the droplet and nowhere else; local development runs without it, or with a second key restricted to the home address.

The first `task deploy` installs the binary and the units, enabling and starting both services, installs and enables the vhost, and installs the nightly backup's timer (see [Backups](#backups)). Every deploy after it stops both, and starts the text service again on the new binary once the web service answers.

The text service (`homelibrary read-text`, unit `<service>-text`) reads the words in documents' files, one at a time, with `pdftotext` and Tesseract. The tools run at idle CPU and disk priority (`chrt`, `ionice`), and the service is capped at 80% of a CPU and 1 GB, with no network at all. The service itself keeps a normal priority: it holds the database's write lock while it saves what the tools read, and at idle priority a busy droplet could keep it there for seconds. A file it fails on three times is left for the owner to type.

### Memory

Each service's `MemoryMax=` is to be about twice its own peak after a stretch of normal use (`Get-ServiceMemory`, or `systemctl show -p MemoryPeak <service>`). Measured 2026-11-15, 2.3 days after the last start:

| Service | Peak | Cap |
| --- | --- | --- |
| `<service>` | 29 MiB | 768M |
| `<service>-text` | 7 MiB | 1G |

Neither window held the services' heaviest work, a phone's cover photo decoded (up to 40 megapixels) or a document's pages read by Tesseract, so the caps stay as they were until a week's peak that includes both is read after the 1.4.0 deploy; then each goes to twice its peak, rounded up, and this table takes the new figures and date.

## Signing in the first time

Passkeys belong to the name they are made on, so it takes one for each name. The owner's passkeys live in 1Password, which syncs them, so one passkey per name serves every device; a device without a syncing password manager makes its own. The first one comes from a setup code made on the droplet:

```bash
systemd-run --uid='<service user>' --pipe --wait -p EnvironmentFile='<env file>' '<opt dir>/homelibrary' setup-code "James"
```

It prints a code good for 30 minutes and the two `/setup` addresses. Open either on the phone, type the code, and the phone makes its passkey. After that, **Settings → Set up another device or address** makes a 15-minute code from inside the app for the next device or the other name. The command also works when every passkey is lost.

## Releasing

1.0.0 was the first release (2026-11-05). A build can reach the droplet to be tried before it is committed with `task deploy -- -Preview`: the working tree as it is, built, swapped, health-checked, rolled back and pulled the same way, stamped `preview-<date-time>` (the version `/healthz` reports). A release:

1. `task check` passes. Raise the version in `web/package.json` (and its lockfile) and move the Unreleased entries in `CHANGELOG.md` under it, with `task shared:changelog:links`, in one commit.
2. Push to `main` (`task push`). Once every CI job passes, the `release` job tags the commit `v<version>` and makes the GitHub Release from that version's CHANGELOG section; a version already released is skipped, so an ordinary push makes no tag. Nothing is tagged by hand.
3. `task deploy`. The script fetches the tags and builds the newest release in a folder of its own, a checkout of its tag with `npm ci`, so neither commits after it nor uncommitted work reach the site; a release the host already runs is left alone (`-- -Force` deploys it again). It runs on shared-tooling's SharedDeploy module over one held SSH connection (`Get-Help .\scripts\deploy.ps1 -Full` gives the whole run), and uploads the binary in one archive with the units and the vhost, filled from the deploy config. On the droplet it checks the archive, and tries the vhost on a copy of Apache's config with any other enabled site serving its names retired, before anything stops. Then it stops both services, snapshots the database (compressed, to `<opt dir>/backups/homelibrary-pre-<time>.sqlite.gz`, the newest three kept), installs the binary and whichever units and vhost changed (the previous ones kept as `.old`), retires those sites, starts both, and waits up to 30 seconds for `/healthz` on the app's port to answer. If it does not, the previous binary, units, vhost and sites go back, with the database snapshot taken a moment before the swap. Then it fetches the site through Cloudflare: `/healthz` must name the new version, and `/` its entry script, or everything goes back the same way. Then it checks the nightly backup's timer, and last it pulls the live database, the covers, and the documents' files into `devdata/`, and removes that copy from the droplet. `-- -NoDbPull` skips the pull.
4. The [production-only checks](#production-only-checks).

## Production-only checks

Right after each deploy, in a fresh browser tab with the console open, on the phone and on a computer:

- Both names load, and each signs in with its own passkey (passkeys exist only for the real names).
- The console shows no error and no Content-Security-Policy refusal, on the library, a book, Scan, Shelves and Settings.
- The service worker took the new release: a tab left open from before shows it after one reload, and the installed app opens without a connection.
- A change made on one device shows on the other within seconds, through Cloudflare (the change feed).
- On the phone, Scan reads a barcode with the camera, and a cover photographed there uploads over the phone's own connection.
- A share link opens in a private window and shows only what it shares; the calendar link still answers in the phone's calendar.

## Upgrading to 1.4.0

Every table with a date or a time is rebuilt at start (migrations 38 to 40): each takes its declared type and a CHECK on its one form, and a date or time not set becomes `NULL`. The sessions move to the app's own store with their expiry converted, so nobody is signed out, and goose's own record of the migrations takes the same form. Borrowers, loans, share links, areas, bookcases and shelves gain when each last changed, so a form's save can be refused when another device changed the record first. Every row is kept, and each migration stops before it commits if a count or a reference comes out wrong. The binary also becomes `<service user>`'s, and the backups folder closes to all but it. In order:

1. Raise the version, commit and push, and let CI tag `v1.4.0`.
2. `task deploy`. The services stop, the snapshot is taken, and the new binary migrates the database as it starts; a migration that fails leaves the database as it was, and the deploy puts 1.3.0 back. The deploy's layout step gives `<opt dir>` and the binary to `<service user>` and sets `backups/` to 0700.
3. Run the [production-only checks](#production-only-checks), then check the rebuild on the droplet: `runuser -u '<service user>' -- sqlite3 '<opt dir>/data/homelibrary.sqlite' 'PRAGMA user_version; PRAGMA integrity_check; PRAGMA foreign_key_check;'` prints `40`, then `ok`, and nothing else.
4. To go back to 1.3.0 after this, put the pre-deploy snapshot back with the binary ([Rolling back](#rolling-back)): 1.3.0 can't read the rebuilt tables.
5. A week later, measure both services' memory and set their caps ([Memory](#memory)).

## Upgrading to 1.2.1

The first deploy on shared-tooling 2.8.0, which fills the units and the vhost from the deploy config. The droplet already runs `<service>.conf` with both ports, enabled, with a disabled `<service>.conf.old` beside it, and no other site serves the two names. In order:

1. On this machine, add the new settings to `scripts/deploy.config.ps1`, each from what the droplet runs now: `$DeployServiceName` and `$DeployServiceUser` (the service and its user), `$DeployRemoteOptDir` (the binary's folder), `$DeployServerName` and `$DeployServerAlias` (the vhost's two names), `$DeployCertName` (the certificate's folder under `/etc/letsencrypt/live`) and `$DeployAppPort` (the port in `HL_ADDR`). The env file and the site's address keep their defaults. Do it as soon as this change is on this machine: the daily backup pull reads `$DeployRemoteOptDir` too.
2. Raise the version, commit and push. When every CI job passes, the `release` job tags `v1.2.1`; a deploy ships that tag.
3. `task deploy`. The units, filled in, differ from the live ones only in their comments, and go in with the binary. The vhost goes in as `<service>.conf`, the previous one kept as `<service>.conf.old` in place of the old copy there; it is already enabled and alone on its names, so no site is retired and Apache reloads once the rest has passed. The health check asks the app on the configured port, so a port that does not match `HL_ADDR` fails here and everything goes back.
4. Check the site as after every release: both names load and sign in.

## Rolling back

The deploy rolls itself back when the new build doesn't answer or fails the smoke check. When a release passed both but is wrong, put the previous one back by hand, as root on the droplet, the way the deploy's own rollback does:

```bash
systemctl stop '<service>-text' '<service>'
install -o '<service user>' -g '<service user>' -m 755 '<opt dir>/homelibrary.old' '<opt dir>/homelibrary'
# Each unit and the vhost the last deploy replaced has its .old copy: the units beside the binary,
# the vhost beside itself in sites-available. Put back any that changed, then:
systemctl daemon-reload && apachectl configtest && systemctl reload apache2
systemctl start '<service>' '<service>-text' && curl -s "http://127.0.0.1:<AppPort>/healthz"
```

A release whose migration the previous one can't read (1.4.0's, for one) needs the database as it was before the deploy too: between the stop and the start, put the newest `homelibrary-pre-<time>.sqlite.gz` back as in [Restore steps](#restore-steps). Anything written since that deploy is lost with it. `task deploy -- -Force` ships the release again once it is fixed.

## Logs

- The web server: `journalctl -u <service>` (JSON lines; `-f` to follow, `--since today`). Each request is one line naming its route, status and time. Debug adds each request's path; the owner turns it on in **Settings → Server log detail** until the next restart, which goes back to `HL_LOG_LEVEL`.
- The text service: `journalctl -u <service>-text`, a line for each file it reads or could not.
- The nightly backup: `journalctl -u <service>-backup -n 20 --no-pager`.
- Apache: `/var/log/apache2/error.log`, and the requests in `other_vhosts_access.log`, which leaves out share-link and calendar addresses.

## Backups

Every morning at 09:30 UTC the timer runs `homelibrary backup` as `<service user>`: a consistent snapshot through SQLite's `VACUUM INTO`, checked with `PRAGMA integrity_check` and compressed, at `<opt dir>/backups/homelibrary-YYYY-MM-DD.sqlite.gz`, and beside it `files-YYYY-MM-DD/` with the covers (`media/`) and documents' files (`documents/`) as they stood, both pruned after 28 days. The files are hard links: covers and documents are written once under new names and never changed, so a day's files take no room until the library lets one go, and a deleted cover or document stays recoverable for 28 days. The folder is 0700, every file 0600.

```bash
systemctl start '<service>-backup.service' && journalctl -u '<service>-backup' -n 5 --no-pager
```

The covers and the documents' files are the host-only folders: they are in no repo, no deploy makes them, and the database holds only their names. A folder up to 250 MB is kept on the host; above that it would be copied to this machine instead. Measured 2026-11-15: covers 40 MiB and documents' files 49 MiB (860 files), the backups folder 138 MiB.

Those snapshots live on the droplet. Two things take a copy off it:

- **Every release** pulls the live database, covers, and documents into `devdata/` on this machine (a working copy, replaced each time).
- **Every day**, Task Scheduler on the owner's PC runs `scripts/pull-backup.ps1`, which copies the droplet's newest snapshot into a dated folder (`homelibrary.sqlite.gz`, `files.tgz`, and `made.txt`, the size and time the droplet's file had, so a day made again is copied again) and keeps the newest 14. The folder is in OneDrive, so the copy reaches the cloud too. Each run adds a line to `pull-backup.log` there. A newest snapshot more than a day old is logged as stale and the task ends with result 1: the droplet's timer has stopped.

```powershell
# By hand, or to try it:
pwsh -NoProfile -File scripts/pull-backup.ps1 -Destination "$env:OneDrive\Backups\Home Library"

# The daily task (once, as the owner, not elevated):
$action = New-ScheduledTaskAction -Execute 'pwsh.exe' -Argument '-NoProfile -WindowStyle Hidden -File "C:\Web Projects\Home Library\scripts\pull-backup.ps1" -Destination "C:\Users\<you>\OneDrive\Backups\Home Library"'
$trigger = New-ScheduledTaskTrigger -Daily -At 12:00
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
Register-ScheduledTask -TaskName 'Home Library backup copy' -Action $action -Trigger $trigger -Settings $settings
```

`-StartWhenAvailable` runs a missed day as soon as the PC is on. The key in `scripts/deploy.config.ps1` must open without a passphrase, or Pageant must be running.

## Restore steps

The database, from a day's snapshot or a deploy's, as root on the droplet. The live file is kept first, under a name the nightly job never prunes:

```bash
ls -1t '<opt dir>/backups/'homelibrary-*.sqlite.gz | head
systemctl stop '<service>-text' '<service>'
runuser -u '<service user>' -- sh -c "umask 077; gzip -c '<opt dir>/data/homelibrary.sqlite' > '<opt dir>/backups/homelibrary-before-restore-$(date -u +%Y%m%d-%H%M%S).sqlite.gz'"
rm -f '<opt dir>/data/homelibrary.sqlite-wal' '<opt dir>/data/homelibrary.sqlite-shm'
runuser -u '<service user>' -- sh -c "gunzip -c '<opt dir>/backups/homelibrary-YYYY-MM-DD.sqlite.gz' > '<opt dir>/data/homelibrary.sqlite'"
systemctl start '<service>' '<service>-text' && curl -s "http://127.0.0.1:<AppPort>/healthz"
```

A snapshot older than the running release is migrated forward as the service starts. The covers and documents, with the services stopped the same way: `cp -a '<opt dir>/backups/files-YYYY-MM-DD/media/.' '<opt dir>/data/media/'` (and the same for `documents`), then `chown -R '<service user>:<service user>' '<opt dir>/data'`. From a PC copy, unpack `files.tgz` and upload the same two folders.

## Restore drill

Not run on the host, by the owner's decision of 2026-11-15 (AGENTS.md, Departures). When one is wanted, as root on the droplet, into a scratch folder, never over the live files:

```bash
SCRATCH=$(mktemp -d) && chown '<service user>' "$SCRATCH"
NEWEST=$(ls -1t '<opt dir>/backups/'homelibrary-????-??-??.sqlite.gz | head -1)
gzip -t "$NEWEST" && runuser -u '<service user>' -- sh -c "gunzip -c '$NEWEST' > '$SCRATCH/drill.sqlite'"
sqlite3 "$SCRATCH/drill.sqlite" 'PRAGMA integrity_check; PRAGMA user_version;'   # ok, and the live schema's version
for t in editions copies loans documents document_files; do
  printf '%s: drill %s, live %s\n' "$t" "$(sqlite3 "$SCRATCH/drill.sqlite" "SELECT count(*) FROM $t")" \
    "$(runuser -u '<service user>' -- sqlite3 '<opt dir>/data/homelibrary.sqlite' "SELECT count(*) FROM $t")"
done
DAY=$(basename "$NEWEST" .sqlite.gz | sed 's/homelibrary-//')
printf 'files: drill %s, live %s\n' "$(find "<opt dir>/backups/files-$DAY" -type f | wc -l)" \
  "$(find '<opt dir>/data/media' '<opt dir>/data/documents' -type f | wc -l)"
rm -rf "$SCRATCH"
```

The counts may differ by what the library gained since 09:30; the drill passes when integrity is `ok`, the version matches, and every difference is that day's work. It last ran: never on the host.

## Importing from another app

On the site: **Import → Choose the export file**, for an export from Bookshelf, Goodreads, or LibraryThing (the columns tell which) or EndNote's XML (in EndNote: select every reference, then File → Export, as XML; it exports only what is selected). Choose first where books not marked as owned go; EndNote marks none. Or on the droplet, with the file uploaded to `/tmp`, and `leave`, `library`, or `wishlist` after it for those books:

```bash
systemd-run --uid='<service user>' --pipe --wait -p EnvironmentFile='<env file>' '<opt dir>/homelibrary' import /tmp/Bookshelf.csv
systemd-run --uid='<service user>' --pipe --wait -p EnvironmentFile='<env file>' '<opt dir>/homelibrary' import "/tmp/HPS Library.xml" wishlist
```

Either way the running service then looks every new book up in the background, a second or two a book.

## Due dates in the phone's calendar

On the site: **Loans → Make a calendar link**, then copy the address. On an iPhone, Settings → Calendar (under Apps from iOS 18) → Calendar Accounts → Add Account → Other → Add Subscribed Calendar; on Android, Google Calendar on the web → Other calendars → From URL (the phone's app has no way to add one). Each loan out with a due date shows as an all-day event with an alert at 9 that morning. Calendars refresh a subscription on their own schedule, from every few minutes to about a day.

**Make a new link** replaces the address (the old one stops at once); **Turn it off** ends it. The address is a password to the borrowers' names and due dates: share it with nobody.

## Developer machine

- `task setup` once, then `task dev` runs Vite on 5173 and the server on 8130. `.env` (from `.env.example`) holds the local settings. The server never makes a database: the first deploy's pull brings `devdata/homelibrary.sqlite`, or an empty file there (`New-Item devdata/homelibrary.sqlite`) starts a new library, which the server migrates.
- The Font Awesome kit (`@awesome.me/kit-4f2bd9b614`, shared with the Gallery and Fantasy Dossier) needs the account's token in the user `~/.npmrc` (`//npm.fontawesome.com/:_authToken=<token>`). CI reads it from the `FONTAWESOME_NPM_TOKEN` repository secret, and Dependabot from a Dependabot secret of the same name.
- `homelibrary dev-session` prints a signed-in cookie for a browser that cannot make a passkey; it refuses unless every origin is localhost.
- `task check` runs what CI runs: at once, `task lint`, the docs checks, the specs' types, the tests with their coverage floors and the OSV scan; then the end-to-end tests and the generated-code check. The vulnerability checks need the network: govulncheck reads the Go vulnerability database, `npm audit` asks the registry, and OSV-Scanner its database.
- `task gen` writes `internal/db/schema.sql` from the migrations, then sqlc's code and the API client; run it after a migration or an API change.
- `task test` ends with the end-to-end tests (`task test:e2e` runs them alone, axe on every main page included). They build the frontend and the binary, start it on 8140 against a database in a temporary folder, and drive Microsoft Edge, so nothing is downloaded; port 8140 must be free.
