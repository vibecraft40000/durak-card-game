# Timeweb Cloud API - get server info, password for deploy
# Set $env:TIMEWEB_API_TOKEN = "your_jwt_token"

param([string]$Token = $env:TIMEWEB_API_TOKEN)
if (-not $Token) { Write-Host "Set TIMEWEB_API_TOKEN"; exit 1 }

$r = Invoke-RestMethod -Uri "https://api.timeweb.cloud/api/v1/servers" -Headers @{ Authorization = "Bearer $Token" }
$r.servers | ForEach-Object {
    $ip = ($_.networks | Where-Object { $_.ips } | ForEach-Object { $_.ips } | Where-Object { $_.type -eq "ipv4" } | Select-Object -First 1).ip
    Write-Host "Server: $($_.name) IP: $ip"
}
