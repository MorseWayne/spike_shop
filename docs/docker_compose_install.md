# 安装基础依赖​
sudo apt-get update
sudo apt install docker.io
sudo apt-get install ca-certificates curl gnupg

# ​创建目录并添加 Docker 的官方 GPG 密钥​
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

# 将 Docker 的官方 APT 源添加到系统​
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 更新软件包列表并安装 docker-compose-plugin
sudo apt-get update
sudo apt-get install docker-compose-plugin