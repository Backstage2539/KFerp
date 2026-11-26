package appmain

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	salespg "orderapp/internal/infrastructure/postgres/sales"
)

func TestEmployeeEntryRevisionsFollowCommittedConfiguration(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("disposable PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("entry_revision_%d", time.Now().UnixNano())
	exec := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE SCHEMA " + schema)
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") }()
	if err := ensureAppSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO " + schema + ".customers(id,name) VALUES(7101,'A'),(7102,'B')")
	repo := salespg.NewRepository(pool, schema)
	rev := func(part string, id int64) string {
		t.Helper()
		v, err := repo.EmployeeEntryRevision(ctx, part, id)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	a, b, base := rev("catalog", 7101), rev("catalog", 7102), rev("base", 0)
	exec("INSERT INTO " + schema + ".bean_list_publications (id,list_type,status,owner_type,owner_key) VALUES (7133,'commercial','draft','customer','7101')")
	if rev("catalog", 7101) != a {
		t.Fatal("draft invalidated catalog")
	}
	sql := "UPDATE " + schema + ".bean_list_publications SET status='published' WHERE id=7133"
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	if rev("catalog", 7101) != a {
		t.Fatal("uncommitted publication visible")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if rev("catalog", 7101) != a {
		t.Fatal("rollback invalidated catalog")
	}
	exec(sql)
	now := rev("catalog", 7101)
	if now == a || rev("catalog", 7102) != b || rev("base", 0) != base {
		t.Fatal("customer publication changed wrong sections")
	}
	exec("UPDATE " + schema + ".bean_list_publications SET status='withdrawn' WHERE id=7133")
	if rev("catalog", 7101) == now {
		t.Fatal("withdrawal not visible")
	}
	before := rev("catalog", 7101)
	exec("UPDATE " + schema + ".customers SET phone='test' WHERE id=7101")
	if rev("base", 0) == base || rev("catalog", 7101) != before {
		t.Fatal("contact changes must only invalidate the bootstrap")
	}
	for _, part := range []string{"base", "catalog"} {
		if _, err := repo.EmployeeEntryData(ctx, part, 7101); err != nil {
			t.Fatalf("load %s: %v", part, err)
		}
	}
	exec("UPDATE " + schema + ".customers SET active=false WHERE id=7101")
	if _, err := repo.EmployeeEntryRevision(ctx, "catalog", 7101); err == nil {
		t.Fatal("inactive customer retained access")
	}
}
