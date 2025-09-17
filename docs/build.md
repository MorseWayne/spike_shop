
# 构建与运行

- 构建：
```bash
make build
# 生成 bin/spike-api
```
- 运行：
```bash
make run
# 或 ./bin/spike-api
```
- 测试与规范：
```bash
make test

make lint 

make tidy
```

如果你想用 `go build` 直接在根目录构建，也可以指定子包：
```bash
go build ./cmd/spike-api
```

# golang-lint安装
一键安装最新版（自动检测架构）
```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.bashrc
source ~/.bashrc
```