#!/usr/bin/env bash
set -euo pipefail

# 目的：停止 Ubuntu 上手动安装运行的 MySQL、Redis、RabbitMQ
# 特性：
# - 读取仓库根目录 .env 端口配置（用于 UFW 规则匹配）
# - 停止服务；可选禁用开机自启；可选关闭 UFW 放行端口
# 用法：
#   bash scripts/linux/manual-stop.sh            # 仅停止服务
#   bash scripts/linux/manual-stop.sh --disable  # 停止并禁用自启
#   bash scripts/linux/manual-stop.sh --close-ports  # 关闭 UFW 放行
#   可组合： --disable --close-ports

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

DISABLE_AUTOSTART=false
CLOSE_PORTS=false
for arg in "$@"; do
  case "$arg" in
    --disable) DISABLE_AUTOSTART=true ;;
    --close-ports) CLOSE_PORTS=true ;;
    *) echo "未知参数：$arg" ; exit 2 ;;
  esac
done

# 读取 .env
if [ -f "$REPO_ROOT/.env" ]; then
  # shellcheck disable=SC2046
  export $(grep -v '^#' "$REPO_ROOT/.env" | xargs) || true
fi

# 端口变量（默认值与 env.example 对齐）
MYSQL_PORT=${MYSQL_PORT:-3306}
REDIS_PORT=${REDIS_PORT:-6379}
RABBITMQ_AMQP_PORT=${RABBITMQ_AMQP_PORT:-5672}
RABBITMQ_MGMT_PORT=${RABBITMQ_MGMT_PORT:-15672}

echo "[1/3] 停止服务..."
sudo systemctl stop mysql || true
sudo systemctl stop redis-server || true
sudo systemctl stop rabbitmq-server || true

if [ "$DISABLE_AUTOSTART" = true ]; then
  echo "[2/3] 禁用开机自启..."
  sudo systemctl disable mysql || true
  sudo systemctl disable redis-server || true
  sudo systemctl disable rabbitmq-server || true
else
  echo "[2/3] 跳过禁用自启（如需禁用请加 --disable）"
fi

echo "[3/3] 防火墙（UFW）收敛..."
if [ "$CLOSE_PORTS" = true ]; then
  if command -v ufw >/dev/null 2>&1 && sudo ufw status | grep -q "Status: active"; then
    sudo ufw delete allow "$MYSQL_PORT"/tcp || true
    sudo ufw delete allow "$REDIS_PORT"/tcp || true
    sudo ufw delete allow "$RABBITMQ_AMQP_PORT"/tcp || true
    sudo ufw delete allow "$RABBITMQ_MGMT_PORT"/tcp || true
  else
    echo "UFW 未启用或未安装，跳过关闭端口。"
  fi
else
  echo "跳过关闭防火墙端口规则（如需关闭请加 --close-ports）"
fi

echo "完成。"


