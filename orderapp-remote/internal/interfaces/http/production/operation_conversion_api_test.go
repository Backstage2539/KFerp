package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestOperationConversionAPIReconcilesRunningOrderBeforeContinuing(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72901)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET process_snapshot_json='{"operations":[{"seq":1,"operation":"烘焙"},{"seq":2,"operation":"包装"}]}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=%[2]d;UPDATE %[1]s.job_cards SET sequence_no=1 WHERE work_order_id=%[2]d;INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,2,'包装','pending',22700)`, schema, woID))
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/production/work-orders/%d/conversion-preview", woID), nil)
	if res.Code != 200 {
		t.Fatalf("preview %d %s", res.Code, res.Body.String())
	}
	var p app.OperationConversionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	q := 11350.0
	p.Cards[0].InputQty = &q
	p.Cards[0].OutputQty = &q
	p.Cards[0].InputUnit = "g"
	materials := []app.OperationMaterialUse{}
	for _, m := range p.Materials {
		v := m.OperationMaterialUse
		if v.MaterialID == 10 {
			v.NodeKey = p.Cards[0].Execution.NodeKey
			v.Qty = 11.35
		} else {
			v.NodeKey = p.Cards[1].Execution.NodeKey
			v.Qty = 0
		}
		materials = append(materials, v)
	}
	payload := map[string]any{"work_order_id": woID, "cards": p.Cards, "materials": materials, "reason": "现场核对前序实际11350g，包装尚未做", "request_id": "conversion-729", "expected_version": p.Version}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, "POST", "/api/production/operation-conversion", payload)
		if res.Code != 200 {
			t.Fatalf("conversion %d %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_balances", "remaining_qty=11350", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 0)
	res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", p.Cards[1].ID), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var next app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &next); err != nil {
		t.Fatal(err)
	}
	if !next.Enabled || next.BlockedReason != "" || len(next.Materials) != 1 || next.Materials[0].MaterialID != 20 {
		t.Fatalf("converted next %+v", next)
	}
	next.Materials[0].Qty = 50
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", p.Cards[1].ID), map[string]any{"actual_input_qty": 11350, "actual_output_qty": 50, "input_unit": next.InputUnit, "output_unit": next.OutputUnit, "expected_version": next.Version, "request_id": "converted-last", "difference_reason": "实际已全部包装50袋", "materials": next.Materials})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=10 AND consumed_g=11350", woID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "action='operation_conversion'", 1)
}

func TestOperationConversionAPISplitsCompletedBatchCost(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72902)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET process_snapshot_json='{"operations":[{"seq":1,"operation":"烘焙"},{"seq":2,"operation":"包装"}]}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET sequence_no=1,actual_operation_cost=0 WHERE work_order_id=%[2]d; UPDATE %[1]s.material_batches SET unit_cost=80 WHERE material_id=10;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'烘焙','completed',11350),(%[2]d,2,'包装','pending',22700);`, schema, woID))
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/production/work-orders/%d/conversion-preview", woID), nil)
	var p app.OperationConversionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	q := 11350.0
	for i := 0; i < 2; i++ {
		p.Cards[i].InputQty = &q
		p.Cards[i].OutputQty = &q
		p.Cards[i].InputUnit = "g"
	}
	materials := []app.OperationMaterialUse{}
	for _, m := range p.Materials {
		v := m.OperationMaterialUse
		if v.MaterialID == 10 {
			v.NodeKey = p.Cards[0].Execution.NodeKey
			v.Qty = 22.7
		} else {
			v.NodeKey = p.Cards[2].Execution.NodeKey
			v.Qty = 0
		}
		materials = append(materials, v)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", "/api/production/operation-conversion", map[string]any{"work_order_id": woID, "cards": p.Cards, "materials": materials, "reason": "现场核实两批各11350g", "request_id": "conversion-multi", "expected_version": p.Version})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var costA, costB float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(total_cost)::float8,MAX(total_cost)::float8 FROM %s.production_operation_balances WHERE work_order_id=$1`, schema), woID).Scan(&costA, &costB); err != nil {
		t.Fatal(err)
	}
	if costA <= 0 || costA != costB {
		t.Fatalf("equal confirmed batches must each carry own half of material cost: %g / %g", costA, costB)
	}
}
