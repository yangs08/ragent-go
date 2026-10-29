# 第 02 课：基于 Eino ADK 的工具型 Agent

- 创建：2026-10-28。
- 状态：本地实现与验收完成；真实供应商工具能力、学员操作与复盘待完成。
- 本课最终实现提交：`5893209`；TDD 检查点详见文末。
- 前置：第一课服务底座、本地协议验证及真实模型正常调用已完成。
- 关联：M0 的 ADK 兼容原型；C13 工具型 Agent 的起点，不代表完整 Agent 平台已交付。
- 实施位置：`/Users/admin/workspace/ragent-go`。

架构更新：第三课已将本课代码按职责重整；当前阅读路径与行为以下文及 [综合整改实践](03c-architecture-refactor.md) 为准，早期提交记录保留作为历史。

路径说明：命令中的 `/path/to/ragent-go` 请替换为自己的项目目录；本机环境记录保留历史实际路径。课程正式文档位于项目 `docs/lessons/`。

## 1. 本课目标

用一个只读模拟订单查询工具，跑通“模型请求调用 → 参数校验 → 执行 → 回传结果 → 继续决策 → 回答”。使用 Eino ADK 执行循环，学习应用如何管理工具边界、错误和事件。

本课先采用 CLI，方便直接观察协议。订单来自固定测试数据，不接真实订单或真实写操作；模拟身份不等于已经实现生产认证。后续再接 HTTP、权限、持久运行和审批。

## 2. 先看一个具体任务

用户：“查一下 A100 订单是什么状态。”

```text
1. 程序向模型提供问题与 get_order 工具定义。
2. 模型输出调用请求：get_order({"order_id":"A100"})，附带 tool_call_id。
3. 程序检查工具名、参数与资源访问范围，然后执行查询函数。
4. 程序回传结构化工具结果，关联同一个 tool_call_id。
5. 模型根据结果回答；若还需要信息，可以提出下一次调用。
```

模型没有直接调用 Go 函数的权限。它返回的是调用请求；注册表和执行框架决定调用哪个实现，应用规则决定是否允许执行。

本课订单状态采用确定的业务枚举；不在模型回答中凭空补充可退货资格。退货资格需要政策和业务规则，后续再加入。

## 3. 三个必须理解的概念

### Tool Schema

Schema 告诉模型工具名称、用途、参数结构和必填项。拟定工具：

```json
{
  "name": "get_order",
  "description": "根据订单编号查询订单状态，只读操作",
  "parameters": {
    "type": "object",
    "properties": {
      "order_id": {"type": "string", "description": "订单编号"}
    },
    "required": ["order_id"],
    "additionalProperties": false
  }
}
```

这个 Schema 已通过 InferTool 实际生成并测试：order_id 为唯一必填字段，additionalProperties=false。Schema 帮助模型生成参数，服务端仍要验证空值、格式、未知字段和业务权限。

### Tool Call 与 Tool Result

一次 assistant 消息可以提出一个或多个调用。结果通过 tool_call_id 关联原调用，不能仅凭工具名称配对。相同工具可以被重复或并行调用，不同调用的结果不能混在一起。

### Agent 执行边界

工具调用失败不意味着无限重试。设置最大模型决策轮数、整次任务截止时间和工具执行限额。将“订单不存在”等业务结果与程序错误区分，取消信号单独向上传播。

## 4. Ragent 源码导读

参考提交：c3b61577；参考仓库：`/Users/admin/workspace/ragent`。

- `agent/.../config/ReActAgentProvider.java`：模型、工具、状态和中间件如何装配；执行循环由 AgentScope 提供。
- `agent/.../tool/KnowledgeSearchTool.java`：参数提取、空值验证、工具结果、错误与取消如何区分。
- `agent/.../tool/AgentToolCatalog.java`：工具定义与实际注册如何关联。

Go 版同样复用框架循环：用 Eino ADK 替代 AgentScope，应用维护可信工具、执行规则和业务数据。

## 5. Go / Eino 实施计划

1. 定义模拟订单仓储、请求/结果类型及查询工具；先写确定性测试。
2. 核对并使用固定版本中的工具辅助组件，检查实际生成的 Schema。
3. 使用 ADK ChatModelAgent 和 Runner 装配模型与工具；明确轮数上限。
4. 消费事件，区分模型输出、工具调用、工具结果与终止错误，不依赖控制台文本猜测是否执行。
5. 先用可控模型/本地协议夹具验证调用序列，再做一次真实模型工具能力联调。

本轮使用本地 Eino v0.9.22 的 `adk/chatmodel.go`、`adk/runner.go`、`components/tool/utils/invokable_func.go` 与 create_options.go 核对接口；已通过真实适配器和 ADK 连接本地模拟模型完成验证，真实供应商工具支持尚未验证。

## 6. 验收清单

| 场景 | 验收目标 | 状态 |
|---|---|---|
| 已知订单 | 实际执行工具并根据固定结果回答 | 本地验证通过 |
| 不存在的订单 | 返回明确业务结果，不虚构订单 | 本地验证通过 |
| 参数缺失/非法/未知字段 | 在业务执行前拒绝无效输入 | 本地验证通过 |
| 模拟主体访问不属于自己的订单 | 查询逻辑拒绝，模型不能通过改参数越权 | 本地验证通过 |
| 工具失败 | 结果可区分、错误不泄露内部信息 | 本地验证通过 |
| 调用与结果匹配 | 关联同一个 tool_call_id | 本地验证通过 |
| 超时、取消与轮数耗尽 | 能终止，没有无限工具循环 | 本地验证通过 |
| 真实供应商工具能力 | 正常触发工具且消费结果；单独记录版本和限制 | 待验证 |

## 7. 复盘题与小练习

- 模型返回一个 get_order 请求时，真正执行查询的是谁？
- 为什么 Tool Schema 声明了必填参数，Go 代码仍需要校验？
- 同一个 get_order 连续调用两次，结果靠什么关联？
- 如果模型绕过工具直接声称订单已完成，程序怎样发现缺少执行证据？

先画出本课的五步调用轨迹，再对照实现观察每个事件。代码和验收结果在实施时持续补入本文。

## 8. 当前进度

第一课真实模型普通文本调用成功。第二课工具、ADK 装配、运行预算、JSONL 事件和 CLI 已实现，本地测试/进程验证通过。第二课没有调用真实供应商，也没有修改本机真实 .env。


## 9. 实际新增的模块与阅读顺序

```text
cmd/agentcheck/main.go         配置、模型、工具装配和 CLI 输出
internal/agent/agent.go        Agent 定义、通用运行、事件与预算
internal/common/user.go  当前请求用户的 context 存取
internal/example/order.go     示例订单工具与本地数据
internal/config/agent.go      集中管理的执行配置
```

顺序阅读入口装配 → Agent 执行 → 示例工具 → 用户上下文。详见 [当前收敛实践](03d-agent-user-context.md)。

### 为什么身份不放在 Tool Schema 中

模型可以自行生成工具参数。如果工具允许模型指定 tenant_id 或 user_id，相当于把资源访问范围交给不可信输入。当前 CLI 注入开发用户 alice；工具仅接受 order_id。后续接入登录认证时，用已认证主体替换演示主体，继续维持这个边界。

当前 Agent 不绑定固定用户；工具集中注册，调用时从 context.Context 读取本轮用户。CLI 仍提供固定开发身份，尚未实现真实登录与服务端会话管理。

### 为什么用业务结果，而不把所有情况都当异常

- ok：订单查询成功，返回必要的 order_id/status。
- not_found：订单不存在或无权访问，统一公开结果以避免枚举其他人的订单。
- invalid_arguments：模型参数不合要求，返回安全信息，允许模型在预算内纠正参数。
- unavailable：存储暂不可用；不把原始数据库错误带回模型。
- 取消与截止时间：通过 error 向上传播，不能伪装成普通业务失败继续重试。

对接真实业务系统时仍需其服务端授权与一致性机制，演示工具不构成完整的生产权限系统。

## 10. 为什么复用 Eino 工具包装，还要定制 JSON

工具使用 utils.InferTool 推导 Schema 和包装 Go 函数，不自研工具框架。通过 WithUnmarshalArguments 指定标准库 Decoder，启用 DisallowUnknownFields 并检查后续是否还有 JSON 值；通过 WithMarshalOutput 使用标准库 Marshal。

因此我们自己的参数与结果路径使用 encoding/json；Eino 内部仍可能使用其 Sonic 依赖。保留当前 Go 版本和既有回退行为，不为消除警告修改框架内部实现。

## 11. Agent 本课采用的执行策略

### 完整步骤事件，暂不逐 Token 输出

Runner 设置 EnableStreaming=false，模型使用完整消息响应，AgentEvent 仍逐步骤产生。这便于先看清工具 Schema、参数、调用与结果的关系，避免同时引入工具参数流式分片的复杂性。第一课的流式模型验证命令继续保留。

当前输出是 CLI JSONL，不是已实现 HTTP SSE，也不保存到数据库，不支持断线恢复。

### 分别限制三种预算

| 配置 | 默认值 | 作用 |
|---|---|---|
| RAGENT_MODEL_TIMEOUT | 20s | 单次模型 HTTP 请求期限 |
| RAGENT_AGENT_TIMEOUT | 30s | 整次任务期限，包含所有模型和工具步骤 |
| RAGENT_AGENT_MAX_ITERATIONS | 4 | 模型决策轮数，允许 1..20 |
| RAGENT_AGENT_MAX_TOOL_CALLS | 3 | 每次运行的工具调用尝试预算，允许 1..20 |

工具限额独立于模型轮数，因为一轮模型响应可能提出多个工具调用。计数器放在每次 Run 中，使用原子操作处理可能的并行执行；参数错误的调用也消耗预算。当前不做自动模型重试或降级，避免无意叠加调用次数。

本课测试还发现并修正了共享模型构造器的超时缺口：只有外层 context 不足以表达复用客户端的单次请求策略，NewClient 现在也设置模型适配器的 HTTP Timeout；测试验证单次请求先于整轮期限终止。

### 工具执行证据与最终回答

当前 Agent 核对工具调用与结果关联，但允许无工具的澄清回答。早期版本的 ErrNoToolEvidence 已移除，不能把订单查询专用规则用于所有回答。订单 Profile 要求具体状态依据工具查询；是否忠实于结果仍需业务核验与效果评估，不能靠提示词保证。

## 12. 你现在怎么运行

现有 .env 的模型地址、名称和密钥会复用；没有新增必填配置，不需要把模板覆盖到真实 .env。

```sh
cd /path/to/ragent-go
make agentcheck
```

该命令会真实访问已配置模型，一轮任务通常包含多次模型请求，可能产生用量。模型必须支持兼容接口的工具调用；第一课普通文本成功不证明这一能力已支持。

默认任务是查询 A100。预期事件形状如下，call ID 与自然语言内容以实际模型为准：

```json
{"type":"tool_call","tool":"get_order","tool_call_id":"call-example"}
{"type":"tool_result","tool":"get_order","tool_call_id":"call-example","result":{"code":"ok","order":{"order_id":"A100","status":"shipped"}}}
{"type":"answer","content":"A100 订单已发货。"}
```

未校验的原始模型参数不会直接输出，避免未知字段中的敏感内容被打印。若模型发出多次调用，会出现多组调用/结果事件。

可通过 .env 的 RAGENT_AGENT_PROMPT 修改问题，或在本次命令临时覆盖：

```sh
RAGENT_AGENT_PROMPT="查询 A101 的订单状态，请调用工具" make agentcheck
RAGENT_AGENT_PROMPT="查询 B200 的订单状态，请调用工具" make agentcheck
```

第一条预期工具结果 delivered；第二条预期 not_found，而不是返回 bob 的订单。不要仅看最后一句自然语言，要检查 tool_result。

模拟数据：

| 编号 | 数据归属 | 当前主体可见的结果 |
|---|---|---|
| A100 | acme / alice | shipped |
| A101 | acme / alice | delivered |
| B200 | acme / bob | not_found |
| C300 | other / alice | not_found |
| Z999 | 不存在 | not_found |

`make run` 仍只启动 HTTP 健康检查，不会自动启动对话或 Agent。退出 Agent 使用 Ctrl+C；真实取消行为仍需你实际操作验证，本课自动化已覆盖取消与超时。

## 13. 不花模型用量的复现方式与验收证据

```sh
make test
make vet
make smoke
make verify-modules
```

make smoke 会在临时目录创建自己的 .env、本地模型协议服务与临时端口，实际运行 server、modelcheck、agentcheck 三个进程，不读取真实凭据。它核对 Agent 向模型发出的 tools、回传的 tool_call_id、实际订单结果以及最终事件序列。

2026-10-28 实测：

| 检查 | 结果 |
|---|---|
| 全量竞态测试 | 九个 Go 包通过 |
| 整体语句覆盖率 | 86.5%，包含 CLI main 未覆盖分支；不把进程冒烟计入此数字 |
| tooling 包 | 94.9% |
| agent 包 | 89.0% |
| model 包 | 95.5% |
| HTTP 包 | 100% |
| 静态检查 / 构建 / 模块校验 | 通过 |
| 三个命令进程冒烟 | 通过，包括 ADK → 工具 → 对应结果 → 回答 |
| 真实供应商工具支持 | 未在本轮调用，待验证 |
| HTTP Agent、持久化、MCP、写操作、人工确认 | 尚未实现，不属于本课已交付能力 |

已覆盖正常与不可访问订单、错误参数、存储故障脱敏、未知工具、模型不调用工具、空回答、模型轮数耗尽、工具预算耗尽、期限与取消、输出失败等场景。

## 14. 本地 Git 检查点

本课按测试先行和实现通过成对保存本地提交，没有推送远程：

| 测试先行 | 实现通过 | 内容 |
|---|---|---|
| 310f742 | f6727ee | 订单工具、Schema、参数与资源范围 |
| 859616c | d477e25 | ADK 循环、关联事件、执行证据与运行边界 |
| 4a9a72b | 5893209 | .env 驱动 CLI、预算配置、单次模型超时与进程冒烟 |

测试先行提交记录了预期失败；最终实现提交已通过全量验证。这些实现提交时文档尚在独立目录；现已迁入项目 docs/ 并共同版本管理。

## 15. 本课先做这三个练习

1. 阅读 NewOrderTool，指出租户过滤在哪里发生、用户归属在哪里检查。解释为何传入 user_id 的模型参数会被拒绝。
2. 执行 make agentcheck，观察 tool_call 和 tool_result 是否使用同一个 tool_call_id。
3. 查询 B200，对照工具结果解释：为什么程序没有返回它的状态，为什么这不靠 System Prompt 保证？

完成后继续补充真实模型联调结果。后续再建设运行/事件持久化、HTTP 接入和身份认证，按总体设计逐步走向完整平台。

接口依据：已锁定版本中的 [ADK ChatModelAgent](https://github.com/cloudwego/eino/blob/v0.9.22/adk/chatmodel.go)、[Runner](https://github.com/cloudwego/eino/blob/v0.9.22/adk/runner.go)、[工具辅助组件](https://github.com/cloudwego/eino/blob/v0.9.22/components/tool/utils/invokable_func.go)。本课具体保证以应用代码和测试为准，框架存在机制不等于应用已自动获得完整业务保障。

当前设计已移除提前引入的租户要求，C300 也按其他用户订单处理；早期多租户描述仅为历史方案，不能作为当前功能声明。见 03d。
