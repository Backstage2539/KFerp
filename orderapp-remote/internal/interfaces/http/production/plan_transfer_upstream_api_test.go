package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	app "orderapp/internal/application/production"
)

// Upstream quantities remain allocated to production even if matching stock
// later appears in a warehouse. Automatic suggestions cover only the planned
// stock portion; explicit transfers can move extra stock without replacing upstream supply.
func TestPlanTransferAPIHonoursUpstreamSupply(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		initialStock, wantStock, wantUpstream int64
	}{
		{"pure upstream", 0, 0, 22700},
		{"mixed upstream and stock", 10000, 10000, 12700},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=0 WHERE id=10`, schema))
			if tc.initialStock > 0 {
				seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "UPSTREAM-SUPPLY-TEST", "在制熟豆", tc.initialStock)
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=1010`, schema))
			}
			e := newProductionFlowTestEcho(pool, schema)
			created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22700}})
			var plan app.ProductionPlanDetail
			if created.Code != http.StatusOK || json.Unmarshal(created.Body.Bytes(), &plan) != nil {
				t.Fatalf("create plan %d %s", created.Code, created.Body.String())
			}
			// Add real stock to a source warehouse only after the plan snapshot; this
			// intentionally does not alter the frozen upstream dependency for the test.
			if tc.initialStock == 0 {
				seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "UPSTREAM-SUPPLY-TEST", "在制熟豆", 20000)
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=1010`, schema))
			} else {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET qty_g=20000,remaining_g=20000,received_g=20000 WHERE id=1010; UPDATE %[1]s.material_batch_locations SET qty_g=20000 WHERE material_batch_id=1010`, schema))
			}
			detail := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
			if detail.Code != http.StatusOK || json.Unmarshal(detail.Body.Bytes(), &plan) != nil {
				t.Fatalf("get plan %d %s", detail.Code, detail.Body.String())
			}
			var source app.ProductionPlanComponentSource
			for _, s := range plan.ComponentSources {
				if s.ComponentType == "material" && s.ComponentID == 10 {
					source = s
					break
				}
			}
			if source.ID == 0 || source.RequiredG != tc.wantStock || source.UpstreamG != tc.wantUpstream {
				t.Fatalf("source %+v, expected stock %d upstream %d", source, tc.wantStock, tc.wantUpstream)
			}
			previewURL := fmt.Sprintf("/api/production-plans/%d/material-transfers/preview", plan.ID)
			createURL := fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID)
			batchItem := func(qty int64) app.PlanMaterialTransferItem {
				return app.PlanMaterialTransferItem{SourceID: source.ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "UPSTREAM-SUPPLY-TEST", QtyG: qty}}}
			}
			checkPreview := func(items []app.PlanMaterialTransferItem) app.PlanMaterialTransferPreview {
				t.Helper()
				rec := serveMultilevelProductionJSON(t, e, http.MethodPost, previewURL, app.PlanMaterialTransferCommand{DraftToken: plan.DraftToken, Items: items})
				var out app.PlanMaterialTransferPreview
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
					t.Fatalf("preview %d %s", rec.Code, rec.Body.String())
				}
				return out
			}
			bulk := checkPreview(nil)
			var suggested int64
			for _, group := range bulk.Groups {
				for _, item := range group.Items {
					if item.SourceID == source.ID {
						for _, b := range item.Batches {
							suggested += b.QtyG
						}
					}
				}
			}
			if suggested != tc.wantStock {
				t.Fatalf("bulk suggested %d, expected %d; preview %+v", suggested, tc.wantStock, bulk)
			}
			upstreamUnavailable := false
			for _, row := range bulk.Unavailable {
				if row.SourceID == source.ID && strings.Contains(row.Reason, "前道") {
					upstreamUnavailable = true
				}
			}
			if !upstreamUnavailable {
				t.Fatalf("bulk preview did not explain unavailable upstream supply: %+v", bulk)
			}
			additional := checkPreview([]app.PlanMaterialTransferItem{batchItem(tc.wantStock + 1)})
			if len(additional.Unavailable) != 0 || len(additional.Groups) != 1 {
				t.Fatalf("explicit extra transfer should be allowed without replacing upstream %+v", additional)
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_material_transfer_links", fmt.Sprintf("production_plan_id=%d", plan.ID), 0)
			if tc.wantStock == 0 {
				return
			}
			// A valid partial draft reduces only the suggested stock portion; the
			// frozen upstream portion and explicit extra transfer remain separate.
			first := serveMultilevelProductionJSON(t, e, http.MethodPost, createURL, app.PlanMaterialTransferCommand{DraftToken: plan.DraftToken, IdempotencyKey: "upstream-partial", Items: []app.PlanMaterialTransferItem{batchItem(4000)}})
			if first.Code != http.StatusOK {
				t.Fatalf("valid partial create %d %s", first.Code, first.Body.String())
			}
			after := checkPreview(nil)
			suggested = 0
			for _, group := range after.Groups {
				for _, item := range group.Items {
					if item.SourceID == source.ID {
						for _, b := range item.Batches {
							suggested += b.QtyG
						}
					}
				}
			}
			if suggested != 6000 {
				t.Fatalf("bulk after pending=%d want 6000: %+v", suggested, after)
			}
			extra := checkPreview([]app.PlanMaterialTransferItem{batchItem(6001)})
			if len(extra.Unavailable) != 0 || len(extra.Groups) != 1 {
				t.Fatalf("explicit extra after pending should be allowed: %+v", extra)
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_material_transfer_links", fmt.Sprintf("production_plan_id=%d AND component_source_id=%d", plan.ID, source.ID), 1)
			var bulkItems []app.PlanMaterialTransferItem
			for _, group := range after.Groups {
				bulkItems = append(bulkItems, group.Items...)
			}
			bulkCommand := app.PlanMaterialTransferCommand{DraftToken: plan.DraftToken, IdempotencyKey: "upstream-bulk", Items: bulkItems}
			bulkCreate := serveMultilevelProductionJSON(t, e, http.MethodPost, createURL, bulkCommand)
			if bulkCreate.Code != http.StatusOK {
				t.Fatalf("create from bulk preview %d %s", bulkCreate.Code, bulkCreate.Body.String())
			}
			replay := serveMultilevelProductionJSON(t, e, http.MethodPost, createURL, bulkCommand)
			if replay.Code != http.StatusOK {
				t.Fatalf("retry bulk create %d %s", replay.Code, replay.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_material_transfer_links", fmt.Sprintf("production_plan_id=%d AND component_source_id=%d", plan.ID, source.ID), 2)
			assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1010 AND warehouse='raw_materials' AND qty_g=20000", 1)
		})
	}
}
