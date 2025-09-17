$ErrorActionPreference = "Stop"

# 目的：在 Windows 环境停止并清理本地开发编排
# 行为：
# - 设置 COMPOSE_PROJECT_NAME=spike_shop 以定位当前项目的容器/网络
# - docker compose down -v 停止容器并删除匿名卷
# 输出：完成提示

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "../..")
$ComposeFile = Join-Path $RepoRoot "deploy/dev/docker-compose.yml"

$env:COMPOSE_PROJECT_NAME = "spike_shop"
docker compose -f $ComposeFile down -v | Out-Host
Write-Host "Services stopped and volumes removed."


