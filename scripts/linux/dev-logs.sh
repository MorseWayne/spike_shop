#!/usr/bin/env bash
set -euo pipefail

# 目的：跟随输出所有（或指定）服务的最近日志
# 用法：
#   ./dev-logs.sh            # 跟随所有服务日志
#   ./dev-logs.sh api        # 仅跟随名为 api 的服务日志
# 行为：
# - 设定 Compose 项目名以匹配 dev-up.sh 创建的环境
# - 默认显示最近 200 行并持续跟随（-f --tail=200）

# 解析路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

export COMPOSE_PROJECT_NAME=spike_shop

# 输出日志（可选第一个参数作为服务名）
docker compose -f "$REPO_ROOT/deploy/dev/docker-compose.yml" logs -f --tail=200 ${1:-}


