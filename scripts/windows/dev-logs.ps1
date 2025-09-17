$ErrorActionPreference = "Stop"

# 目的：在 Windows 下跟随输出 Docker Compose 服务日志
# 用法：
#   .\dev-logs.ps1              # 跟随所有服务日志
#   .\dev-logs.ps1 api          # 仅跟随名为 api 的服务日志
# 行为：设定 COMPOSE_PROJECT_NAME 并显示最近 200 行日志后持续跟随

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "../..")
$ComposeFile = Join-Path $RepoRoot "deploy/dev/docker-compose.yml"

$env:COMPOSE_PROJECT_NAME = "spike_shop"
param(
  [string]$Service
)

if ($Service) {
  docker compose -f $ComposeFile logs -f --tail=200 $Service | Out-Host
} else {
  docker compose -f $ComposeFile logs -f --tail=200 | Out-Host
}


