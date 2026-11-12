package support

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR767RequirementSeedsPreserveVanAcceptanceAfterRestart(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_pr767_req_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	if err = ensureReqTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err = seedReqWorkflowA(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	rows := []struct{ table, code, initial string }{
		{"req_product", "PR-767-FINANCE-DAILY-LEDGER", "review"},
		{"req_dev", "DEV-PR767-COST", "done"},
		{"req_dev", "DEV-PR767-API", "done"},
		{"req_dev", "DEV-PR767-UI", "done"},
		{"req_dev", "DEV-PR767-DELIVERY", "done"},
	}
	for _, row := range rows {
		var status string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.%s WHERE code=$1`, schema, row.table), row.code).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != row.initial {
			t.Errorf("fresh seed %s status=%q want %q", row.code, status, row.initial)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET status='done',evidence='Van 已验收的证据',title='已确认的需求',assignee='Van' WHERE code=$1`, schema, row.table), row.code); err != nil {
			t.Fatal(err)
		}
	}
	if err = seedReqWorkflowA(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var status, evidence, title, assignee string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,evidence,title,assignee FROM %s.%s WHERE code=$1`, schema, row.table), row.code).Scan(&status, &evidence, &title, &assignee); err != nil {
			t.Fatal(err)
		}
		if status != "done" || evidence != "Van 已验收的证据" || title != "已确认的需求" || assignee != "Van" {
			t.Errorf("restart erased progress %s: status=%q evidence=%q title=%q assignee=%q", row.code, status, evidence, title, assignee)
		}
	}
}
