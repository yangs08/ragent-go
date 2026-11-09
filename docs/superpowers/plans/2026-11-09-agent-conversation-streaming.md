# 多轮 Agent 与增量输出 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.
>
> 本次用户已授权在当前会话继续实现，按现有项目约定原地执行；不分派子任务、不自动提交。执行依据以下清单。

**Goal:** 将多轮完整消息输入和增量输出落入现有订单 Agent，提供可运行 CLI。

**Architecture:** 保留 Run(ctx, question, emit) 的单轮完整事件行为。新增 RunMessages(ctx, messages, emit)，由调用方管理会话历史和可信身份；成功返回本轮完整消息，失败仅返回部分正文，不返回可追加历史。执行器使用 Runner.Run，流只读一次，同时累积消息和发出 text_delta；不添加自动重试、数据库会话或 HTTP SSE。

**Tech Stack:** Go 1.27.3、Eino v0.9.22、现有 OpenAI 适配器、本地协议替身。

---

## Task 1：执行器与关键行为验证

Files: internal/agent/agent.go、internal/agent/conversation_test.go。

- [x] 添加先失败的行为测试：完整工具历史进入下一轮；工具参数分片先合并再执行；正文分片先于流结束发出；流失败返回部分正文且无 Messages；消费失败/取消停止；非法未配对历史在模型调用前拒绝；不同用户历史由调用方隔离。
- [x] 执行 go test ./internal/agent，确认新 API 未实现时失败。
- [x] 新增公开契约：

```go
type RunResult struct {
    Messages []*schema.Message // 仅成功时返回本轮新消息，不含输入历史
    Answer string // 失败时可能为最后一条 assistant 的部分正文
}
func (a *Agent) RunMessages(ctx context.Context, messages []*schema.Message, emit EventSink) (*RunResult, error)
```

- [x] 共用执行路径：Run 保持非流式；RunMessages 启用流式。输入检查最终用户消息和工具配对，复制历史避免框架修改调用方数据。工具仍复用每轮预算与身份 context。
- [x] text_delta 包含 message_id；answer 是最终完整快照，消费者不能再次追加。失败不返回不完整的历史，不泄露原始工具参数或供应商错误。
- [x] 运行关键测试并修复失败，注释解释流消费、历史提交与消息配对。

## Task 2：真实 CLI 接入

Files: cmd/agentcheck/main.go、cmd/agentcheck/main_test.go、scripts/smoke.py。

- [x] 保留 runOrderCheck 和默认单轮 JSONL 行为；增加 -stream、-interactive。
- [x] 交互模式启动一次依赖，先执行 -prompt，再逐行读取后续问题，/exit 或 EOF 结束；仅成功轮次追加输入和 RunResult.Messages，历史不共享给其他用户。
- [x] stdin 行长度上限 64 KiB；Ctrl+C 取消当前调用并解除 stdin 等待。输出仍为 JSONL，不混入交互提示。
- [x] 本地 HTTP/SSE 替身验证交互第二轮携带完整历史、增量事件和最终快照；进程冒烟保留旧行为并覆盖新交互路径。

## Task 3：课程记录与验收

Files: docs/lessons/A01-agent-conversation-and-streaming.md、docs/README.md、docs/ROADMAP.md、docs/ARCHITECTURE.md、README.md。

- [x] 记录 Ragent 对照、Eino 版本、公开契约、可复现 CLI 命令、实际测试结果与未验证范围。
- [x] go test -race -timeout=60s ./...；go vet ./...；构建和 Python 本地进程冒烟；git diff --check、文档链接检查。
- [x] 不调用真实付费模型、不改 .env、不提交或推送；报告修改和验证结果，继续用项目代码教学。

## 执行结果

2026-11-09 已在项目落地。全量 race 在 stdin 取消修复前通过；修复后构建、vet、完整进程冒烟及文档检查通过。最终重复 race 因自动权限审核超时未执行。详见 docs/lessons/A01-agent-conversation-and-streaming.md。未调用真实供应商、未修改 .env、未提交或推送。
