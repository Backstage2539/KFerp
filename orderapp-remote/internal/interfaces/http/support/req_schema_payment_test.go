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

// Exercise the same tables and seeds as application startup in a fresh schema.
func TestRequirementStartupBillingPaymentSeeds(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("PostgreSQL test database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("req_payment_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	for i := 0; i < 2; i++ {
		if err = ensureRequirementSupportSchema(ctx, pool, schema); err != nil {
			t.Fatalf("startup pass %d: %v", i, err)
		}
	}
	for _, tc := range []struct{ table, code string }{{"req_product", "PR-FULFILLMENT-BILL-PAYMENT"}, {"req_dev", "DEV-FULFILLMENT-BILL-PAYMENT-01"}} {
		var count int
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.%s WHERE code=$1`, schema, tc.table), tc.code).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s %s count=%d err=%v", tc.table, tc.code, count, err)
		}
	}
}
