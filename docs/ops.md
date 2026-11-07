# Operations

How the droplet is set up for Home Library, how a release goes out, where things live, and how the data is backed up and restored.

## Host layout

| What | Where |
| --- | --- |
| Binary | `/opt/homelibrary/homelibrary` (previous build at `homelibrary.old`), root's, in a directory that is root's: the service runs it and cannot change it |
| Database | `/opt/homelibrary/data/homelibrary.sqlite` |
| Covers | `/opt/homelibrary/data/media/` |
| Documents' files | `/opt/homelibrary/data/documents/` (served only through the API) |
| Backups and deploy snapshots | `/opt/homelibrary/backups/` (0750, every file 0600) |
| Secrets and settings | `/etc/homelibrary/homelibrary.env` (root-owned, 0600) |
| Service unit | `/etc/systemd/system/homelibrary.service` (copy of `deploy/homelibrary.service`) |
| Text service | `/etc/systemd/system/homelibrary-text.service` (copy of `deploy/homelibrary-text.service`): reads the words in documents' files |
| Backup timer | `/etc/systemd/system/homelibrary-backup.{service,timer}` |
| Apache | `/etc/apache2/sites-available/homelibrary.conf` (copy of `deploy/apache-vhost.conf`) |

The units and the vhost are the repository's: every deploy installs them where they differ, so a change to one is made in `deploy/` and shipped, never on the droplet by hand.
| Logs | journald: `journalctl -u homelibrary` |

The service runs as the unprivileged `homelibrary` user on `127.0.0.1:8130`, writing nothing outside `data/`, in at most 768 MB. Apache terminates TLS for **books.jamescwill.com** and **books.mathdad.me** (one VirtualHost, the second name a `ServerAlias`) with the existing `jamescwill.com` certificate, and proxies both to the service with the name preserved. Cloudflare fronts both names, and the vhost takes the visitor's address from `CF-Connecting-IP` with the list Kindred Nights keeps in `/etc/apache2/cloudflare-ips.txt`. The owner's pages hold one request open each, the change feed (`/api/v1/changes`, Server-Sent Events), which the app pings every 25 seconds inside Apache's 120-second proxy timeout and Cloudflare's 100; Apache does not compress `text/event-stream`, so nothing holds the events back. Requests for share links (`/s/`) and the calendar (`/calendar/`) are not logged, their addresses being their passwords, and `/healthz` answers only on the droplet itself. Ports already taken on the droplet: 8080 (Senpan), 8090 (Family Card Game), 8100 (Kindred Nights), 8120 (Conlang Translator).

## One-time host setup

DNS: an A record for each of `books.jamescwill.com` and `books.mathdad.me` pointing at the droplet (done 2026-11-04).

From this machine, copy the units and the vhost up:

```powershell
pscp -i <key.ppk> deploy/homelibrary.service deploy/homelibrary-text.service deploy/apache-vhost.conf root@<host>:/tmp/
```

Then once on the droplet as root. Idempotent.

```bash
id homelibrary >/dev/null 2>&1 || useradd --system --home /opt/homelibrary --shell /usr/sbin/nologin homelibrary
mkdir -p /opt/homelibrary/data/media /opt/homelibrary/data/documents /opt/homelibrary/backups /etc/homelibrary
chown root:root /opt/homelibrary
chown -R homelibrary:homelibrary /opt/homelibrary/data /opt/homelibrary/backups
chmod 750 /opt/homelibrary/data /opt/homelibrary/backups
[ -f /etc/homelibrary/homelibrary.env ] || cat > /etc/homelibrary/homelibrary.env <<'EOF'
HL_ADDR=127.0.0.1:8130
HL_LOG_LEVEL=info
HL_ORIGINS=https://books.jamescwill.com,https://books.mathdad.me
HL_DB_PATH=/opt/homelibrary/data/homelibrary.sqlite
HL_MEDIA_DIR=/opt/homelibrary/data/media
HL_BACKUP_DIR=/opt/homelibrary/backups
HL_BACKUP_KEEP=28
HL_GOOGLE_BOOKS_KEY=
HL_HARDCOVER_TOKEN=
EOF
chmod 600 /etc/homelibrary/homelibrary.env
apt-get install -y sqlite3 poppler-utils tesseract-ocr tesseract-ocr-eng   # the deploy's snapshots; the text service's programs
install -m 644 /tmp/homelibrary.service /tmp/homelibrary-text.service /etc/systemd/system/
systemctl daemon-reload && systemctl enable homelibrary homelibrary-text
install -m 644 /tmp/apache-vhost.conf /etc/apache2/sites-available/homelibrary.conf
a2enmod proxy proxy_http headers rewrite ssl remoteip
a2ensite homelibrary && apachectl configtest && systemctl reload apache2
certbot certificates | grep -A3 'jamescwill.com'   # both books.* names must be listed
```

Put the Google Books key in `HL_GOOGLE_BOOKS_KEY` with an editor. It is restricted in Google Cloud to the Books API and the droplet's address, so it works from the droplet and nowhere else; local development runs without it, or with a second key restricted to the home address.

The first `task deploy` installs the binary, starts both services, and installs the nightly backup's timer (see Backups). Every deploy after it stops both, and starts the text service again on the new binary once the web service answers.

The text service (`homelibrary read-text`, unit `homelibrary-text`) reads the words in documents' files, one at a time, with `pdftotext` and Tesseract. The tools run at idle CPU and disk priority (`chrt`, `ionice`), and the service is capped at 80% of a CPU and 1 GB, with no network at all. The service itself keeps a normal priority: it holds the database's write lock while it saves what the tools read, and at idle priority a busy droplet could keep it there for seconds. `journalctl -u homelibrary-text` shows what it could not read; a file it fails on three times is left for the owner to type.

## Signing in the first time

Passkeys belong to the name they are made on, so it takes one for each name. The owner's passkeys live in 1Password, which syncs them, so one passkey per name serves every device; a device without a syncing password manager makes its own. The first one comes from a setup code made on the droplet:

```bash
systemd-run --uid=homelibrary --pipe --wait -p EnvironmentFile=/etc/homelibrary/homelibrary.env /opt/homelibrary/homelibrary setup-code "James"
```

It prints a code good for 30 minutes and the two `/setup` addresses. Open either on the phone, type the code, and the phone makes its passkey. After that, **Settings → Set up another device or address** makes a 15-minute code from inside the app for the next device or the other name. The command also works when every passkey is lost.

## Releasing

1.0.0 was the first commit (2026-11-05). A build can reach the droplet to be tried before it is committed with `task deploy -- -Preview`: the same build, swap, health check, rollback, and pull, without the clean-and-tagged check, stamped `preview-<date-time>` (the version `/healthz` reports). A release:

1. `task check` passes. Raise the version in `web/package.json` (and its lockfile) and move the Unreleased entries in `CHANGELOG.md` under it, in one commit.
2. Push to `main`. Once every CI job passes, the `release` job tags the commit `v<version>` and makes the GitHub Release from that version's CHANGELOG section; a version already released is skipped, so an ordinary push makes no tag. Nothing is tagged by hand.
3. `task deploy`. The script fetches the tags, refuses a dirty tree or a HEAD without the tag `web/package.json` names, and builds the Linux binary. It runs on shared-tooling's SharedDeploy module over one held SSH connection (`Get-Help .\scripts\deploy.ps1 -Full` gives the whole run), and uploads the binary in one archive with the units and the vhost. On the droplet it checks the archive and that `apachectl configtest` takes the vhost before anything stops, then stops both services, snapshots the database (compressed, to `/opt/homelibrary/backups/homelibrary-pre-<time>.sqlite.gz`, the newest three kept), installs the binary and whichever units and vhost changed, starts both, and waits up to 30 seconds for `/healthz` to answer. If it does not, the previous binary, units and vhost go back, with the database snapshot taken a moment before the swap. Then it fetches the site through Cloudflare: `/healthz` must name the new version, and `/` its entry script, or everything goes back the same way. Then it checks the nightly backup's timer, and last it pulls the live database, the covers, and the documents' files into `devdata/`, and removes that copy from the droplet. `-- -NoDbPull` skips the pull.

## Backups

Every morning at 09:30 UTC the timer runs `homelibrary backup` as the service user: a consistent snapshot through SQLite's `VACUUM INTO`, checked with `PRAGMA integrity_check` and compressed, at `/opt/homelibrary/backups/homelibrary-YYYY-MM-DD.sqlite.gz`, and beside it `files-YYYY-MM-DD/` with the covers (`media/`) and documents' files (`documents/`) as they stood, both pruned after 28 days. The files are hard links: covers and documents are written once under new names and never changed, so a day's files take no room until the library lets one go, and a deleted cover or document stays recoverable for 28 days.

```bash
systemctl start homelibrary-backup.service && journalctl -u homelibrary-backup -n 5 --no-pager
```

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

To restore covers and documents from a snapshot, on the droplet: `cp -a /opt/homelibrary/backups/files-YYYY-MM-DD/media/. /opt/homelibrary/data/media/` (and the same for `documents`), then `chown -R homelibrary:homelibrary /opt/homelibrary/data`. From a PC copy, unpack `files.tgz` and upload the same two directories.

## Restore

```bash
ls -1t /opt/homelibrary/backups/homelibrary-????-??-??.sqlite.gz | head
systemctl stop homelibrary-text homelibrary
gzip -c /opt/homelibrary/data/homelibrary.sqlite > /opt/homelibrary/backups/homelibrary-before-restore-$(date -u +%Y%m%d-%H%M%S).sqlite.gz
rm -f /opt/homelibrary/data/homelibrary.sqlite-wal /opt/homelibrary/data/homelibrary.sqlite-shm
gunzip -c /opt/homelibrary/backups/homelibrary-YYYY-MM-DD.sqlite.gz > /opt/homelibrary/data/homelibrary.sqlite
systemctl start homelibrary homelibrary-text && curl -s http://127.0.0.1:8130/healthz
```

## Importing from another app

On the site: **Import → Choose the export file**, for an export from Bookshelf, Goodreads, or LibraryThing (the columns tell which) or EndNote's XML (in EndNote: select every reference, then File → Export, as XML; it exports only what is selected). Choose first where books not marked as owned go; EndNote marks none. Or on the droplet, with the file uploaded to `/tmp`, and `leave`, `library`, or `wishlist` after it for those books:

```bash
systemd-run --uid=homelibrary --pipe --wait -p EnvironmentFile=/etc/homelibrary/homelibrary.env /opt/homelibrary/homelibrary import /tmp/Bookshelf.csv
systemd-run --uid=homelibrary --pipe --wait -p EnvironmentFile=/etc/homelibrary/homelibrary.env /opt/homelibrary/homelibrary import "/tmp/HPS Library.xml" wishlist
```

Either way the running service then looks every new book up in the background, a second or two a book.

## Due dates in the phone's calendar

On the site: **Loans → Make a calendar link**, then copy the address. On an iPhone, Settings → Calendar (under Apps from iOS 18) → Calendar Accounts → Add Account → Other → Add Subscribed Calendar; on Android, Google Calendar on the web → Other calendars → From URL (the phone's app has no way to add one). Each loan out with a due date shows as an all-day event with an alert at 9 that morning. Calendars refresh a subscription on their own schedule, from every few minutes to about a day.

**Make a new link** replaces the address (the old one stops at once); **Turn it off** ends it. The address is a password to the borrowers' names and due dates: share it with nobody.

## Developer machine

- `task setup` once, then `task dev` runs Vite on 5173 and the server on 8130. `.env` (from `.env.example`) holds the local settings.
- The Font Awesome kit (`@awesome.me/kit-4f2bd9b614`, shared with the Gallery and Fantasy Dossier) needs the account's token in the user `~/.npmrc` (`//npm.fontawesome.com/:_authToken=<token>`). CI reads it from the `FONTAWESOME_NPM_TOKEN` repository secret, and Dependabot from a Dependabot secret of the same name.
- `homelibrary dev-session` prints a signed-in cookie for a browser that cannot make a passkey; it refuses unless every origin is localhost.
- `task check` runs what CI runs, in its order: `task lint`, then the tests with their coverage floors, then the end-to-end tests. `task lint` includes the vulnerability scans, which need the network: govulncheck reads the Go vulnerability database, and `npm audit` asks the registry.
- `task test` ends with the end-to-end tests (`task test:e2e` runs them alone, axe on every main page included). They build the frontend and the binary, start it on 8140 against a database in a temporary folder, and drive Microsoft Edge, so nothing is downloaded; port 8140 must be free.
