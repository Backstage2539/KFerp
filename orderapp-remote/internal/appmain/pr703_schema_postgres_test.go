package appmain

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPR703SchemaBootstrapAndRepeatPreserveFulfillmentTables(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("disposable PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("pr703_bootstrap_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	for run := 0; run < 2; run++ {
		if err = ensureAppSchema(ctx, pool, schema); err != nil {
			t.Fatalf("bootstrap %d: %v", run, err)
		}
	}
	for _, table := range []string{"customer_stock_entitlements", "customer_stock_supply_reservations", "customer_stock_entitlement_receipts", "customer_fulfillment_settings", "shipping_fee_templates", "fulfillment_order_fee_snapshots"} {
		var exists bool
		if err = pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", schema+"."+table).Scan(&exists); err != nil || !exists {
			t.Errorf("table=%s exists=%v error=%v", table, exists, err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='orders' AND column_name IN('delivery_mode','delivery_warehouse_code','delivery_status','fulfillment_mode')`, schema).Scan(&count); err != nil || count != 4 {
		t.Fatalf("delivery columns=%d error=%v", count, err)
	}
}
