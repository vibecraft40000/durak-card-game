param(
  [string]$BaseUrl = "http://localhost:8080",
  [int]$PauseSeconds = 10
)

$ErrorActionPreference = "Continue"

Write-Host "before pause /ready:"
Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/ready" | Select-Object -ExpandProperty Content

docker pause docker-redis-1 | Out-Null
Start-Sleep -Seconds $PauseSeconds

Write-Host "during pause /ready:"
try {
  Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/ready" -TimeoutSec 3 | Select-Object -ExpandProperty Content
} catch {
  Write-Host "ready probe failed as expected during redis outage"
}

docker unpause docker-redis-1 | Out-Null
Start-Sleep -Seconds 2

Write-Host "after unpause /ready:"
Invoke-WebRequest -UseBasicParsing -Uri "$BaseUrl/ready" | Select-Object -ExpandProperty Content
