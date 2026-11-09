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
