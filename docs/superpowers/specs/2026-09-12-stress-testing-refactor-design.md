# Stress-Testing 重构设计（方案 A）

日期: 2026-09-12  
状态: 待用户确认后进入实现计划

## 目标

把现有 Go HTTP 压测框架演进为**通用、可配置的 HTTP 压测工具**：用 YAML/JSON + CLI 启动，支持固定请求量与按时长压测，SSH 系统监控保持可选，报告与指标更完善。密钥不再写死在代码里。

## 非目标（本期不做）

- 完整多步骤业务编排 DSL（登录后串业务接口可作为后续扩展）
- GUI / Web 控制台
- 分布式压测集群
- 完全替换为第三方引擎（vegeta/k6）薄封装

## 约束与现状

- 语言: Go 1.23，模块名 `stress-testing`
- 现有结构: `framework/`（engine / monitor / reporter / types）、`scenarios/login_scenario.go`、`main.go` / `quick_main.go`
- 本地路径: `/Users/mm/document/workspace/Stress-Testing`（当前无 git remote）
- 已确认偏好: 通用 HTTP；YAML/JSON + CLI；固定请求量 + duration；SSH 监控可选（默认关）

## 架构

```
stress-testing
├── cmd/stress-testing/main.go     # CLI 入口
├── internal/config/               # 加载/校验 YAML|JSON，环境变量展开
├── internal/httpx/                # 通用 HTTP 客户端与请求构建
├── framework/                     # 保留并增强：Engine / Monitor / Reporter / Stats
├── scenarios/
│   └── http_scenario.go           # 通用 HTTP 场景（主场景）
├── examples/                      # 示例配置（含原登录场景迁移样例）
├── reports/                       # 输出目录（运行时）
└── docs/superpowers/specs/        # 本设计文档
```

保留可插拔 `Scenario` / `Stage` 接口；默认运行路径走配置驱动的通用 HTTP 场景。原登录逻辑以 `examples/login.yaml` 体现，不再依赖改 `main.go` 才能压测。

## CLI

```bash
stress-testing run -c config.yaml
stress-testing run -c config.json
stress-testing validate -c config.yaml   # 只校验配置
stress-testing version
```

关键 flag（可覆盖配置文件同名字段）：

- `-c/--config`（必填，对 `run`/`validate`）
- `--concurrency`、`--requests`、`--duration`
- `--out` 报告输出目录（默认 `reports/`）

## 配置模型（YAML 示意）

```yaml
name: api-smoke
target:
  base_url: https://example.com
  path: /api/v1/ping
  method: GET
  headers:
    Content-Type: application/json
    Authorization: "Bearer ${TOKEN}"   # 环境变量展开
  body: ""                              # 或 JSON 字符串 / 文件引用
  timeout: 10s
  insecure_skip_verify: false

load:
  mode: requests          # requests | duration
  requests: 1000          # mode=requests 时有效
  duration: 30s           # mode=duration 时有效
  concurrency: 20
  # rate: 100             # 可选：目标 RPS（有则限速）

monitor:                  # 可选，默认关闭
  enabled: false
  host: ""
  username: ""
  password: "${SSH_PASSWORD}"
  private_key: ""
  interval: 2s
  metrics: [cpu, memory, disk, network]

report:
  formats: [html, json, csv]
  dir: reports
```

密钥与敏感字段只允许来自环境变量或外部 secrets 引用，示例与文档明确禁止把真实密码提交进仓库。

## 引擎与指标

- 支持 `requests`：发完 N 次结束；`duration`：到达时长停止接新任务并 drain。
- 并发：信号量 / worker pool；可选 RPS 限速。
- 统计增强：成功率、QPS、min/avg/max，以及 **P50 / P90 / P95 / P99**；按错误类型/HTTP 状态粗分类。
- 运行中打印简易进度（已完成数、当前成功率、近似 QPS）。

## 监控与报告

- SSH Monitor：默认 `enabled: false`；开启时沿用现有采集能力，配置来自 YAML。
- Reporter：继续 HTML / JSON / CSV；HTML 图表纳入分位数与错误分布；系统监控图在启用监控时输出。

## 迁移策略

1. 抽出 CLI 与 config，替换硬编码 `main.go` 配置。
2. 实现通用 HTTP scenario；登录能力用示例配置覆盖。
3. 增强 stats / reporter。
4. 监控改为配置驱动可选。
5. 更新 README；`quick_main.go` 删除或改为薄封装调用 `run`。
6. 提供 `examples/*.yaml`。

## 验收标准

- `go build ./...` 通过；`stress-testing validate -c examples/...` 通过。
- 可用示例配置完成一次本地对公网或 mock 的压测（requests 与 duration 各至少一种）。
- 报告含分位数；未配置 monitor 时不尝试 SSH。
- 仓库内无明文真实密码/密钥；原 `main.go` 硬编码凭据移除。

## 实现备注

- 无远程仓库时，落地前需先建立可被 Cursor Cloud Agent 访问的仓库（推送现有代码或新建远程），或约定本机直接改代码的路径。
- 实现按实现计划分步提交，避免一次性大爆炸改动。
