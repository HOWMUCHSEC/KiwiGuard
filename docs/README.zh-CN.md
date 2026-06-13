# KiwiGuard 文档

KiwiGuard 是面向企业 LLM 流量的 OpenAI-compatible 内容安全网关。它部署在业务应用和上游模型 Provider 之间，对输入和输出进行检测、策略评估、垂直安全模型判定、流量镜像和安全事件记录。

项目概览和本地开发命令请先阅读根目录 `README.md`。

## 语言

- English: `docs/README.md`
- 简体中文: `docs/README.zh-CN.md`

## 企业落地路径

1. 使用 `docs/quickstart.zh-CN.md` 启动本地评估环境。
2. 阅读 `docs/enterprise-deployment.zh-CN.md`，确认企业网关部署拓扑。
3. 使用 `docs/production-checklist.zh-CN.md` 准备生产上线控制项。
4. 修改后端代码前，阅读 `docs/go-standards.zh-CN.md`。

## 文档索引

| 主题 | English | 简体中文 |
| --- | --- | --- |
| 本地快速试用 | `docs/quickstart.md` | `docs/quickstart.zh-CN.md` |
| 企业部署 | `docs/enterprise-deployment.md` | `docs/enterprise-deployment.zh-CN.md` |
| 生产上线检查清单 | `docs/production-checklist.md` | `docs/production-checklist.zh-CN.md` |
| Go 工程规范 | `docs/go-standards.md` | `docs/go-standards.zh-CN.md` |

## 企业部署模型

默认部署模型是把 KiwiGuard 放在 LLM 调用链路中作为内容安全网关：

```text
企业业务应用
  -> KiwiGuard Gateway
  -> 上游 OpenAI-compatible LLM Provider
```

运维和管理服务与网关并行运行：

```text
KiwiGuard Control API 和 Console
  -> PostgreSQL 配置存储
  -> ClickHouse 流量与安全事件存储
  -> Prometheus 指标和 OpenTelemetry Trace
```

## 核心企业工作流

- 将 OpenAI-compatible chat 和 response 流量路由到 KiwiGuard。
- 使用规则和 PII 风格 detector 检测 prompt 与模型输出。
- 将请求和响应送入垂直安全模型 Verdict Provider。
- 在策略允许时镜像请求和响应载荷，用于审计工作流。
- 将结构化流量和安全事件写入 ClickHouse。
- 通过 Console 查看流量、检测命中、策略动作、网关状态、上游状态、延迟和保留策略。

## 安全提醒

不要在 issue、pull request、测试、截图或公开文档中放入真实密钥、生产 prompt、客户响应或私有部署细节。敏感报告请遵循 `SECURITY.md` 中的披露流程。
