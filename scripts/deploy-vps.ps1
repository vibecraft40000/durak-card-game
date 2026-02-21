# Deploy Durak Online to VPS
# Usage: .\scripts\deploy-vps.ps1 [-VpsHost 72.56.74.7] [-RemotePath /root/durakonline]
# Requires: SSH key for root@VPS, or run remote block manually after upload.

param(
    [string]$VpsHost = "72.56.74.7",
    [string]$RemotePath = "/root/durakonline",
    [string]$SshUser = "root",
    [switch]$SkipBuild,
    [switch]$UploadOnly
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
if (-not (Test-Path "$ProjectRoot\docker\docker-compose.vps.yml")) {
    $ProjectRoot = Resolve-Path "$PSScriptRoot\.."
}

Write-Host "Project root: $ProjectRoot"
Set-Location $ProjectRoot

# 1. Build frontend
if (-not $SkipBuild) {
    Write-Host "Building frontend..."
    & npm run build
    if (-not $?) { throw "npm run build failed" }
    New-Item -ItemType Directory -Force -Path "$ProjectRoot\docker\frontend-dist" | Out-Null
    Copy-Item -Path "$ProjectRoot\dist\*" -Destination "$ProjectRoot\docker\frontend-dist" -Recurse -Force
    Write-Host "Frontend copied to docker/frontend-dist"
}

# 2. Upload backend + docker to VPS (exclude large dirs)
$RemoteHost = "${SshUser}@${VpsHost}"
Write-Host "Uploading to ${RemoteHost}${RemotePath} ..."

# Create remote dir
& ssh -o StrictHostKeyChecking=accept-new $RemoteHost "mkdir -p $RemotePath"
if (-not $?) {
    Write-Host "SSH failed. If you use password auth, upload manually:"
    Write-Host "  scp -r backend docker $RemoteHost`:${RemotePath}/"
    Write-Host "Then on VPS: cd $RemotePath && docker compose -f docker/docker-compose.vps.yml up -d --build"
    exit 1
}

# Upload backend (exclude .gopath, coverage, tmp)
$backendExclude = @("--exclude", ".gopath", "--exclude", "*.test", "--exclude", "coverage.txt")
# Use scp -r for simplicity (no rsync on Windows by default)
& scp -r -o StrictHostKeyChecking=accept-new "$ProjectRoot\backend" $RemoteHost`:${RemotePath}/
if (-not $?) { throw "scp backend failed" }
& scp -r -o StrictHostKeyChecking=accept-new "$ProjectRoot\docker" $RemoteHost`:${RemotePath}/
if (-not $?) { throw "scp docker failed" }
if (Test-Path "$ProjectRoot\.env") {
    & scp -o StrictHostKeyChecking=accept-new "$ProjectRoot\.env" $RemoteHost`:${RemotePath}/.env
}
Write-Host "Upload done."

if ($UploadOnly) {
    Write-Host "Upload only. On VPS run: cd $RemotePath && docker compose -f docker/docker-compose.vps.yml up -d --build"
    exit 0
}

# 3. Run docker compose on VPS
Write-Host "Running docker compose on VPS..."
$cmd = "cd $RemotePath && docker compose -f docker/docker-compose.vps.yml up -d --build"
& ssh -o StrictHostKeyChecking=accept-new $RemoteHost $cmd
if (-not $?) { throw "docker compose on VPS failed" }

Write-Host "Deploy finished. App: http://${VpsHost}/ (use HTTPS domain in BotFather for Telegram)"
Write-Host "Set ALLOWED_ORIGIN to your HTTPS URL (e.g. https://your-domain.com) in .env on VPS or in compose."
