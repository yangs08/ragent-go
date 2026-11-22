# 结构化知识工具与 Agent 联合问答 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.
>
> 用户已授权继续在项目实现，当前会话直接执行，不分派子任务，不自动提交。

**Goal:** 将 Eino Retriever 包装为 search_knowledge 工具，接入现有多轮流式 CLI，完成订单事实与演示政策联合问答。

**Architecture:** internal/example/knowledge.go 包含结构化知识工具和明确标注的本地关键词演示 Retriever；真实后端将来通过同一 Eino 接口替换，不改变 Agent。工具只检索，不嵌套模型生成；身份使用 context，不接受模型指定用户。返回来源 ID、标题、版本、正文与截断标识。当前语料对所有已登录演示用户可见，不声称实现知识 ACL。

**Tech Stack:** Go 1.27.3、Eino v0.9.22 Retriever / InferTool / ADK、现有本地 SSE 测试。

---

## Task 1：工具契约与关键测试

Files: internal/example/knowledge.go、internal/example/knowledge_test.go。

- [x] 先加入测试：无效/超长/未知字段参数在检索前拒绝；无身份不访问后端；empty 与 error 分别返回 no_match / unavailable；取消原样传播；只返回白名单证据字段；数量/长度预算与截断标识。
- [x] 新接口与结构：

```go
func NewKnowledgeTool(backend retriever.Retriever) (tool.InvokableTool, error)
func DemoKnowledgeRetriever() retriever.Retriever

type KnowledgeResult struct {
    Code string `json:"code"`
    Message string `json:"message,omitempty"`
    Evidence []KnowledgeEvidence `json:"evidence,omitempty"`
}
type KnowledgeEvidence struct {
    SourceID string `json:"source_id"`
    Title string `json:"title"`
    Version string `json:"version"`
    Content string `json:"content"`
    Truncated bool `json:"truncated,omitempty"`
}
```

- [x] 运行 go test ./internal/example，确认新接口未实现时失败。
- [x] 实现严格 JSON 参数解析、context 用户传递、最多 3 条/每条 1200 rune 的输出预算。query 非空且至多 256 rune；原始 JSON 限制 4096 字节。异常不打印后端原文。演示 Retriever 使用固定关键词匹配并遵循 TopK/ScoreThreshold，不作为向量/生产中文检索。

## Task 2：接入 CLI 和端到端联合调用

Files: cmd/agentcheck/main.go、cmd/agentcheck/main_test.go、scripts/smoke.py。

- [x] CLI 启动时创建知识工具，与 get_order 一起注册，更新服务端提示词：规则依据知识证据并标来源；订单状态不足时不得断言退款资格；不执行写操作。
- [x] 本地 OpenAI/SSE 替身依次请求 get_order、search_knowledge，再核对两类结果都进入最终模型输入，source_id 与工具 call ID 完整；最终正文带演示标识与真实返回来源。
- [x] 保持旧单轮协议、多轮历史、分片输出和取消进程测试通过；进程冒烟增加联合路径，不访问真实模型。

## Task 3：课程与验收

Files: docs/lessons/A02-knowledge-tool-and-agentic-rag.md、docs/README.md、docs/ROADMAP.md、docs/ARCHITECTURE.md、README.md。

- [x] 记录 Ragent KnowledgeSearchFacade 的嵌套生成与本轮“只返回证据”的区别、Eino Retriever 输入输出、可复现 CLI 命令、已验证/未验证边界。
- [x] 全量 race、vet、构建与本地进程冒烟；检查文档链接和 git diff --check。不调用真实供应商，不修改 .env，不提交。

## 2026-11-09 执行结果

- 新工具测试先在接口缺失时编译失败，实现后 internal/example 测试通过。
- 联合测试首次误用了 Event.ToolCallID，已改成现有 Event.CallID；修正后 go test -race ./... 全量通过。
- go vet ./...、go build -o bin/ ./cmd/... 通过。
- python3 scripts/smoke.py 全部通过，包含新增的订单+知识联合链路、旧单轮、多轮完整历史、分片参数与 SIGINT 退出。
- Markdown 本地链接检查、git diff --check 通过。
- 未调用真实供应商，未修改 .env，未提交或推送。真实模型的选择/引用质量与真实检索仍待验证。
