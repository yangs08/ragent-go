# 应用架构基线与综合整改

更新：2026-10-29。范围：当前已存在的代码；不把未来数据库、认证、聊天接口写成已实现能力。

## 基准

以 Ragent 的业务流程与能力分工为参考，Go + Eino 实现模块化单体。Ragent 中 AgentEngineConfiguration 装配模型与状态，ReActAgentProvider 组合 Agent，AgentToolCatalog 注册工具，KnowledgeSearchTool 委托业务 Facade；Go 对应职责，不逐类翻译 Java。

配置统一管理、启动层显式装配、身份按执行传入、业务权限在服务内检查。目录数量和测试覆盖率不是规范性依据。

## 综合检查与处理结果

| 原问题 | 本轮结果 |
|---|---|
| 环境配置在三个模块分别解析 | 已统一 Config.Server/Model/Agent，模块接收配置值 |
| OrderAgent 写死工具名、订单结果和提示词 | 已替换通用 Executor；Definition 和 ToolCatalog 由启动装配传入 |
| Agent 构造时捕获用户身份 | Request 每轮提供 Principal；Catalog 为本轮绑定新工具 |
| 工具、订单实体、授权、仓储、演示数据混合 | tooling 适配框架，order.Service 执行业务校验与授权，devdata 提供开发仓储 |
| 执行器负责 JSONL 编码 | 已移到 agentcheck；执行层输出类型化事件 |
| model.StreamText 创建客户端并写终端 | 改为使用已配置模型与文本回调，modelcheck 管理整轮期限及输出 |
| 模型回答长度写死在客户端 | 已移入 ModelConfig.MaxCompletionTokens，环境变量默认仍为 256 |
| 每个回答都强制要求调用订单工具 | 移除通用硬约束，允许澄清；订单提示词要求具体状态依据工具结果，尚无答案语义保证 |
| 工具结果事件强依赖订单结构 | 改为可选通用 JSON 载荷，默认不展示工具结果；开发订单 Profile 显式开启已脱敏结果 |
| HTTP 路由和访问日志代码混合 | 日志中间件独立；保留现有健康检查、恢复和服务退出行为 |
| 设计文档与代码、历史路径不一致 | 更新课文入口和接口基准；本课记录已实现与待办 |

## 当前实际调用链

```text
agentcheck
  → config.Load
  → bootstrap.NewDevelopmentAgent
      → model.NewClient
      → order.NewService(devdata.DemoOrders)
      → tooling.NewCatalog
      → agent.NewExecutor(model, catalog, definition, cfg.Agent)
  → executor.Run(Request{Principal, Question}, EventSink)
      → catalog.Resolve(Principal)
      → buildEngine(Eino ADK + 本轮工具与共享预算)
      → 工具适配 → order.Service → Repository
  → EventSink → JSONL
```

server 当前只有健康检查，与开发订单适配器没有自动连接。modelcheck 创建模型客户端、设置期限、消费文本回调并输出。

## 当前目录与依赖

```text
internal/config/          统一配置加载、默认值和校验
internal/bootstrap/       进程生命周期与显式依赖装配
internal/identity/        可信主体，不负责当前尚未实现的认证
internal/agent/           Agent 定义、Eino 装配、通用执行和事件
internal/tooling/         工具目录、Eino 工具适配
internal/order/           订单业务、授权、仓储接口
internal/devdata/         开发数据适配器，不在生产入口隐式启用
internal/model/           模型适配器与文本流能力
internal/transport/http/  Gin 路由与中间件
cmd/                     进程入口、诊断输入与输出
```

agent 不依赖 order、tooling 的具体实现或 devdata；ToolCatalog 接口定义在使用它的 agent 包。tooling 依赖 order 服务，order 依赖身份与自身 Repository 接口；devdata 实现 Repository。bootstrap 是装配层，可以依赖上述模块。

Eino 组件类型允许直接用于 AI 适配和编排边界，不额外造一套同构工具框架。订单查询业务不依赖 Eino、Gin 或启动配置。工具负责解析不可信模型 JSON，业务服务负责 ID 合法性与资源授权，两道边界职责不同。

## 生命周期和事件

共享：配置快照、模型客户端、无当前用户字段的 Executor/Catalog/业务服务、支持并发的仓储。
每轮：Principal、context、ADK 实例、工具实例、共享工具预算、调用关联表和事件回调。每轮重建 ADK 是当前隔离策略，不照搬 Ragent 的单例缓存；有实际测量和安全依据后再考虑缓存。

同步回调必须及时返回；事件消费失败触发返回和 context 取消，后续不再回调。回调阻塞无法由 context 强制打断，未来 SSE 适配必须设置写期限。工具次数预算在本轮所有工具间共享，不是每个工具分别获得额度。

工具错误脱敏与资源授权在工具/服务边界处理；模型输出仍不可信。是否展示工具结果由服务端 Definition 控制，当前只有已脱敏订单开发 Profile 开启。未来多个异构工具接入时应逐工具审查展示策略，不直接全量暴露 MCP 响应。业务服务返回的安全错误仍不代表模型回答可信。

## 下一阶段的应用链路（尚未实现）

Gin Handler → ChatService → Executor；ChatService 协调会话归属、历史消息、活动任务、同会话互斥和结算。SSE Adapter 将事件转换为 Ragent 协议。会话仓储、TaskRegistry 随真实需求落地，不预建空壳或全局 Service 容器。

第一版接口依第三课：GET /agent/v1/chat、POST /agent/v1/stop，首个 meta 带 conversationId/taskId；SSE 断线取消，无自动重连。当前仅有 /healthz，不能用诊断 CLI 冒充企业聊天服务。

## 待解决与验证边界

认证、持久会话、请求上下文历史、持久状态、应用服务、SSE 字段与收尾协议、主动停止、跨实例互斥、模型增量流及供应商兼容、审批、MCP 和实际业务适配仍按课程建设。Executor 当前接收单次问题，未声称支持多轮历史或模型 Token 流。

保留已有关键回归：工具授权、参数校验、取消/期限、预算、事件关联和输出失败。新增只聚焦身份隔离、通用工具能力与澄清行为；不为搬目录和简单传参新增镜像测试，也不为覆盖率数字补测试。

详见 [综合整改实践](lessons/03c-architecture-refactor.md)。
