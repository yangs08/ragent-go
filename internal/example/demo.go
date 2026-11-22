package example

import (
	"context"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

type demoOrders map[string]Order

func DemoOrders() OrderStore {
	return demoOrders{
		"A100": {ID: "A100", UserID: "alice", Status: "shipped"},
		"A101": {ID: "A101", UserID: "alice", Status: "delivered"},
		"B200": {ID: "B200", UserID: "bob", Status: "pending"},
		"C300": {ID: "C300", UserID: "carol", Status: "delivered"},
	}
}
func (s demoOrders) Find(ctx context.Context, userID, id string) (Order, bool, error) {
	if err := ctx.Err(); err != nil {
		return Order{}, false, err
	}
	value, found := s[id]
	if !found || value.UserID != userID {
		return Order{}, false, nil
	}
	return value, true, nil
}

// DemoKnowledgeRetriever 是 Eino Retriever 的最小可运行演示。
// 固定关键词命中便于观察 Agent 如何使用证据，不是向量检索、中文分词或生产召回实现。
func DemoKnowledgeRetriever() retriever.Retriever { return demoKnowledgeRetriever{} }

type demoKnowledgeRetriever struct{}

func (demoKnowledgeRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	entries := []struct {
		id, title, content string
		keywords           []string
	}{
		{"demo-returns-v1", "演示售后政策", "仅供学习的演示政策：订单状态为 shipped 时，应先核实是否签收；状态为 delivered 时，可进一步收集商品是否拆封、签收日期等信息并提交人工审核。仅凭订单状态不能判断退款资格。本演示只解释流程，不执行退货申请或退款。", []string{"退货", "退款", "售后", "申请", "签收", "return", "refund"}},
		{"demo-shipping-v1", "演示物流政策", "仅供学习的演示政策：shipped 表示已发货，delivered 表示已送达。订单状态不包含物流单号或预计到达时间，缺少这些信息时应明确说明，不能编造物流轨迹或送达日期。", []string{"物流", "发货", "配送", "送达", "shipping", "delivery"}},
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var docs []*schema.Document
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		score := 0
		for _, term := range entry.keywords {
			if strings.Contains(query, term) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		docs = append(docs, (&schema.Document{ID: entry.id, Content: entry.content, MetaData: map[string]any{"title": entry.title, "version": "demo-v1"}}).WithScore(float64(score)))
	}
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].Score() > docs[j].Score() })
	options := retriever.GetCommonOptions(nil, opts...)
	if options.ScoreThreshold != nil {
		filtered := docs[:0]
		for _, doc := range docs {
			if doc.Score() >= *options.ScoreThreshold {
				filtered = append(filtered, doc)
			}
		}
		docs = filtered
	}
	if options.TopK != nil {
		count := max(0, *options.TopK)
		docs = docs[:min(len(docs), count)]
	}
	return docs, nil
}
