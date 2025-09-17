$ErrorActionPreference = "Stop"

# 目的：在 Windows 环境启动本地开发编排（后台运行）
# 行为：
# - 解析仓库路径与 docker-compose 文件
# - 若存在 .env，则加载非注释行到进程环境变量
# - 设置 COMPOSE_PROJECT_NAME=spike_shop，保证命名隔离一致
# - 执行 docker compose up -d 并输出 RabbitMQ 管理界面地址提示

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "../..")
$ComposeFile = Join-Path $RepoRoot "deploy/dev/docker-compose.yml"

if (Test-Path (Join-Path $RepoRoot ".env")) {
  Get-Content (Join-Path $RepoRoot ".env") | ForEach-Object {
    if ($_ -and -not $_.StartsWith('#')) {
      $kv = $_.Split('=',2)
      if ($kv.Length -eq 2) { [System.Environment]::SetEnvironmentVariable($kv[0], $kv[1]) }
    }
  }
}

$env:COMPOSE_PROJECT_NAME = "spike_shop"
docker compose -f $ComposeFile up -d | Out-Host
Write-Host ("Services started. RabbitMQ UI: http://localhost:{0}" -f ($env:RABBITMQ_MGMT_PORT ? $env:RABBITMQ_MGMT_PORT : 15672))


