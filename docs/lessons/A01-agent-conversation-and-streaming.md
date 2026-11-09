# A01：把多轮消息与增量输出接入订单 Agent

日期：2026-11-09。基线：9166f6d；本课实现尚未提交。状态：代码已接入，验收结果见第 6 节；学员独立修改尚未验证。

## 1. 学习问题与实际能力

让已有订单 Agent 支持先查询 A100，再追问“它发货了吗？”，并逐片输出 assistant 正文。学习重点是 Runner.Query 与 Run 的输入差异、完整协议消息与展示事件的区别，以及消息流失败时如何保留部分结果。

此前 /tmp 中的实验只是学习验证；本课代码正式位于项目 internal/agent 和 cmd/agentcheck，不依赖临时目录。暂不接 HTTP/SSE、数据库聊天记录、摘要、自动重试或审批。

## 2. Ragent 源码对照

参考本机 /Users/yang/workspace/ragent：

- agent/.../state/PgAgentStateStore.java：按用户与会话保存框架状态，不能用 UI 展示记录代替完整运行上下文。
- agent/.../service/handler/AgentStreamEventBridge.java：将框架执行事件映射为产品协议；Go 本课先保持 CLI JSONL，HTTP SSE 另行适配。
- agent/.../service/impl/AgentChatServiceImpl.java：会话归属、执行上下文与收尾属于应用职责。本课仅有一个 CLI 内存会话，尚未实现它的持久化能力。

Eino API 依据项目锁定 v0.9.22 的 adk/runner.go、adk/interface.go、schema/message.go。Runner.Query 包装单条用户消息；Runner.Run 接受完整消息切片；MessageVariant.GetMessage 在流模式下会合并整条流，不能依靠它逐片展示。

## 3. 公开接口与责任边界

旧接口保持原行为：

```go
err := assistant.Run(ctx, question, emit)
```

新增接口：

```go
result, err := assistant.RunMessages(ctx, messages, emit)
```

messages 是可信会话中的完整历史加本轮 user 消息，最后一条必须为非空 user；System 指令统一由服务端 Definition 提供。执行器检查历史角色与 tool call/result 配对，并深拷贝输入，避免 ADK 元数据改写调用方保存的数据。历史属主必须由调用方校验，执行器没有会话数据库，不能从文本推断归属。

RunResult.Messages 成功时只包含本轮新 assistant/tool 消息；调用方自行追加 user 和这些新消息。完整消息保留参数与工具结果，即使展示事件隐藏工具结果，下一轮仍能使用原始证据；它们不能直接暴露给普通日志或用户界面。

发生供应商错误、取消、超时或消费失败时，Messages 为空。Answer 可能保留最后一条 assistant 的部分正文，仅用于显示/故障记录，不能作为正常历史提交。这是本课简化策略，不是完整故障审计存储。

## 4. 流消费与事件

流只有一个读取者：在 Recv 循环里输出可见文本并累积片段，消息结束后用 schema.ConcatMessages 合并完整工具调用及结果。工具参数可能跨片，执行器不逐片解析/执行，真实调用仍由 ADK 调度。

| 事件 | 用途 |
|---|---|
| tool_call | 完整工具调用的展示事件，不输出原始参数 |
| tool_result | 结果展示，受 ExposeToolResults 控制 |
| text_delta | assistant 普通文本增量，含 message_id；也可能是调用工具前的说明 |
| answer | 最终完整答案快照；message_id 对应最后答案消息，不应再次追加到增量正文 |

同一消息的 text_delta 共用 message_id，不同 assistant 消息不同。不要把工具前的说明标成 reasoning，不要求模型提供内部思维链。answer 成功事件只在本轮执行和消息配对检查全部通过后发出。

EventSink 同步消费且可返回错误；返回错误会取消本轮并向调用者返回该错误。阻塞回调不能被 context 强制终止，未来网络适配器必须自行设置输出期限。本课未加入流式重试，因此不会透明重放已输出正文。

## 5. 实际运行

执行目录：项目根目录。复用已有配置，勿覆盖 .env。

```sh
# 旧单轮模式，保留 tool_call/tool_result/answer 协议
make agentcheck

# 单轮增量输出
go run ./cmd/agentcheck -stream -prompt "查询 A100"

# 先执行首问，再从 stdin 逐行读取后续问题
go run ./cmd/agentcheck -interactive -prompt "查询 A100"
```

交互启动后输入“它发货了吗？”并回车。空行跳过，/exit 或 EOF 退出。Ctrl+C 取消当前执行；stdin 在单独输入协程中读取，主循环同时监听取消，空闲等待也能退出。普通 Reader 可能无法中断阻塞读取，因此该协程只用于 CLI 进程生命周期，不用于 HTTP 请求。单行输入缓冲上限 64 KiB。

stdout 始终是 JSONL；text_delta 用于追加显示，answer 是核对和结算用的快照。以上真实命令会调用所配置的供应商，可能产生用量；本课自动化仅使用本地替身。

CLI 的 alice 是模拟用户。历史只存在于当前进程的局部变量，成功后追加，失败则退出；不同用户的真实 HTTP 会话隔离、持久化和继续失败会话尚未实现。当前不做历史窗口裁剪，长对话会增长，后续上下文专题处理。

## 6. 验收记录

测试文件：internal/agent/conversation_test.go、cmd/agentcheck/main_test.go；进程检查：scripts/smoke.py。

验收场景：

- 第一轮分片工具参数合并后调用，第二轮包含原工具调用、调用 ID、结果和回答。
- 首个正文片段到达即输出，不等待整条流结束。
- 流失败保留部分正文，不交出不完整历史，不泄露供应商内部错误。
- 消费失败和取消后无后续展示回调，生产者响应取消退出。
- 坏历史在调用模型前拒绝；同一 Agent 分别执行 alice、bob 请求，不共享历史或串工具身份。
- 工具前说明与最终回答使用不同消息标识；最终答案不拼入工具前说明。
- 本地 OpenAI 兼容 SSE 端到端交互、旧 CLI 协议、空闲 stdin 的 SIGINT 退出。

2026-11-09 实际执行结果：

| 检查 | 结果 |
|---|---|
| 初始新测试 | 新接口未实现时编译失败，确认测试覆盖新增契约 |
| Agent 与 CLI 本地测试 | 通过，包括完整历史、工具参数分片与增量 SSE |
| 全量 go test -race -timeout=60s ./... | 通过；该次在后述 stdin 修复之前执行 |
| 构建全部 cmd | 通过；stdin 修复后重新构建 agentcheck 通过 |
| go vet ./... | 通过，stdin 修复后已复验 |
| 修复后 python3 scripts/smoke.py | 全部通过，包括旧协议、多轮增量、工具历史、SIGINT 空闲退出 |
| git diff --check 与本地文档链接 | 通过 |
| stdin 修复后重复 race | 自动权限审核超时，未执行，不写成测试通过 |

进程实验发现关闭 os.Stdin 未能解除阻塞读取，已改为单输入协程加主循环 context 选择；修复后的同一进程冒烟通过。未调用真实供应商，未验收 HTTP 聊天或数据库会话。

## 7. 结合代码学习

阅读顺序：cmd/agentcheck/main.go 的 runTurn → Agent.RunMessages → 内部 run → consumeMessage → validateMessages。

本课的两个关键决定：展示事件不能作为历史来源；成功轮次才提交完整历史。复盘时直接对照第二轮实际发出的模型请求，解释其中每条 assistant/tool 消息从何而来，并修改一次工具输出或取消时机观察差异。代码完成不代表学员已掌握。

下一步：在这条真实执行路径上引入上下文治理或返回结构化证据的知识工具；HTTP/SSE 以已有契约为边界接入，不重复开发一套执行器。

## 8. 模型联调排障（2026-11-09）

出现通用 agent execution failed 时，不能直接判断为 Sonic、流式协议或预算问题。Sonic 在当前工具链的提示表示回退标准 JSON，已有本地测试也会出现该提示并正常通过。

本次检查工作区未发现 .env，configs/model.yaml 的模型名称与地址为空，Agent 进程也未设置模型环境变量。若运行终端另有环境变量，以那个终端的实际设置为准。首次联调先从 .env.example 创建本机 .env，再配置 RAGENT_MODEL_BASE_URL、RAGENT_MODEL_NAME、RAGENT_MODEL_API_KEY；不要将真实凭据提交或粘贴到聊天中。

执行器现在沿用 BusinessError 保留内部 SDK 原因和失败阶段，同时保持 ErrRunFailed 分类和对外安全文本。CLI 输出 operation、error_type，以及能识别时的 http_status，并提示配置来源；不打印 API Key 或供应商响应正文。401 认证失败的本地回归验证状态码可提取且响应正文不进入错误提示。真实供应商仍需在用户的有效配置下验证。
