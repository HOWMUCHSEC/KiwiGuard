# 本地快速试用

本文帮助安全团队和平台团队在本地启动 KiwiGuard，发送一条示例 LLM 请求，并确认流量和安全事件已经被采集。

## 前置条件

- Go 1.25 或更高版本
- 支持 Compose 的 Docker
- Node.js 22 或更高版本
- pnpm 10 或更高版本

## 启动评估环境

复制示例环境文件：

```bash
cp .env.example .env
```

安装前端依赖：

```bash
pnpm -C web install
```

启动 PostgreSQL、ClickHouse、Mock LLM API、KiwiGuard 服务和 Web Console：

```bash
make dev-env
```

默认本地地址：

| 服务 | 地址 |
| --- | --- |
| Gateway | `http://127.0.0.1:18080` |
| Control API | `http://127.0.0.1:18081` |
| Mock LLM API | `http://127.0.0.1:18082` |
| Console | `http://127.0.0.1:5173` |

## 发送示例请求

在另一个终端中，通过 KiwiGuard 发送一条合成客户端请求：

```bash
make dev-client-smoke
```

该脚本会发送 OpenAI-compatible 流量，并检查 ClickHouse 是否收到结构化事件元数据。

## 查看结果

打开 Console，重点查看：

- 流量事件
- 请求和响应镜像字段
- detector 命中
- policy action
- gateway 和 upstream 状态
- 延迟字段
- 存储和保留策略状态

运行时服务暴露 Prometheus 指标：

```bash
curl http://127.0.0.1:18080/metrics
curl http://127.0.0.1:18081/metrics
```

## 停止环境

停止本地存储服务：

```bash
make dev-env-stop
```

## 接入真实 OpenAI-compatible Provider

如需对真实上游 Provider 做集成 smoke，请只在本地 shell 中导出凭据：

```bash
KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make dev-env
```

然后执行：

```bash
KIWIGUARD_BETA_OPENAI_API_KEY=sk-... \
KIWIGUARD_BETA_OPENAI_BASE_URL=https://api.openai.com \
KIWIGUARD_BETA_OPENAI_MODEL=gpt-4o-mini \
make beta-openai-smoke
```

本地 smoke 测试只能使用合成 prompt，不要使用客户数据。
