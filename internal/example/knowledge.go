package example

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"ragent-go/internal/common"
)

// KnowledgeEvidence 只公开引用所需字段，不把检索后端的全部元数据交给模型。
type KnowledgeEvidence struct {
	SourceID  string `json:"source_id"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}
type KnowledgeResult struct {
	Code     string              `json:"code"`
	Message  string              `json:"message,omitempty"`
	Evidence []KnowledgeEvidence `json:"evidence,omitempty"`
}
type knowledgeInput struct {
	Query   string `json:"query" jsonschema:"description=用于检索演示政策的完整独立问题，包含需要的订单事实"`
	invalid bool
}

// NewKnowledgeTool 只检索证据，不在工具内部再次调用模型生成答案。
// backend 必须按 context 中的可信身份控制知识范围；demo.go 的演示语料对所有演示用户可见。
func NewKnowledgeTool(backend retriever.Retriever) (tool.InvokableTool, error) {
	if backend == nil {
		return nil, errors.New("knowledge retriever is required")
	}
	return utils.InferTool("search_knowledge", "检索本地演示售后和物流政策，返回带来源和版本的证据；不是实际商家政策，不执行申请或退款。", func(ctx context.Context, input knowledgeInput) (KnowledgeResult, error) {
		if err := ctx.Err(); err != nil {
			return KnowledgeResult{}, err
		}
		if _, err := common.RequireUser(ctx); err != nil {
			return KnowledgeResult{}, err
		}
		query := strings.TrimSpace(input.Query)
		if input.invalid || query == "" || utf8.RuneCountInString(query) > 256 {
			return KnowledgeResult{Code: "invalid_arguments", Message: "需要非空且不超过256字符的 query，不接受其他参数"}, nil
		}
		docs, err := backend.Retrieve(ctx, query, retriever.WithTopK(3))
		// 有些后端会在取消后返回空列表，不能把这种情况报告为没有命中。
		if ctx.Err() != nil {
			return KnowledgeResult{}, ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return KnowledgeResult{}, err
		}
		unavailable := KnowledgeResult{Code: "unavailable", Message: "知识检索暂不可用，不能据此判断是否存在相关政策"}
		if err != nil {
			return unavailable, nil
		}
		if len(docs) == 0 {
			return KnowledgeResult{Code: "no_match", Message: "本次查询未命中演示语料；不代表相关规定不存在"}, nil
		}
		result := KnowledgeResult{Code: "found"}
		// 除了向后端传 TopK，工具出口再次限制证据量，防止不遵循选项的后端撑满模型上下文。
		for _, doc := range docs[:min(len(docs), 3)] {
			if doc == nil {
				return unavailable, nil
			}
			title, _ := doc.MetaData["title"].(string)
			version, _ := doc.MetaData["version"].(string)
			if strings.TrimSpace(doc.ID) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(version) == "" || strings.TrimSpace(doc.Content) == "" {
				return unavailable, nil
			}
			text := []rune(doc.Content)
			clipped := len(text) > 1200
			if clipped {
				text = text[:1200]
			}
			result.Evidence = append(result.Evidence, KnowledgeEvidence{SourceID: doc.ID, Title: title, Version: version, Content: string(text), Truncated: clipped})
		}
		return result, nil
	}, utils.WithUnmarshalArguments(decodeKnowledgeInput), utils.WithMarshalOutput(func(_ context.Context, output any) (string, error) {
		b, err := json.Marshal(output)
		return string(b), err
	}))
}
func decodeKnowledgeInput(_ context.Context, args string) (any, error) {
	invalid := knowledgeInput{invalid: true}
	if len(args) > 4096 {
		return invalid, nil
	}
	decoder := json.NewDecoder(strings.NewReader(args))
	decoder.DisallowUnknownFields()
	var input knowledgeInput
	if err := decoder.Decode(&input); err != nil {
		return invalid, nil
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalid, nil
	}
	return input, nil
}
