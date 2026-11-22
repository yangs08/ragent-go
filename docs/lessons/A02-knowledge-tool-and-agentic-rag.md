# A02 实践：知识检索工具与订单联合问答

日期：2026-11-09。基线：1a8aae4；本轮在其上新增知识工具。涉及 A02 工具契约，以及 A05/A06 的 Retriever 和 Agentic RAG 入门；不表示这些专题已全部完成或学员已掌握。

## 核心问题

用户问“查询 A100 的状态，并检索演示退货政策说明下一步”时，订单事实来自 get_order，政策依据来自 search_knowledge。模型决定何时调用哪个工具，ADK 执行并把结果放回消息历史，最后模型结合两类结果回答。这就是本轮要观察的 Agentic RAG 最小链路。

固定 RAG 通常由程序安排检索后生成；这里检索作为工具由模型选择，可以与订单查询交替执行。当前没有实现固定 RAG 对照实验，也不保证真实模型固定按“订单→知识”的顺序调用；测试替身只用确定顺序验证协议。

## 对照参考项目

本地 Ragent 提交 c3b6157730ac1f66a312bcd286c47d3ee6aaac42，阅读 rag/src/main/java/com/nageoffer/ai/ragent/rag/service/KnowledgeSearchFacade.java 的 search：改写问题、意图解析、歧义引导、检索，再由 LLMService 合成文本交给主 Agent。它不带会话历史，要求主 Agent 提供已消解指代的查询；内部来源锚点会在合成前清理。

Go 本轮选择较小边界：工具只检索和整理证据，主 Agent 生成一次最终回答。这样可以直接观察原文、来源与最终回答的关系，避免现在就引入工具内部模型调用、额外预算和二次生成误差。不是完整移植 Java 检索流水线。

Eino 接口依据项目锁定 v0.9.22 的 components/retriever/interface.go、option.go、schema/document.go 和 components/tool/utils/invokable_func.go；以本地依赖源码和编译为准。本轮没有升级依赖。

## 代码与消息链

- internal/example/knowledge.go：NewKnowledgeTool 接收 Eino retriever.Retriever，DemoKnowledgeRetriever 提供固定本地语料。
- cmd/agentcheck/main.go：同时注册 get_order 与 search_knowledge，服务端 Definition 说明证据、来源、演示性质和只读边界。
- internal/agent/agent.go：复用已有工具预算、context 身份、流消费和完整历史，无需增加第二套执行器。

```text
user：查询订单与政策
assistant：tool_calls(get_order, id=order)
tool：tool_call_id=order，A100=shipped
assistant：tool_calls(search_knowledge, id=knowledge)
tool：tool_call_id=knowledge，found + evidence
assistant：结合订单事实与政策证据回答，标出返回的 source_id
```

Retriever.Retrieve(ctx, query, opts...) 返回 []*schema.Document。工具显式传 WithTopK(3)，将 Document.ID、MetaData 中的 title/version、Content 转成来源证据。后端其他元数据不暴露给模型；输出最多三条，每条正文最多 1200 个 Unicode 字符，裁剪时 truncated=true。这个字符预算不等于模型 token 预算，真实后端仍需限制返回规模。

query 必须是非空且最多 256 字符的完整独立问题，JSON 参数最多 4096 字节，拒绝未知字段。模型不能传 user_id；身份从 context 读取并原样传给 Retriever。本地演示文档对所有已登录演示用户可见，尚未实现知识库 ACL；真实后端必须在检索阶段限制用户可见范围。

## 结果和失败边界

| code / error | 含义与处理 |
|---|---|
| found | 有可用证据；每条包含 source_id、title、version、content，必要时有 truncated |
| no_match | 本次没有命中，不能推断现实中不存在该政策 |
| invalid_arguments | 参数不合格，未调用检索后端；模型可在已有预算内修正 |
| unavailable | 后端失败或来源字段不完整；不返回部分证据，不向模型泄露内部错误 |
| error | 缺少可信身份或执行取消/超时；终止执行，不能伪装成没有命中 |

unavailable 当前只给安全结果，尚未增加后端错误分类与诊断观测；真实检索接入时补齐。取消在检索前后检查，防止后端取消后返回空列表被误判为 no_match。

固定关键词 Retriever 包含“演示售后政策”和“演示物流政策”，按命中关键词数排序，遵循 TopK/ScoreThreshold。它不做向量化、中文分词、混合召回、重排、文档入库或持久索引；固定数据用于观察工具机制。

订单只有状态，没有签收日期、拆封情况或物流轨迹。即使返回 shipped/delivered，也不能断言退款资格；本轮不执行申请或退款。来源 ID 是可信后端提供的公开演示标识，真实后端应输出可公开且可追溯的标识，避免暴露内部数据库主键。

提示词要求把证据当资料、按真实 source_id 引用；这不是程序化的答案忠实性或提示注入防护验收。后续需用失败样例检验证据是否支持结论，不能仅凭答案带引用就判正确。

## 运行与验收

在项目根目录复用已有模型配置，供应商需支持工具调用与流式响应：

```sh
go run ./cmd/agentcheck -stream -prompt "查询 A100 的状态，并检索演示退货政策说明下一步"
go run ./cmd/agentcheck -interactive -prompt "查询 A100"
# 交互后输入：按演示退货政策，下一步还需要确认什么？
```

真实命令调用配置的服务。可观察工具选择、结果 code、source_id 和最终回答；真实模型的工具顺序与次数可能不同。工具预算默认三次，不能无限重试。

自动验证只访问本机模拟供应商：internal/example/knowledge_test.go 验证参数、身份、取消、错误/空结果、来源白名单和输出预算；cmd/agentcheck/main_test.go 验证两个工具均注册、call/result 配对、订单事实与真实证据进入最终输入；scripts/smoke.py 验证编译后的进程联合路径，并保留旧单轮、多轮、分片和 SIGINT 场景。替身给出的正确答案不能证明真实模型的工具选择和回答质量。

2026-11-09 实际执行：全量 go test -race ./...、go vet ./...、构建和 scripts/smoke.py 均通过，本地 Markdown 链接与 diff 格式检查通过。执行细节见 [项目计划](../superpowers/plans/2026-11-09-agent-knowledge-tool.md)。尚未验证真实供应商联合问答、真实检索质量、知识权限和答案忠实性。下一次学习重点是沿这条消息链读清 Retriever 与 Tool 的职责，再用实际失败样例决定是否引入固定 RAG、向量召回或查询改写。
