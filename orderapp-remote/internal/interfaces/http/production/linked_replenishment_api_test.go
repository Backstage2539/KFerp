package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestLinkedReplenishmentPreviewCreateIdempotencyAndWorkOrderAllocation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12010, 10, "MB-ROASTED-RESERVED", "在制熟豆", 10_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12030, 30, "MB-GREEN-RESERVED", "生产生豆", 20_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 12020, 20, "MB-BAG-RESERVED", "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)

	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create original plan status=%d body=%s", created.Code, created.Body.String())
	}
	var originalPlan struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &originalPlan); err != nil {
		t.Fatalf("decode original plan: %v", err)
	}
	preparePlanningTest(t, ctx, pool, schema, app, originalPlan.ID)
	submitted := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", originalPlan.ID), nil)
	if submitted.Code != http.StatusOK {
		t.Fatalf("submit original plan status=%d body=%s", submitted.Code, submitted.Body.String())
	}
	var sourceID, consumerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT supplier.id,consumer.id
		FROM %s.work_order_dependencies dependency
		JOIN %s.work_orders supplier ON supplier.id=dependency.depends_on_work_order_id
		JOIN %s.work_orders consumer ON consumer.id=dependency.work_order_id
		WHERE supplier.production_plan_id=$1 AND supplier.output_type='material' AND supplier.output_material_id=10
		  AND consumer.production_plan_id=$1 AND consumer.output_type='product' AND consumer.output_product_id=1
	`, schema, schema, schema), originalPlan.ID).Scan(&sourceID, &consumerID); err != nil {
		t.Fatalf("load source and consumer work orders: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='completed' WHERE id=$1`, schema), sourceID); err != nil {
		t.Fatalf("complete isolated source: %v", err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12031, 30, "MB-GREEN-SUPPLEMENT", "生产生豆", 100_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12011, 10, "MB-ROASTED-FREE", "在制熟豆", 2_000)

	body := map[string]any{"source_work_order_id": sourceID, "consumer_work_order_ids": []int64{consumerID}}
	previewRec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment/preview", body)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("linked replenishment preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	var preview struct {
		Version      string  `json:"version"`
		OutputQty    float64 `json:"output_qty"`
		ReplenishG   int64   `json:"replenish_g"`
		AvailableG   int64   `json:"available_g"`
		MaterialName string  `json:"material_name"`
		Consumers    []struct {
			WorkOrderID int64 `json:"work_order_id"`
			ReplenishG  int64 `json:"replenish_g"`
		} `json:"consumers"`
	}
	if err := json.Unmarshal(previewRec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode linked preview: %v", err)
	}
	if preview.Version == "" || preview.AvailableG != 2_000 || preview.ReplenishG != 10_700 || preview.OutputQty != 10.7 || preview.MaterialName != "在制熟豆" || len(preview.Consumers) != 1 || preview.Consumers[0].WorkOrderID != consumerID || preview.Consumers[0].ReplenishG != 10_700 {
		t.Fatalf("linked preview=%+v, want free stock deducted from 12.7kg shortage before creating a 10.7kg supplement", preview)
	}

	request := map[string]any{
		"source_work_order_id":    sourceID,
		"consumer_work_order_ids": []int64{consumerID},
		"expected_version":        preview.Version,
		"request_id":              "linked-replenishment-idempotent-1",
	}
	planRec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment", request)
	if planRec.Code != http.StatusOK {
		t.Fatalf("create linked replenishment status=%d body=%s", planRec.Code, planRec.Body.String())
	}
	var linkedPlan struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(planRec.Body.Bytes(), &linkedPlan); err != nil {
		t.Fatalf("decode linked plan: %v", err)
	}
	if linkedPlan.ID <= 0 {
		t.Fatalf("linked plan id=%d", linkedPlan.ID)
	}
	retry := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment", request)
	if retry.Code != http.StatusOK {
		t.Fatalf("idempotent create retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	var retriedPlan struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(retry.Body.Bytes(), &retriedPlan); err != nil || retriedPlan.ID != linkedPlan.ID {
		t.Fatalf("idempotent retry plan=%+v err=%v, original=%d", retriedPlan, err, linkedPlan.ID)
	}
	changedRequest := map[string]any{
		"source_work_order_id":    sourceID,
		"consumer_work_order_ids": []int64{consumerID},
		"expected_version":        preview.Version + "changed",
		"request_id":              "linked-replenishment-idempotent-1",
	}
	if changed := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment", changedRequest); changed.Code == http.StatusOK {
		t.Fatal("same idempotency key accepted changed request content")
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d", linkedPlan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_replenishment_plan_links", fmt.Sprintf("production_plan_id=%d AND consumer_work_order_id=%d AND required_g=10700", linkedPlan.ID, consumerID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='create_linked_replenishment' AND actor='测试员'", linkedPlan.ID), 1)
	coveredPreview := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment/preview", body)
	var covered struct {
		OutputQty float64 `json:"output_qty"`
	}
	if coveredPreview.Code != http.StatusOK || json.Unmarshal(coveredPreview.Body.Bytes(), &covered) != nil || covered.OutputQty != 0 {
		t.Fatalf("draft coverage preview status=%d body=%s, want no second supplement", coveredPreview.Code, coveredPreview.Body.String())
	}

	preparePlanningTest(t, ctx, pool, schema, app, linkedPlan.ID)
	supplementSubmit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", linkedPlan.ID), nil)
	if supplementSubmit.Code != http.StatusOK {
		t.Fatalf("submit linked replenishment status=%d body=%s", supplementSubmit.Code, supplementSubmit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", fmt.Sprintf("work_order_id=%d AND material_id=10 AND required_g=10700 AND delivered_g=0", consumerID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(`id IN (SELECT link.production_plan_item_id FROM %s.production_replenishment_plan_links link WHERE link.production_plan_id=%d) AND process_route_snapshot_json=(SELECT source_item.process_route_snapshot_json FROM %s.work_orders source JOIN %s.production_plan_items source_item ON source_item.id=source.production_plan_item_id WHERE source.id=%d) AND component_snapshot_json=(SELECT source_item.component_snapshot_json FROM %s.work_orders source JOIN %s.production_plan_items source_item ON source_item.id=source.production_plan_item_id WHERE source.id=%d)`, schema, linkedPlan.ID, schema, schema, sourceID, schema, schema, sourceID), 1)
}

func TestLinkedReplenishmentRejectsStalePreviewVersion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12110, 10, "MB-ROASTED-STALE", "在制熟豆", 10_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12130, 30, "MB-GREEN-STALE", "生产生豆", 20_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 12120, 20, "MB-BAG-STALE", "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22_700}})
	if created.Code != http.StatusOK {
		t.Fatalf("create original plan status=%d body=%s", created.Code, created.Body.String())
	}
	var originalPlan struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &originalPlan); err != nil {
		t.Fatal(err)
	}
	preparePlanningTest(t, ctx, pool, schema, app, originalPlan.ID)
	submitted := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", originalPlan.ID), nil)
	if submitted.Code != http.StatusOK {
		t.Fatalf("submit original plan status=%d body=%s", submitted.Code, submitted.Body.String())
	}
	var sourceID, consumerID, reservationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT supplier.id,consumer.id,reservation.id
		FROM %s.work_order_dependencies dependency
		JOIN %s.work_orders supplier ON supplier.id=dependency.depends_on_work_order_id
		JOIN %s.work_orders consumer ON consumer.id=dependency.work_order_id
		JOIN %s.work_order_material_reservations reservation ON reservation.work_order_id=consumer.id AND reservation.material_id=dependency.material_id
		WHERE supplier.production_plan_id=$1 AND supplier.output_type='material' AND supplier.output_material_id=10
		  AND consumer.production_plan_id=$1 AND consumer.output_type='product' AND consumer.output_product_id=1
		LIMIT 1
	`, schema, schema, schema, schema), originalPlan.ID).Scan(&sourceID, &consumerID, &reservationID); err != nil {
		t.Fatalf("load source, consumer and reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='completed' WHERE id=$1`, schema), sourceID); err != nil {
		t.Fatal(err)
	}
	appPreview := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment/preview", map[string]any{"source_work_order_id": sourceID, "consumer_work_order_ids": []int64{consumerID}})
	if appPreview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", appPreview.Code, appPreview.Body.String())
	}
	var preview struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(appPreview.Body.Bytes(), &preview); err != nil || preview.Version == "" {
		t.Fatalf("decode preview=%+v err=%v", preview, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=required_g+1 WHERE id=$1`, schema), reservationID); err != nil {
		t.Fatal(err)
	}
	stale := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/work-orders/replenishment", map[string]any{
		"source_work_order_id": sourceID, "consumer_work_order_ids": []int64{consumerID}, "expected_version": preview.Version, "request_id": "linked-replenishment-stale-1",
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale linked replenishment status=%d body=%s", stale.Code, stale.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "source_type='stock' AND status='draft'", 0)
}
