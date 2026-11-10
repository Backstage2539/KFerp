package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestOperationConversionRejectsInvalidOrForgedFrozenTypes(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88952)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=%[2]d;UPDATE %[1]s.job_cards SET sequence_no=1 WHERE work_order_id=%[2]d;INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,2,'包装','pending',22700)`, schema, wo))
	setRoute := func(execution string) {
		t.Helper()
		raw, _ := json.Marshal(execution)
		route := fmt.Sprintf(`{"operations":[{"seq":1,"operation":"烘焙","execution_json":%s},{"seq":2,"operation":"包装"}]}`, raw)
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb WHERE id=$1`, schema), wo, route); err != nil {
			t.Fatal(err)
		}
	}
	preview := func() app.OperationConversionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/production/work-orders/%d/conversion-preview", wo), nil)
		if res.Code != 200 {
			t.Fatalf("preview: %s", res.Body.String())
		}
		var p app.OperationConversionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, config := range []string{`{"reporting_type":"piece"}`, `{"version":0,"reporting_type":"batch"}`, `{"version":2,"reporting_type":"incoming"}`, `{"version":"bad","reporting_type":"piece"}`} {
		setRoute(config)
		p := preview()
		if !strings.Contains(p.ReviewBlockedReason, "版本") {
			t.Fatalf("invalid frozen mode was not blocked: %+v", p)
		}
		res := serveMultilevelProductionJSON(t, e, "POST", "/api/production/operation-conversion", map[string]any{"work_order_id": wo, "cards": p.Cards, "materials": []any{}, "reason": "test", "request_id": "invalid-conversion", "expected_version": p.Version})
		if res.Code == 200 || !strings.Contains(res.Body.String(), "版本") {
			t.Fatalf("invalid conversion: %s", res.Body.String())
		}
	}
	setRoute(`{"version":1,"reporting_type":"batch","node_key":"roast","output_unit":"g","source_warehouse":"wip"}`)
	p := preview()
	if p.Cards[0].Execution.ReportingType != "batch" {
		t.Fatalf("type missing: %+v", p.Cards[0])
	}
	p.Cards[0].Execution.ReportingType = "piece"
	materials := []app.OperationMaterialUse{}
	for _, m := range p.Materials {
		materials = append(materials, m.OperationMaterialUse)
	}
	res := serveMultilevelProductionJSON(t, e, "POST", "/api/production/operation-conversion", map[string]any{"work_order_id": wo, "cards": p.Cards, "materials": materials, "reason": "test", "request_id": "forged-conversion", "expected_version": p.Version})
	if res.Code == 200 || !strings.Contains(res.Body.String(), "类型") {
		t.Fatalf("forged conversion: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "request_id IN('invalid-conversion','forged-conversion')", 0)
}
