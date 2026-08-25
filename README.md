# 舞台吊挂开演安全放行台

本项目面向舞台机械主管、吊挂操作员和独立安全复核员，把一次演出的吊挂配置从建档一直推进到不可变放行凭据。系统在同一档案中记录吊点与悬挂设备、计算载荷利用率、执行相互独立的双人检查、跟踪问题整改和复核关闭、保存分阶段有界试吊，并在全部门禁通过后冻结清单、签发可核验凭据。

服务由 Go 标准库实现，同时提供同源 JSON API 和原生 HTML/CSS/JavaScript 工作台，不需要 Node 构建链。业务事实写入本地长度前缀事件日志；事件帧使用递增序号、前序摘要和校验和形成可验证链，查询投影通过同步临时文件后原子替换。启动会校验快照和完整事件链，截断或篡改会明确拒绝启动。

工作台侧栏提供临演放行风险看板，可按状态、场地、风险等级和演出时间筛选，并汇总待放行数量、门禁原因、缺失检查角色、最近试吊结论和下一项待办。载荷配置采用“预检差异—查看失效影响—确认提交”流程，预检摘要同时绑定档案版本与候选配置。问题整改保留递增轮次及每轮通过或驳回决定；试吊则先由主管为当前配置确认四阶段持续时间、挠度、回差与总时限，失败会在同一事务中生成关联阻断问题，完成独立复核后才能全阶段补测。

## 状态流程

档案依次经过 `draft`、`inspection`、`remediation`（发现问题时）、`trial_ready`、`freeze_ready`、`frozen` 和 `released`。所有写请求均携带 `expectedVersion` 与 `idempotencyKey`：版本不一致会返回可恢复的冲突，同一幂等键用于不同请求会被拒绝。冻结后载荷、检查、问题和试吊数据均不可修改。

`GET /api/v1/cases` 支持 `status`、`venue`、`riskLevel`、`performanceFrom` 和 `performanceTo` 查询参数，时间使用 RFC3339。配置预检使用 `POST /api/v1/cases/{caseID}/configuration/preflight`，问题轮次复核使用 `POST /api/v1/cases/{caseID}/findings/{findingID}/review`，试吊标准确认使用 `PUT /api/v1/cases/{caseID}/trial-standard`。

## 构建

```text
go build ./cmd/rigging-clearance
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`，数据写入 `./data`：

```text
go run ./cmd/rigging-clearance
```

可以显式指定安全监听地址和数据目录：

```text
go run ./cmd/rigging-clearance -addr=127.0.0.1:19181 -data-dir=./data
```

未显式提供 `-addr` 时，也可把 `PORT` 设置为端口号，服务会绑定 `127.0.0.1:<PORT>`。显式 `-addr` 的优先级更高。服务拒绝缺少主机的地址和 `0.0.0.0`、`::` 通配地址。

打开 `http://127.0.0.1:19081/` 即可使用完整浏览器工作台。健康检查位于 `GET /healthz`，业务接口使用 `/api/v1` 前缀。

## 测试与自检

运行全部回归测试：

```text
go test ./...
```

以下命令会创建临时数据目录，在真实回环监听器上按 HTTP 链路完成建档、配置、双人检查、试吊、冻结、签发和凭据核验，并在限定时间内自行退出：

```text
go run ./cmd/rigging-clearance -addr=127.0.0.1:19081 -selfcheck
```

正常运行时可使用 `Ctrl+C` 或发送 `SIGTERM`，服务会在超时范围内优雅关闭。
