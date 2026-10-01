package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/costing"
	"os"
	"testing"
	"time"
)

func TestPurchasedMaterialTrialAllowsZeroCostPostgres(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated postgres")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("pc_zero_raw_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.materials(id bigint,code text,name text,is_semi_finished bool,unit text,purchase_price numeric,estimated_unit_price numeric,deprecated_at timestamptz);
 CREATE TABLE %[1]s.material_batches(id bigint,unit_cost numeric,status text,quality_status text);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,material_id bigint,qty_g numeric,qty_units numeric);
	 INSERT INTO %[1]s.materials VALUES(1,'RAW','快乐樱桃-生豆',false,'kg',0,NULL,NULL);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	got, err := NewRepository(pool, schema).LoadMaterialCostTrial(ctx, app.MaterialCostTrialCommand{MaterialID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.CostStatus != "complete" || got.UnitCost != 0 || len(got.UnresolvedComponents) != 0 || got.BaseCostDetails[0].Description != "暂无采购成本，暂按 0 计算" {
		t.Fatalf("zero-cost purchased trial: %+v", got)
	}
}
