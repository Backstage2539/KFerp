package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFulfillmentOrderConfirmationSameOrderPostgres(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	e := newOrderAPITestEcho(pool, schema)
	employee := int64(5)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", employee); return next(c) }
	})
	payload := `{"order_date":"2026-09-10","customer_id":3,"pay_status_id":1,"ship_status_id":1,"product_id":["7"],"bom_spec_id":["9002"],"bom_variant_id":["9102"],"item_name":["橘皮乌龙"],"tier_id":["manual"],"unit_price":["88"],"qty":["1"],"unit":["件"],"spec":["454"],"request_id":"confirmation-test-new-order"}`
	req := httptest.NewRequest("POST", "/api/fulfillment/order", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("save=%d %s", rec.Code, rec.Body.String())
	}
	var saved struct {
		ID     int64  `json:"order_id"`
		No     string `json:"order_no"`
		Status string `json:"confirmation_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID <= 0 || saved.No == "" || saved.Status != "pending" {
		t.Fatalf("save=%+v", saved)
	}
	var total float64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), saved.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("unconfirmed order has receivable %v", total)
	}
	req = httptest.NewRequest("GET", fmt.Sprintf("/api/orders/%d/confirmation", saved.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"confirmation_status":"pending"`) {
		t.Fatalf("detail=%d %s", rec.Code, rec.Body.String())
	}

	request := func(method, path, body string, code int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != code {
			t.Fatalf("%s %s => %d want %d: %s", method, path, rec.Code, code, rec.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if dir := os.Getenv("ORDERAPP_TEST_EVIDENCE_DIR"); dir != "" {
		for name, value := range map[string]any{"form": request("GET", fmt.Sprintf("/api/order/form?edit_id=%d", saved.ID), "", 200), "confirmation": request("GET", fmt.Sprintf("/api/orders/%d/confirmation", saved.ID), "", 200)} {
			raw, _ := json.Marshal(value)
			if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	statusPath := fmt.Sprintf("/api/orders/%d/confirmation", saved.ID)
	detailPath := fmt.Sprintf("/api/orders/%d/detail", saved.ID)
	detail := request("GET", detailPath, "", 200)["edit_data"].(map[string]any)
	if detail["order_date"] != "2026-09-10" || len(detail["items"].([]any)) != 1 {
		t.Fatalf("current detail=%v", detail)
	}
	repository := postgressales.NewRepository(pool, schema)
	if err := repository.InlineUpdate(ctx, saved.ID, "测试", salesapp.InlineUpdateCommand{PayStatusID: "1", ShipStatusID: "1", Notes: "绕过确认"}); err == nil || !strings.Contains(err.Error(), "履约订单") {
		t.Fatalf("legacy inline bypass: %v", err)
	}
	// The repeat returns the same order; neither stock nor production can consume it.
	again := request("POST", "/api/fulfillment/order", payload, 200)
	if int64(again["order_id"].(float64)) != saved.ID {
		t.Fatal("duplicate order")
	}
	for _, sql := range []string{fmt.Sprintf("INSERT INTO %s.production_plan_items(production_plan_id,order_nos) VALUES(1,'%s')", schema, saved.No), fmt.Sprintf("INSERT INTO %s.work_orders(id,order_nos,status) VALUES(1,'%s','draft')", schema, saved.No), fmt.Sprintf("UPDATE %s.orders SET process_status_id=1 WHERE id=%d", schema, saved.ID), fmt.Sprintf("UPDATE %s.orders SET ship_status_id=(SELECT id FROM %s.ship_statuses WHERE name='已发货') WHERE id=%d", schema, schema, saved.ID)} {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Fatalf("pending execution allowed: %s", sql)
		}
	}
	for _, stmt := range []string{
		fmt.Sprintf("INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,bom_spec_id,bom_variant_id,batch_code,allocated_units) VALUES(%d,7,9002,9102,'BLOCKED',1)", schema, saved.ID),
		fmt.Sprintf("INSERT INTO %s.order_stock_deductions(order_id,product_id,bom_spec_id,bom_variant_id,batch_code,deducted_units) VALUES(%d,7,9002,9102,'BLOCKED',1)", schema, saved.ID),
		fmt.Sprintf("INSERT INTO %s.order_shipment_orders(shipment_id,order_id) VALUES(1,%d)", schema, saved.ID),
	} {
		if _, err := pool.Exec(ctx, stmt); err == nil || !strings.Contains(err.Error(), "订单待确认") {
			t.Fatalf("execution guard failed: %v", err)
		}
	}
	employee = 1
	request("POST", statusPath, `{"revision":1,"decision":"accepted"}`, 403)
	employee = 5
	request("POST", statusPath, `{"revision":1,"decision":"rejected"}`, 409)
	request("POST", statusPath, `{"revision":1,"decision":"accepted"}`, 200)
	request("POST", statusPath, `{"revision":1,"decision":"accepted"}`, 403)
	var no string
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT order_no,grand_total FROM %s.orders WHERE id=$1", schema), saved.ID).Scan(&no, &total); err != nil {
		t.Fatal(err)
	}
	if no != saved.No || total != 88 {
		t.Fatalf("confirmed identity/total %s %v", no, total)
	}
	detail = request("GET", detailPath, "", 200)["edit_data"].(map[string]any)
	var edited map[string]any
	json.Unmarshal([]byte(payload), &edited)
	edited["edit_id"] = saved.ID
	edited["edit_revision"] = detail["edit_revision"]
	edited["request_id"] = "confirmation-test-edit-order"
	edited["order_date"] = "2026-09-09"
	edited["qty"] = []string{"2"}
	editedBytes, _ := json.Marshal(edited)
	request("POST", "/api/fulfillment/order", string(editedBytes), 200)
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), saved.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 88 {
		t.Fatalf("pending edit changed accepted total: %v", total)
	}
	detail = request("GET", detailPath, "", 200)["edit_data"].(map[string]any)
	if detail["order_date"] != "2026-09-09" {
		t.Fatalf("pending edit date=%v", detail)
	}
	request("POST", statusPath, `{"revision":1,"decision":"accepted"}`, 409)
	request("POST", statusPath, `{"revision":2,"decision":"rejected","reason":"数量有误"}`, 200)
	detail = request("GET", detailPath, "", 200)["edit_data"].(map[string]any)
	if detail["order_date"] != "2026-09-10" {
		t.Fatalf("rejected edit did not restore accepted date=%v", detail)
	}
	edited["edit_revision"] = detail["edit_revision"]
	edited["request_id"] = "confirmation-test-edit-again"
	editedBytes, _ = json.Marshal(edited)
	request("POST", "/api/fulfillment/order", string(editedBytes), 200)
	request("POST", statusPath, `{"revision":3,"decision":"accepted"}`, 200)
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), saved.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 176 {
		t.Fatalf("accepted edit total=%v", total)
	}
	detail = request("GET", detailPath, "", 200)["edit_data"].(map[string]any)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("INSERT INTO %s.work_orders(id,order_nos,status) VALUES(1,'%s','running')", schema, saved.No))
	edited["edit_revision"] = detail["edit_revision"]
	edited["request_id"] = "confirmation-test-started-edit"
	editedBytes, _ = json.Marshal(edited)
	request("POST", "/api/fulfillment/order", string(editedBytes), 400) // An old creation retry remains idempotent even after edits replaced the current request key.
	oldAgain := request("POST", "/api/fulfillment/order", payload, 200)
	if int64(oldAgain["order_id"].(float64)) != saved.ID {
		t.Fatal("old retry duplicated an edited order")
	}

	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.orders", schema)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("orders=%d", count)
	}

	// New rejection keeps the same order available for revision; failed validation rolls back.
	var second map[string]any
	json.Unmarshal([]byte(payload), &second)
	second["request_id"] = "confirmation-new-rejected-order"
	bytes, _ := json.Marshal(second)
	secondSaved := request("POST", "/api/fulfillment/order", string(bytes), 200)
	secondID := int64(secondSaved["order_id"].(float64))
	secondPath := fmt.Sprintf("/api/orders/%d/confirmation", secondID)
	request("POST", secondPath, `{"revision":1,"decision":"rejected","reason":"请调整数量"}`, 200)
	secondStatus := request("GET", secondPath, "", 200)
	if secondStatus["confirmation_status"] != "rejected" {
		t.Fatalf("new rejection=%v", secondStatus)
	}
	secondDetail := request("GET", fmt.Sprintf("/api/orders/%d/detail", secondID), "", 200)["edit_data"].(map[string]any)
	second["edit_id"] = secondID
	second["edit_revision"] = secondDetail["edit_revision"]
	second["request_id"] = "confirmation-new-rejected-resave"
	bytes, _ = json.Marshal(second)
	request("POST", "/api/fulfillment/order", string(bytes), 200)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.products SET active=false WHERE id=7", schema))
	request("POST", secondPath, `{"revision":2,"decision":"accepted"}`, 409)
	secondStatus = request("GET", secondPath, "", 200)
	if secondStatus["confirmation_status"] != "pending" {
		t.Fatal("validation failure changed status")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), secondID).Scan(&total); err != nil || total != 0 {
		t.Fatalf("failed confirmation altered money=%v err=%v", total, err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.products SET active=true WHERE id=7", schema))
	// A late audit failure must roll back restored content, metadata and history.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_confirmation_audit() RETURNS trigger LANGUAGE plpgsql AS $f$ BEGIN IF NEW.action='confirm_accepted' AND NEW.entity_id=%[2]d THEN RAISE EXCEPTION 'confirmation audit unavailable'; END IF; RETURN NEW; END $f$;CREATE TRIGGER fail_confirmation_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_confirmation_audit()`, schema, secondID))
	request("POST", secondPath, `{"revision":2,"decision":"accepted"}`, 409)
	if state := request("GET", secondPath, "", 200); state["confirmation_status"] != "pending" {
		t.Fatal("late failure committed confirmation")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), secondID).Scan(&total); err != nil || total != 0 {
		t.Fatalf("late failure committed content: %v %v", total, err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("DROP TRIGGER fail_confirmation_audit ON %s.audit_logs;DROP FUNCTION %s.fail_confirmation_audit()", schema, schema))
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", secondPath, strings.NewReader(`{"revision":2,"decision":"accepted"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			codes <- rec.Code
		}()
	}
	wg.Wait()
	close(codes)
	successes := 0
	for code := range codes {
		if code == 200 {
			successes++
		} else if code != 403 && code != 409 {
			t.Fatalf("unexpected concurrency response %d", code)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent approvals=%d", successes)
	}

	// Warehouse batches advance shipment status by cumulative quantity, not the first batch.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.orders SET receiver_name='收件人',receiver_phone='13800000000',receiver_address='测试地址' WHERE id=%d;INSERT INTO %s.order_stock_deductions(order_id,product_id,bom_spec_id,bom_variant_id,batch_code,deducted_units,operator) VALUES(%d,7,9002,9102,'STATUS-BATCH-1',1,'测试仓管')", schema, saved.ID, schema, saved.ID))
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.orders SET ship_status_id=(SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1) WHERE id=%d", schema, schema, saved.ID))
	shipped := request("GET", statusPath, "", 200)
	if shipped["ship_status"] != "部分发货" {
		t.Fatalf("partial shipment=%v", shipped)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("INSERT INTO %s.order_stock_deductions(order_id,product_id,bom_spec_id,bom_variant_id,batch_code,deducted_units,operator) VALUES(%d,7,9002,9102,'STATUS-BATCH-2',1,'测试仓管')", schema, saved.ID))
	shipped = request("GET", statusPath, "", 200)
	if shipped["ship_status"] != "已发货" {
		t.Fatalf("full shipment=%v", shipped)
	}

}

func seedConfirmationExecutionFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE IF NOT EXISTS %[1]s.customer_order_production_demands(order_id BIGINT);
 CREATE TABLE IF NOT EXISTS %[1]s.production_plans(id BIGINT PRIMARY KEY,status TEXT);
 CREATE TABLE IF NOT EXISTS %[1]s.production_plan_items(production_plan_id BIGINT,order_nos TEXT);
 CREATE TABLE IF NOT EXISTS %[1]s.work_orders(id BIGINT PRIMARY KEY,order_nos TEXT,status TEXT);
 CREATE TABLE IF NOT EXISTS %[1]s.produce_running_items(order_nos TEXT,status TEXT);
 CREATE TABLE IF NOT EXISTS %[1]s.produce_batches(batch_id BIGINT PRIMARY KEY,status TEXT);
 CREATE TABLE IF NOT EXISTS %[1]s.produce_batch_order_items(batch_id BIGINT,order_id BIGINT);
 `, schema))
}
