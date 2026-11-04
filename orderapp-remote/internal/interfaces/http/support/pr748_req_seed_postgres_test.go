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

func TestPR748RequirementSeedPreservesVerifiedProgress(t *testing.T) {
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
	schema := fmt.Sprintf("test_pr748_req_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	for _, row := range []reqSeedRow{
		{table: "req_product", code: "PR-748-BOM-START-CONSUMPTION", title: "按批开工", status: "doing", assignee: "Van", evidence: "待验收"},
		{table: "req_dev", code: "DEV-PR748-04", title: "开发环境交付", status: "doing", assignee: "Codex", evidence: "待部署"},
	} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.%s(code TEXT PRIMARY KEY,title TEXT NOT NULL,status TEXT NOT NULL,assignee TEXT NOT NULL,evidence TEXT NOT NULL)`, schema, row.table)); err != nil {
			t.Fatal(err)
		}
		if err = seedReqRow(ctx, pool, schema, row); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET status='review',evidence='开发环境验证完成' WHERE code=$1`, schema, row.table), row.code); err != nil {
			t.Fatal(err)
		}
		if err = seedReqRow(ctx, pool, schema, row); err != nil {
			t.Fatal(err)
		}
		var status, evidence string
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,evidence FROM %s.%s WHERE code=$1`, schema, row.table), row.code).Scan(&status, &evidence); err != nil {
			t.Fatal(err)
		}
		if status != "review" || evidence != "开发环境验证完成" {
			t.Fatalf("restart erased verified progress for %s: %s %s", row.code, status, evidence)
		}
	}
}
