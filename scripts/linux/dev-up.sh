#!/usr/bin/env bash
set -euo pipefail

# 目的：启动本地开发环境（Docker Compose，后台运行）
# 行为：
# - 设定 Compose 项目名为 spike_shop，隔离容器与网络命名
# - 若存在 .env，加载其中的环境变量（忽略注释行）；用于端口/凭据等配置
# - 使用 deploy/dev/docker-compose.yml 启动所有服务（-d 后台）
# 输出：打印 RabbitMQ 管理界面的本地访问地址

# 解析脚本与仓库根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 统一 Compose 项目名，便于容器、网络、卷的命名与隔离
export COMPOSE_PROJECT_NAME=spike_shop

# 加载仓库根目录下的 .env（若存在）；忽略注释行
if [ -f "$REPO_ROOT/.env" ]; then
  export $(grep -v '^#' "$REPO_ROOT/.env" | xargs) || true
fi

# 启动开发编排（后台运行）
docker compose -f "$REPO_ROOT/deploy/dev/docker-compose.yml" up -d

# 友好提示：RabbitMQ 控制台地址（使用 .env 中的 RABBITMQ_MGMT_PORT，默认 15672）
echo "Services started. RabbitMQ UI: http://localhost:${RABBITMQ_MGMT_PORT:-15672}"


