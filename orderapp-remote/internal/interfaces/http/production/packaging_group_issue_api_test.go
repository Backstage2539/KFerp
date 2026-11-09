package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	supporthttp "orderapp/internal/interfaces/http/support"
	"testing"
)

func TestPackagingGroupRefillThreeSimplifiedCards44Units(t *testing.T) {
	f := newBulkPickingFixtureUnit(t, "个")
	registerProductionWorkstationAPI(f.app, productionapp.NewService(postgresproduction.NewRepository(f.pool, f.schema)))
	f.app.Use(supporthttp.OperationLogMiddleware(f.pool, f.schema))
	workIDs := []int64{f.workOrderID, f.createWorkOrder(t, 20), f.createWorkOrder(t, 12)}
	cardIDs := make([]int64, 0, len(workIDs))
	ctx := context.Background()
	snapshot := `{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"receipt_warehouse\":\"wip\",\"source_warehouse\":\"wip\"}"}]}`
	for _, workID := range workIDs {
		if _, err := f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$1::jsonb WHERE id=$2`, f.schema), snapshot, workID); err != nil {
			t.Fatal(err)
		}
		var id int64
		if err := f.pool.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET operation='包装',workstation='包装台' WHERE work_order_id=$1 RETURNING id`, f.schema), workID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		cardIDs = append(cardIDs, id)
	}
	response := serveMultilevelProductionJSON(t, f.app, http.MethodGet, "/api/production/workstation-overview?scope=all", nil)
	var overview productionapp.ProductionWorkstationOverview
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &overview) != nil {
		t.Fatalf("overview: %d %s", response.Code, response.Body.String())
	}
	if len(overview.Tasks) != 3 {
		t.Fatalf("expected three packaging cards, got %+v", overview.Tasks)
	}
	for _, task := range overview.Tasks {
		if !task.AutomaticCompletion || task.StockAction != "" || task.MaterialReadinessState != "shortage" {
			t.Fatalf("simplified shortage card has no legacy issue action: %+v", task)
		}
	}
	preview := batchPreview(t, f, cardIDs)
	if len(preview.Blocked) != 0 || len(preview.Groups) != 1 || preview.Groups[0].Remaining != 44 || preview.Groups[0].Available != 44 || len(preview.Groups[0].Allocations) != 3 {
		t.Fatalf("three-card refill preview: %+v", preview)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
	command := batchCommand(preview, "packaging-three-cards-44", 44)
	for attempt := 0; attempt < 2; attempt++ {
		response = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", command)
		var result productionapp.BatchMaterialIssueResult
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.SuccessCount != 1 || result.FailedCount != 0 || len(result.Groups) != 1 || result.Groups[0].Quantity != 44 || len(result.Groups[0].Documents) != 3 {
			t.Fatalf("refill attempt %d: %d %s", attempt, response.Code, response.Body.String())
		}
	}
	var source, wip int64
	if err := f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_units) FILTER(WHERE warehouse='raw_materials'),0),COALESCE(SUM(qty_units) FILTER(WHERE warehouse='wip'),0) FROM %s.material_batch_locations WHERE material_batch_id=72230`, f.schema)).Scan(&source, &wip); err != nil {
		t.Fatal(err)
	}
	if source != 76 || wip != 44 {
		t.Fatalf("refill and replay should move exactly 44 units: source=%d WIP=%d", source, wip)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted'", 3)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservations", "consumed_units>0 OR consumed_g>0", 0)
	assertProductionFlowCount(t, f.pool, f.schema, "production_started_inputs", "true", 0)
	assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", "entity_type='workstation_material_issue' AND action='submit'", 1)
	assertProductionFlowCount(t, f.pool, f.schema, "operation_logs", "method='POST' AND path='/api/production/workstation/material-issue/submit' AND status=200", 2)
	fresh := batchPreview(t, f, cardIDs)
	if len(fresh.Groups) != 0 || len(fresh.Blocked) != 0 {
		t.Fatalf("all three tasks should be supplied after refill: %+v", fresh)
	}
}
