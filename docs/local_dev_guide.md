#  个人测试环境部署指导
## 前置条件

- Docker（Windows 建议安装 Docker Desktop）
- Docker Compose（Docker Desktop 已内置；Linux 需安装 compose 插件, 参考[安装指导](./docker_compose_install.md)）
- Go 1.22+

## 直接部署到本地环境
### 启动
```bash
sudo bash scripts/linux/manual-deploy.sh
```
### 停止
- 仅停止：
```bash
sudo bash scripts/linux/manual-stop.sh
```
- 停止并禁用开机自启：
```bash
sudo bash scripts/linux/manual-stop.sh --disable
```
- 额外移除 UFW 放行规则：
```bash
sudo bash scripts/linux/manual-stop.sh --close-ports
```

## 使用docker compose 部署

### 目录位置

- Compose 文件：`deploy/dev/docker-compose.yml`
- 环境变量：仓库根目录创建 `.env`
- 脚本：`scripts/linux/`（bash）与 `scripts/windows/`（PowerShell）

### 配置 `.env`

在仓库根目录新建 `.env`（可根据需要调整端口/账号）：

```env
APP_PORT=8080
APP_ENV=dev

MYSQL_ROOT_PASSWORD=root
MYSQL_DB=spike
MYSQL_USER=spike
MYSQL_PASSWORD=spike
MYSQL_PORT=3306

REDIS_PORT=6379

RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest
RABBITMQ_AMQP_PORT=5672
RABBITMQ_MGMT_PORT=15672
```

说明：未在 `.env` 指定的变量，Compose 会使用 `deploy/dev/docker-compose.yml` 中的默认值。

### 启动/日志/停止（使用脚本）

Windows（PowerShell）：

```powershell
# 启动依赖
./scripts/windows/dev-up.ps1

# 查看日志（全部 / 指定服务）
./scripts/windows/dev-logs.ps1
./scripts/windows/dev-logs.ps1 mysql

# 停止并清理（含数据卷）
./scripts/windows/dev-down.ps1
```

Linux/macOS（bash）：

```bash
# 启动依赖
bash scripts/linux/dev-up.sh

# 查看日志（全部 / 指定服务）
bash scripts/linux/dev-logs.sh
bash scripts/linux/dev-logs.sh mysql

# 停止并清理（含数据卷）
bash scripts/linux/dev-down.sh
```

查看状态与健康：优先使用 Docker Desktop（Windows/macOS）或 `docker ps`；也可以通过日志脚本观察容器输出。

### 运行应用与验证

启动应用：

```bash
go run ./cmd/spike-server
```

健康检查：

```bash
curl -s http://localhost:8080/healthz
```

RabbitMQ 管理台：`http://localhost:15672`（默认账号密码 `guest/guest`）

可选连通性自检：

```bash
# MySQL（需要本地安装 mysql 客户端）
mysql -h127.0.0.1 -P3306 -uspike -pspike -e "select 1" spike

# Redis（需要本地安装 redis-cli）
redis-cli -p 6379 ping
```


### 常见问题

- 端口占用：修改 `.env` 中的端口或释放本机占用端口后重启。
- 数据重置：执行 `docker compose -f deploy/dev/docker-compose.yml down -v` 清理数据卷后再 `up`。
- Windows 网络：优先使用 PowerShell 或在 WSL 中执行同样命令；确保 Docker Desktop 正常运行。

- Windows 执行策略：若脚本无法执行，可在 PowerShell（管理员或当前用户范围）运行：

```powershell
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
```


