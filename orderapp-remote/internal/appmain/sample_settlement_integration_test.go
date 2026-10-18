package appmain

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	app "orderapp/internal/application/samples"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	saleshttp "orderapp/internal/interfaces/http/sales"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSampleFreeSettlementAPI(t *testing.T) {
	f := newSampleFixture(t)
	f.e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("X-Test-Admin") == "yes" {
				c.Set("employee_id", f.admin.EmployeeID)
				c.Set("operator_employee", "样品管理员")
				c.Set("actor", "样品管理员")
			}
			return next(c)
		}
	})
	saleshttp.RegisterRoutes(f.e, saleshttp.Dependencies{Sales: salesapp.NewService(salespg.NewRepository(f.pool, f.schema)), Authz: wechatTestAuthz{}})
	spec := f.validBOM(f.productIDs[0])
	source := f.id("INSERT INTO %s.sources(name) VALUES('结清测试') RETURNING id")
	typ := f.id("INSERT INTO %s.order_types(name) VALUES('样品') RETURNING id")
	v := f.assign(f.submit("free-settlement"), f.sales.EmployeeID)
	cmd := app.ConfirmCommand{Revision: v.Revision, NewCustomer: &app.NewCustomer{Name: "零元结清客户", CustomerType: "wholesale", SourceID: source, OrderTypeID: typ}, Receiver: v.Receiver, Items: []app.ConfirmItem{{ProductID: f.productIDs[0], BomSpecID: spec, Qty: 1, UnitPrice: 0}}}
	v = sampleDecode(t, f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/confirm", v.ID), cmd, f.salesToken, 200))
	var reason, pay, name, phone, address string
	var goods, freight float64
	err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT COALESCE(to_jsonb(o)->>'settlement_reason',''),p.name,o.receiver_name,o.receiver_phone,o.receiver_address,o.payment_goods_amount,o.payment_shipping_amount FROM %[1]s.orders o JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1", f.schema), v.OrderID).Scan(&reason, &pay, &name, &phone, &address, &goods, &freight)
	if err != nil {
		t.Fatal(err)
	}
	if reason != "free_sample" || (pay != "已收款" && pay != "已付款") || goods != 0 || freight != 0 {
		t.Fatalf("free sample not settled: reason=%q pay=%q goods=%v freight=%v", reason, pay, goods, freight)
	}
	if name != v.Receiver.Name || phone != v.Receiver.Phone || address != v.Receiver.Address {
		t.Fatal("confirmation lost recipient")
	}
	if err := salespg.NewRepository(f.pool, f.schema).InlineUpdate(f.ctx, v.OrderID, "test", salesapp.InlineUpdateCommand{}); err == nil || !strings.Contains(err.Error(), "免费样品请从订单详情顶部保存") {
		t.Fatalf("legacy edit bypassed free settlement: %v", err)
	}
	if dir := os.Getenv("PR697_ARTIFACT_DIR"); dir != "" {
		os.MkdirAll(dir, 0755)
		form := f.call("GET", fmt.Sprintf("/api/order/form?edit_id=%d", v.OrderID), nil, "basic-admin", 200)
		body, _ := json.Marshal(form)
		if err := os.WriteFile(filepath.Join(dir, "free-sample-form.json"), body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("ERP read edit save preserves recipient and source and rejects stale edits", func(t *testing.T) {
		f.t = t
		payload := sampleERPBody(f, v.OrderID)
		payload["notes"] = "只修改备注"
		// Simulate an old UI erroneously transmitting a fulfillment marker.
		payload["portal_service_code"] = "direct_ship"
		payload["orders_scope"] = "fulfillment"
		f.call("POST", "/api/order", payload, "basic-admin", 200)
		f.call("POST", "/api/order", payload, "basic-admin", 409)
		var code, address, reason string
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf("SELECT portal_service_code,receiver_address,settlement_reason FROM %s.orders WHERE id=$1", f.schema), v.OrderID).Scan(&code, &address, &reason); err != nil {
			t.Fatal(err)
		}
		if code != "" || address != v.Receiver.Address || reason != "free_sample" {
			t.Fatalf("edit lost snapshot: %q %q %q", code, address, reason)
		}
		if n := f.id("SELECT count(*) FROM %s.order_audit_logs WHERE order_id=$1 AND field='order'", v.OrderID); n == 0 {
			t.Fatal("missing audit")
		}
	})
	t.Run("API read exposes no-payment semantics", func(t *testing.T) {
		f.t = t
		got := f.call("GET", fmt.Sprintf("/api/orders?order_id=%d", v.OrderID), nil, "basic-admin", 200)
		raw, _ := json.Marshal(got)
		if !strings.Contains(string(raw), "无需收款") || !strings.Contains(string(raw), "free_sample") {
			t.Fatalf("list lacks settlement: %s", raw)
		}
	})
	t.Run("zero goods with charged freight reopens settlement then requires real receipt", func(t *testing.T) {
		f.t = t
		payload := sampleERPBody(f, v.OrderID)
		payload["shipping_amount"] = "8.50"
		f.call("POST", "/api/order", payload, "basic-admin", 200)
		if n := f.id("SELECT count(*) FROM %[1]s.orders o JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1 AND settlement_reason='' AND p.name IN ('未付款','未收款') AND grand_total=8.50", v.OrderID); n != 1 {
			t.Fatal("free-to-paid retained settlement")
		}
		paid := f.id("INSERT INTO %s.pay_statuses(name) VALUES('已收款') RETURNING id")
		payload = sampleERPBody(f, v.OrderID)
		payload["settlement_reason"] = "free_sample"
		payload["pay_status_id"] = paid
		payload["payment_method"] = "微信"
		payload["payment_goods_amount"] = "0"
		payload["payment_shipping_amount"] = "8.50"
		f.call("POST", "/api/order", payload, "basic-admin", 400)
		voucher := f.id("INSERT INTO %s.sales_order_assets(kind,filename,content_type,bytes,sha256,object_key,created_by) VALUES('payment_voucher','test.png','image/png',1,'test','test.png','test') RETURNING id")
		payload["payment_voucher_asset_id"] = voucher
		f.call("POST", "/api/order", payload, "basic-admin", 200)
		payload = sampleERPBody(f, v.OrderID)
		payload["shipping_amount"] = "0"
		payload["payment_goods_amount"] = "0"
		payload["payment_shipping_amount"] = "0"
		payload["payment_voucher_asset_id"] = 0
		f.call("POST", "/api/order", payload, "basic-admin", 400)
		if n := f.id("SELECT count(*) FROM %s.orders WHERE id=$1 AND payment_shipping_amount=8.50 AND payment_voucher_asset_id=$2 AND grand_total=8.50", v.OrderID, voucher); n != 1 {
			t.Fatal("failed free settlement erased actual receipt")
		}
	})
	t.Run("missing receiver rejected before shipment and transaction rolls back", func(t *testing.T) {
		f.t = t
		f.exec("UPDATE %s.orders SET receiver_address='' WHERE id=$1", v.OrderID)
		response := f.call("POST", fmt.Sprintf("/api/orders/%d/shipping-tracking", v.OrderID), map[string]any{"tracking_no": "TEST-697"}, "basic-admin", 400)
		if !strings.Contains(string(response["error"]), "收件") {
			t.Fatalf("not recipient failure: %v", response)
		}
		if n := f.id("SELECT count(*) FROM %s.order_shipping_trackings WHERE order_id=$1", v.OrderID); n != 0 {
			t.Fatal("failed shipment retained tracking")
		}
		f.exec("UPDATE %s.orders SET receiver_address=$2 WHERE id=$1", v.OrderID, v.Receiver.Address)
		variant := f.id("SELECT bom_variant_id FROM %s.order_items WHERE order_id=$1", v.OrderID)
		f.exec("INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,onhand_units,onhand_loose_g) VALUES($1,$2,$3,0,1,0)", f.productIDs[0], spec, variant)
		f.call("POST", fmt.Sprintf("/api/orders/%d/shipping-tracking", v.OrderID), map[string]any{"tracking_no": "TEST-697"}, "basic-admin", 200)
	})

}

func TestSampleRecipientRepairTransaction(t *testing.T) {
	f := newSampleFixture(t)
	spec := f.validBOM(f.productIDs[0])
	source := f.id("INSERT INTO %s.sources(name) VALUES('修复测试') RETURNING id")
	typ := f.id("INSERT INTO %s.order_types(name) VALUES('样品') RETURNING id")
	v := f.assign(f.submit("repair-test"), f.sales.EmployeeID)
	cmd := app.ConfirmCommand{Revision: v.Revision, NewCustomer: &app.NewCustomer{Name: "修复测试客户", CustomerType: "wholesale", SourceID: source, OrderTypeID: typ}, Receiver: v.Receiver, Items: []app.ConfirmItem{{ProductID: f.productIDs[0], BomSpecID: spec, Qty: 1, UnitPrice: 0}}}
	v = sampleDecode(t, f.call("POST", fmt.Sprintf("/api/mini/employee/sample-requests/%d/confirm", v.ID), cmd, f.salesToken, 200))
	_, err := f.pool.Exec(f.ctx, fmt.Sprintf(`UPDATE %s.orders SET receiver_name='保留人工修改',receiver_phone='',receiver_address='',receiver_company='保留公司',settlement_reason='',pay_status_id=(SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1) WHERE id=$1`, f.schema, f.schema), v.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("scripts/repair_sample_order_receipt.sql")
	if err != nil {
		t.Fatal(err)
	}
	run := func(fail bool) {
		t.Helper()
		tx, err := f.pool.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(f.ctx)
		if _, err = tx.Exec(f.ctx, fmt.Sprintf(`SET LOCAL search_path TO %s,public`, f.schema)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, `SELECT set_config('kferp.repair_order_no',(SELECT order_no FROM orders WHERE id=$1),true)`, v.OrderID); err != nil {
			t.Fatal(err)
		}
		if fail {
			_, err = tx.Exec(f.ctx, `CREATE FUNCTION fail_repair_audit() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'injected audit failure'; END$$; CREATE TRIGGER fail_repair BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION fail_repair_audit()`)
			if err != nil {
				t.Fatal(err)
			}
		}
		_, err = tx.Exec(f.ctx, string(script))
		if fail {
			if err == nil || !strings.Contains(err.Error(), "injected audit failure") {
				t.Fatalf("expected audit rollback: %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	run(true)
	if n := f.id("SELECT count(*) FROM %s.orders WHERE id=$1 AND receiver_address='' AND settlement_reason=''", v.OrderID); n != 1 {
		t.Fatal("failed repair was not rolled back")
	}
	run(false)
	run(false)
	if n := f.id("SELECT count(*) FROM %s.orders WHERE id=$1 AND receiver_name='保留人工修改' AND receiver_company='保留公司' AND receiver_phone=$2 AND receiver_address=$3 AND settlement_reason='free_sample' AND portal_service_code=''", v.OrderID, v.Receiver.Phone, v.Receiver.Address); n != 1 {
		t.Fatal("repair lost snapshot or modified existing fields")
	}
	if n := f.id("SELECT count(*) FROM %s.order_audit_logs WHERE order_id=$1 AND field='sample_receipt_repair'", v.OrderID); n != 1 {
		t.Fatal("repair audit not idempotent")
	}
}

func sampleERPBody(f *sampleFixture, id int64) map[string]any {
	f.t.Helper()
	form := f.call("GET", fmt.Sprintf("/api/order/form?edit_id=%d", id), nil, "basic-admin", 200)
	var data map[string]any
	if err := json.Unmarshal(form["edit_data"], &data); err != nil {
		f.t.Fatal(err)
	}
	body := map[string]any{"edit_id": id, "edit_revision": data["edit_revision"], "order_date": data["order_date"], "document_date": data["document_date"], "prepayment_amount": "0"}
	for _, key := range []string{"customer_id", "source_id", "order_type_id", "pay_status_id", "ship_status_id", "logistics_company_id", "logistics_product_id", "payment_voucher_asset_id"} {
		v, _ := strconv.ParseInt(fmt.Sprint(data[key]), 10, 64)
		body[key] = v
	}
	for _, key := range []string{"receiver_name", "receiver_phone", "receiver_address", "receiver_company", "notes", "shipping_amount", "ship_method", "payment_method", "payment_goods_amount", "payment_shipping_amount"} {
		body[key] = fmt.Sprint(data[key])
	}
	for _, key := range []string{"item_id", "product_id", "parent_product_id", "bom_spec_id", "bom_variant_id", "qty", "unit_price", "sales_unit", "item_bean_list_publication_id"} {
		values := []string{}
		for _, item := range data["items"].([]any) {
			m := item.(map[string]any)
			value := m[key]
			if key == "item_bean_list_publication_id" {
				value = m["bean_list_publication_id"]
			}
			if value == nil {
				value = 0
			}
			values = append(values, fmt.Sprint(value))
		}
		body[key] = values
	}
	body["tier_id"] = []string{"manual"}
	body["price_selection_mode"] = []string{"manual"}
	return body
}
