package support

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestAuditAPIReturnsReadableDecoratedRows(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for audit API tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("audit_api_%d", time.Now().UnixNano())
	mustExecAuditAPISQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if err := EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureAuditTables: %v", err)
	}
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.pay_statuses(id BIGINT PRIMARY KEY, name TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.ship_statuses(id BIGINT PRIMARY KEY, name TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.orders(id BIGINT PRIMARY KEY, order_no TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.products(id BIGINT PRIMARY KEY, name TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.materials(id BIGINT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.customers(id BIGINT PRIMARY KEY, name TEXT NOT NULL)`, schema))
	mustExecAuditAPISQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.audit_logs(actor, entity_type, entity_id, action, field, old_value, new_value, meta)
		VALUES('order','material_receipt',100,'submit','qty_g',NULL,'5000','{"batch_code":"MR20260502001","material_id":22}'::jsonb)
	`, schema))

	e := echo.New()
	registerCoreRoutes(e, pool, schema)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/audit?type=material_receipt&limit=1", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Rows []AuditLogRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(payload.Rows) != 1 {
		t.Fatalf("rows len = %d", len(payload.Rows))
	}
	row := payload.Rows[0]
	if row.Menu != "库存管理 / 采购入库" {
		t.Fatalf("Menu = %q", row.Menu)
	}
	if row.Feature != "提交原料入库" {
		t.Fatalf("Feature = %q", row.Feature)
	}
	if row.EntityLabel == nil || *row.EntityLabel != "原料入库单 MR20260502001" {
		t.Fatalf("EntityLabel = %v", row.EntityLabel)
	}
	if !strings.Contains(row.Summary, "原料入库单 MR20260502001") {
		t.Fatalf("Summary = %q", row.Summary)
	}
}

func mustExecAuditAPISQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec SQL failed: %v\n%s", err, sql)
	}
}
