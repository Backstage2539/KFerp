package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
)

// The competing transfer holds the same material lock used by stock posting.
// Change its location after the request has calculated its initial candidates,
// but before it acquires that lock. The initial result must not over-transfer.
func TestDirectSelectedBatchConcurrentExternalMove(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CHOSEN-RACE", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',15000)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plans SET semi_finished_stock_policy='manual' WHERE id=%[2]d;
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'raw_materials',1030,15000,0),(%[2]d,'material:30',30,'wip',1030,6000,0)`, schema, plan.ID))
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil {
		t.Fatalf("get %d %s", get.Code, get.Body.String())
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE id=30 FOR UPDATE`, schema)); err != nil {
		t.Fatal(err)
	}
	type reply struct {
		code int
		body string
	}
	responses := make(chan reply, 1)
	go func() {
		r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "race"})
		responses <- reply{r.Code, r.Body.String()}
	}()
	deadline := time.Now().Add(10 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var n int
		err = pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE $1`, "%"+schema+".materials WHERE id=ANY%").Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			blocked = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("direct transfer did not reach the inventory lock")
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET qty_g=11000 WHERE material_batch_id=1030 AND warehouse='wip'; UPDATE %[1]s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=1030 AND warehouse='raw_materials'`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var response reply
	select {
	case response = <-responses:
	case <-time.After(10 * time.Second):
		t.Fatal("direct transfer remained blocked")
	}
	var wip, raw int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT qty_g FROM %[1]s.material_batch_locations WHERE material_batch_id=1030 AND warehouse='wip'),(SELECT qty_g FROM %[1]s.material_batch_locations WHERE material_batch_id=1030 AND warehouse='raw_materials')`, schema)).Scan(&wip, &raw); err != nil {
		t.Fatal(err)
	}
	if wip+raw != 21000 || wip > 21000 || raw < 0 {
		t.Fatalf("stock duplicated or overdrawn after concurrent transfer: response=%d %s wip=%d raw=%d", response.code, response.body, wip, raw)
	}
	if response.code == 200 && (wip != 21000 || raw != 0) {
		t.Fatalf("successful transfer did not fully move the remainder: %s wip=%d raw=%d", response.body, wip, raw)
	}
	if response.code != 200 && (wip != 11000 || raw != 10000) {
		t.Fatalf("failed transfer wrote partial data: %s wip=%d raw=%d", response.body, wip, raw)
	}
}
