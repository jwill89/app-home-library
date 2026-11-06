#!/usr/bin/env pwsh
<#
.SYNOPSIS
Deploy the tagged commit to the droplet and bring a copy of the library home.

.DESCRIPTION
Builds the static Linux binary and uploads it with pscp, beside the systemd units and the
Apache vhost from deploy/. On the droplet it stops the service and the text service, snapshots
the database, installs the binary and the units, and waits up to 30 seconds for /healthz. A
build that does not answer goes back: the binary and units before it, and the database as the
snapshot left it. Then the vhost, kept only if `apachectl configtest` takes it.

Last, a consistent online snapshot of the live database and an archive of the covers and
documents' files come down to devdata/ (skip with -NoPull), and leave the droplet. Both
include the owner's own photographs and scans, which exist nowhere else, so every release
leaves a copy of them on this machine.

Settings resolve from the parameter, then $env:DEPLOY_*, then the git-ignored
scripts/deploy.config.ps1. A run opens at most four SSH connections, which stays under
the host's `ufw limit ssh`.

Requires a clean tree at a tagged commit, PuTTY's pscp and plink on PATH, and the key
loaded in Pageant. -Preview ships the working tree as it is, stamped preview-<time>: the
owner commits nothing until 1.0.0, and a build still has to reach a phone to be tried.
#>
[CmdletBinding()]
param(
    [string]$VpsHost = $env:DEPLOY_VPS_HOST,
    [string]$VpsUser = $env:DEPLOY_VPS_USER,
    [string]$KeyPath = $env:DEPLOY_KEY_PATH,
    [switch]$SkipBuild,
    [switch]$NoPull,
    [switch]$Preview
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$remoteDir = '/opt/homelibrary'
$envFile = '/etc/homelibrary/homelibrary.env'
$localDb = Join-Path $root 'devdata/homelibrary.sqlite'
$localMedia = Join-Path $root 'devdata/media'
$localDocs = Join-Path $root 'devdata/documents'
$binary = Join-Path $root 'dist/homelibrary-linux-amd64'
$release = Join-Path $root 'dist/release'

$configFile = Join-Path $PSScriptRoot 'deploy.config.ps1'
if (Test-Path $configFile) { . $configFile }
if (-not $VpsHost) { $VpsHost = $DeployVpsHost }
if (-not $VpsUser) { $VpsUser = $DeployVpsUser }
if (-not $KeyPath) { $KeyPath = $DeployKeyPath }
foreach ($name in 'VpsHost', 'VpsUser', 'KeyPath') {
    if (-not (Get-Variable $name -ValueOnly)) {
        throw "Missing $name. Pass -$name, set DEPLOY_*, or fill scripts/deploy.config.ps1."
    }
}
$target = "$VpsUser@$VpsHost"

function Invoke-Remote([string]$Script) {
    # Merge stderr so a failing remote step shows its reason before the throw.
    $out = & plink -batch -i $KeyPath $target ($Script -replace "`r", '') 2>&1 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) {
        $out | Write-Host
        throw "Remote command failed ($LASTEXITCODE)"
    }
    $out
}

if ($Preview) {
    $version = 'preview-' + (Get-Date -Format 'yyyyMMdd-HHmm')
} else {
    if (git status --porcelain) { throw 'Working tree is not clean. Commit or stash first.' }
    $version = git describe --tags --exact-match 2>$null
    if (-not $version) { throw 'HEAD is not tagged. Deploys ship tagged commits only, or pass -Preview.' }
}

if (-not $SkipBuild) {
    if ($Preview) { task build:linux VERSION=$version } else { task build:linux }
    if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
}
if (-not (Test-Path $binary)) { throw "Binary not found: $binary" }

Write-Host "Deploying $version to $target"

# The binary, the units, and the vhost go up together, beside the live ones; nothing stops
# until the upload is complete.
Remove-Item -Recurse -Force $release -ErrorAction SilentlyContinue
New-Item -ItemType Directory $release | Out-Null
Copy-Item $binary (Join-Path $release 'homelibrary')
Copy-Item (Join-Path $root 'deploy/*') $release
& pscp -batch -C -r -i $KeyPath $release "${target}:$remoteDir/"
if ($LASTEXITCODE -ne 0) { throw 'Upload failed.' }
Remove-Item -Recurse -Force $release

# One connection: stop, snapshot, install, start, verify, roll back on failure, then the
# vhost and the snapshot to bring home. The settings come first; the script after them is
# bash as it stands, nothing in it expanded here.
$settings = @"
dir=$remoteDir
env_file=$envFile
stamp=$(Get-Date -Format 'yyyyMMdd-HHmmss')
pull=$(if ($NoPull) { '' } else { '1' })
"@
$install = @'
set -eu
# What this writes (snapshots, the archive to bring home) is for root and the service alone.
umask 077
rel=$dir/release
db=$dir/data/homelibrary.sqlite
snapshot=$dir/backups/homelibrary-pre-$stamp.sqlite
units='homelibrary.service homelibrary-text.service homelibrary-backup.service homelibrary-backup.timer'
vhost=/etc/apache2/sites-available/homelibrary.conf

text=
if systemctl is-enabled --quiet homelibrary-text 2>/dev/null; then text=1; fi
restart=
finish() {
  # Whatever fails while the services are down, they come back up on what is in place.
  if [ -n "$restart" ]; then
    systemctl start homelibrary || true
    if [ -n "$text" ]; then systemctl start homelibrary-text || true; fi
  fi
  rm -rf "$rel"
}
trap finish EXIT

# The text service writes the database too: neither the new build's migration nor a
# rollback's restore runs under it.
restart=1
systemctl stop homelibrary-text 2>/dev/null || true
systemctl stop homelibrary

# The binary and its directory are root's, so the service runs the one and cannot change it;
# the data and the backups are the service's, and nobody else's to read.
chown root:root "$dir"
chmod 755 "$dir"
chmod 750 "$dir/data" "$dir/backups"
find "$dir/backups" -maxdepth 1 -type f ! -perm 600 -exec chmod 600 {} +

if [ -f "$dir/homelibrary" ]; then install -m 755 -o root -g root "$dir/homelibrary" "$dir/homelibrary.old"; fi
if [ -f "$db" ]; then
  sqlite3 "$db" ".backup '$snapshot'"
  # The newest three deploy snapshots stay; their names sort as their times do.
  find "$dir/backups" -maxdepth 1 -name 'homelibrary-pre-*.sqlite' -print0 | sort -zr | tail -zn +4 | xargs -0r rm --
fi

# The units as they were, for a rollback, then the new ones where they differ.
mkdir "$rel/before"
units_changed=
for u in $units; do
  if [ -f "/etc/systemd/system/$u" ]; then cp -p "/etc/systemd/system/$u" "$rel/before/$u"; fi
  if ! cmp -s "$rel/$u" "/etc/systemd/system/$u"; then
    install -m 644 -o root -g root "$rel/$u" "/etc/systemd/system/$u"
    units_changed=1
  fi
done
if [ -n "$units_changed" ]; then systemctl daemon-reload; fi
install -m 755 -o root -g root "$rel/homelibrary" "$dir/homelibrary"

# A start that fails outright is left for the check, which rolls back. The check waits for an
# answer, not only a running process: a build that migrates, then fails to listen, goes back.
systemctl start homelibrary || true
addr=$(sed -n 's/^HL_ADDR=//p' "$env_file")
addr=${addr:-127.0.0.1:8130}
healthy=
for _ in $(seq 30); do
  if curl -fs -m 2 -o /dev/null "http://$addr/healthz"; then healthy=1; break; fi
  sleep 1
done
if [ -z "$healthy" ]; then
  echo 'The new build did not answer /healthz within 30 seconds; rolling back.' >&2
  journalctl -u homelibrary -n 30 --no-pager >&2 || true
  systemctl stop homelibrary || true
  if [ -f "$dir/homelibrary.old" ]; then install -m 755 -o root -g root "$dir/homelibrary.old" "$dir/homelibrary"; fi
  for u in $units; do
    if [ -f "$rel/before/$u" ]; then install -m 644 -o root -g root "$rel/before/$u" "/etc/systemd/system/$u"; fi
  done
  systemctl daemon-reload
  # The new build may have migrated the database before it fell over, and the old one cannot
  # read a schema ahead of it, so the snapshot taken a moment ago goes back too.
  if [ -f "$snapshot" ]; then
    rm -f "$db-wal" "$db-shm"
    cat "$snapshot" > "$db"
    echo 'The database is back as the snapshot before the deploy left it.' >&2
  fi
  exit 1
fi
restart=
echo "homelibrary is active: $(curl -fsS -m 5 "http://$addr/healthz")"
if [ -n "$text" ]; then
  systemctl start homelibrary-text || echo 'The text service did not start: journalctl -u homelibrary-text' >&2
fi
if [ -n "$units_changed" ]; then systemctl try-restart homelibrary-backup.timer; fi

# The vhost, where it differs, kept only if Apache takes it; the one before stays otherwise.
if ! cmp -s "$rel/apache-vhost.conf" "$vhost"; then
  cp -p "$vhost" "$rel/vhost.before"
  install -m 644 -o root -g root "$rel/apache-vhost.conf" "$vhost"
  if verdict=$(apachectl configtest 2>&1); then
    systemctl reload apache2
    echo 'Apache took the new vhost.'
  else
    cat "$rel/vhost.before" > "$vhost"
    echo "vhost-refused: $verdict"
  fi
fi

# A consistent online snapshot and the covers and documents' files, for the trip home. The
# service runs on: the backup API copies a live database whole, and the files never change.
if [ -n "$pull" ] && [ -f "$db" ]; then
  out=$dir/backups/pull-deploy
  rm -rf "$out"
  mkdir "$out"
  sqlite3 "$db" ".backup '$out/homelibrary.sqlite'"
  install -d -o homelibrary -g homelibrary -m 750 "$dir/data/documents"
  tar -czf "$out/files.tgz" -C "$dir/data" media documents
  echo pull-ready
fi
'@
$installed = Invoke-Remote ($settings + "`n" + $install)
$installed | Where-Object { $_ -notmatch '^(pull-ready|vhost-refused: )' } | Write-Host
$refused = $installed | Where-Object { $_ -match '^vhost-refused: ' }
if ($refused) {
    Write-Warning "Apache refused the new vhost, so the one before it stays. $($refused -replace '^vhost-refused: ', '')"
}
Write-Host "Deployed $version"
if ($NoPull) { return }
if ($installed -notcontains 'pull-ready') {
    Write-Host 'No database on the host yet; skipping the pull.'
    return
}

# Down in one connection, then off the droplet: a copy of the whole library stays there no
# longer than the trip, whether or not it arrives.
$pulled = Join-Path $root 'devdata/pull'
Remove-Item -Recurse -Force $pulled -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $pulled | Out-Null
try {
    & pscp -batch -C -i $KeyPath "${target}:$remoteDir/backups/pull-deploy/*" $pulled
    if ($LASTEXITCODE -ne 0) { throw 'The copy of the library could not be downloaded.' }
} finally {
    Invoke-Remote "rm -rf $remoteDir/backups/pull-deploy" | Out-Null
}

if (Test-Path $localDb) {
    try {
        Move-Item -Force $localDb "$localDb.bak"
    } catch {
        # A running local dev server holds the file. The deploy is done; the swap can wait.
        Write-Warning "Local database is in use (stop the dev server to refresh it). The live copy and its files are in $pulled."
        return
    }
}
Remove-Item -Force "$localDb-wal", "$localDb-shm" -ErrorAction SilentlyContinue
Move-Item (Join-Path $pulled 'homelibrary.sqlite') $localDb
Write-Host "Local database refreshed: $localDb"

Remove-Item -Recurse -Force $localMedia, $localDocs -ErrorAction SilentlyContinue
# Windows' own tar by its path: Git's GNU tar, first on a Git Bash PATH, reads "C:" as a host.
& (Join-Path $env:SystemRoot 'System32/tar.exe') -xzf (Join-Path $pulled 'files.tgz') -C (Join-Path $root 'devdata')
if ($LASTEXITCODE -ne 0) { throw "The files' archive could not be unpacked; it is in $pulled." }
Remove-Item -Recurse -Force $pulled
Write-Host "Covers and documents copied: $localMedia, $localDocs"
