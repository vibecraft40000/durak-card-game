param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$WsUrl = "ws://lb/ws",
  [string]$VusList = "1200,1500,2000",
  [int]$HoldMinSec = 10,
  [int]$HoldMaxSec = 30,
  [int]$ProfileVus = 1500,
  [int]$ProfileSeconds = 25
)

$ErrorActionPreference = "Continue"
$PSNativeCommandUseErrorActionPreference = $false

function Get-MetricValue {
  param([string]$MetricsText, [string]$Pattern)
  $line = ($MetricsText -split "`n" | Where-Object { $_ -match $Pattern } | Select-Object -First 1)
  if (-not $line) { return 0.0 }
  return [double](($line -split "\s+")[-1])
}

function Convert-ToMiB {
  param([string]$MemToken)
  if (-not $MemToken) { return 0.0 }
  if ($MemToken -match "([0-9.]+)([KMG]i?)B") {
    $value = [double]$matches[1]
    $unit = $matches[2]
    switch ($unit) {
      "Ki" { return $value / 1024.0 }
      "K"  { return $value / 1024.0 }
      "Mi" { return $value }
      "M"  { return $value }
      "Gi" { return $value * 1024.0 }
      "G"  { return $value * 1024.0 }
    }
  }
  return 0.0
}

function New-Auth {
  param([int]$TelegramId, [string]$Username)
  docker exec docker-redis-1 sh -c "redis-cli --scan --pattern 'rl:login*' | xargs -r redis-cli DEL" | Out-Null
  docker exec docker-redis-1 redis-cli DEL auth:replay:dev | Out-Null
  $payload = @{
    initData = "auth_date=1999999999&user=%7B%22id%22%3A$TelegramId%2C%22username%22%3A%22$Username%22%7D&hash=dev"
  } | ConvertTo-Json
  $resp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/auth/telegram" -ContentType "application/json" -Body $payload
  if (-not $resp.accessToken) { throw "Auth failed: access token missing." }
  return $resp
}

function New-LoadRoom {
  param([int]$Seed)
  $auth1 = New-Auth -TelegramId (500000 + $Seed) -Username ("u" + (500000 + $Seed))
  $auth2 = New-Auth -TelegramId (600000 + $Seed) -Username ("u" + (600000 + $Seed))
  $u1 = $auth1.user.id
  $u2 = $auth2.user.id
  $token1 = $auth1.accessToken
  $token2 = $auth2.accessToken
  docker exec docker-postgres-1 psql -U durak -d durak -c "INSERT INTO transactions (id,user_id,type,amount,status,created_at) VALUES (gen_random_uuid(),'$u1','deposit',1000,'confirmed',NOW()),(gen_random_uuid(),'$u2','deposit',1000,'confirmed',NOW()) ON CONFLICT DO NOTHING;" | Out-Null

  $headers1 = @{ Authorization = "Bearer $token1" }
  $headers2 = @{ Authorization = "Bearer $token2" }
  $roomReq = @{ title = "Degradation Room $Seed"; stake = 1; maxPlayers = 2; deck = 36; mode = "Подкидной" } | ConvertTo-Json
  $roomResp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/rooms" -Headers $headers1 -ContentType "application/json" -Body $roomReq
  $roomId = $roomResp.room.id
  if (-not $roomId) { throw "Room creation failed: empty room id." }
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
$vusLevels = $VusList.Split(",") | ForEach-Object { [int]$_.Trim() }
$report = @()

$containerNames = @(docker ps --format "{{.Names}}" | Where-Object { $_ -match "^docker-(api|api2|redis|postgres)-" })
if ($containerNames.Count -eq 0) {
  Write-Error "No docker containers found for monitoring."
  exit 1
}

foreach ($vus in $vusLevels) {
  $seed = Get-Random -Minimum 100000 -Maximum 900000
  $room = New-LoadRoom -Seed $seed
  $summaryFile = Join-Path $resultsDir "degradation-$vus.json"
  $logFile = Join-Path $resultsDir "degradation-$vus.log"
  $monitorFile = Join-Path $resultsDir "degradation-monitor-$vus.csv"
  $stopFile = Join-Path $resultsDir "degradation-stop-$vus.flag"
  if (Test-Path $stopFile) { Remove-Item $stopFile -Force }
  if (Test-Path $monitorFile) { Remove-Item $monitorFile -Force }

  $monitorJob = Start-Job -ScriptBlock {
    param($baseUrl, $monitorPath, $stopPath, $containers)
    while (-not (Test-Path $stopPath)) {
      $ts = Get-Date -Format o
      $metricsText = ""
      try { $metricsText = (Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/metrics" -TimeoutSec 2).Content } catch {}
      $ws = 0.0
      $active = 0.0
      $gc99 = 0.0
      $gc95 = 0.0
      $redisCount = 0.0
      if ($metricsText) {
        $wsLine = ($metricsText -split "`n" | Where-Object { $_ -match "^ws_active_connections(\{.*\})?\s+" } | Select-Object -First 1)
        $amLine = ($metricsText -split "`n" | Where-Object { $_ -match "^active_matches(\{.*\})?\s+" } | Select-Object -First 1)
        $gc99Line = ($metricsText -split "`n" | Where-Object { $_ -match '^go_gc_duration_seconds\{quantile="0.99"\}\s+' } | Select-Object -First 1)
        $gc95Line = ($metricsText -split "`n" | Where-Object { $_ -match '^go_gc_duration_seconds\{quantile="0.95"\}\s+' } | Select-Object -First 1)
        $redisLines = ($metricsText -split "`n" | Where-Object { $_ -match '^redis_latency_seconds_count\{.*\}\s+' })
        if ($wsLine) { $ws = [double](($wsLine -split "\s+")[-1]) }
        if ($amLine) { $active = [double](($amLine -split "\s+")[-1]) }
        if ($gc99Line) { $gc99 = [double](($gc99Line -split "\s+")[-1]) }
        if ($gc95Line) { $gc95 = [double](($gc95Line -split "\s+")[-1]) }
        foreach ($r in $redisLines) { $redisCount += [double](($r -split "\s+")[-1]) }
      }
      "$ts|metrics|$ws|$active|$gc95|$gc99|$redisCount" | Out-File -FilePath $monitorPath -Append

      $stats = docker stats --no-stream --format "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}" $containers 2>$null
      foreach ($line in $stats) { "$ts|stats|$line" | Out-File -FilePath $monitorPath -Append }

      $redisMem = 0
      try {
        $info = docker exec docker-redis-1 redis-cli INFO memory
        $usedLine = ($info | Where-Object { $_ -match "^used_memory:" } | Select-Object -First 1)
        if ($usedLine) { $redisMem = [double](($usedLine -replace "used_memory:","")) }
      } catch {}
      "$ts|redis_mem|$redisMem" | Out-File -FilePath $monitorPath -Append

      Start-Sleep -Milliseconds 1000
    }
  } -ArgumentList $BaseUrl, $monitorFile, $stopFile, $containerNames

  $profileFile = Join-Path $resultsDir "pprof-$vus.pb.gz"
  $profileJob = $null
  if ($vus -eq $ProfileVus) {
    if (Test-Path $profileFile) { Remove-Item $profileFile -Force }
    $profileJob = Start-Job -ScriptBlock {
      param($seconds, $outputPath)
      Start-Sleep -Seconds 5
      try {
        Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:6060/debug/pprof/profile?seconds=$seconds" -OutFile $outputPath -TimeoutSec ($seconds + 30) | Out-Null
      } catch {
      }
    } -ArgumentList $ProfileSeconds, $profileFile
  }

  & docker compose -f "docker/docker-compose.yml" run --rm --no-deps `
    -e "BASE_URL=http://host.docker.internal:8080" `
    -e "WS_URL=$WsUrl" `
    -e "VUS=$vus" `
    -e "HOLD_MIN_SEC=$HoldMinSec" `
    -e "HOLD_MAX_SEC=$HoldMaxSec" `
    -e "WS_TOKEN=$($room.token1)" `
    -e "ROOM_ID=$($room.roomId)" `
    k6 run --summary-export "/work/loadtest/results/degradation-$vus.json" loadtest/ws-degradation.js 2>&1 | Tee-Object -FilePath $logFile
  if (-not (Test-Path $summaryFile)) {
    throw "k6 summary file was not generated for VUS=$vus"
  }

  New-Item -ItemType File -Path $stopFile -Force | Out-Null
  Wait-Job $monitorJob | Out-Null
  Remove-Job $monitorJob -Force
  if ($profileJob) {
    Wait-Job $profileJob | Out-Null
    Remove-Job $profileJob -Force
  }

  $summary = Get-Content $summaryFile -Raw | ConvertFrom-Json
  $moveAvg = [double]$summary.metrics.move_latency_ms.avg
  $moveP95 = [double]$summary.metrics.move_latency_ms.'p(95)'
  $moveP99 = [double]$summary.metrics.move_latency_ms.'p(99)'
  $wsErrors = if ($summary.metrics.ws_errors_total) { [double]$summary.metrics.ws_errors_total.count } else { 0.0 }
  $wsUnexpected = if ($summary.metrics.ws_unexpected_closes) { [double]$summary.metrics.ws_unexpected_closes.count } else { 0.0 }
  $moveTimeouts = if ($summary.metrics.move_timeouts_total) { [double]$summary.metrics.move_timeouts_total.count } else { 0.0 }
  $moveSent = if ($summary.metrics.move_sent_total) { [double]$summary.metrics.move_sent_total.count } else { 0.0 }
  $errorRate = if ($moveSent -gt 0) { (($wsErrors + $wsUnexpected + $moveTimeouts) / $moveSent) * 100.0 } else { 0.0 }
  $reconnectAttempts = if ($summary.metrics.reconnect_attempts_total) { [double]$summary.metrics.reconnect_attempts_total.count } else { 0.0 }
  $wsSessions = [double]$summary.metrics.ws_sessions.count
  $reconnectRate = if ($wsSessions -gt 0) { ($reconnectAttempts / $wsSessions) * 100.0 } else { 0.0 }
  $redisOpsPerSec = if ($summary.metrics.redis_ops_total) { [double]$summary.metrics.redis_ops_total.rate } else { 0.0 }

  $peakWs = 0.0
  $peakMatches = 0.0
  $peakGc95Ms = 0.0
  $peakGc99Ms = 0.0
  $peakRedisMemBytes = 0.0
  $cpuMem = @{}

  foreach ($line in (Get-Content $monitorFile)) {
    $parts = $line -split "\|"
    if ($parts.Length -lt 3) { continue }
    if ($parts[1] -eq "metrics" -and $parts.Length -ge 7) {
      $peakWs = [math]::Max($peakWs, [double]$parts[2])
      $peakMatches = [math]::Max($peakMatches, [double]$parts[3])
      $peakGc95Ms = [math]::Max($peakGc95Ms, [double]$parts[4] * 1000.0)
      $peakGc99Ms = [math]::Max($peakGc99Ms, [double]$parts[5] * 1000.0)
    } elseif ($parts[1] -eq "stats" -and $parts.Length -ge 5) {
      $name = $parts[2]
      $cpu = [double](($parts[3] -replace "%",""))
      $memUsedToken = (($parts[4] -split " / ")[0]).Trim()
      $memMiB = Convert-ToMiB $memUsedToken
      if (-not $cpuMem.ContainsKey($name)) {
        $cpuMem[$name] = @{ maxCpu = 0.0; maxMemMiB = 0.0 }
      }
      if ($cpu -gt $cpuMem[$name].maxCpu) { $cpuMem[$name].maxCpu = $cpu }
      if ($memMiB -gt $cpuMem[$name].maxMemMiB) { $cpuMem[$name].maxMemMiB = $memMiB }
    } elseif ($parts[1] -eq "redis_mem" -and $parts.Length -ge 3) {
      $peakRedisMemBytes = [math]::Max($peakRedisMemBytes, [double]$parts[2])
    }
  }

  $report += [PSCustomObject]@{
    VUs                          = $vus
    AvgLatencyMs                 = [math]::Round($moveAvg, 2)
    P95LatencyMs                 = [math]::Round($moveP95, 2)
    P99LatencyMs                 = [math]::Round($moveP99, 2)
    ErrorRatePercent             = [math]::Round($errorRate, 3)
    ReconnectRatePercent         = [math]::Round($reconnectRate, 2)
    RedisOpsPerSec               = [math]::Round($redisOpsPerSec, 2)
    WsActiveConnectionsPeak      = [int]$peakWs
    ActiveMatchesPeak            = [int]$peakMatches
    PeakGcPause95Ms              = [math]::Round($peakGc95Ms, 3)
    PeakGcPause99Ms              = [math]::Round($peakGc99Ms, 3)
    RedisMemoryPeakMiB           = [math]::Round(($peakRedisMemBytes / 1024 / 1024), 2)
    ApiCpuPeakPercent            = if ($cpuMem.ContainsKey("docker-api-1")) { [math]::Round($cpuMem["docker-api-1"].maxCpu, 2) } else { 0 }
    ApiMemPeakMiB                = if ($cpuMem.ContainsKey("docker-api-1")) { [math]::Round($cpuMem["docker-api-1"].maxMemMiB, 2) } else { 0 }
    PostgresCpuPeakPercent       = if ($cpuMem.ContainsKey("docker-postgres-1")) { [math]::Round($cpuMem["docker-postgres-1"].maxCpu, 2) } else { 0 }
    PostgresMemPeakMiB           = if ($cpuMem.ContainsKey("docker-postgres-1")) { [math]::Round($cpuMem["docker-postgres-1"].maxMemMiB, 2) } else { 0 }
    RedisCpuPeakPercent          = if ($cpuMem.ContainsKey("docker-redis-1")) { [math]::Round($cpuMem["docker-redis-1"].maxCpu, 2) } else { 0 }
    RedisContainerMemPeakMiB     = if ($cpuMem.ContainsKey("docker-redis-1")) { [math]::Round($cpuMem["docker-redis-1"].maxMemMiB, 2) } else { 0 }
  }
}

$report | Tee-Object -FilePath (Join-Path $resultsDir "degradation-report.txt") | Format-Table -AutoSize
$report | ConvertTo-Json -Depth 4 | Out-File -FilePath (Join-Path $resultsDir "degradation-report.json") -Encoding utf8

$profileInput = Join-Path $resultsDir "pprof-$ProfileVus.pb.gz"
if (Test-Path $profileInput) {
  $topOut = Join-Path $resultsDir "pprof-top-$ProfileVus.txt"
  & go tool pprof -top $profileInput | Out-File -FilePath $topOut -Encoding utf8
  $svgOut = Join-Path $resultsDir "pprof-flamegraph-$ProfileVus.svg"
  try {
    & go tool pprof -svg $profileInput | Out-File -FilePath $svgOut -Encoding utf8
  } catch {
  }
}
