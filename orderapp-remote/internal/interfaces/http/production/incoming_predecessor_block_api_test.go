package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestIncomingCannotStartUntilAllPredecessorsCompleteEvenWithBalance(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88951)
	route := `{"operations":[{"seq":1,"operation":"first","execution_json":"{\"version\":1,\"node_key\":\"first\",\"reporting_type\":\"batch\",\"output_unit\":\"kg\"}"},{"seq":2,"operation":"sort","execution_json":"{\"version\":1,\"node_key\":\"sort\",\"reporting_type\":\"incoming\",\"output_unit\":\"kg\"}"},{"seq":3,"operation":"pack","execution_json":"{\"version\":1,\"node_key\":\"last\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json='%[2]s'::jsonb WHERE id=%[3]d;
 UPDATE %[1]s.job_cards SET sequence_no=1,status='completed' WHERE work_order_id=%[3]d;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,workstation,status,planned_input_qty) VALUES(%[3]d,1,'first','炉','running',1000),(%[3]d,2,'sort','分选','pending',84000),(%[3]d,3,'pack','包装','pending',84000);
 INSERT INTO %[1]s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,warehouse,payload) SELECT %[3]d,id,'complete','posted','partial-source','v','test',100,'kg',84,'kg','wip','{}' FROM %[1]s.job_cards WHERE work_order_id=%[3]d AND sequence_no=1 AND status='completed';
 INSERT INTO %[1]s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost) SELECT id,work_order_id,job_card_id,1,'wip','kg',84,84,100,100 FROM %[1]s.production_operation_events WHERE request_id='partial-source';`, schema, route, wo))
	var sortID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&sortID); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", sortID), nil)
	if res.Code != 200 {
		t.Fatalf("preview: %s", res.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.BlockedReason, "前序工序未完成") {
		t.Fatalf("partial upstream must block preview: %+v", p)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", sortID), map[string]any{"actual_input_qty": 84, "input_unit": "kg", "expected_version": p.Version, "request_id": "must-block-partial-source", "materials": p.Materials})
	if res.Code == 200 || !strings.Contains(res.Body.String(), "前序工序未完成") {
		t.Fatalf("partial upstream start not blocked: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_handoffs", fmt.Sprintf("job_card_id=%d", sortID), 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "request_id='must-block-partial-source'", 0)
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND metrics_json ? 'incoming_allocation'", sortID), 0)
}
