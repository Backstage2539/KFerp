package production

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPackagingComponentAllocationTx(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE production_started_inputs(id bigint primary key,job_card_id bigint,status text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_components(id bigint primary key,input_id bigint,component_key text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_component_allocations(component_id bigint,event_id bigint,qty numeric,cost numeric);
 CREATE TEMP TABLE production_started_input_allocations(input_id bigint,event_id bigint,qty numeric,cost numeric);`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO production_started_inputs VALUES(1,10,'posted',180,266); INSERT INTO production_started_input_components VALUES(11,1,'bean',83000,166),(12,1,'bag',200,100)`); err != nil {
		t.Fatal(err)
	}
	repo := Repository{schema: "pg_temp"}
	cost, err := repo.allocatePackagingComponentsTx(ctx, tx, 10, 20, []packagingInputComponentRequest{{InputID: 1, InputQty: 180, Components: map[string]float64{"bean": 81720, "bag": 180}}})
	if err != nil {
		t.Fatal(err)
	}
	if cost != 253.44 {
		t.Fatalf("cost %v", cost)
	}
	var beans, bags float64
	if err = tx.QueryRow(ctx, `SELECT (SELECT remaining_qty FROM production_started_input_components WHERE id=11),(SELECT remaining_qty FROM production_started_input_components WHERE id=12)`).Scan(&beans, &bags); err != nil {
		t.Fatal(err)
	}
	if beans != 1280 || bags != 20 {
		t.Fatalf("remaining beans=%v bags=%v", beans, bags)
	}
}

func TestPackagingInputRequiresWholePieces(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `CREATE TEMP TABLE production_started_inputs(id bigint primary key,job_card_id bigint,status text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_components(id bigint primary key,input_id bigint,component_key text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_component_allocations(component_id bigint,event_id bigint,qty numeric,cost numeric);
 CREATE TEMP TABLE production_started_input_allocations(input_id bigint,event_id bigint,qty numeric,cost numeric);
 INSERT INTO production_started_inputs VALUES(1,10,'posted',2,2); INSERT INTO production_started_input_components VALUES(1,1,'bag',2,2)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Repository{schema: "pg_temp"}).allocatePackagingComponentsTx(ctx, tx, 10, 20, []packagingInputComponentRequest{{InputID: 1, InputQty: 0.5, Components: map[string]float64{"bag": 1}}})
	if err == nil {
		t.Fatal("fractional package count accepted")
	}
}

func TestPackagingAllocationRejectsShortageBeforePosting(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `CREATE TEMP TABLE production_started_inputs(id bigint primary key,job_card_id bigint,status text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_components(id bigint primary key,input_id bigint,component_key text,remaining_qty numeric,remaining_cost numeric);
 CREATE TEMP TABLE production_started_input_component_allocations(component_id bigint,event_id bigint,qty numeric,cost numeric);
 CREATE TEMP TABLE production_started_input_allocations(input_id bigint,event_id bigint,qty numeric,cost numeric);
 INSERT INTO production_started_inputs VALUES(1,10,'posted',2,2); INSERT INTO production_started_input_components VALUES(1,1,'bag',2,2)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Repository{schema: "pg_temp"}).allocatePackagingComponentsTx(ctx, tx, 10, 20, []packagingInputComponentRequest{{InputID: 1, InputQty: 2, Components: map[string]float64{"bag": 3}}})
	if err == nil {
		t.Fatal("component shortage accepted")
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_started_input_component_allocations`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial allocation count %d, err %v", count, err)
	}
}
