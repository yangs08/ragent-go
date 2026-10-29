package example

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ragent-go/internal/usercontext"
)

type storeStub struct {
	calls  int
	userID string
	err    error
}

func (s *storeStub) Find(ctx context.Context, userID, id string) (Order, bool, error) {
	s.calls++
	s.userID = userID
	if s.err != nil {
		return Order{}, false, s.err
	}
	return Order{ID: id, UserID: "alice", Status: "shipped"}, true, nil
}

func TestOrderToolSchema(t *testing.T) {
	tool, err := NewOrderTool(DemoOrders())
	if err != nil {
		t.Fatal(err)
	}
	info, err := tool.Info(aliceContext())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "get_order" {
		t.Fatalf("name=%s", info.Name)
	}
	schema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Type                 string
		Required             []string
		Properties           map[string]any
		AdditionalProperties any
	}
	if err := json.Unmarshal(data, &shape); err != nil {
		t.Fatal(err)
	}
	if shape.Type != "object" || len(shape.Properties) != 1 || shape.Properties["order_id"] == nil || len(shape.Required) != 1 || shape.Required[0] != "order_id" || shape.AdditionalProperties != false {
		t.Fatalf("schema=%s", data)
	}
}

func TestOrderToolResults(t *testing.T) {
	for _, tt := range []struct{ name, args, code, status string }{
		{"own", `{"order_id":"A100"}`, "ok", "shipped"},
		{"another own", `{"order_id":"A101"}`, "ok", "delivered"},
		{"missing", `{"order_id":"Z999"}`, "not_found", ""},
		{"other user", `{"order_id":"B200"}`, "not_found", ""},
		{"another user", `{"order_id":"C300"}`, "not_found", ""},
		{"empty", `{}`, "invalid_arguments", ""},
		{"blank", `{"order_id":""}`, "invalid_arguments", ""},
		{"format", `{"order_id":"../A100"}`, "invalid_arguments", ""},
		{"unknown field", `{"order_id":"A100","user_id":"bob"}`, "invalid_arguments", ""},
		{"wrong type", `{"order_id":100}`, "invalid_arguments", ""},
		{"null", `null`, "invalid_arguments", ""},
		{"broken", `{"order_id":"private-value`, "invalid_arguments", ""},
		{"multiple values", `{"order_id":"A100"}{}`, "invalid_arguments", ""},
		{"oversized", strings.Repeat("x", 4097), "invalid_arguments", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := NewOrderTool(DemoOrders())
			if err != nil {
				t.Fatal(err)
			}
			output, err := tool.InvokableRun(aliceContext(), tt.args)
			if err != nil {
				t.Fatal(err)
			}
			var result OrderResult
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code != tt.code {
				t.Fatalf("result=%s", output)
			}
			if tt.status != "" && (result.Order == nil || result.Order.Status != tt.status) {
				t.Fatalf("result=%s", output)
			}
			if tt.code != "ok" && result.Order != nil {
				t.Fatal("unexpected order disclosure")
			}
			for _, secret := range []string{"alice", "bob", "private-value", "user_id"} {
				if strings.Contains(output, secret) {
					t.Fatalf("private data disclosed in result for %s", tt.name)
				}
			}
		})
	}
}

func TestOrderToolRejectsBeforeStoreAndUsesBoundUser(t *testing.T) {
	store := &storeStub{}
	tool, err := NewOrderTool(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(aliceContext(), `{"order_id":"A100","user_id":"other"}`); err != nil {
		t.Fatal(err)
	}
	if store.calls != 0 {
		t.Fatal("invalid input reached store")
	}
	if _, err := tool.InvokableRun(aliceContext(), `{"order_id":"A100"}`); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || store.userID != "alice" {
		t.Fatal("scope was not bound by application")
	}
}

func TestOrderToolCancellationAndStoreFailure(t *testing.T) {
	store := &storeStub{err: errors.New("private-database-detail")}
	tool, err := NewOrderTool(store)
	if err != nil {
		t.Fatal(err)
	}
	output, err := tool.InvokableRun(aliceContext(), `{"order_id":"A100"}`)
	if err != nil {
		t.Fatal(err)
	}
	var result OrderResult
	_ = json.Unmarshal([]byte(output), &result)
	if result.Code != "unavailable" || strings.Contains(output, "private-database-detail") {
		t.Fatal("unsafe store error")
	}
	ctx, cancel := context.WithCancel(aliceContext())
	cancel()
	_, err = tool.InvokableRun(ctx, `{"order_id":"A100"}`)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestOrderToolNeedsTrustedDependencies(t *testing.T) {
	if _, err := NewOrderTool(nil); err == nil {
		t.Fatal("nil store accepted")
	}
	tool, err := NewOrderTool(DemoOrders())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tool.InvokableRun(context.Background(), `{"order_id":"A100"}`); err == nil {
		t.Fatal("missing user accepted")
	}
}

func TestDemoOrdersRespectsContextAndUser(t *testing.T) {
	store := DemoOrders()
	ctx, cancel := context.WithCancel(aliceContext())
	cancel()
	if _, _, err := store.Find(ctx, "alice", "A100"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, found, err := store.Find(aliceContext(), "alice", "C300"); err != nil || found {
		t.Fatal("userID filter failed")
	}
}

func aliceContext() context.Context {
	return usercontext.WithUser(context.Background(), usercontext.User{ID: "alice"})
}
