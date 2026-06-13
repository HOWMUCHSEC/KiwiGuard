# Go 工程规范

KiwiGuard 位于 LLM 应用的安全链路中。后端修改必须简单、可审查、可观测，并且在故障场景下保持安全。

## 语言与工具

- 使用 Go 1.25 或更高版本。
- 使用 `gofmt` 和 `goimports` 保持格式一致。
- 开发中运行聚焦测试，提交评审前运行 `make verify`。
- 保持 package API 小而明确。

## 包设计

- domain logic 不依赖 HTTP、SQL、CLI 或框架细节。
- 编排逻辑放在 application service 或 use case 中。
- adapter 负责协议或持久化转换。
- composition 放在 bootstrap package 中。
- 除非有明确的 shared infrastructure 理由，不新增跨 context 依赖。

## 错误处理

- request、storage、worker 路径返回 error，不使用 panic。
- error 字符串使用小写，不加句号。
- 传播 error 时使用 `%w` 包装。
- 保留足够上下文，帮助运维定位失败边界。

## 并发

- goroutine 尽量绑定 `context.Context` 取消。
- 生产路径使用有界队列和明确 backpressure。
- 避免在 hot path 中无界创建 goroutine。
- 当 wait group 用于启动 goroutine 时，使用 `sync.WaitGroup.Go`。

## 安全与隐私

- 不记录原始 secret。
- 不在配置行中保存上游 API key。
- 测试中使用合成 prompt、response 和 credential。
- 将请求和响应载荷处理视为隐私敏感行为。

## 测试

- 行为变更需要补充测试。
- policy、detector、routing 和 configuration 行为优先使用 table-driven tests。
- storage、gateway 和 event pipeline 代码需要覆盖失败路径。
- benchmark 聚焦 hot path 行为。

## 注释

- 为导出的 Go symbol 编写注释。
- 当注释解释 intent、invariant、并发、隐私或安全行为时才添加。
- 避免重复代码含义的注释。

## 验证

常用命令：

```bash
make test-go
make test-go-race
make test-go-cover
make bench-go
make lint-go
make vuln-go
make tidy-check
make build-go
make verify
```
