package production

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	app "orderapp/internal/application/production"
	productionrepo "orderapp/internal/infrastructure/postgres/production"
)

func TestWorkstationPredecessorUsesPostedEventUnit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88940)
	registerProductionWorkstationAPI(e, app.NewService(productionrepo.NewRepository(pool, schema)))
	route := `{"operations":[{"seq":1,"operation":"sort","execution_json":"{\"version\":1,\"node_key\":\"sort\",\"reporting_type\":\"incoming\",\"output_unit\":\"kg\"}"},{"seq":2,"operation":"assembly","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"reporting_type\":\"piece\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,material_consumption_policy='bom_start_v1',inventory_unit='袋' WHERE id=$1`, schema), wo, route); err != nil {
		t.Fatal(err)
	}
	var first, second int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1,operation='sort',status='completed',actual_output_qty=83 WHERE work_order_id=$1 RETURNING id`, schema), wo).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,status) VALUES($1,2,'assembly','包装台','pending') RETURNING id`, schema), wo).Scan(&second); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,warehouse,payload) VALUES(%[2]d,%[3]d,'complete','posted','unit1','v1','test',84,'kg',83,'kg','wip','{}')`, schema, wo, first))
	read := func() app.ProductionTask {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", "/api/production/workstation-overview?scope=all", nil)
		if res.Code != 200 {
			t.Fatalf("overview: %s", res.Body.String())
		}
		var overview app.ProductionWorkstationOverview
		if err := json.Unmarshal(res.Body.Bytes(), &overview); err != nil {
			t.Fatal(err)
		}
		for _, task := range overview.Tasks {
			if task.JobCardID == second {
				return task
			}
		}
		t.Fatalf("missing task: %+v", overview.Tasks)
		return app.ProductionTask{}
	}
	task := read()
	if task.PredecessorOutputQty != 83 || task.PredecessorOutputUnit != "kg" || task.PredecessorOutputUnitState != "known" || task.InventoryUnit != "袋" {
		t.Fatalf("wrong frozen predecessor unit: %+v", task)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_operation_events SET output_unit='' WHERE request_id='unit1'`, schema))
	task = read()
	if task.PredecessorOutputUnit != "" || task.PredecessorOutputUnitState != "missing" {
		t.Fatalf("missing unit inferred: %+v", task)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_operation_events SET output_unit='kg' WHERE request_id='unit1'; INSERT INTO %[1]s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,warehouse,payload) VALUES(%[2]d,%[3]d,'complete','posted','unit2','v1','test',1,'kg',1000,'g','wip','{}')`, schema, wo, first))
	task = read()
	if task.PredecessorOutputUnit != "" || task.PredecessorOutputUnitState != "mixed" {
		t.Fatalf("mixed units misrepresented: %+v", task)
	}
}
