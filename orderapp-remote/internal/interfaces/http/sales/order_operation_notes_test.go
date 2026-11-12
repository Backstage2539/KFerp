package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	materialspg "orderapp/internal/infrastructure/postgres/materials"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestOrderOperationNotesRequestPreservesOmittedAndEmpty(t *testing.T) {
	if notes, err := parseOrderOperationNotes(0, nil); err != nil || notes != nil {
		t.Fatalf("omitted notes: %+v %v", notes, err)
	}
	if notes, err := parseOrderOperationNotes(0, []string{"[]"}); err != nil || notes == nil || len(*notes) != 0 {
		t.Fatalf("explicit clear: %+v %v", notes, err)
	}
	if _, err := parseOrderOperationNotes(0, []string{"bad"}); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestOrderOperationNotesAreMappedPerOrderLine(t *testing.T) {
	req := CreateOrderRequest{ItemName: []string{"曲奇", "初晓"}, ProductID: []string{"1", "2"}, ItemOperationNotes: []string{`[{"output_type":"material","output_id":3,"route_id":5,"step_id":7,"operation_id":8,"operation_name":"烘焙","note":"浅烘"}]`, `[]`}}
	items := orderItemCommandsFromCreateRequest(req)
	if len(items) != 2 || items[0].OperationNotes == nil || len(*items[0].OperationNotes) != 1 || (*items[0].OperationNotes)[0].Note != "浅烘" || items[1].OperationNotes == nil || len(*items[1].OperationNotes) != 0 {
		t.Fatalf("notes: %+v", items)
	}
	req.ItemOperationNotes = nil
	items = orderItemCommandsFromCreateRequest(req)
	if items[0].OperationNotes != nil {
		t.Fatal("missing note fields must remain omitted")
	}
}

func TestOrderAPIReceivesStructuredOperationNotes(t *testing.T) {
	repo := &capturingSaveOrderRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			return next(c)
		}
	})
	registerOrderAPI(e, salesapp.NewService(repo), nil)
	payload := map[string]any{"order_date": "2026-05-09", "customer_id": 3, "source_id": 1, "order_type_id": 1, "pay_status_id": 2, "payment_method": "微信支付", "ship_status_id": 1, "product_id": []string{"7"}, "tier_id": []string{"manual"}, "unit_price": []string{"88"}, "item_name": []string{"橘皮乌龙"}, "item_operation_notes": []string{`[{"output_type":"material","output_id":3,"route_id":5,"step_id":7,"operation_id":8,"operation_name":"烘焙","note":"浅烘"}]`}, "qty": []string{"2"}, "unit": []string{"件"}, "spec": []string{"454"}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || len(repo.cmd.Items) != 1 || repo.cmd.Items[0].OperationNotes == nil || (*repo.cmd.Items[0].OperationNotes)[0].Note != "浅烘" {
		t.Fatalf("status %d body %s items %+v", rec.Code, rec.Body.String(), repo.cmd.Items)
	}
}

func TestERPOrderOldClientPreservesOperationNotesAcrossRebuild(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	if err := materialspg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("materials fixture: %v", err)
	}
	if err := stockpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock fixture: %v", err)
	}
	if err := messagepg.EnsureBusinessEventSchema(ctx, pool, schema); err != nil {
		t.Fatalf("business event fixture: %v", err)
	}
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	for _, table := range []string{"sales_order_documents", "sales_order_images", "delivery_note_documents"} {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.%s(order_id bigint,is_latest boolean)`, schema, table))
	}
	for _, table := range []string{"combined_sales_order_documents", "combined_sales_order_images", "combined_delivery_note_documents"} {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.%s(order_ids jsonb,is_latest boolean)`, schema, table))
	}
	// The identity guard checks this optional workflow even for an ordinary
	// order insert. Keep its empty lookup table without disabling the guard.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.order_resolution_stock_returns(id bigint PRIMARY KEY, source_kind text);`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN delivery_mode text NOT NULL DEFAULT 'ship', ADD COLUMN delivery_warehouse_code text NOT NULL DEFAULT '', ADD COLUMN delivery_status text NOT NULL DEFAULT '', ADD COLUMN fulfillment_mode text NOT NULL DEFAULT '';`, schema))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.order_items ADD COLUMN operation_notes_json jsonb NOT NULL DEFAULT '[]', ADD COLUMN customer_product_reference_id bigint, ADD COLUMN material_source_mode text;`, schema))
	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{"order_date": "2026-08-17", "customer_id": 3, "pay_status_id": 1, "ship_status_id": 1, "product_id": []string{"7"}, "parent_product_id": []string{"7"}, "bom_spec_id": []string{"9001"}, "bom_variant_id": []string{"9101"}, "item_name": []string{"橘皮乌龙 227g 袋装"}, "tier_id": []string{"manual"}, "unit_price": []string{"88"}, "qty": []string{"2"}, "unit": []string{"袋"}, "spec": []string{"227g 袋装"}}
	post := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	rec := post()
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var saved struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	var itemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_items WHERE order_id=$1`, schema), saved.OrderID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.order_items SET operation_notes_json='[{"output_type":"product","output_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"route_id":5,"step_id":7,"operation_id":8,"operation_name":"包装","note":"贴标签"}]' WHERE id=%d`, schema, itemID))
	editReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/order/form?edit_id=%d", saved.OrderID), nil)
	editRec := httptest.NewRecorder()
	e.ServeHTTP(editRec, editReq)
	if editRec.Code != 200 || !strings.Contains(editRec.Body.String(), "贴标签") {
		t.Fatalf("form %d %s", editRec.Code, editRec.Body.String())
	}
	payload["edit_id"] = saved.OrderID
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	rec = post()
	if rec.Code != 200 {
		t.Fatalf("edit %d %s", rec.Code, rec.Body.String())
	}
	var raw string
	var rebuiltItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,operation_notes_json::text FROM %s.order_items WHERE order_id=$1`, schema), saved.OrderID).Scan(&rebuiltItemID, &raw); err != nil || !strings.Contains(raw, "贴标签") {
		t.Fatalf("notes lost: %s %v", raw, err)
	}
	if rebuiltItemID == itemID {
		t.Fatal("fixture must exercise order-item replacement")
	}
	// A newer client explicitly sending [] must clear the same note which the
	// older client's omitted field preserved through the ordinary edit API.
	payload["item_id"] = []string{fmt.Sprint(rebuiltItemID)}
	payload["item_operation_notes"] = []string{"[]"}
	rec = post()
	if rec.Code != http.StatusOK {
		t.Fatalf("clear %d %s", rec.Code, rec.Body.String())
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT operation_notes_json::text FROM %s.order_items WHERE order_id=$1`, schema), saved.OrderID).Scan(&raw); err != nil || raw != "[]" {
		t.Fatalf("explicit empty notes not cleared: %s %v", raw, err)
	}
	var clearAudits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND field='operation_notes' AND new_value='[]' AND actor='测试员'`, schema), saved.OrderID).Scan(&clearAudits); err != nil || clearAudits != 1 {
		t.Fatalf("clear audit count %d: %v", clearAudits, err)
	}
}
