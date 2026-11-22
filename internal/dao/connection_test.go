package dao

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"ragent-go/internal/bootstrap"
)

// 记录 GORM 实际选择的连接和事务；不模拟 PostgreSQL 查询结果。
type recordingPool struct {
	calls       int
	lastContext context.Context
	tx          *recordingTx
}
type recordingTx struct {
	*recordingPool
	committed, rolledBack bool
}

func (p *recordingPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (p *recordingPool) ExecContext(ctx context.Context, _ string, _ ...any) (sql.Result, error) {
	p.calls++
	p.lastContext = ctx
	return driver.RowsAffected(1), ctx.Err()
}
func (p *recordingPool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (p *recordingPool) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("unexpected QueryRowContext")
}
func (p *recordingPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	p.tx = &recordingTx{recordingPool: &recordingPool{}}
	return p.tx, nil
}
func (p *recordingTx) Commit() error   { p.committed = true; return nil }
func (p *recordingTx) Rollback() error { p.rolledBack = true; return nil }
func testDB(t *testing.T) (*gorm.DB, *recordingPool) {
	t.Helper()
	pool := &recordingPool{}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	old := bootstrap.DB
	bootstrap.DB = db
	t.Cleanup(func() { bootstrap.DB = old })
	return db, pool
}

func TestTransactionContextKeepsConnectionAndRollback(t *testing.T) {
	_, pool := testDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rollback := errors.New("requested rollback")
	err := User.Transaction(ctx, func(txCtx context.Context) error {
		if err := User.CheckSchema(txCtx); err != nil {
			return err
		}
		if err := Session.Delete(txCtx, "digest", "owner"); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) || pool.tx == nil || !pool.tx.rolledBack || pool.tx.committed {
		t.Fatal("transaction did not roll back")
	}
	if pool.calls != 0 || pool.tx.calls != 2 {
		t.Fatal("DAO escaped transaction or lost context")
	}
	if ctx.Value(transactionKey{}) != nil {
		t.Fatal("transaction binding mutated parent context")
	}
	if err := User.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Session.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if pool.calls != 2 || pool.tx.calls != 2 {
		t.Fatal("default table calls retained transaction")
	}
}

func TestTableQueriesDoNotShareConditionsOrContext(t *testing.T) {
	db, _ := testDB(t)
	firstCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	secondCtx := context.Background()
	boundCtx := WithTx(firstCtx, db.Where("username = ?", "stale"))
	first := User.Conn(boundCtx).Session(&gorm.Session{DryRun: true}).Where("username = ?", "alice").Find(&[]UserStruct{})
	second := User.Conn(secondCtx).Session(&gorm.Session{DryRun: true}).Where("username = ?", "bob").Find(&[]UserStruct{})
	for _, q := range []*gorm.DB{first, second} {
		if q.Error != nil {
			t.Fatal(q.Error)
		}
	}
	if len(first.Statement.Vars) != 1 || first.Statement.Vars[0] != "alice" || len(second.Statement.Vars) != 1 || second.Statement.Vars[0] != "bob" {
		t.Fatal("query conditions leaked between calls")
	}
	if first.Statement.Context != boundCtx || second.Statement.Context != secondCtx {
		t.Fatal("request context leaked between calls")
	}
	one := Session.Conn(firstCtx).Session(&gorm.Session{DryRun: true}).Where("user_id = ?", "alice").Find(&[]SessionStruct{})
	two := Session.Conn(secondCtx).Session(&gorm.Session{DryRun: true}).Find(&[]SessionStruct{})
	if len(one.Statement.Vars) != 1 || len(two.Statement.Vars) != 0 || two.Statement.Context != secondCtx {
		t.Fatal("session table retained query state")
	}
}

func TestUserInsertPreservesDatabaseTimestampDefaults(t *testing.T) {
	db, _ := testDB(t)
	result := User.Conn(WithTx(context.Background(), db)).Session(&gorm.Session{DryRun: true}).Create(&UserStruct{ID: "id", Username: "alice", PasswordHash: "hash"})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	query := result.Statement.SQL.String()
	if !strings.Contains(query, `INSERT INTO "users"`) || strings.Contains(query, "created_at") || strings.Contains(query, "updated_at") {
		t.Fatalf("timestamp defaults changed: %s", query)
	}
}

func TestNilTransactionDoesNotFallBackToDefault(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil transaction silently accepted")
		}
	}()
	WithTx(context.Background(), nil)
}

func TestTransactionContextPreservesCancellationAndValues(t *testing.T) {
	db, pool := testDB(t)
	type requestKey struct{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), requestKey{}, "request"))
	txCtx := WithTx(parent, db)
	if txCtx.Value(requestKey{}) != "request" {
		t.Fatal("lost request identity")
	}
	cancel()
	if err := User.CheckSchema(txCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if pool.lastContext != txCtx {
		t.Fatal("DAO did not forward transaction context")
	}
}

func TestTableTransactionCommitsOnSuccess(t *testing.T) {
	_, pool := testDB(t)
	err := Session.Transaction(context.Background(), func(ctx context.Context) error {
		return User.CheckSchema(ctx)
	})
	if err != nil {
		t.Fatal(err)
	}
	if pool.tx == nil || !pool.tx.committed || pool.tx.rolledBack || pool.tx.calls != 1 || pool.calls != 0 {
		t.Fatal("successful callback did not commit on the shared transaction connection")
	}
}

func TestTableTransactionRollsBackAndPropagatesPanic(t *testing.T) {
	_, pool := testDB(t)
	failure := errors.New("callback panic")
	defer func() {
		if got := recover(); got != failure {
			t.Fatalf("panic changed or swallowed: %v", got)
		}
		if pool.tx == nil || !pool.tx.rolledBack || pool.tx.committed {
			t.Fatal("panic did not roll back")
		}
	}()
	_ = User.Transaction(context.Background(), func(context.Context) error { panic(failure) })
}
