package production

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPackagingAllocationMigratesLegacyPositiveQuantityConstraint(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("packmigration_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s; CREATE TABLE %s.work_orders(id bigint primary key); CREATE TABLE %s.job_cards(id bigint primary key)`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, fmt.Sprintf(`DROP SCHEMA %s CASCADE`, schema))
	if err = ensureOperationCompletionSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.production_started_input_allocations DROP CONSTRAINT production_started_input_allocations_qty_check; ALTER TABLE %[1]s.production_started_input_allocations ADD CONSTRAINT production_started_input_allocations_qty_check CHECK(qty>0);
 INSERT INTO %[1]s.production_operation_events(id,work_order_id,job_card_id,request_id,expected_version,operator,input_unit,output_unit,warehouse,payload) VALUES(1,1,1,'a','a','test','袋','袋','wip','{}'),(2,1,1,'b','b','test','袋','袋','wip','{}');
 INSERT INTO %[1]s.production_started_inputs(id,work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,operator) VALUES(1,1,1,'x',10,10,'袋','test');
 INSERT INTO %[1]s.production_started_input_allocations(input_id,event_id,qty,cost) VALUES(1,1,1,5)`, schema)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = ensureOperationCompletionSchema(ctx, pool, schema); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_input_allocations(input_id,event_id,qty,cost) VALUES(1,2,0,6)`, schema)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_started_input_allocations WHERE (qty=1 AND cost=5) OR (qty=0 AND cost=6)`, schema)).Scan(&count); err != nil || count != 2 {
		t.Fatalf("migration lost allocations: count=%d err=%v", count, err)
	}
}
