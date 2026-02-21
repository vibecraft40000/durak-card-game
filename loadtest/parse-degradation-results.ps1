$levels = @(1200, 1500, 2000)

function MemToMiB {
  param([string]$token)
  $value = [double](($token -replace "[^0-9\.]", ""))
  if ($token -match "GiB") { return $value * 1024.0 }
  if ($token -match "KiB") { return $value / 1024.0 }
  return $value
}

foreach ($v in $levels) {
  $summaryPath = "loadtest/results/degradation-$v.json"
  $monitorPath = "loadtest/results/degradation-monitor-$v.csv"
  if (-not (Test-Path $summaryPath) -or -not (Test-Path $monitorPath)) {
    continue
  }

  $j = Get-Content $summaryPath -Raw | ConvertFrom-Json
  $m = $j.metrics
  $sent = [double]$m.move_sent_total.count
  $wsErr = if ($m.ws_errors_total) { [double]$m.ws_errors_total.count } else { 0.0 }
  $unexpected = if ($m.ws_unexpected_closes) { [double]$m.ws_unexpected_closes.count } else { 0.0 }
  $timeouts = if ($m.move_timeouts_total) { [double]$m.move_timeouts_total.count } else { 0.0 }
  $errorRate = (($wsErr + $unexpected + $timeouts) / [math]::Max($sent, 1)) * 100.0

  $peakWs = 0.0
  $peakActive = 0.0
  $apiCpu = 0.0; $apiMem = 0.0
  $api2Cpu = 0.0; $api2Mem = 0.0
  $pgCpu = 0.0; $pgMem = 0.0
  $redisCpu = 0.0; $redisMemContainer = 0.0; $redisMemBytesPeak = 0.0

  foreach ($line in (Get-Content $monitorPath)) {
    $p = $line -split "\|"
    if ($p.Length -lt 3) { continue }
    if ($p[1] -eq "metrics" -and $p.Length -ge 4) {
      $peakWs = [math]::Max($peakWs, [double]$p[2])
      $peakActive = [math]::Max($peakActive, [double]$p[3])
    } elseif ($p[1] -eq "stats" -and $p.Length -ge 5) {
      $name = $p[2]
      $cpu = [double](($p[3] -replace "%", ""))
      $memToken = (($p[4] -split " / ")[0]).Trim()
      $mem = MemToMiB $memToken
      if ($name -eq "docker-api-1") {
        $apiCpu = [math]::Max($apiCpu, $cpu)
        $apiMem = [math]::Max($apiMem, $mem)
      } elseif ($name -eq "docker-api2-1") {
        $api2Cpu = [math]::Max($api2Cpu, $cpu)
        $api2Mem = [math]::Max($api2Mem, $mem)
      } elseif ($name -eq "docker-postgres-1") {
        $pgCpu = [math]::Max($pgCpu, $cpu)
        $pgMem = [math]::Max($pgMem, $mem)
      } elseif ($name -eq "docker-redis-1") {
        $redisCpu = [math]::Max($redisCpu, $cpu)
        $redisMemContainer = [math]::Max($redisMemContainer, $mem)
      }
    } elseif ($p[1] -eq "redis_mem" -and $p.Length -ge 3) {
      $redisMemBytesPeak = [math]::Max($redisMemBytesPeak, [double]$p[2])
    }
  }

  "{0}|avg={1}|p95={2}|p99={3}|err={4}%|reconnect={5}%|redis_ops={6}|ws_peak={7}|active_peak={8}|api1_cpu={9}%|api1_mem={10}MiB|api2_cpu={11}%|api2_mem={12}MiB|api_total_cpu={13}%|redis_cpu={14}%|redis_mem={15}MiB|redis_used_mem_peak={16}MiB|pg_cpu={17}%|pg_mem={18}MiB" -f `
    $v, `
    [math]::Round([double]$m.move_latency_ms.avg, 2), `
    [math]::Round([double]$m.move_latency_ms.'p(95)', 2), `
    [math]::Round([double]$m.move_latency_ms.'p(99)', 2), `
    [math]::Round($errorRate, 2), `
    [math]::Round(([double]$m.reconnect_attempts_total.count / [math]::Max([double]$m.ws_sessions.count,1) * 100.0), 2), `
    [math]::Round([double]$m.redis_ops_total.rate, 2), `
    [int]$peakWs, `
    [int]$peakActive, `
    [math]::Round($apiCpu, 2), `
    [math]::Round($apiMem, 2), `
    [math]::Round($api2Cpu, 2), `
    [math]::Round($api2Mem, 2), `
    [math]::Round($apiCpu + $api2Cpu, 2), `
    [math]::Round($redisCpu, 2), `
    [math]::Round($redisMemContainer, 2), `
    [math]::Round($redisMemBytesPeak / 1024.0 / 1024.0, 2), `
    [math]::Round($pgCpu, 2), `
    [math]::Round($pgMem, 2)
}
