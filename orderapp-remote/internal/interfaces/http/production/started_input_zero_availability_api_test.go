package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestBOMStartZeroWIPCanReplenishReservedRawMaterial(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74950, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	// The work order owns a raw-warehouse reservation, but its operation consumes WIP.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.materials SET onhand_g=22700 WHERE id=10;
		UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10;
		UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10;
	`, schema, work))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	preview := func(wantAvailable, wantShortage float64) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
		if res.Code != http.StatusOK {
			t.Fatalf("start preview: %d %s", res.Code, res.Body.String())
		}
		var wire struct {
			Materials []struct {
				MaterialID int64    `json:"material_id"`
				Available  *float64 `json:"available_qty"`
				Shortage   float64  `json:"shortage_qty"`
				Warehouse  string   `json:"warehouse"`
			} `json:"materials"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &wire); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, m := range wire.Materials {
			if m.MaterialID == 10 {
				found = true
				// The existing API omits zero; clients must still offer replenishment.
				available := 0.0
				if m.Available != nil {
					available = *m.Available
				}
				if available != wantAvailable || m.Shortage != wantShortage || m.Warehouse != "wip" {
					t.Fatalf("material availability: %+v; want available=%v shortage=%v in WIP", m, wantAvailable, wantShortage)
				}
			}
		}
		if !found {
			t.Fatal("preview omitted the reserved material")
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	preview(0, 22.7)
	res := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"job_card_ids": []int64{card}})
	var issue app.BatchMaterialIssuePreview
	if res.Code != http.StatusOK {
		t.Fatalf("issue preview: %d %s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &issue); err != nil {
		t.Fatal(err)
	}
	if len(issue.Groups) != 1 || issue.Groups[0].ItemID != 10 || issue.Groups[0].FromWarehouse != "raw_materials" {
		t.Fatalf("expected the work order's reserved raw material: %+v", issue)
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(issue, "zero-wip-replenish", 22.7))
	var issued app.BatchMaterialIssueResult
	if err := json.Unmarshal(res.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if res.Code != http.StatusOK || issued.SuccessCount != 1 || issued.FailedCount != 0 {
		t.Fatalf("replenishment: %d %s", res.Code, res.Body.String())
	}
	p := preview(22.7, 0)
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "zero-wip-start"})
	if res.Code != http.StatusOK {
		t.Fatalf("start after replenishment: %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND status='running'", card), 1)
}

func TestReplenishmentHonorsEditedActualStartInput(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74951, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=22700 WHERE id=10; UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10; UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10`, schema, work))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil || res.Code != 200 {
		t.Fatalf("start preview: %s %v", res.Body.String(), err)
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"job_card_ids": []int64{card}})
	var issue app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &issue); err != nil || res.Code != 200 || len(issue.Groups) != 1 {
		t.Fatalf("issue preview: %s %v", res.Body.String(), err)
	}
	actual := append([]app.OperationMaterialUse(nil), p.Materials...)
	for i := range actual {
		actual[i].Qty /= 2
	}
	input := &app.ReplenishmentStartInput{ExpectedVersion: p.Version, InputUnit: p.InputUnit, ActualInputQty: p.InputQty / 2, Materials: actual, DifferenceReason: "现场减少"}
	cmd := batchCommand(issue, "edited-too-much", 11.351)
	cmd.StartInput = input
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code == 200 {
		t.Fatalf("over shortage accepted: %s", res.Body.String())
	}
	cmd = batchCommand(issue, "edited-exact", 11.35)
	cmd.StartInput = input
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code != 200 {
		t.Fatalf("draft: %d %s", res.Code, res.Body.String())
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "edited-exact"})
	if res.Code != 200 {
		t.Fatalf("confirm: %d %s", res.Code, res.Body.String())
	}
}

func TestReplenishmentWithoutStartInputStillChecksActualWIP(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74952, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=45400 WHERE id=10; UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10; UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10`, schema, work))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 74953, 10, "EXISTING-WIP-74953", "在制熟豆", 22700)
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var start app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &start); err != nil || res.Code != 200 {
		t.Fatalf("preview: %s %v", res.Body.String(), err)
	}
	enough := false
	for _, m := range start.Materials {
		if m.MaterialID == 10 && m.AvailableQty >= m.Qty {
			enough = true
		}
	}
	if !enough {
		t.Fatalf("expected sufficient WIP: %+v", start.Materials)
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"job_card_ids": []int64{card}})
	var issue app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &issue); err != nil || len(issue.Groups) == 0 {
		t.Fatalf("issue preview: %s %v", res.Body.String(), err)
	}
	cmd := batchCommand(issue, "wip-already-enough", 1)
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code == 200 {
		t.Fatalf("draft with sufficient WIP accepted: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 0)
}

func TestReplenishmentThenStartAndCompleteEveryBatch(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74955, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=22700 WHERE id=10; UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10; UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10`, schema, work))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("/api/job-cards/%d", card)
	startPreview := func() app.OperationCompletionPreview {
		r := serveMultilevelProductionJSON(t, e, http.MethodGet, prefix+"/start-preview", nil)
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil || r.Code != 200 {
			t.Fatalf("start preview: %d %s %v", r.Code, r.Body.String(), err)
		}
		return p
	}
	p := startPreview()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_execution_batches WHERE job_card_id=%[2]d; UPDATE %[1]s.job_cards SET scheduling_mode='batch',batch_size_qty=%[3]f,batch_size_unit='%[4]s',planned_batch_count=2 WHERE id=%[2]d`, schema, card, p.InputQty/2, p.InputUnit))
	p = startPreview()
	res := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"job_card_ids": []int64{card}})
	var issue app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &issue); err != nil || res.Code != 200 || len(issue.Groups) != 1 {
		t.Fatalf("issue preview: %s %v", res.Body.String(), err)
	}
	cmd := batchCommand(issue, "full-flow-replenish", 22.7)
	cmd.StartInput = &app.ReplenishmentStartInput{ExpectedVersion: p.Version, InputUnit: p.InputUnit, ActualInputQty: p.InputQty, Materials: p.Materials}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code != 200 {
		t.Fatalf("draft: %d %s", res.Code, res.Body.String())
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "full-flow-replenish"})
	if res.Code != 200 {
		t.Fatalf("confirm: %d %s", res.Code, res.Body.String())
	}
	p = startPreview()
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, prefix+"/start", map[string]any{"request_id": "full-flow-start", "expected_version": p.Version, "input_unit": p.InputUnit, "actual_input_qty": p.InputQty, "materials": p.Materials})
	if res.Code != 200 {
		t.Fatalf("start: %d %s", res.Code, res.Body.String())
	}
	batchesURL := prefix + "/execution-batches"
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, batchesURL, nil)
	var list app.ExecutionBatchList
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil || res.Code != 200 {
		t.Fatalf("batches: %s %v", res.Body.String(), err)
	}
	if len(list.Batches) == 0 {
		res = serveMultilevelProductionJSON(t, e, http.MethodPost, batchesURL, map[string]any{"request_id": "full-flow-init", "expected_version": list.Version})
		if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil || res.Code != 200 {
			t.Fatalf("batch init: %d %s %v", res.Code, res.Body.String(), err)
		}
	}
	if len(list.Batches) != 2 {
		t.Fatalf("expected two batches: %+v", list)
	}
	for i, b := range list.Batches {
		res = serveMultilevelProductionJSON(t, e, http.MethodGet, prefix+"/completion-preview", nil)
		var completion app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &completion); err != nil || res.Code != 200 {
			t.Fatalf("completion preview: %s %v", res.Body.String(), err)
		}
		res = serveMultilevelProductionJSON(t, e, http.MethodPost, prefix+"/complete", map[string]any{"request_id": fmt.Sprintf("full-flow-%d", i), "expected_version": completion.Version, "input_unit": completion.InputUnit, "output_unit": completion.OutputUnit, "actual_input_qty": b.PlannedInputQty, "actual_output_qty": p.PlannedQty / 2, "execution_batch_id": b.ID, "difference_reason": "现场实测"})
		if res.Code != 200 {
			t.Fatalf("complete %d: %d %s", i, res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_execution_batches", fmt.Sprintf("job_card_id=%d AND status='completed'", card), 2)
}
