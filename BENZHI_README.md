# BENZHI_README

基于 Go 实现的舞台吊挂开演安全放行台 Web 项目，一款后端服务，完整实现面向剧场舞台技术团队的吊挂开演安全放行台，以版本化事件档案贯通载荷登记、双人检查、问题整改、分阶段试吊、清单冻结、凭据签发与审计核验。

## 项目说明
- 项目：benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93
- 项目用途：完整实现面向剧场舞台技术团队的吊挂开演安全放行台，以版本化事件档案贯通载荷登记、双人检查、问题整改、分阶段试吊、清单冻结、凭据签发与审计核验。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/rigging-clearance -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93-arm64 linux/arm64
docker run -it benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/rigging-clearance -addr=127.0.0.1:19081 -selfcheck`
