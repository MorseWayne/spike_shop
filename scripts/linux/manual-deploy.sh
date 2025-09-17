#!/usr/bin/env bash
set -euo pipefail

# 目的：在 Ubuntu 上手动安装并配置 MySQL、Redis、RabbitMQ，使其可被外部（含 Windows 宿主）访问
# 特性：
# - 读取仓库根目录 .env 的端口/凭据（若不存在使用默认值）
# - 安装并启动 mysql-server、redis-server、rabbitmq-server
# - 配置 MySQL 监听 0.0.0.0，创建数据库与用户并授权远程访问
# - 配置 Redis 监听 0.0.0.0，可选设置密码（REDIS_PASSWORD 非空时）
# - 启用 RabbitMQ 管理插件，创建用户并授权（guest 仅允许本地，故另外创建）
# - 可选放通 UFW 防火墙端口（如已启用 UFW）
# - 自检：MySQL/Redis/RabbitMQ 连接与健康

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 加载 .env（若存在）
if [ -f "$REPO_ROOT/.env" ]; then
  # shellcheck disable=SC2046
  export $(grep -v '^#' "$REPO_ROOT/.env" | xargs) || true
fi

# 变量默认值（与 env.example 对齐）
APP_PORT=${APP_PORT:-8080}

MYSQL_PORT=${MYSQL_PORT:-3306}
MYSQL_DB=${MYSQL_DB:-spike}
MYSQL_USER=${MYSQL_USER:-spike}
MYSQL_PASSWORD=${MYSQL_PASSWORD:-spike}
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD:-}
MYSQL_HOST=${MYSQL_HOST:-0.0.0.0}

REDIS_PORT=${REDIS_PORT:-6379}
REDIS_PASSWORD=${REDIS_PASSWORD:-}
REDIS_HOST=${REDIS_HOST:-0.0.0.0}

RABBITMQ_USER=${RABBITMQ_USER:-guest}
RABBITMQ_PASSWORD=${RABBITMQ_PASSWORD:-guest}
RABBITMQ_AMQP_PORT=${RABBITMQ_AMQP_PORT:-5672}
RABBITMQ_MGMT_PORT=${RABBITMQ_MGMT_PORT:-15672}
RABBITMQ_HOST=${RABBITMQ_HOST:-0.0.0.0}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "命令缺失：$1，需要先安装。" >&2
    exit 1
  }
}

apt_install_if_missing() {
  local pkg="$1"
  dpkg -s "$pkg" >/dev/null 2>&1 || {
    sudo apt-get update -y
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y "$pkg"
  }
}

echo "[1/7] 安装必需包..."
apt_install_if_missing curl
apt_install_if_missing ca-certificates
apt_install_if_missing lsb-release
apt_install_if_missing software-properties-common
apt_install_if_missing jq || true

resolve_bind() {
  local host="$1"
  if [ "$host" = "localhost" ]; then
    echo "127.0.0.1"
  else
    echo "$host"
  fi
}

choose_check_host() {
  local bind="$1"
  case "$bind" in
    0.0.0.0|::|0.0.0.0/0)
      echo 127.0.0.1 ;;
    *)
      echo "$bind" ;;
  esac
}

MYSQL_BIND=$(resolve_bind "$MYSQL_HOST")
REDIS_BIND=$(resolve_bind "$REDIS_HOST")
RABBITMQ_BIND=$(resolve_bind "$RABBITMQ_HOST")

echo "[2/7] 安装并启用 MySQL..."
apt_install_if_missing mysql-server
sudo systemctl enable --now mysql

# 配置 MySQL 绑定地址
MYSQL_CNF="/etc/mysql/mysql.conf.d/mysqld.cnf"
if [ -f "$MYSQL_CNF" ]; then
  if grep -qE '^bind-address\s*=' "$MYSQL_CNF"; then
    sudo sed -ri "s/^bind-address\s*=.*/bind-address = $MYSQL_BIND/" "$MYSQL_CNF"
  elif ! grep -qE '^bind-address\s*=' "$MYSQL_CNF"; then
    # 在 [mysqld] 段后追加
    sudo sed -i "/^\\[mysqld\\]/a bind-address = $MYSQL_BIND" "$MYSQL_CNF" || echo -e "[mysqld]\nbind-address = $MYSQL_BIND" | sudo tee -a "$MYSQL_CNF" >/dev/null
  fi
fi
sudo systemctl restart mysql

# 配置数据库与用户（通过本地 socket 免密码登录 root）
echo "配置 MySQL 数据库与用户..."
MYSQL_SQL=""
MYSQL_SQL+="CREATE DATABASE IF NOT EXISTS \`$MYSQL_DB\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"
MYSQL_SQL+="CREATE USER IF NOT EXISTS '$MYSQL_USER'@'%' IDENTIFIED BY '$MYSQL_PASSWORD';\n"
MYSQL_SQL+="GRANT ALL PRIVILEGES ON \`$MYSQL_DB\`.* TO '$MYSQL_USER'@'%';\n"
MYSQL_SQL+="FLUSH PRIVILEGES;\n"

# 如提供了 MYSQL_ROOT_PASSWORD，则设置 root 密码并改为 mysql_native_password 以与 Compose 行为一致
if [ -n "$MYSQL_ROOT_PASSWORD" ]; then
  MYSQL_SQL+="ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '$MYSQL_ROOT_PASSWORD';\n"
fi

# 兼容多种 root 认证方式：
# 1) sudo 本地 socket（auth_socket）
# 2) Debian 维护账号（/etc/mysql/debian.cnf）
# 3) 明文密码（.env 提供 MYSQL_ROOT_PASSWORD，经 TCP 127.0.0.1:$MYSQL_PORT）
run_mysql_sql() {
  local sql_content="$1"
  # 1) sudo socket
  if echo -e "$sql_content" | sudo mysql >/dev/null 2>&1; then
    return 0
  fi
  # 2) debian-sys-maint
  if [ -f "/etc/mysql/debian.cnf" ]; then
    if echo -e "$sql_content" | sudo mysql --defaults-file=/etc/mysql/debian.cnf >/dev/null 2>&1; then
      return 0
    fi
  fi
  # 3) 指定 root 密码（仅当提供）
  if [ -n "$MYSQL_ROOT_PASSWORD" ]; then
    if echo -e "$sql_content" | mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -h 127.0.0.1 -P "$MYSQL_PORT" >/dev/null 2>&1; then
      return 0
    fi
  fi
  return 1
}

if ! run_mysql_sql "$MYSQL_SQL"; then
  echo "MySQL 初始化执行失败，请检查日志：/var/log/mysql/" >&2
  echo "可选修复：" >&2
  echo "  1) 运行：sudo mysql -e 'select 1'（若失败说明非 socket 认证）" >&2
  echo "  2) 运行：sudo mysql --defaults-file=/etc/mysql/debian.cnf -e 'select 1'（Debian 维护账号）" >&2
  echo "  3) 在 .env 设置 MYSQL_ROOT_PASSWORD 后重跑脚本" >&2
  exit 1
fi

echo "[3/7] 安装并启用 Redis..."
apt_install_if_missing redis-server
sudo systemctl enable --now redis-server

# 配置 Redis 绑定地址与密码（可选）
REDIS_CNF="/etc/redis/redis.conf"
if [ -f "$REDIS_CNF" ]; then
  sudo sed -ri "s/^#?\s*bind\s+.*/bind $REDIS_BIND/" "$REDIS_CNF" || true
  sudo sed -ri 's/^#?\s*protected-mode\s+.*/protected-mode no/' "$REDIS_CNF" || true
  if [ -n "$REDIS_PASSWORD" ]; then
    if grep -qE '^#?\s*requirepass\s+' "$REDIS_CNF"; then
      sudo sed -ri "s/^#?\s*requirepass\s+.*/requirepass $REDIS_PASSWORD/" "$REDIS_CNF"
    else
      echo "requirepass $REDIS_PASSWORD" | sudo tee -a "$REDIS_CNF" >/dev/null
    fi
  fi
fi
sudo systemctl restart redis-server

echo "[4/7] 安装并启用 RabbitMQ..."
apt_install_if_missing rabbitmq-server
sudo systemctl enable --now rabbitmq-server

# 启用管理插件
sudo rabbitmq-plugins enable --quiet rabbitmq_management || true

# 配置 RabbitMQ 监听地址与端口（AMQP 与管理台）
RABBITMQ_CONF="/etc/rabbitmq/rabbitmq.conf"
sudo mkdir -p /etc/rabbitmq
if [ -f "$RABBITMQ_CONF" ]; then
  sudo sed -ri '/^listeners\.tcp\.default\s*=.*/d' "$RABBITMQ_CONF"
  sudo sed -ri '/^management\.listener\.ip\s*=.*/d' "$RABBITMQ_CONF"
  sudo sed -ri '/^management\.listener\.port\s*=.*/d' "$RABBITMQ_CONF"
fi
{
  echo "listeners.tcp.default = $RABBITMQ_BIND:$RABBITMQ_AMQP_PORT"
  echo "management.listener.ip   = $RABBITMQ_BIND"
  echo "management.listener.port = $RABBITMQ_MGMT_PORT"
} | sudo tee -a "$RABBITMQ_CONF" >/dev/null
sudo systemctl restart rabbitmq-server

# 创建/更新用户并授予权限（避免使用 guest 远程登录限制）
if [ -n "$RABBITMQ_USER" ] && [ -n "$RABBITMQ_PASSWORD" ]; then
  if sudo rabbitmqctl list_users | awk '{print $1}' | grep -qx "$RABBITMQ_USER"; then
    sudo rabbitmqctl change_password "$RABBITMQ_USER" "$RABBITMQ_PASSWORD" || true
  else
    sudo rabbitmqctl add_user "$RABBITMQ_USER" "$RABBITMQ_PASSWORD" || true
  fi
  sudo rabbitmqctl set_user_tags "$RABBITMQ_USER" administrator || true
  sudo rabbitmqctl set_permissions -p / "$RABBITMQ_USER" ".*" ".*" ".*" || true
fi

echo "[5/7] 监听与端口信息（来自 .env）"
echo "- MySQL 绑定: $MYSQL_BIND  端口: $MYSQL_PORT (服务默认 3306)"
echo "- Redis  绑定: $REDIS_BIND  端口: $REDIS_PORT (服务默认 6379)"
echo "- Rabbit AMQP 绑定: $RABBITMQ_BIND  端口: $RABBITMQ_AMQP_PORT"
echo "- Rabbit 管理  绑定: $RABBITMQ_BIND  端口: $RABBITMQ_MGMT_PORT"

# 注意：原生安装下，服务端口不受 .env 影响（不是容器端口映射）。如需变更端口，应分别修改服务配置文件。

echo "[6/7] 防火墙（UFW）放通（若启用）..."
if command -v ufw >/dev/null 2>&1; then
  if sudo ufw status | grep -q "Status: active"; then
    sudo ufw allow "$MYSQL_PORT"/tcp || true
    sudo ufw allow "$REDIS_PORT"/tcp || true
    sudo ufw allow "$RABBITMQ_AMQP_PORT"/tcp || true
    sudo ufw allow "$RABBITMQ_MGMT_PORT"/tcp || true
  else
    echo "UFW 未启用，跳过放行。"
  fi
else
  echo "未安装 UFW，跳过放行。"
fi

echo "[7/7] 自检..."
VM_IP=$(hostname -I | awk '{print $1}')
MYSQL_CHECK_HOST=$(choose_check_host "$MYSQL_BIND")
REDIS_CHECK_HOST=$(choose_check_host "$REDIS_BIND")
RABBITMQ_CHECK_HOST=$(choose_check_host "$RABBITMQ_BIND")

echo "- MySQL 连通性自检..."
if command -v mysql >/dev/null 2>&1; then
  mysql -h "$MYSQL_CHECK_HOST" -P "$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "select 1" "$MYSQL_DB" >/dev/null && echo "  OK: 本机连接通过" || echo "  WARN: 本机连接失败（可能客户端未安装或凭据不匹配）"
else
  echo "  跳过：本机未安装 mysql 客户端"
fi

echo "- Redis 连通性自检..."
if command -v redis-cli >/dev/null 2>&1; then
  if [ -n "$REDIS_PASSWORD" ]; then
    redis-cli -h "$REDIS_CHECK_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" ping | grep -qi PONG && echo "  OK: 本机连接通过" || echo "  WARN: 本机连接失败"
  else
    redis-cli -h "$REDIS_CHECK_HOST" -p "$REDIS_PORT" ping | grep -qi PONG && echo "  OK: 本机连接通过" || echo "  WARN: 本机连接失败"
  fi
else
  echo "  跳过：本机未安装 redis-cli"
fi

echo "- RabbitMQ 管理台自检..."
if command -v curl >/dev/null 2>&1; then
  if curl -fsS "http://$RABBITMQ_CHECK_HOST:$RABBITMQ_MGMT_PORT" >/dev/null; then
    echo "  OK: 管理台可访问"
  else
    echo "  WARN: 管理台不可访问（服务未就绪或端口被防火墙阻拦）"
  fi
fi

cat <<EOF

完成！请在 Windows 中通过以下地址访问（确保 Windows 与该 Ubuntu 互通）：

- MySQL:   mysql -h $VM_IP -P $MYSQL_PORT -u$MYSQL_USER -p$MYSQL_PASSWORD $MYSQL_DB
- Redis:   redis-cli -h $VM_IP -p $REDIS_PORT${REDIS_PASSWORD:+ -a $REDIS_PASSWORD} ping
- RabbitMQ 管理台:  http://$VM_IP:$RABBITMQ_MGMT_PORT  （登录：$RABBITMQ_USER / $RABBITMQ_PASSWORD）

安全提示：当前监听地址（来自 .env）：MySQL=$MYSQL_BIND，Redis=$REDIS_BIND，RabbitMQ=$RABBITMQ_BIND。
如绑定为 127.0.0.1/localhost，则仅本机可访问；如绑定 0.0.0.0，请务必使用强口令并在 UFW 限制来源网段。
EOF


