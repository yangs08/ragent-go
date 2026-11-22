# Agent 与 Eino 学习路线：以 Ragent 为参考实现 Go 企业项目

更新：2026-11-22。课程目标按用户本次明确的优先级修订：理解 Agent，掌握 Eino，通过参考 Ragent 的真实机制逐步形成 Go 企业项目。

## 1. 学习主线与项目载体

Agent / Eino 的机制与设计判断决定学习顺序；Go 项目用于承接实验和验证企业约束。原来的 40 节计划以功能交付顺序为主，导致数据库、认证、CRUD 和封装整改挤占核心学习，本次替换为专题路线，不预先固定课数。旧的开发顺序计划已清理，后续以本路线为准。

完整企业项目仍是长期实践目标；用户端、后台、入库、检索、权限和部署范围保留在 PROJECT.md，但不逐项变成核心课程，也不把已有代码等同于学员已掌握。每个专题可根据理解情况拆成多次学习。

- 概念：这个机制解决什么问题，输入、输出、状态和失败边界分别是什么？
- 参考：Ragent 如何实现，哪些属于 AgentScope，哪些属于应用业务？
- 框架：Eino 提供什么，对照项目锁定版本源码理解实际契约。
- 实验：先预测轨迹，修改一个变量，观察执行证据，再解释差异。
- 沉淀：把理解后的能力接入现有 Go 项目，补充必要工程保障。

普通 Go/Gin/GORM 知识按需讲解，不反复作为主课。CLI、内存历史和固定文档可用于机制实验；真实身份隔离、事务、取消、审批绑定等约束随能力落地验证，不拖到最后。

## 2. 已有基础与学习证据

当前实现包含多轮流式 Agent、交互 CLI、演示知识工具，以及统一后的 HTTP/Service/DAO 和公共基础设施。验证状态分别记录在对应实践文档与计划中，不把代码完成等同学员掌握。

| 现有内容 | 可复用基础 | 尚需学习/验证 |
|---|---|---|
| 模型与订单 Agent | ChatModelAgent、Runner、InferTool、预算、取消、事件回调 | 能否独立解释完整消息序列、工具结果关联、停止条件；真实工具联调记录仍待补 |
| HTTP、数据库、认证 | 配置、日志、迁移、可信用户 context | 作为后续实验基础复用，不重讲整套后台开发 |
| API / SSE 契约 | 已定义路径与事件形状 | 当前无聊天 HTTP / SSE，不把契约当实现 |
| 历史课程 01–05 | 保存实现过程与当时验证证据 | 部分目录、状态为历史记录，当前架构以 ARCHITECTURE.md 和代码为准 |

2026-11-09 已把多轮与增量能力接入真实项目：保留非流式 Run，新增启用流式的 RunMessages；调用方传入完整历史并接收成功轮次的新消息。CLI 提供 -stream 和 -interactive。详见 [本轮实践](lessons/A01-agent-conversation-and-streaming.md)。尚未接入 ADK Handlers、数据库聊天历史或 HTTP/SSE。

## 3. 核心专题

A 编号表示新学习专题，与历史课程 01–05 不互相覆盖。顺序按概念依赖安排，可以回访，不表示每个专题只上一课。全部专题尚未验收学员掌握程度。

| 专题 | 要理解的问题与 Eino 重点 | Ragent 阅读锚点 | 关键实验与项目沉淀 |
|---|---|---|---|
| A01 Agent 执行循环 | LLM 调用、Tool Calling、Agent 有何区别；ChatModelAgent、Runner.Query/Run、Message、停止条件 | ReActAgentProvider、AgentToolCatalog | 手绘并核对 user → assistant(tool_calls) → tool → assistant；比较无工具、单次、多次调用及预算耗尽 |
| A02 工具与中间件 | Schema、InferTool、参数校验、业务结果与 error；Handlers 的模型/工具扩展点、执行顺序 | AgentToolCatalog、AgentToolBatchMiddleware | 修改工具描述/返回结果，观察选择与纠错；在合适的工具边界验证权限和预算，说明与 Gin 中间件的区别 |
| A03 事件流与可观测性 | AgentEvent、AsyncIterator、完整消息与增量流、callbacks 与事件消费的职责 | AgentStreamEventBridge、AgentSSEEventType | 先在 CLI 观察实际事件和时序，再适配已有 SSE 契约；验证中途失败和取消，不伪造 reasoning |
| A04 多轮上下文与记忆 | UI 消息、模型上下文、运行状态、检查点、长期记忆的区别；Runner.Run、Reduction、Summarization | PgAgentStateStore、AgentContextTrimmer、AgentContextCompactionMiddleware、AgentUserMemoryMiddleware | “查 A100”后追问“它现在什么状态”；比较无历史/完整历史/摘要；保护 tool call/result 配对，再持久化必要状态 |
| A05 Eino 组件与固定 RAG | ChatTemplate、Loader/Transformer、Embedding、Indexer、Retriever；Chain/Graph/Workflow 的选择依据 | StreamChatPipeline、IngestionEngine | 用少量固定文档打通检索→证据→生成；先分离检索失败与生成失败，再接真实索引和必要入库流程 |
| A06 Agentic RAG | Agent 何时决定检索、再次检索或结合业务工具；固定 RAG 与 Agent 的责任边界 | KnowledgeSearchTool、KnowledgeSearchFacade、MultiChannelRetrievalEngine | 同题比较固定流程与 Agent；订单+政策联合问答；按失败样例引入混合召回/RRF/Rerank，而非先堆组件 |
| A07 MCP 与工具治理 | 本地 Go Tool 与 MCP 的差别；发现、Schema、调用、连接和授权 | McpToolProxy、AgentToolCatalog | 用本地 MCP 演示服务替换一个工具；模拟超时、断开及工具目录变化；扩展模块版本单独验证 |
| A08 人工确认与恢复 | interrupt/resume、CheckPointStore；模型意图、用户批准与业务执行之间的关系 | agent/confirm、AgentConfirmDenialMiddleware、AgentChatServiceImpl | 独立测试数据中的申请操作：批准/拒绝/恢复、参数绑定与重复提交；先理解执行恢复，再接审批 API |
| A09 Skills 与上下文治理 | Skill 与 Prompt/Tool/MCP 的区别；按需加载、工具开放、摘要与长期事实 | SkillLoadTool、AgentSkillMaskingMiddleware、AgentMemoryLearner | 比较始终注入与按需加载；验证技能加载不等于授权、摘要后行为是否仍成立；记忆来源与撤销 |
| A10 质量与企业运行约束 | 正确性、调用轨迹、成本/延迟、重试与幂等、观测、并发取消；机制取舍 | AgentRunGate、ModelRoutingExecutor、resources/regression | 用前面累积的固定样例比较策略；注入模型/工具失败；验证隔离和取消，将有效能力整合进服务 |

A10 是集中复盘和补齐，不是第一次做评估。A01 起保留小型任务集、预期工具行为和失败样例；有真实调用授权与配置时才补真实供应商证据。

多 Agent、AgentAsTool、Sequential/Parallel/Loop、Plan-Execute、DeepAgents 作为后续进阶专题：先用同一任务与单 Agent 比较质量、上下文和成本，再决定是否融入项目。Ragent 未覆盖的机制，明确标注为 Eino 扩展学习，不虚构 Java 对应实现。

## 4. 当前实践：A01 执行循环、多轮与增量输出

不是重做第二课代码，而是补齐原课尚未证明的理解。

阅读范围只取三处：

1. cmd/agentcheck/main.go：模型、工具、Definition 和用户 context 在哪里装配？
2. internal/agent/agent.go：Runner 在哪里启动？何时调用工具、发事件、停止或报错？
3. internal/example/order.go：Schema 如何产生？模型参数如何到达 Go 函数？身份校验在哪里？

再定向对照 Ragent 的 ReActAgentProvider 和 AgentToolCatalog，以及 Eino v0.9.22 的 adk/chatmodel.go、adk/runner.go、compose/tools_node.go；不要求通读整个仓库。

下列为学习观察清单；本轮多轮、分片工具调用、流失败和取消的实际结果见课文，不把已通过的自动化等同学员已掌握：

- “查询 A100”：工具结果如何成为下一次模型调用的输入？
- “查询 A100 和 A101”：一个决策轮次能否提出多个调用？以实际轨迹为准，观察结果而不强求模型固定行为。
- 缺少订单编号：为什么可以直接澄清，而非强制调用工具？
- 参数无效、工具不可用、预算耗尽：哪些反馈回模型，哪些终止执行？
- 区分用户轮次、模型决策轮次、工具调用次数，解释三个预算各限制什么。

允许在小型实验中手写一个最小 Tool Calling 循环，与 ADK 对照职责；正式项目继续复用 ADK，不长期维护第二套执行器。观察的是可见消息与调用轨迹，不要求模型披露内部思维链。

学完的证据：能画出真实消息序列，指出“模型决定调用”和“程序执行工具”的代码位置，解释 call ID 配对及停止条件，并在不照抄答案的情况下修改一个边界。完成后再进入 A02；没有掌握就沿该问题继续，而不按文件数量或固定课数推进。

## 当前推进：知识工具与联合问答

已把 Eino Retriever 包装为 search_knowledge，与 get_order 注册到同一 Agent。这里跨 A02/A05/A06 做最小实践，先观察“模型选择工具→程序检索→证据回到模型”的关系；仅使用固定关键词演示语料，不表示固定 RAG、向量索引和完整 Agentic RAG 专题已完成。实际代码、失败契约与验证边界见 [知识工具实践](lessons/A02-knowledge-tool-and-agentic-rag.md)。

## 5. 教学方式

每次只聚焦一至两个核心问题。先提问和预测，再讲概念和读关键源码；给出必要的最小示例，由学员解释、修改或复现实验，最后把有价值的结果接回项目。

课程记录分开写“实现已有”“实验已执行”“学员能解释/修改”；后两者没有证据时保持待验证。工程补充只覆盖本专题需要的范围，不连续用课程讨论封装、目录命名或普通 CRUD。

企业目标体现在可验证的边界：工具授权不靠 Prompt、写操作有确认与幂等、状态可追溯、失败可定位、取消和资源退出有证据。平台页面与通用基础设施可以集中补齐，但这些直接影响 Agent 正确性的机制必须随专题学习。

## 6. 资料与版本依据

2026-11-02 查阅：

- [Eino ADK Quickstart](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/agent_quickstart/)：核心入口和学习导航。
- [ChatModelAgent 入门](https://www.cloudwego.io/docs/eino/overview/eino_adk_quickstart/)：决策、工具反馈、上下文与运行时增强的关系。
- [Middleware](https://www.cloudwego.io/docs/eino/core_modules/eino_adk/eino_adk_chatmodelagentmiddleware/)：生命周期扩展与执行顺序。
- [Eino 编排](https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/)：组件与编排层职责。
- [Eino Cookbook](https://www.cloudwego.io/docs/eino/cookbook/)：按专题挑选官方示例，不逐个照搬。
- [Building effective agents](https://www.anthropic.com/research/building-effective-agents)：2024 年文章，借鉴其固定 Workflow 与模型自主决策的区分及简单可组合的设计思路；不作为当前 SDK API 依据。

以上是课程设计参考，不是兼容性验证。本项目保持 Eino v0.9.22；本次核对本地该版本的 Runner.Query/Run、EnableStreaming、Handlers 及相关包存在性。具体接口以该 tag 源码和编译实验为准，不因官网新示例自动升级；MCP 等独立扩展需分别记录版本。2026-11-02 的资料核对未产生运行验证；2026-11-09 的实际代码与验证记录见本轮实践，真实供应商联调仍待验证。
