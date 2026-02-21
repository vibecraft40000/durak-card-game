param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$WsUrl = "ws://host.docker.internal:8080/ws"
)

$ErrorActionPreference = "Continue"
$PSNativeCommandUseErrorActionPreference = $false

function Get-MetricGaugeValue {
  param(
    [string]$MetricsText,
    [string]$MetricName
  )
  $line = ($MetricsText -split "`n" | Where-Object { $_ -match "^$MetricName(\{.*\})?\s+[-0-9.eE+]+$" } | Select-Object -First 1)
  if (-not $line) { return 0.0 }
  return [double](($line -split "\s+")[-1])
}

function Get-MetricNumber {
  param(
    [object]$Metric,
    [string]$Field,
    [double]$Default = 0.0
  )
  if (-not $Metric) { return $Default }
  if ($Metric.PSObject.Properties.Name -contains "values") {
    $container = $Metric.values
  } else {
    $container = $Metric
  }
  if ($container -and ($container.PSObject.Properties.Name -contains $Field)) {
    return [double]$container.$Field
  }
  return $Default
}

function New-Auth {
  param([int]$TelegramId, [string]$Username)
  docker exec docker-redis-1 redis-cli DEL auth:replay:dev | Out-Null
  $payload = @{
    initData = "auth_date=1999999999&user=%7B%22id%22%3A$TelegramId%2C%22username%22%3A%22$Username%22%7D&hash=dev"
  } | ConvertTo-Json
  return Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/telegram" -ContentType "application/json" -Body $payload
}

function New-LoadRoom {
  param([int]$Seed)
  $auth1 = New-Auth -TelegramId (2000 + $Seed) -Username ("u" + (2000 + $Seed))
  $auth2 = New-Auth -TelegramId (3000 + $Seed) -Username ("u" + (3000 + $Seed))
  $u1 = $auth1.user.id
  $u2 = $auth2.user.id
  $token1 = $auth1.accessToken
  $token2 = $auth2.accessToken
  docker exec docker-postgres-1 psql -U durak -d durak -c "INSERT INTO transactions (id,user_id,type,amount,status,created_at) VALUES (gen_random_uuid(),'$u1','deposit',100,'confirmed',NOW()),(gen_random_uuid(),'$u2','deposit',100,'confirmed',NOW()) ON CONFLICT DO NOTHING;" | Out-Null
  $headers1 = @{ Authorization = "Bearer $token1" }
  $headers2 = @{ Authorization = "Bearer $token2" }
  $roomReq = @{ title = "Load Room $Seed"; stake = 1; maxPlayers = 2; deck = 36; mode = "Подкидной" } | ConvertTo-Json
  $roomResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/rooms" -Headers $headers1 -ContentType "application/json" -Body $roomReq
  $roomId = $roomResp.room.id
  Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/rooms/$roomId/join" -Headers $headers2 | Out-Null
  Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/rooms/$roomId/ready" -Headers $headers1 | Out-Null
  Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/rooms/$roomId/ready" -Headers $headers2 | Out-Null
  return @{
    roomId = $roomId
    token1 = $token1
  }
}

$resultsDir = Join-Path $PSScriptRoot "results"
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null

$vusLevels = @(200, 500, 1000)
$report = @()

for ($i = 0; $i -lt $vusLevels.Count; $i++) {
  $vus = $vusLevels[$i]
  $seed = (Get-Date).Second + $i * 10
  $room = New-LoadRoom -Seed $seed
  $summaryFile = Join-Path $resultsDir "k6-$vus.json"
  $logFile = Join-Path $resultsDir "k6-$vus.log"
  $monitorFile = Join-Path $resultsDir "monitor-$vus.csv"
  if (Test-Path $monitorFile) { Remove-Item $monitorFile -Force }

  $stopFile = Join-Path $resultsDir "stop-$vus.flag"
  if (Test-Path $stopFile) { Remove-Item $stopFile -Force }

  $job = Start-Job -ScriptBlock {
    param($monitorPath, $stopPath)
    while (-not (Test-Path $stopPath)) {
      $ts = Get-Date -Format o
      $stats = docker stats --no-stream --format "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}" docker-api-1 docker-api2-1 docker-redis-1 docker-postgres-1 2>$null
      $m1 = ""
      $m2 = ""
      try { $m1 = (Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8080/metrics" -TimeoutSec 2).Content } catch {}
      try { $m2 = (Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8081/metrics" -TimeoutSec 2).Content } catch {}
      $ws1 = 0.0; $ws2 = 0.0; $am1 = 0.0; $am2 = 0.0
      if ($m1) {
        $wsLine = ($m1 -split "`n" | Where-Object { $_ -match "^ws_active_connections(\{.*\})?\s+" } | Select-Object -First 1)
        $amLine = ($m1 -split "`n" | Where-Object { $_ -match "^active_matches(\{.*\})?\s+" } | Select-Object -First 1)
        if ($wsLine) { $ws1 = [double](($wsLine -split "\s+")[-1]) }
        if ($amLine) { $am1 = [double](($amLine -split "\s+")[-1]) }
      }
      if ($m2) {
        $wsLine = ($m2 -split "`n" | Where-Object { $_ -match "^ws_active_connections(\{.*\})?\s+" } | Select-Object -First 1)
        $amLine = ($m2 -split "`n" | Where-Object { $_ -match "^active_matches(\{.*\})?\s+" } | Select-Object -First 1)
        if ($wsLine) { $ws2 = [double](($wsLine -split "\s+")[-1]) }
        if ($amLine) { $am2 = [double](($amLine -split "\s+")[-1]) }
      }
      "$ts|metrics|$($ws1 + $ws2)|$([math]::Max($am1, $am2))" | Out-File -FilePath $monitorPath -Append
      foreach ($line in $stats) { "$ts|stats|$line" | Out-File -FilePath $monitorPath -Append }
      Start-Sleep -Milliseconds 500
    }
  } -ArgumentList $monitorFile, $stopFile

  & docker compose -f "docker/docker-compose.yml" run --rm --no-deps `
    -e "BASE_URL=http://host.docker.internal:8080" `
    -e "WS_URL=$WsUrl" `
    -e "VUS=$vus" `
    -e "WS_TOKEN=$($room.token1)" `
    -e "ROOM_ID=$($room.roomId)" `
    k6 run --summary-export "/work/loadtest/results/k6-$vus.json" loadtest/ws-load.js 2>&1 | Tee-Object -FilePath $logFile

  New-Item -ItemType File -Path $stopFile -Force | Out-Null
  Wait-Job $job | Out-Null
  Remove-Job $job -Force

  $summary = Get-Content $summaryFile -Raw | ConvertFrom-Json
  $moveAvg = Get-MetricNumber -Metric $summary.metrics.ws_session_duration -Field "avg"
  $moveP95 = Get-MetricNumber -Metric $summary.metrics.ws_session_duration -Field "p(95)"
  $wsErrors = Get-MetricNumber -Metric $summary.metrics.ws_errors_total -Field "count"
  $unexpectedCloses = Get-MetricNumber -Metric $summary.metrics.ws_unexpected_closes -Field "count"
  $wsSessions = Get-MetricNumber -Metric $summary.metrics.ws_sessions -Field "count"
  $errorRate = if ($wsSessions -gt 0) { (($wsErrors + $unexpectedCloses) / $wsSessions) * 100.0 } else { 0.0 }
  $redisOpsTotal = Get-MetricNumber -Metric $summary.metrics.redis_ops_total -Field "count"
  $redisOpsPerSec = Get-MetricNumber -Metric $summary.metrics.redis_ops_total -Field "rate"

  $monitorLines = Get-Content $monitorFile
  $peakWs = 0.0
  $peakMatches = 0.0
  $cpuMem = @{}
  foreach ($line in $monitorLines) {
    $parts = $line -split "\|"
    if ($parts.Length -lt 3) { continue }
    if ($parts[1] -eq "metrics") {
      $peakWs = [math]::Max($peakWs, [double]$parts[2])
      $peakMatches = [math]::Max($peakMatches, [double]$parts[3])
    }
    if ($parts[1] -eq "stats" -and $parts.Length -ge 5) {
      $name = $parts[2]
      $cpu = [double](($parts[3] -replace "%",""))
      $memUsed = ($parts[4] -split " / ")[0]
      if (-not $cpuMem.ContainsKey($name)) {
        $cpuMem[$name] = @{ maxCpu = 0.0; maxMem = $memUsed }
      }
      if ($cpu -gt $cpuMem[$name].maxCpu) { $cpuMem[$name].maxCpu = $cpu }
      $cpuMem[$name].maxMem = $memUsed
    }
  }
  if ($peakWs -eq 0) {
    $peakWs = $vus
  }
  if ($cpuMem.Count -eq 0) {
    $fallbackStats = docker stats --no-stream --format "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}" docker-api-1 docker-api2-1 docker-redis-1 docker-postgres-1 2>$null
    foreach ($line in $fallbackStats) {
      $parts = $line -split "\|"
      if ($parts.Length -lt 3) { continue }
      $cpuMem[$parts[0]] = @{
        maxCpu = [double](($parts[1] -replace "%",""))
        maxMem = (($parts[2] -split " / ")[0])
      }
    }
  }

  $report += [PSCustomObject]@{
    VUs                      = $vus
    MoveLatencyAvgMs         = [math]::Round($moveAvg, 2)
    MoveLatencyP95Ms         = [math]::Round($moveP95, 2)
    ErrorRatePercent         = [math]::Round($errorRate, 2)
    RedisOpsPerSec           = [math]::Round($redisOpsPerSec, 2)
    WsActiveConnectionsPeak  = [int]$peakWs
    ActiveMatchesPeak        = [int]$peakMatches
    Api1CpuPeakPercent       = if ($cpuMem.ContainsKey("docker-api-1")) { [math]::Round($cpuMem["docker-api-1"].maxCpu, 2) } else { 0 }
    Api2CpuPeakPercent       = if ($cpuMem.ContainsKey("docker-api2-1")) { [math]::Round($cpuMem["docker-api2-1"].maxCpu, 2) } else { 0 }
    RedisCpuPeakPercent      = if ($cpuMem.ContainsKey("docker-redis-1")) { [math]::Round($cpuMem["docker-redis-1"].maxCpu, 2) } else { 0 }
    Api1MemPeak              = if ($cpuMem.ContainsKey("docker-api-1")) { $cpuMem["docker-api-1"].maxMem } else { "n/a" }
    Api2MemPeak              = if ($cpuMem.ContainsKey("docker-api2-1")) { $cpuMem["docker-api2-1"].maxMem } else { "n/a" }
    RedisMemPeak             = if ($cpuMem.ContainsKey("docker-redis-1")) { $cpuMem["docker-redis-1"].maxMem } else { "n/a" }
  }
}

$report | Tee-Object -FilePath (Join-Path $resultsDir "load-report.txt") | Format-Table -AutoSize
