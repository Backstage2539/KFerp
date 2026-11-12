package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	pgproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestReplenishmentActualDeficitCanUseFreeBatchAfterOriginalReservationIsInWIP(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 75800, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(pgproduction.NewRepository(pool, schema)))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 75810, 10, "FREE-RAW-75810", "在制熟豆", 10000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=75810; UPDATE %[1]s.materials SET onhand_g=32700 WHERE id=10`, schema))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil || res.Code != 200 {
		t.Fatalf("start preview: %s %v", res.Body.String(), err)
	}
	actual := append([]app.OperationMaterialUse(nil), p.Materials...)
	for i := range actual {
		if actual[i].MaterialID == 10 {
			actual[i].Qty += 0.241
		}
	}
	input := &app.ReplenishmentStartInput{ExpectedVersion: p.Version, ActualInputQty: p.InputQty, InputUnit: p.InputUnit, Materials: actual, DifferenceReason: "按现场实际补料"}
	request := map[string]any{"workflow": "replenishment", "job_card_ids": []int64{card}, "start_input": input}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", request)
	var preview app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil || res.Code != 200 {
		t.Fatalf("issue preview: %s %v", res.Body.String(), err)
	}
	if len(preview.Groups) != 1 || preview.Groups[0].ItemID != 10 || preview.Groups[0].Available != 241 || preview.Groups[0].MaxQuantity != 10000 {
		t.Fatalf("actual shortage must find unreserved raw stock: %+v", preview)
	}
	g := preview.Groups[0]
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment'", 0)
	request["request_id"] = "free-replenishment-overage"
	request["preview_version"] = preview.Version
	request["groups"] = []app.BatchMaterialIssueSelection{{Key: g.Key, Version: g.Version, Quantity: 1}}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/submit", request)
	if res.Code == 200 {
		t.Fatal("replenishment bypassed the draft confirmation")
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", request)
	if res.Code != 200 {
		t.Fatalf("draft: %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment' AND status='draft'", 1)
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", request)
	if res.Code != 200 {
		t.Fatalf("draft retry: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment' AND status='draft'", 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=9000 WHERE material_batch_id=75810`, schema))
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "free-replenishment-overage"})
	if res.Code == 200 {
		t.Fatal("stale stock confirmation succeeded")
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment' AND status='draft'", 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=75810`, schema))
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/cancel", map[string]any{"request_id": "free-replenishment-overage"})
	if res.Code != 200 {
		t.Fatalf("cancel stale: %s", res.Body.String())
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil || res.Code != 200 {
		t.Fatalf("refresh start: %s %v", res.Body.String(), err)
	}
	input.ExpectedVersion = p.Version
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", request)
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil || res.Code != 200 || len(preview.Groups) != 1 {
		t.Fatalf("refresh preview: %s %v", res.Body.String(), err)
	}
	g = preview.Groups[0]
	request["request_id"] = "free-replenishment-rechecked"
	request["preview_version"] = preview.Version
	request["groups"] = []app.BatchMaterialIssueSelection{{Key: g.Key, Version: g.Version, Quantity: 1}}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", request)
	if res.Code != 200 {
		t.Fatalf("rechecked draft: %s", res.Body.String())
	}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "free-replenishment-rechecked"})
		if res.Code != 200 {
			t.Fatalf("confirm: %d %s", res.Code, res.Body.String())
		}
	}
	var raw, wip, reserved int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g) FILTER(WHERE warehouse='raw_materials'),0),COALESCE(SUM(qty_g) FILTER(WHERE warehouse='wip'),0) FROM %s.material_batch_locations WHERE material_batch_id=75810`, schema)).Scan(&raw, &wip); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT reserved_g FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=10`, schema), work).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if raw != 9000 || wip != 1000 || reserved != 22700 {
		t.Fatalf("free transfer should not consume or enlarge reservation: raw=%d wip=%d reserved=%d", raw, wip, reserved)
	}
}

func TestReplenishmentFreeSourceDoesNotReplaceUnavailableFrozenBatch(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 75900, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(pgproduction.NewRepository(pool, schema)))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 75910, 10, "FREE-RAW-75910", "在制熟豆", 10000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10; UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10; UPDATE %[1]s.material_batches SET quality_status='hold' WHERE id=75900; UPDATE %[1]s.materials SET onhand_g=32700 WHERE id=10`, schema, work))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"workflow": "replenishment", "job_card_ids": []int64{card}})
	var preview app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil || res.Code != 200 {
		t.Fatalf("preview: %s %v", res.Body.String(), err)
	}
	for _, g := range preview.Groups {
		if g.ItemID == 10 && g.MaxQuantity > 0 {
			t.Fatalf("replaced held frozen batch with free stock: %+v", g)
		}
	}
	if len(preview.Blocked) == 0 {
		t.Fatal("missing explicit unavailable source feedback")
	}
}

func TestReplenishmentFreeSourcesMultipleWarehousesRollbackTogether(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 76000, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(pgproduction.NewRepository(pool, schema)))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 76010, 10, "FREE-RAW-76010", "在制熟豆", 400)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 76011, 10, "FREE-OTHER-76011", "在制熟豆", 600)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse=CASE WHEN material_batch_id=76010 THEN 'raw_materials' ELSE 'packaging' END WHERE material_batch_id IN(76010,76011); UPDATE %[1]s.materials SET onhand_g=23700 WHERE id=10`, schema))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil || res.Code != 200 {
		t.Fatalf("start: %s %v", res.Body.String(), err)
	}
	actual := append([]app.OperationMaterialUse(nil), p.Materials...)
	for i := range actual {
		if actual[i].MaterialID == 10 {
			actual[i].Qty += 1
		}
	}
	cmd := app.BatchMaterialIssueCommand{Workflow: "replenishment", JobCardIDs: []int64{card}, StartInput: &app.ReplenishmentStartInput{ExpectedVersion: p.Version, InputUnit: p.InputUnit, ActualInputQty: p.InputQty, Materials: actual, DifferenceReason: "现场追加"}}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", cmd)
	var preview app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil || res.Code != 200 || len(preview.Groups) != 2 {
		t.Fatalf("preview: %s %v", res.Body.String(), err)
	}
	cmd.RequestID = "multiwarehouse-rollback"
	cmd.PreviewVersion = preview.Version
	for _, g := range preview.Groups {
		cmd.Groups = append(cmd.Groups, app.BatchMaterialIssueSelection{Key: g.Key, Version: g.Version, Quantity: float64(g.Available) / app.OperationUnitFactor(g.Unit)})
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code != 200 {
		t.Fatalf("draft: %s", res.Body.String())
	}
	failingBatch := 76010
	if preview.Groups[1].FromWarehouse == "packaging" {
		failingBatch = 76011
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_free_move() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF OLD.material_batch_id=%[2]d THEN RAISE EXCEPTION 'isolated second source failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_free_move BEFORE UPDATE ON %[1]s.material_batch_locations FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_free_move()`, schema, failingBatch))
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": cmd.RequestID})
	if res.Code == 200 {
		t.Fatal("second source failure accepted")
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment' AND status='submitted'", 0)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "return_source='workstation_replenishment' AND status='draft'", 1)
	var raw, wip int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g) FILTER(WHERE warehouse<>'wip'),0),COALESCE(SUM(qty_g) FILTER(WHERE warehouse='wip'),0) FROM %s.material_batch_locations WHERE material_batch_id IN(76010,76011)`, schema)).Scan(&raw, &wip); err != nil {
		t.Fatal(err)
	}
	if raw != 1000 || wip != 0 {
		t.Fatalf("partial warehouse transfer survived: raw=%d wip=%d", raw, wip)
	}
}

func TestReplenishmentActualPreviewPreservesReservedSource(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 76100, "bom_start_v1")
	registerProductionWorkstationAPI(e, app.NewService(pgproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10; UPDATE %[1]s.work_order_material_reservation_batches SET warehouse='raw_materials' WHERE work_order_id=%[2]d AND material_id=10`, schema, work))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"workflow": "replenishment", "job_card_ids": []int64{card}})
	var preview app.BatchMaterialIssuePreview
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil || res.Code != 200 || len(preview.Groups) != 1 {
		t.Fatalf("preview: %s %v", res.Body.String(), err)
	}
	cmd := batchCommand(preview, "reserved-enhanced-flow", 22.7)
	cmd.Workflow = "replenishment"
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if res.Code != 200 {
		t.Fatalf("draft: %s", res.Body.String())
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": cmd.RequestID})
	if res.Code != 200 {
		t.Fatalf("confirm: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND material_id=10 AND material_batch_id=76100 AND warehouse='wip' AND reserved_g=22700", work), 1)
}
