package sales

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"testing"
)

func TestOrderDeliveryNonCourierFulfillmentAndTracking(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer pool.Close()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	for _, q := range []string{
		"CREATE TABLE %s.ship_statuses(id bigint,name text)",
		"CREATE TABLE %s.pay_statuses(id bigint,name text)",
		"CREATE TABLE %s.orders(id bigint,ship_method text)",
		"INSERT INTO %s.ship_statuses VALUES (1,'未发货'),(2,'已发货')",
		"INSERT INTO %s.pay_statuses VALUES (1,'未付款')",
		"INSERT INTO %s.orders VALUES (1,'pickup'),(2,'local_delivery')",
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(q, schema)); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, method := range []string{"pickup", "local_delivery"} {
		if err := validateOrderFulfillmentRequirementsTx(ctx, tx, schema, 1, 2, 0, 0, 0, 0, 0, method); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	if err := validateOrderFulfillmentRequirementsTx(ctx, tx, schema, 1, 2, 0, 0, 0, 0, 0, "sf_small"); err == nil {
		t.Fatal("courier still needs logistics")
	}
	for _, id := range []int64{1, 2} {
		if err := requireCourierOrderTx(ctx, tx, schema, id); err == nil {
			t.Fatal("non courier order allowed into courier flow")
		}
	}
}
