#!/usr/bin/env pwsh
<#
.SYNOPSIS
Bring the droplet's newest nightly backup to this machine.

.DESCRIPTION
Every morning the droplet snapshots the library beside itself: the database, and the covers
and documents' files as they stood. This copies the newest snapshot home, into a folder named
for its day under -Destination (homelibrary.sqlite and files.tgz), and keeps the newest -Keep
of those folders, so the library has a copy off the droplet. A destination inside OneDrive
carries the copy on to the cloud. Task Scheduler runs it daily (see docs/ops.md). A day
already copied is left as it is, so running it again costs nothing; a day the droplet made
again (its size or time changed) is copied again.

Each run adds a line to pull-backup.log in -Destination, so a failed night shows. A newest
backup more than a day old is logged as stale and the run ends with exit code 1, so Task
Scheduler shows it too: the droplet's nightly backup has stopped.

Settings resolve like deploy.ps1's: the parameter, then $env:DEPLOY_*, then the git-ignored
scripts/deploy.config.ps1. Requires PuTTY's pscp and plink on PATH and a key that needs no
typing (no passphrase, or Pageant running). Opens at most three SSH connections.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Destination,
    [ValidateRange(1, 365)][int]$Keep = 14,
    [string]$VpsHost = $env:DEPLOY_VPS_HOST,
    [string]$VpsUser = $env:DEPLOY_VPS_USER,
    [string]$KeyPath = $env:DEPLOY_KEY_PATH
)

$ErrorActionPreference = 'Stop'
$remote = '/opt/homelibrary/backups'
$outbox = "$remote/pull-daily"

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
function Invoke-Remote([string]$Script) {
    $out = & plink -batch -i $KeyPath $target ($Script -replace "`r", '') 2>&1 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { throw "The droplet could not be asked: $($out -join ' ')" }
    $out
}

# What the newest folder here holds: its day, and the size and time the droplet's file had.
$days = @(Get-ChildItem -Directory $Destination | Where-Object Name -Match '^\d{4}-\d{2}-\d{2}$' | Sort-Object Name -Descending)
$have = ''
if ($days.Count -gt 0) {
    $mark = Join-Path $days[0].FullName 'made.txt'
    if (Test-Path $mark) { $have = "$($days[0].Name):$((Get-Content $mark -Raw).Trim())" }
}

$stale = $false
try {
    # The newest day with both its database and its files. Unless this machine has that very
    # file already, the two go in one folder for one download: the database by a hard link,
    # the files archived.
    $settings = @"
remote=$remote
outbox=$outbox
have='$have'
"@
    $prepare = @'
set -eu
umask 077
day=$(ls -1 "$remote" | sed -n 's/^files-\([0-9-]*\)$/\1/p' | sort | tail -1)
if [ -z "$day" ] || [ ! -f "$remote/homelibrary-$day.sqlite" ]; then echo no-backup; exit 0; fi
made=$(stat -c '%s-%Y' "$remote/homelibrary-$day.sqlite")
echo "day=$day"
echo "made=$made"
if [ "$day:$made" = "$have" ]; then echo same; exit 0; fi
rm -rf "$outbox"
mkdir "$outbox"
ln "$remote/homelibrary-$day.sqlite" "$outbox/homelibrary.sqlite"
tar -czf "$outbox/files.tgz" -C "$remote" "files-$day"
echo ready
'@
    $out = Invoke-Remote ($settings + "`n" + $prepare)
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
        # Into a working folder first, so a day's folder is only ever whole; and off the droplet
        # after, whether or not it arrived.
        $partial = "$folder.partial"
        Remove-Item -Recurse -Force $partial -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Force $partial | Out-Null
        try {
            & pscp -batch -q -C -i $KeyPath "${target}:$outbox/*" $partial
            if ($LASTEXITCODE -ne 0) { throw "The backup of $day could not be downloaded." }
        } finally {
            Invoke-Remote "rm -rf $outbox" | Out-Null
        }
        foreach ($name in 'homelibrary.sqlite', 'files.tgz') {
            if (-not (Test-Path (Join-Path $partial $name))) { throw "The backup of $day came without $name." }
        }
        Set-Content -Path (Join-Path $partial 'made.txt') -Value $made
        $again = Test-Path $folder
        Remove-Item -Recurse -Force $folder -ErrorAction SilentlyContinue
        Move-Item $partial $folder
        $size = (Get-ChildItem $folder | Measure-Object Length -Sum).Sum / 1MB
        $verb = if ($again) { 'Copied again, the droplet having made it anew,' } else { 'Copied' }
        Add-LogLine ("$verb $day ({0:N1} MB) to $folder." -f $size)
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
