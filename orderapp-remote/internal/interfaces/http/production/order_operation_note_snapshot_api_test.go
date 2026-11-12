package production

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestPlanSubmitFreezesOrderNotesForEachManufacturingStage(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "MB-ROASTED-OPEN", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "MB-GREEN-OPEN", "生产生豆", 20000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "MB-BAG-OPEN", "227g包装袋", 100)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS operation_notes_json jsonb NOT NULL DEFAULT '[]';
 INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(901,'包装'),(902,'烘焙');
 UPDATE %[1]s.process_route_operations SET operation_id=CASE route_id WHEN 31 THEN 901 ELSE 902 END;
 UPDATE %[1]s.order_items SET operation_notes_json=jsonb_build_array(
 jsonb_build_object('output_type','product','output_id',1,'route_id',31,'step_id',(SELECT id FROM %[1]s.process_route_operations WHERE route_id=31),'operation_id',901,'operation_name','包装','note','门店贴标'),
 jsonb_build_object('output_type','material','output_id',10,'route_id',32,'step_id',(SELECT id FROM %[1]s.process_route_operations WHERE route_id=32),'operation_id',902,'operation_name','烘焙','note','浅烘')) WHERE order_id=1;`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22700}})
	if create.Code != 200 {
		t.Fatalf("create %d %s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,CASE LOWER(inventory_unit) WHEN 'kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%d`, schema, planID))
	selectPlanningTestSources(t, e, planID)
	submit := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != 200 {
		t.Fatalf("submit %d %s", submit.Code, submit.Body.String())
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT output_type,related_orders_snapshot_json::text FROM %s.work_orders WHERE production_plan_id=$1 ORDER BY output_type`, schema), planID)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := map[string]string{}
	for rows.Next() {
		var kind, raw string
		if err = rows.Scan(&kind, &raw); err != nil {
			t.Fatal(err)
		}
		snapshots[kind] = raw
	}
	rows.Close()
	if !strings.Contains(snapshots["product"], "门店贴标") || strings.Contains(snapshots["product"], "浅烘") || !strings.Contains(snapshots["material"], "浅烘") || strings.Contains(snapshots["material"], "门店贴标") {
		t.Fatalf("snapshots mismatch %+v", snapshots)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.order_items SET operation_notes_json='[]' WHERE order_id=1`, schema))
	var retained string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT related_orders_snapshot_json::text FROM %s.work_orders WHERE production_plan_id=$1 AND output_type='material'`, schema), planID).Scan(&retained); err != nil || !strings.Contains(retained, "浅烘") {
		t.Fatalf("not frozen: %s %v", retained, err)
	}
}
