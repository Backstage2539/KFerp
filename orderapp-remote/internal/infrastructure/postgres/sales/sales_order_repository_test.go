package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSalesOrderSettingsRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存", PaymentText: "微信或对公转账",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	got, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if got.CompanyName != "浅焙作坊咖啡" || got.Note != "请密封保存" || got.PaymentText != "微信或对公转账" {
		t.Fatalf("settings = %+v", got)
	}
}

func newSalesPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for sales postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_sales_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func prepareSalesSchemaPrerequisites(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s.order_process_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, sort INTEGER NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT true)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.ship_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders (id BIGSERIAL PRIMARY KEY, order_no TEXT NOT NULL DEFAULT '')`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare schema: %v", err)
		}
	}
}
