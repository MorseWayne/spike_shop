#!/usr/bin/env bash
set -euo pipefail

# 目的：停止并清理本地开发环境
# 行为：
# - 设定 Compose 项目名以匹配 dev-up.sh 启动的环境
# - 执行 docker compose down -v，停止容器并删除关联的匿名卷
# 输出：完成提示

# 解析路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

export COMPOSE_PROJECT_NAME=spike_shop

# 停止并移除容器与匿名卷
docker compose -f "$REPO_ROOT/deploy/dev/docker-compose.yml" down -v

echo "Services stopped and volumes removed."


