# 生产上线检查清单

在将 KiwiGuard 部署到生产 LLM 流量链路之前，请使用本清单。

## 运行时与网络

- 将 Gateway、Control API、Console、worker、PostgreSQL 和 ClickHouse 部署在批准的网络区域。
- 仅允许批准的业务应用访问 Gateway。
- 仅允许管理员访问 Control API 和 Console。
- 在批准的负载均衡器、Ingress、代理或服务网格上终止 TLS。
- 显式配置 request、upstream、verdict 和 shutdown timeout。

## 认证与授权

- 将 Control API 和 Console 放在企业认证之后。
- 按角色限制管理操作。
- 轮换 gateway client 凭据。
- 不要把上游模型 Provider key 直接交给业务应用团队。

## Secret 管理

- 在企业 secret manager 或运行时 secret mount 中保存上游凭据。
- 在 KiwiGuard 配置中使用 `credential_ref`。
- 不要在 PostgreSQL 行、Git、issue、日志或截图中保存原始 Provider API key。

## 配置存储

- 使用具备备份、监控和网络访问控制的 PostgreSQL。
- 在受控发布窗口执行 migration。
- 在低环境验证配置变更后再激活到生产。
- 为 policy 和 routing revision 保留回滚路径。

## 流量与安全日志

- 按预期 LLM 流量容量规划 ClickHouse。
- 为请求和响应镜像载荷定义保留周期。
- 在隐私政策不允许保存载荷时关闭 raw capture。
- 验证 ClickHouse 故障期间 event sink health 和 durable spool 行为。
- 如需对接 SIEM 或企业数据平台，提前定义下游导出方式。

## Policies 和 Detectors

- 对业务关键 route，先以 monitor 模式验证规则，再启用 block。
- 使用合成数据验证 PII 风格正则表达式。
- 在大范围上线前复盘误报和漏报。
- 对 policy 变更做版本管理，并记录审批责任人。

## Verdict Provider

- 配置明确的 verdict timeout。
- 按 route 和风险等级定义 fail-open 或 fail-closed。
- 监控 verdict latency 和 error rate。
- 生产上线前测试 verdict provider 故障场景。

## 观测

- 从 Gateway 和 Control API 服务抓取 `/metrics`。
- 配置 tracer provider 后转发 OpenTelemetry trace。
- 对 gateway error rate、upstream failure、verdict failure、event sink failure 和 spool overflow 设置告警。
- 建立流量规模、policy decision、延迟和存储健康的运维仪表盘。

## 隐私与合规

- 判断 prompt 和 response 是否可能包含受监管数据。
- 让 raw capture 和 retention 符合隐私、法律和合同要求。
- 测试、演示、截图和公开 issue 中只能使用合成 prompt 和 response。
- 为被拦截流量和疑似数据暴露明确 incident response 责任。

## 发布准备

- 运行 `make verify`。
- 运行 `make docker-config`。
- 运行 `make docker-production-config`。
- 使用 `make build-go` 构建后端二进制。
- 使用 `make build-web` 构建 Web Console。
- 通过 Gateway 发送一条合成请求做 smoke test。
