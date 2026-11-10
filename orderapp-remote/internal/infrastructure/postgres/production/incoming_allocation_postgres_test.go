package production

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/production"
)

func TestIncomingAllocationFreezeIsStableAcrossPartialClaims(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("incoming_allocation_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.job_cards (id BIGINT PRIMARY KEY, work_order_id BIGINT,sequence_no INT,workstation TEXT,status TEXT,planned_input_qty NUMERIC,metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb); INSERT INTO %s.job_cards VALUES(1,1,2,'A','pending',60000,'{}'),(2,1,2,'B','pending',40000,'{}')`, schema, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	p := app.OperationCompletionPreview{WorkOrderID: 1, InputUnit: "kg", PredecessorBatches: []app.OperationBatchBalance{{Remaining: 93001, Unit: "g"}}}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	allocations, err := repo.incomingAllocationsTx(ctx, tx, p, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 2 || allocations[0].Quantity != 55.801 || allocations[1].Quantity != 37.2 {
		t.Fatalf("allocation: %+v", allocations)
	}
	if err = repo.freezeIncomingAllocationsTx(ctx, tx, allocations); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='running' WHERE id=1`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	p.PredecessorBatches = []app.OperationBatchBalance{{Remaining: 37.2, Unit: "kg"}}
	repeated, err := repo.incomingAllocationsTx(ctx, tx, p, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if !repeated[0].Frozen || repeated[0].Quantity != 55.801 || repeated[1].Quantity != 37.2 {
		t.Fatalf("recalculated frozen allocations: %+v", repeated)
	}
	if err = repo.freezeIncomingAllocationsTx(ctx, tx, repeated); err != nil {
		t.Fatal(err)
	}
}
