#!/usr/bin/env pwsh
#Requires -Version 7.4

<#
.SYNOPSIS
Bring the droplet's newest nightly backup to this machine.

.DESCRIPTION
Every morning the droplet snapshots the library beside itself: the database, compressed, and
the covers and documents' files as they stood. This copies the newest snapshot home, into a
folder named for its day under -Destination (homelibrary.sqlite.gz and files.tgz), and keeps
the newest -Keep of those folders, so the library has a copy off the droplet. A destination
inside OneDrive carries the copy on to the cloud. Task Scheduler runs it daily. A day
already copied is left as it is, so running it again costs nothing; a day the droplet made
again (its size or time changed) is copied again.

Each run adds a line to pull-backup.log in -Destination, so a failed night shows. A newest
backup more than a day old is logged as stale and the run ends with exit code 1, so Task
Scheduler shows it too: the droplet's nightly backup has stopped.

Built on the SharedDeploy module (shared/deploy/pwsh): one held SSH connection,
and each download checked against the droplet's SHA-256. Settings resolve like deploy.ps1's:
the parameter, then $env:DEPLOY_*, then the git-ignored scripts/deploy.config.ps1: VpsHost,
VpsUser, KeyPath, and RemoteOptDir, the app's folder, whose backups/ it reads. Requires
PuTTY's plink 0.85 or later and a key that needs no typing (no passphrase, or Pageant
running).
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Destination,
    [ValidateRange(1, 365)][int]$Keep = 14,
    [string]$VpsHost,
    [string]$VpsUser,
    [string]$KeyPath,
    [string]$RemoteOptDir
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
Import-Module (Join-Path $PSScriptRoot '..\shared\deploy\pwsh\SharedDeploy.psd1') -Force

$settings = Resolve-DeploySetting -ConfigPath (Join-Path $PSScriptRoot 'deploy.config.ps1') `
    -Value @{ VpsHost = $VpsHost; VpsUser = $VpsUser; KeyPath = $KeyPath; RemoteOptDir = $RemoteOptDir } `
    -Required VpsHost, VpsUser, KeyPath, RemoteOptDir
# The backups sit in the app's folder on the host, which goes into a shell command.
$remoteDir = "$($settings.RemoteOptDir)".TrimEnd('/')
if ($remoteDir -notmatch '^/[A-Za-z0-9._/-]+$' -or $remoteDir -match '(^|/)\.\.?(/|$)') { throw "RemoteOptDir must be an absolute path of plain names, not '$remoteDir'." }
$remote = "$remoteDir/backups"
$outbox = "$remote/pull-daily"
$files = 'homelibrary.sqlite.gz', 'files.tgz'

New-Item -ItemType Directory -Force $Destination | Out-Null
$log = Join-Path $Destination 'pull-backup.log'
function Add-LogLine([string]$Line) {
    $stamped = "$(Get-Date -Format 'yyyy-MM-dd HH:mm') $Line"
    Add-Content -Path $log -Value $stamped
    Write-Host $stamped
}
function Get-Answer([string[]]$Lines, [string]$Pattern) {
    $found = $Lines | Select-String $Pattern | Select-Object -First 1
    if ($found) { $found.Matches[0].Groups[1].Value } else { '' }
}

# What the newest folder here holds: its day, and the size and time the droplet's file had.
$days = @(Get-ChildItem -Directory $Destination | Where-Object Name -Match '^\d{4}-\d{2}-\d{2}$' | Sort-Object Name -Descending)
$have = ''
if ($days.Count -gt 0) {
    $mark = Join-Path $days[0].FullName 'made.txt'
    if (Test-Path $mark) { $have = "$($days[0].Name):$((Get-Content $mark -Raw).Trim())" }
}

# The newest day with both its database and its files. Unless this machine has that very file
# already, the two go in one folder for the trip: the database by a hard link (one made before
# the snapshots were compressed is compressed on the way), the files archived.
$prepare = Format-RemoteScript -Value @{ REMOTE = $remote; OUTBOX = $outbox; HAVE = $have } -Template @'
set -eu
umask 077
day=$(ls -1 '__REMOTE__' | sed -n 's/^files-\([0-9-]*\)$/\1/p' | sort | tail -1)
db=
if [ -n "$day" ]; then
  for f in "__REMOTE__/homelibrary-$day.sqlite.gz" "__REMOTE__/homelibrary-$day.sqlite"; do
    if [ -f "$f" ]; then db=$f; break; fi
  done
fi
if [ -z "$db" ]; then echo no-backup; exit 0; fi
made=$(stat -c '%s-%Y' "$db")
echo "day=$day"
echo "made=$made"
if [ "$day:$made" = '__HAVE__' ]; then echo same; exit 0; fi
rm -rf '__OUTBOX__'
mkdir '__OUTBOX__'
case "$db" in
  *.gz) ln "$db" '__OUTBOX__/homelibrary.sqlite.gz' ;;
  *) gzip -c "$db" > '__OUTBOX__/homelibrary.sqlite.gz' ;;
esac
tar -czf '__OUTBOX__/files.tgz' -C '__REMOTE__' "files-$day"
echo ready
'@

$stale = $false
try {
    $null = $settings | Open-DeployConnection
    try {
        $out = @(Invoke-Remote -Script $prepare)
        $day = Get-Answer $out '^day=(\d{4}-\d{2}-\d{2})$'
        $made = Get-Answer $out '^made=(\d+-\d+)$'
        if (-not $day) {
            Add-LogLine 'No nightly backup on the droplet yet; nothing copied.'
            return
        }

        $folder = Join-Path $Destination $day
        if ($out -contains 'same') {
            Add-LogLine "$day was copied already."
        } else {
            # Into a working folder first, so a day's folder is only ever whole; and off the
            # droplet after, whether or not it arrived.
            $partial = "$folder.partial"
            Remove-Item -Recurse -Force $partial -ErrorAction SilentlyContinue
            New-Item -ItemType Directory -Force $partial | Out-Null
            try {
                foreach ($name in $files) {
                    if (-not (Get-File -Source "$outbox/$name" -Destination (Join-Path $partial $name))) {
                        throw "The backup of $day could not be downloaded ($name)."
                    }
                }
            } finally {
                $null = Invoke-Remote -Script "rm -rf '$outbox'"
            }
            Set-Content -Path (Join-Path $partial 'made.txt') -Value $made
            $again = Test-Path $folder
            Remove-Item -Recurse -Force $folder -ErrorAction SilentlyContinue
            Move-Item $partial $folder
            $size = (Get-ChildItem $folder | Measure-Object Length -Sum).Sum / 1MB
            $verb = if ($again) { 'Copied again, the droplet having made it anew,' } else { 'Copied' }
            Add-LogLine ("$verb $day ({0:N1} MB) to $folder." -f $size)
        }
    } finally {
        Close-DeployConnection
    }

    # The droplet backs up every morning, so a newest day before yesterday means it stopped.
    $age = ((Get-Date).ToUniversalTime().Date - [datetime]::ParseExact($day, 'yyyy-MM-dd', $null)).Days
    if ($age -gt 1) {
        $stale = $true
        Add-LogLine "Stale: the droplet's newest backup is from $day, $age days ago. Check its timer: systemctl list-timers homelibrary-backup.timer"
    }

    # The newest days stay; dated folders past them go.
    $days = Get-ChildItem -Directory $Destination | Where-Object Name -Match '^\d{4}-\d{2}-\d{2}$' | Sort-Object Name -Descending
    foreach ($old in $days | Select-Object -Skip $Keep) {
        Remove-Item -Recurse -Force $old.FullName
        Add-LogLine "Removed $($old.Name), past the newest $Keep."
    }
} catch {
    Add-LogLine "Failed: $_"
    throw
}
if ($stale) { exit 1 }
