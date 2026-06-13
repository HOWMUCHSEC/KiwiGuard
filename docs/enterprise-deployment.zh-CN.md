# 企业部署指南

KiwiGuard 的推荐部署方式，是作为 OpenAI-compatible 内容安全网关部署在企业 LLM 流量前方。

## 推荐拓扑

```text
企业业务应用
  -> KiwiGuard Gateway
  -> 上游 OpenAI-compatible LLM Provider
```

管理和观测组件：

```text
KiwiGuard Control API 和 Console
  -> PostgreSQL 配置存储
  -> ClickHouse 流量与安全事件存储
  -> Prometheus 指标
  -> OpenTelemetry Trace
```

## 接入模式

将业务应用中的 OpenAI-compatible client 指向 KiwiGuard Gateway，而不是直接访问上游 Provider。在尽量保持业务请求结构不变的前提下，通过 KiwiGuard 配置把流量路由到批准的上游模型 Provider。

典型企业流程：

1. 注册上游 Provider 配置。
2. 配置模型流量的 route mapping。
3. 配置 gateway client 和 client limit。
4. 配置 detector 规则，包括 PII 风格正则。
5. 配置 allow、monitor、block 等 policy action。
6. 配置垂直安全模型 Verdict Provider。
7. 按保留策略启用流量和安全事件采集。
8. 在 Console 和下游观测系统中监控活动与事件。

## 配置存储

PostgreSQL 是标准配置存储。企业应使用受管、可备份、网络访问受限的 PostgreSQL。

配置数据包括：

- routes 和 model mappings
- 上游 Provider 元数据和 credential references
- detector 和 policy 配置
- verdict provider 配置
- client limits
- observability 和 capture 设置

请保存 credential reference，而不是原始 API key。支持的 reference 模式包括环境变量和文件引用。真实 secret 应保存在企业 secret manager 或运行时环境中。

## 流量与安全事件存储

ClickHouse 是高吞吐流量和安全日志的一等事件存储。

采集记录可用于：

- 请求和响应审计
- detector 和 rule 命中分析
- policy action 复盘
- 安全事件排查
- 延迟和上游健康分析
- 对接 SIEM 或企业数据平台

保留策略应符合企业制度、隐私要求和区域数据保护义务。

## Verdict Provider

KiwiGuard 可以将请求和响应送入垂直安全模型 Verdict Provider。推荐基线是：即使本地 detector 规则没有命中，也继续执行 verdict 评估，以便网关执行专用模型安全检查。

生产环境中的 verdict provider 必须配置明确的 timeout、fallback 和故障模式。安全关键场景建议在 verdict 服务不可用时采用 fail-closed，除非企业风险策略另有规定。

## Console 使用

Console 可用于操作常见工作流：

- 查看最近流量事件
- 在 capture 启用时检查请求和响应镜像载荷
- 测试正则表达式
- 更新 policy 和 routing 配置
- 查看 storage 和 retention 设置
- 检查 policy decision、detector hit 和 upstream status

生产环境前，必须把 Console 和 Control API 放在企业认证、授权和网络控制之后。

## 观测

Gateway 和 Control 服务在 `/metrics` 暴露 Prometheus-compatible 指标。运行时安装 tracer provider 后，也可以输出 OpenTelemetry trace。

推荐生产仪表盘：

- gateway 请求速率、延迟和错误率
- upstream 状态和延迟
- detector 延迟和命中率
- verdict provider 延迟和失败率
- block 与 monitor 请求速率
- event sink batch 成功率、失败率、spool depth 和 overflow

## 生产准备

生产上线前，请完成 `docs/production-checklist.zh-CN.md`。
