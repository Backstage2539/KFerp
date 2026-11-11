package production

import (
	"context"
	"fmt"
	"math"
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

func TestIncomingAllocationPreservesSubgramAndFrozenQuantities(t *testing.T) {
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

	for _, tc := range []struct {
		name    string
		unit    string
		batches []app.OperationBatchBalance
		weights []float64
		want    []float64
	}{
		{"single_subgram", "kg", []app.OperationBatchBalance{{Remaining: 15, Unit: "kg"}, {Remaining: 15, Unit: "kg"}, {Remaining: 15, Unit: "kg"}, {Remaining: 14.961751, Unit: "kg"}}, []float64{1}, []float64{59.961751}},
		{"multiple_subgram", "kg", []app.OperationBatchBalance{{Remaining: 59.961751, Unit: "kg"}}, []float64{60, 40}, []float64{35.9770506, 23.9847004}},
		{"whole_gram_legacy_allocation", "kg", []app.OperationBatchBalance{{Remaining: 93001, Unit: "g"}}, []float64{60, 40}, []float64{55.801, 37.2}},
		{"gram_eight_decimal_precision", "g", []app.OperationBatchBalance{{Remaining: 1, Unit: "g"}, {Remaining: 0.00000001, Unit: "g"}}, []float64{1}, []float64{1.00000001}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := fmt.Sprintf("incoming_precision_%d", time.Now().UnixNano())
			if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
				t.Fatal(err)
			}
			defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.job_cards (id BIGINT PRIMARY KEY, work_order_id BIGINT,sequence_no INT,workstation TEXT,status TEXT,planned_input_qty NUMERIC,metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb)`, schema))
			if err != nil {
				t.Fatal(err)
			}
			for i, w := range tc.weights {
				_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.job_cards(id,work_order_id,sequence_no,workstation,status,planned_input_qty) VALUES($1,1,2,'A','pending',$2)`, schema), i+1, w)
				if err != nil {
					t.Fatal(err)
				}
			}
			repo := NewRepository(pool, schema)
			p := app.OperationCompletionPreview{WorkOrderID: 1, InputUnit: tc.unit, PredecessorBatches: tc.batches}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			allocations, err := repo.incomingAllocationsTx(ctx, tx, p, 2, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(allocations) != len(tc.want) {
				t.Fatalf("got %d allocations", len(allocations))
			}
			for i, a := range allocations {
				if math.Abs(a.Quantity-tc.want[i]) > 1e-10 {
					t.Fatalf("batch %d got %.8f want %.8f", i, a.Quantity, tc.want[i])
				}
			}
			if err = repo.freezeIncomingAllocationsTx(ctx, tx, allocations); err != nil {
				t.Fatal(err)
			}
			p.PredecessorBatches = []app.OperationBatchBalance{{Remaining: 1, Unit: "kg"}}
			reloaded, err := repo.incomingAllocationsTx(ctx, tx, p, 2, true)
			if err != nil {
				t.Fatal(err)
			}
			for i, a := range reloaded {
				if !a.Frozen || math.Abs(a.Quantity-tc.want[i]) > 1e-10 {
					t.Fatalf("frozen batch %d: %+v", i, a)
				}
			}
		})
	}
}
