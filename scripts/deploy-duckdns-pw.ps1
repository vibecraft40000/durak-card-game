# Deploy to durakonline.duckdns.org VPS with password
# Usage: .\deploy-duckdns-pw.ps1
# You will be prompted for password (or pass as $env:DEPLOY_PW)

param(
    [string]$Password = $env:DEPLOY_PW,
    [string]$VpsHost = "72.56.74.7",
    [string]$User = "root",
    [string]$RemotePath = "/root/durakonline"
)

$ErrorActionPreference = "Stop"
$ProjectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $ProjectRoot

# Build
Write-Host "Building frontend..." -ForegroundColor Cyan
& npm run build
New-Item -ItemType Directory -Force -Path "$ProjectRoot\docker\frontend-dist" | Out-Null
Copy-Item -Path "$ProjectRoot\dist\*" -Destination "$ProjectRoot\docker\frontend-dist" -Recurse -Force

if (-not $Password) {
    $sec = Read-Host -AsSecureString "Password for ${User}@${VpsHost}"
    $BSTR = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
    $Password = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($BSTR)
}

# Posh-SSH
if (-not (Get-Module Posh-SSH -ErrorAction SilentlyContinue)) {
    Import-Module Posh-SSH -ErrorAction Stop
}
$secPass = ConvertTo-SecureString $Password -AsPlainText -Force
$cred = New-Object System.Management.Automation.PSCredential ($User, $secPass)

$session = $null
try {
    Write-Host "Connecting to ${VpsHost}..." -ForegroundColor Cyan
    $session = New-SSHSession -ComputerName $VpsHost -Credential $cred -AcceptKey -Force
    if (-not $session) { throw "SSH connection failed" }

    Write-Host "Creating remote dir..." -ForegroundColor Cyan
    Invoke-SSHCommand -SessionId $session.SessionId -Command "mkdir -p $RemotePath" | Out-Null

    Write-Host "Uploading backend..." -ForegroundColor Cyan
    Set-SCPItem -ComputerName $VpsHost -Credential $cred -Path "$ProjectRoot\backend" -Destination $RemotePath -Recurse -AcceptKey -Force

    Write-Host "Uploading docker..." -ForegroundColor Cyan
    Set-SCPItem -ComputerName $VpsHost -Credential $cred -Path "$ProjectRoot\docker" -Destination $RemotePath -Recurse -AcceptKey -Force

    if (Test-Path "$ProjectRoot\.env") {
        Write-Host "Uploading .env..." -ForegroundColor Cyan
        Set-SCPItem -ComputerName $VpsHost -Credential $cred -Path "$ProjectRoot\.env" -Destination "$RemotePath/.env" -AcceptKey -Force
    }

    Write-Host "Running setup on VPS..." -ForegroundColor Cyan
    $result = Invoke-SSHCommand -SessionId $session.SessionId -Command "cd $RemotePath && chmod +x scripts/setup-duckdns-ssl.sh 2>/dev/null; sh scripts/setup-duckdns-ssl.sh"
    Write-Host $result.Output
    if ($result.Error) { Write-Host $result.Error -ForegroundColor Yellow }

    Write-Host ""
    Write-Host "Done. App: https://durakonline.duckdns.org" -ForegroundColor Green
} finally {
    if ($session) { Remove-SSHSession -SessionId $session.SessionId -ErrorAction SilentlyContinue }
}
