#!/usr/bin/env pwsh
#Requires -Version 7.4

<#
.SYNOPSIS
Deploy the tagged commit to the droplet and bring a copy of the library home.

.DESCRIPTION
Built on shared-tooling's SharedDeploy module (shared/deploy/pwsh), the shape every
deploy follows: one archive, one held SSH connection, one remote run.

Builds the static Linux binary and packs it into one archive with the systemd units and the
Apache vhost from deploy/, opens one SSH connection and keeps it for the run, and uploads the
archive. In one remote run it checks the archive and that Apache takes the vhost before
anything stops, then stops the service and the text service, snapshots the database
(compressed), installs the binary and whichever units and vhost differ, starts both, and waits
up to 30 seconds for /healthz. A build that does not answer goes back: the binary, units and
vhost before it, and the database as the snapshot left it, since the new build may have
migrated it before it fell over.

Then the site is fetched through Cloudflare: /healthz must name this version, and / this
build's entry script, which must load. A failure there rolls back the same way. Then the
nightly backup's timer is checked (shared-tooling's Install-BackupTimer: where the droplet's
units differ from the ones it writes, the new ones go in and `homelibrary backup` runs once,
the old ones coming back if it fails).

Last, an online snapshot of the live database comes down to devdata/homelibrary.sqlite, and an
archive of the covers and documents' files to devdata/media and devdata/documents (skip both
with -NoDbPull); neither stays on the droplet. Both include the owner's own photographs and
scans, which exist nowhere else, so every release leaves a copy of them on this machine.

Settings resolve from the parameter, then $env:DEPLOY_*, then the git-ignored
scripts/deploy.config.ps1. A run is one SSH connection (plus one only if it drops and
reconnects), which stays under the host's `ufw limit ssh`.

Requires a clean tree at the commit CI tagged with web/package.json's version (the script
fetches the tags first), PuTTY's plink 0.85 or later, and the key loaded in Pageant.
-Preview ships the working tree as it is, stamped preview-<time>, so a build reaches a phone
to be tried before it is committed.
#>
[CmdletBinding()]
param(
    [string]$VpsHost,
    [string]$VpsUser,
    [string]$KeyPath,
    [switch]$SkipBuild,
    [switch]$NoDbPull,
    [switch]$Preview
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
Import-Module (Join-Path $PSScriptRoot '..\shared\deploy\pwsh\SharedDeploy.psd1') -Force
trap { Close-DeployConnection; Write-Host "`nERROR: $($_.Exception.Message)" -ForegroundColor Red; exit 1 }

$root = Split-Path -Path $PSScriptRoot -Parent
Set-Location $root

$remoteDir = '/opt/homelibrary'
$envFile = '/etc/homelibrary/homelibrary.env'
$serviceUser = 'homelibrary'
# The text service writes the database too: neither the new build's migration nor a
# rollback's restore runs under it, so the two stop, start and go back together.
$services = 'homelibrary', 'homelibrary-text'
$units = 'homelibrary.service', 'homelibrary-text.service'
$vhostPath = '/etc/apache2/sites-available/homelibrary.conf'
$remoteDb = "$remoteDir/data/homelibrary.sqlite"
$snapshotPrefix = "$remoteDir/backups/homelibrary-pre-"
$pullFolder = "$remoteDir/backups/pull-deploy"
$localDb = Join-Path $root 'devdata/homelibrary.sqlite'
$binary = Join-Path $root 'dist/homelibrary-linux-amd64'

$settings = Resolve-DeploySetting -ConfigPath (Join-Path $PSScriptRoot 'deploy.config.ps1') `
    -Value @{ VpsHost = $VpsHost; VpsUser = $VpsUser; KeyPath = $KeyPath } `
    -Required VpsHost, VpsUser, KeyPath

if ($Preview) {
    $version = 'preview-' + (Get-Date -Format 'yyyyMMdd-HHmm')
} else {
    Assert-CleanWorkingTree
    # CI tags a release once main is green (the release job), so its tag is fetched before
    # HEAD is checked; the tag must be the version web/package.json names.
    git -C $root fetch --tags --quiet
    if ($LASTEXITCODE -ne 0) { Write-Warning 'Could not fetch tags; checking against the local ones.' }
    $version = Get-ReleaseTag -Path $root
    $expected = 'v' + (Get-Content -LiteralPath (Join-Path $root 'web\package.json') -Raw | ConvertFrom-Json).version
    if ($version -ne $expected) {
        throw "HEAD is tagged $version, but web/package.json says $expected."
    }
}

if ($SkipBuild) {
    Write-Step 'Skipping the build (-SkipBuild); using dist/homelibrary-linux-amd64'
} else {
    if ($Preview) { task build:linux VERSION=$version } else { task build:linux }
    if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
}
if (-not (Test-Path $binary)) { throw "Binary not found: $binary" }

# The built page names its entry script, which the smoke check looks for and loads.
$entryScript = [regex]::Match((Get-Content -LiteralPath (Join-Path $root 'web/dist/index.html') -Raw), 'src="(/assets/[^"]+\.js)"').Groups[1].Value
if (-not $entryScript) { throw 'web/dist/index.html names no entry script under /assets; build the front end first.' }

$archive = Compress-DeployArchive -Content ([ordered]@{
        'homelibrary'              = $binary
        'homelibrary.service'      = Join-Path $root 'deploy/homelibrary.service'
        'homelibrary-text.service' = Join-Path $root 'deploy/homelibrary-text.service'
        'apache-vhost.conf'        = Join-Path $root 'deploy/apache-vhost.conf'
    })

# The binary and its directory are root's, so the service runs the one and cannot change it;
# the data and the backups are the service's, and nobody else's to read.
$layout = Format-RemoteScript -Value @{ DIR = $remoteDir; USER = $serviceUser } -Template @'
chown root:root '__DIR__'
chmod 755 '__DIR__'
chmod 750 '__DIR__/data' '__DIR__/backups'
find '__DIR__/backups' -maxdepth 1 -type f ! -perm 600 -exec chmod 600 {} +
install -d -o '__USER__' -g '__USER__' -m 750 '__DIR__/data/documents'
'@

$restore = {
    Restore-ServiceBinary -ServiceName $services -Binary "$remoteDir/homelibrary" -Owner 'root' -Unit $units `
        -VhostPath $vhostPath -Database $remoteDb -SnapshotPrefix $snapshotPrefix -SnapshotSuffix '.sqlite'
}

$connection = $settings | Open-DeployConnection
try {
    Write-Step "Deploying $version to $($connection.Target)"
    # Nothing stops until the upload is complete and checked.
    $upload = Invoke-Upload -Path $archive.Path -Destination 'homelibrary-deploy.tgz' -Activity "Uploading Home Library $version" -FileCount $archive.FileCount
    if (-not $upload.Ok) { throw "Upload failed; the live server is untouched.`n$($upload.Err)" }

    Invoke-DeployBundle -Archive 'homelibrary-deploy.tgz' -Sha256 $archive.Sha256 -ExpectedFileCount $archive.FileCount -Step @(
        $layout
        Install-ServiceBinary -AsStep -ServiceName $services -Binary "$remoteDir/homelibrary" -Owner 'root' `
            -Unit $units -Vhost 'apache-vhost.conf' -VhostPath $vhostPath -Umask '077' `
            -Database $remoteDb -SnapshotPrefix $snapshotPrefix -SnapshotSuffix '.sqlite' -RestoreDatabaseOnRollback `
            -EnvFile $envFile -HealthUrl 'http://{HL_ADDR}/healthz' -HealthDefault @{ HL_ADDR = '127.0.0.1:8130' } `
            -HealthTimeoutSeconds 30
    )
    Write-Ok "Deployed $version"

    # The site as visitors reach it, through Cloudflare and Apache: the first of its origins.
    $origins = "$(@(Invoke-Remote -Script "sed -n 's/^HL_ORIGINS=//p' '$envFile'") | Select-Object -First 1)"
    $site = ($origins -split ',')[0].Trim().TrimEnd('/')
    if ($site -notmatch '^https?://') { throw "The host names no site address (HL_ORIGINS in $envFile), so the site could not be checked. $version is running." }
    Write-Step "Checking $site..."
    # A fresh token on each address, so no answer Cloudflare kept from before can pass.
    $token = [guid]::NewGuid().ToString('N').Substring(0, 12)
    Invoke-SmokeCheck -Attempts 3 -DelaySeconds 2 -Rollback $restore -Check @(
        @{ Url = "$site/healthz?deploy=$token"; Contains = "`"version`":`"$version`"" }
        @{ Url = "$site/?deploy=$token"; Contains = $entryScript }
        @{ Url = "$site$entryScript" }
    )

    # The nightly backup: 09:30 UTC, half an hour before Kindred Nights' on the same droplet,
    # `homelibrary backup` as the service user, with the service's hardening. One writable path
    # over both the data and the backups, not one each: each would be its own mount, and the
    # files' snapshot is hard links, which cannot cross mounts.
    Write-Step 'Checking the nightly backup...'
    try {
        Install-BackupTimer -Name 'homelibrary' -Command "$remoteDir/homelibrary backup" -Owner $serviceUser `
            -EnvironmentFile $envFile -WorkingDirectory $remoteDir -ReadWritePath $remoteDir `
            -OnCalendar '*-*-* 09:30:00 UTC' -RandomizedDelay '10min' -Description 'Home Library nightly backup'
    } catch {
        throw "$version is deployed and running, but its nightly backup is not: $($_.Exception.Message)"
    }

    if (-not $NoDbPull) {
        $null = Copy-LiveDatabase -Database $remoteDb -Destination $localDb -Owner $serviceUser

        # The covers and documents' files, in one archive, then off the droplet whether or not
        # it arrives. The service runs on: the files are written once and never change.
        Write-Step 'Bringing the covers and documents home...'
        $tgz = Join-Path ([IO.Path]::GetTempPath()) "homelibrary-files-$token.tgz"
        try {
            $null = Invoke-Remote -Script (Format-RemoteScript -Value @{ DIR = $remoteDir; OUT = $pullFolder } -Template @'
umask 077
rm -rf '__OUT__'
mkdir '__OUT__'
tar -czf '__OUT__/files.tgz' -C '__DIR__/data' media documents
'@)
            if (-not (Get-File -Source "$pullFolder/files.tgz" -Destination $tgz)) {
                throw 'The covers and documents could not be downloaded.'
            }
        } finally {
            $null = Invoke-Remote -Script "rm -rf '$pullFolder'"
        }
        $devdata = Join-Path $root 'devdata'
        Remove-Item -Recurse -Force (Join-Path $devdata 'media'), (Join-Path $devdata 'documents') -ErrorAction SilentlyContinue
        & (Resolve-TarTool) -xzf $tgz -C $devdata
        if ($LASTEXITCODE -ne 0) { throw "The files' archive could not be unpacked; it is in $tgz." }
        Remove-Item -LiteralPath $tgz
        Write-Ok "Covers and documents copied to $devdata"
    }
} finally {
    Close-DeployConnection
    Remove-Item -LiteralPath $archive.Path -ErrorAction SilentlyContinue
}
Write-Host "`n[OK] Deployed $version" -ForegroundColor Green
