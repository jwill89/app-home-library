#!/usr/bin/env pwsh
#Requires -Version 7.4

<#
.SYNOPSIS
Deploy the newest release to the droplet and bring a copy of the library home.

.DESCRIPTION
Built on the SharedDeploy module (shared/deploy/pwsh): one archive, one held SSH
connection, one remote run.

The release is the newest tag CI made on main (the script fetches the tags first). Commits
after it, an update of shared/ or work not yet released, are not part of it, and neither is
anything uncommitted: the release is checked out into a folder of its own, with npm ci from its
lockfile, and built there. The host records the release each deploy leaves running; when that
is already this release, nothing is done (-Force deploys it again).

Opens one SSH connection and keeps it for the run, then builds the static Linux binary. The
two systemd units and the Apache vhost are templates in deploy/: the settings fill in what is
the host's (the service, its user and folders, the site's names, its certificate and the app's
port), and they go into one archive with the binary. In one remote run it checks the archive
and that Apache takes the vhost before anything stops, then stops the service and the text
service, snapshots the database (compressed), installs the binary and whichever units differ,
starts both, and waits up to 30 seconds for /healthz on the app's port. The vhost is the one
file /etc/apache2/sites-available/<service>.conf holding both ports: it goes in where it
differs and is enabled where it isn't, and any other enabled site serving the same names is
retired, all tried on a copy of Apache's config first. A build that does not answer goes back:
the binary, units, vhost and retired sites before it, and the database as the snapshot left it,
since the new build may have migrated it before it fell over.

Then the site is fetched through Cloudflare: /healthz must name this version, and / this
build's entry script, which must load. A failure there rolls back the same way. Then the
nightly backup's timer is checked (the module's Install-BackupTimer: where the droplet's
units differ from the ones it writes, the new ones go in and `homelibrary backup` runs once,
the old ones coming back if it fails).

Last, an online snapshot of the live database comes down to devdata/homelibrary.sqlite, and an
archive of the covers and documents' files to devdata/media and devdata/documents (skip both
with -NoDbPull); neither stays on the droplet. Both include the owner's own photographs and
scans, which exist nowhere else, so every release leaves a copy of them on this machine.

Settings resolve from the parameter, then $env:DEPLOY_*, then the git-ignored
scripts/deploy.config.ps1 (copy scripts/deploy.config.example.ps1 and fill it in). A run is one
SSH connection (plus one only if it drops and reconnects), which stays under the host's
`ufw limit ssh`.

Requires PuTTY's plink 0.85 or later and the key loaded in Pageant. -Preview ships the working
tree as it is, stamped preview-<time>, so a build reaches a phone to be tried before it is
committed; -SkipBuild, with it alone, reuses the last dist/homelibrary-linux-amd64.

.PARAMETER VpsHost
The droplet's address or name.

.PARAMETER VpsUser
The SSH user.

.PARAMETER KeyPath
The PuTTY .ppk key.

.PARAMETER ServiceName
The web server's systemd service. The text service is the same name with -text, and the vhost
file is named after it.

.PARAMETER ServiceUser
The account both services run as. The service's name when omitted.

.PARAMETER RemoteOptDir
The folder on the host that holds the binary, with data/ and backups/ inside it.

.PARAMETER EnvFile
The services' env file on the host. /etc/<service>/<service>.env when omitted.

.PARAMETER ServerName
The site's main host name, the vhost's ServerName.

.PARAMETER ServerAlias
The site's other host names, space-separated, the vhost's ServerAlias. None when omitted.

.PARAMETER CertName
The certificate's folder under /etc/letsencrypt/live, which must cover every name.

.PARAMETER AppPort
The loopback port the web server listens on (HL_ADDR in the env file), which Apache proxies
to and the health check asks.

.PARAMETER SiteUrl
The site's address as visitors reach it, for the smoke check. https:// and the ServerName
when omitted.

.PARAMETER SkipBuild
Reuse the last built binary. Goes with -Preview only: a release is always built from its tag.

.PARAMETER NoDbPull
Leave the live database, covers and documents on the droplet; bring no copy home.

.PARAMETER Preview
Ship the working tree as it is, stamped preview-<time>, instead of the newest release.

.PARAMETER Force
Deploy even when the host already runs this release.

.EXAMPLE
task deploy                      # the newest release

.EXAMPLE
task deploy -- -Preview          # the working tree, to try a build before committing it
#>
[CmdletBinding()]
param(
    [string]$VpsHost,
    [string]$VpsUser,
    [string]$KeyPath,
    [string]$ServiceName,
    [string]$ServiceUser,
    [string]$RemoteOptDir,
    [string]$EnvFile,
    [string]$ServerName,
    [string]$ServerAlias,
    [string]$CertName,
    [string]$AppPort,
    [string]$SiteUrl,
    [switch]$SkipBuild,
    [switch]$NoDbPull,
    [switch]$Preview,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $false
Import-Module (Join-Path $PSScriptRoot '..\shared\deploy\pwsh\SharedDeploy.psd1') -Force
trap { Close-DeployConnection; Write-Host "`nERROR: $($_.Exception.Message)" -ForegroundColor Red; exit 1 }

$root = Split-Path -Path $PSScriptRoot -Parent
Set-Location $root

$given = @{
    VpsHost      = $VpsHost
    VpsUser      = $VpsUser
    KeyPath      = $KeyPath
    ServiceName  = $ServiceName
    ServiceUser  = $ServiceUser
    RemoteOptDir = $RemoteOptDir
    EnvFile      = $EnvFile
    ServerName   = $ServerName
    ServerAlias  = $ServerAlias
    CertName     = $CertName
    AppPort      = $AppPort
    SiteUrl      = $SiteUrl
}
$settings = Resolve-DeploySetting -ConfigPath (Join-Path $PSScriptRoot 'deploy.config.ps1') -Value $given `
    -Required VpsHost, VpsUser, KeyPath, ServiceName, RemoteOptDir, ServerName, CertName, AppPort

# Each value goes into a unit, the vhost or a shell command, so each is held to its plain form.
$service = [string]$settings.ServiceName
if ($service -notmatch '^[a-z][a-z0-9-]*$') { throw "ServiceName must be a plain service name, not '$service'." }
$serviceUser = if ($settings.ServiceUser) { [string]$settings.ServiceUser } else { $service }
if ($serviceUser -notmatch '^[a-z_][a-z0-9_-]*$' -or $serviceUser -eq 'root') {
    throw "ServiceUser must be the service's own unprivileged account, not '$serviceUser'."
}
$remoteDir = "$($settings.RemoteOptDir)".TrimEnd('/')
$envFile = if ($settings.EnvFile) { [string]$settings.EnvFile } else { "/etc/$service/$service.env" }
foreach ($path in $remoteDir, $envFile) {
    if ($path -notmatch '^/[A-Za-z0-9._/-]+$' -or $path -match '(^|/)\.\.?(/|$)') { throw "'$path' must be an absolute path of plain names." }
}
$serverName = "$($settings.ServerName)".ToLowerInvariant()
$serverAlias = "$($settings.ServerAlias)".Trim().ToLowerInvariant() -replace '\s+', ' '
foreach ($name in @($serverName) + @($serverAlias -split ' ' | Where-Object { $_ })) {
    if ($name -notmatch '^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$') { throw "ServerName and ServerAlias must be plain host names, not '$name'." }
}
if ($settings.CertName -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]*$') { throw 'CertName must be a folder name of letters, digits and . _ - only, not a path.' }
if ($settings.AppPort -notmatch '^[0-9]+$' -or [int]$settings.AppPort -lt 1024 -or [int]$settings.AppPort -gt 65535) {
    throw "AppPort must be a port from 1024 to 65535, not '$($settings.AppPort)'."
}
$site = if ($settings.SiteUrl) { "$($settings.SiteUrl)".TrimEnd('/') } else { "https://$serverName" }

# The text service writes the database too: neither the new build's migration nor a
# rollback's restore runs under it, so the two stop, start and go back together.
$services = $service, "$service-text"
$units = "$service.service", "$service-text.service"
$remoteDb = "$remoteDir/data/homelibrary.sqlite"
$snapshotPrefix = "$remoteDir/backups/homelibrary-pre-"
$pullFolder = "$remoteDir/backups/pull-deploy"
$localDb = Join-Path $root 'devdata/homelibrary.sqlite'
# What the host records as running, for the next deploy to compare with.
$record = 'homelibrary'

if ($Preview) {
    $version = 'preview-' + (Get-Date -Format 'yyyyMMdd-HHmm')
} else {
    if ($SkipBuild) { throw 'A release is always built from its tag; -SkipBuild goes with -Preview only.' }
    $release = Get-DeployRelease -Path $root
    $version = $release.Tag
}

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
        -Database $remoteDb -SnapshotPrefix $snapshotPrefix -SnapshotSuffix '.sqlite'
}

# The units and the vhost are rendered here, from the release's templates and these settings.
$rendered = Join-Path ([IO.Path]::GetTempPath()) "homelibrary-deploy-$([guid]::NewGuid().ToString('N').Substring(0, 8))"

$connection = $settings | Open-DeployConnection
$tree = $null
$archive = $null
try {
    # A preview builds the working tree; a release, its tag checked out on its own.
    $build = $root
    if (-not $Preview) {
        $live = Get-DeployedRelease -Name $record
        if ($live -eq $version -and -not $Force) {
            Write-Ok "$version is already running on the host; nothing to deploy. (A version pushed since goes out once CI tags it; -Force deploys this one again.)"
            return
        }
        $after = if ($release.Since) { "; the $($release.Since) commit(s) on main after it are not part of it" } else { '' }
        Write-Step "Releasing $version (the host runs $(if ($live) { $live } else { 'a release not recorded' }))$after"
        $tree = New-ReleaseTree -Path $root -Tag $version -Npm 'web'
        $build = $tree
    }
    $binary = Join-Path $build 'dist/homelibrary-linux-amd64'
    if ($SkipBuild) {
        Write-Step 'Skipping the build (-SkipBuild); using dist/homelibrary-linux-amd64'
    } else {
        # A release's version is its tag, which the build reads with git describe.
        Push-Location -LiteralPath $build
        try {
            if ($Preview) { task build:linux VERSION=$version } else { task build:linux }
            if ($LASTEXITCODE -ne 0) { throw 'Build failed.' }
        } finally {
            Pop-Location
        }
    }
    if (-not (Test-Path $binary)) { throw "Binary not found: $binary" }

    # The built page names its entry script, which the smoke check looks for and loads.
    $entryScript = [regex]::Match((Get-Content -LiteralPath (Join-Path $build 'web/dist/index.html') -Raw), 'src="(/assets/[^"]+\.js)"').Groups[1].Value
    if (-not $entryScript) { throw 'web/dist/index.html names no entry script under /assets; build the front end first.' }

    $null = New-Item -ItemType Directory -Path $rendered
    $values = @{
        SERVICE      = $service
        USER         = $serviceUser
        DIR          = $remoteDir
        ENV_FILE     = $envFile
        SERVER_NAME  = $serverName
        SERVER_ALIAS = $serverAlias
        CERT         = $settings.CertName
        APP_PORT     = $settings.AppPort
    }
    $templates = [ordered]@{
        "$service.service"      = 'homelibrary.service'
        "$service-text.service" = 'homelibrary-text.service'
        'apache-vhost.conf'     = 'apache-vhost.conf'
    }
    foreach ($name in $templates.Keys) {
        $text = Get-Content -LiteralPath (Join-Path $build "deploy/$($templates[$name])") -Raw
        # A site with one name has no ServerAlias line: Apache refuses one with nothing on it.
        if (-not $serverAlias) { $text = $text -replace '(?m)^[ \t]*ServerAlias[ \t]+__SERVER_ALIAS__[ \t]*\r?\n', '' }
        $text = Format-RemoteScript -Template $text -Value $values
        if ($text -match '__[A-Z_]+__') { throw "deploy/$($templates[$name]) has a placeholder the settings do not fill: $($Matches[0])" }
        [IO.File]::WriteAllText((Join-Path $rendered $name), $text, [Text.UTF8Encoding]::new($false))
    }

    $content = [ordered]@{ 'homelibrary' = $binary }
    foreach ($name in $templates.Keys) { $content[$name] = Join-Path $rendered $name }
    $archive = Compress-DeployArchive -Content $content

    Write-Step "Deploying $version to $($connection.Target)"
    # Nothing stops until the upload is complete and checked.
    $upload = Invoke-Upload -Path $archive.Path -Destination 'homelibrary-deploy.tgz' -Activity "Uploading Home Library $version" -FileCount $archive.FileCount
    if (-not $upload.Ok) { throw "Upload failed; the live server is untouched.`n$($upload.Err)" }

    Invoke-DeployBundle -Archive 'homelibrary-deploy.tgz' -Sha256 $archive.Sha256 -ExpectedFileCount $archive.FileCount -Step @(
        $layout
        Install-ServiceBinary -AsStep -ServiceName $services -Binary "$remoteDir/homelibrary" -Owner 'root' `
            -Unit $units -Vhost 'apache-vhost.conf' -Umask '077' `
            -Database $remoteDb -SnapshotPrefix $snapshotPrefix -SnapshotSuffix '.sqlite' -RestoreDatabaseOnRollback `
            -HealthUrl "http://127.0.0.1:$($settings.AppPort)/healthz" -HealthTimeoutSeconds 30
    )
    Write-Ok "Deployed $version"

    # The site as visitors reach it, through Cloudflare and Apache.
    Write-Step "Checking $site..."
    # A fresh token on each address, so no answer Cloudflare kept from before can pass.
    $token = [guid]::NewGuid().ToString('N').Substring(0, 12)
    Invoke-SmokeCheck -Attempts 3 -DelaySeconds 2 -Rollback $restore -Check @(
        @{ Url = "$site/healthz?deploy=$token"; Contains = "`"version`":`"$version`"" }
        @{ Url = "$site/?deploy=$token"; Contains = $entryScript }
        @{ Url = "$site$entryScript" }
    )
    # Only a deploy that stayed is recorded; a preview's stamp makes the next release go out.
    Set-DeployedRelease -Name $record -Release $version

    # The nightly backup: 09:30 UTC, staggered from the other sites' backups on the same
    # droplet, `homelibrary backup` as the service user, with the service's hardening. One
    # writable path over both the data and the backups, not one each: each would be its own
    # mount, and the files' snapshot is hard links, which cannot cross mounts.
    Write-Step 'Checking the nightly backup...'
    try {
        Install-BackupTimer -Name $service -Command "$remoteDir/homelibrary backup" -Owner $serviceUser `
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
    if ($archive) { Remove-Item -LiteralPath $archive.Path -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $rendered -Recurse -Force -ErrorAction SilentlyContinue
    if ($tree) { Remove-ReleaseTree -Path $tree -Repository $root }
}
Write-Host "`n[OK] Deployed $version" -ForegroundColor Green
