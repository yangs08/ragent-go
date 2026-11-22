package example

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"ragent-go/internal/common"
)

type knowledgeBackend struct {
	calls       int
	query, user string
	topK        int
	docs        []*schema.Document
	err         error
}

func (b *knowledgeBackend) Retrieve(ctx context.Context, q string, opts ...retriever.Option) ([]*schema.Document, error) {
	b.calls++
	b.query = q
	if user, err := common.RequireUser(ctx); err == nil {
		b.user = user.ID
	}
	if opt := retriever.GetCommonOptions(nil, opts...); opt.TopK != nil {
		b.topK = *opt.TopK
	}
	return b.docs, b.err
}
func knowledgeDocument(id, body string) *schema.Document {
	return &schema.Document{ID: id, Content: body, MetaData: map[string]any{"title": "演示政策", "version": "demo-v1", "private_key": "private-metadata"}}
}
func knowledgeResult(t *testing.T, tool tool.InvokableTool, args string) KnowledgeResult {
	t.Helper()
	raw, err := tool.InvokableRun(aliceContext(), args)
	if err != nil {
		t.Fatal(err)
	}
	var result KnowledgeResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "private-metadata") {
		t.Fatal("metadata was exposed")
	}
	return result
}
func TestKnowledgeToolRejectsInvalidInputBeforeRetrieval(t *testing.T) {
	for _, args := range []string{`{}`, `null`, `{"query":" "}`, `{"query":3}`, `{"query":"退货","user_id":"bob"}`, `{"query":"退货"}{}`, `{"query":"private-value`, strings.Repeat("x", 4097), encodedQuery(strings.Repeat("退", 257))} {
		backend := &knowledgeBackend{}
		tool, err := NewKnowledgeTool(backend)
		if err != nil {
			t.Fatal(err)
		}
		result := knowledgeResult(t, tool, args)
		if result.Code != "invalid_arguments" || backend.calls != 0 || len(result.Evidence) != 0 {
			t.Fatalf("result=%+v calls=%d", result, backend.calls)
		}
	}
}
func encodedQuery(q string) string {
	raw, _ := json.Marshal(map[string]string{"query": q})
	return string(raw)
}
func TestKnowledgeToolReturnsBoundedEvidenceWithProvenance(t *testing.T) {
	backend := &knowledgeBackend{docs: []*schema.Document{knowledgeDocument("public-v1-c1", strings.Repeat("证", 1300)), knowledgeDocument("public-v1-c2", "第二条证据"), knowledgeDocument("public-v1-c3", "第三条证据"), knowledgeDocument("not-returned", "第四条证据")}}
	tool, err := NewKnowledgeTool(backend)
	if err != nil {
		t.Fatal(err)
	}
	result := knowledgeResult(t, tool, `{"query":"  退货政策  "}`)
	if result.Code != "found" || len(result.Evidence) != 3 || backend.topK != 3 || backend.query != "退货政策" || backend.user != "alice" {
		t.Fatalf("result=%+v backend=%+v", result, backend)
	}
	first := result.Evidence[0]
	if first.SourceID != "public-v1-c1" || first.Title != "演示政策" || first.Version != "demo-v1" || len([]rune(first.Content)) != 1200 || !first.Truncated {
		t.Fatalf("bad provenance or budget: %+v", first)
	}
	if result.Evidence[1].Truncated {
		t.Fatal("short evidence marked truncated")
	}
}
func TestKnowledgeToolDistinguishesEmptyFailureAndInvalidDocument(t *testing.T) {
	for _, tt := range []struct {
		name, code string
		docs       []*schema.Document
		err        error
	}{
		{name: "empty", code: "no_match"},
		{name: "backend failure", code: "unavailable", err: errors.New("private-backend-address")},
		{name: "missing source", code: "unavailable", docs: []*schema.Document{knowledgeDocument("", "正文")}},
		{name: "missing version", code: "unavailable", docs: []*schema.Document{{ID: "x", Content: "正文", MetaData: map[string]any{"title": "演示"}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			backend := &knowledgeBackend{docs: tt.docs, err: tt.err}
			tool, err := NewKnowledgeTool(backend)
			if err != nil {
				t.Fatal(err)
			}
			result := knowledgeResult(t, tool, `{"query":"退货"}`)
			if result.Code != tt.code || len(result.Evidence) != 0 || strings.Contains(result.Message, "private-backend") {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}
func TestKnowledgeToolRequiresIdentityAndPropagatesCancellation(t *testing.T) {
	backend := &knowledgeBackend{}
	tool, err := NewKnowledgeTool(backend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), `{"query":"退货"}`); err == nil || backend.calls != 0 {
		t.Fatal("anonymous retrieval allowed")
	}
	ctx, cancel := context.WithCancel(aliceContext())
	cancel()
	if _, err := tool.InvokableRun(ctx, `{"query":"退货"}`); !errors.Is(err, context.Canceled) || backend.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, backend.calls)
	}
	backend.err = context.DeadlineExceeded
	if _, err := tool.InvokableRun(aliceContext(), `{"query":"退货"}`); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline became business result: %v", err)
	}
}
func TestDemoKnowledgeIsExplicitlyDemoAndDoesNotInventMatches(t *testing.T) {
	tool, err := NewKnowledgeTool(DemoKnowledgeRetriever())
	if err != nil {
		t.Fatal(err)
	}
	result := knowledgeResult(t, tool, `{"query":"A100 已发货，退货政策是什么"}`)
	if result.Code != "found" || len(result.Evidence) == 0 {
		t.Fatalf("result=%+v", result)
	}
	found := false
	for _, e := range result.Evidence {
		if e.SourceID == "demo-returns-v1" {
			found = true
		}
		if !strings.Contains(e.Title, "演示") {
			t.Fatal("demo policy not labeled")
		}
	}
	if !found {
		t.Fatal("missing return policy")
	}
	if empty := knowledgeResult(t, tool, `{"query":"火星基地考勤制度"}`); empty.Code != "no_match" {
		t.Fatalf("invented match: %+v", empty)
	}
}
