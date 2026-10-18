package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestOrderAPIRetainsCustomerStockDeliveryFields(t *testing.T) {
	var request orderSaveAPIRequest
	if err := json.Unmarshal([]byte(`{"order_date":"2026-10-19","customer_id":74,"delivery_mode":"customer_stock","delivery_warehouse_code":"cust_74_finished"}`), &request); err != nil {
		t.Fatal(err)
	}
	cmd, err := saveOrderCommandFromCreateRequest(request.toCreateRequest(), 0, "员工")
	if err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(cmd)
	for field, want := range map[string]string{"DeliveryMode": "customer_stock", "DeliveryWarehouseCode": "cust_74_finished"} {
		got := v.FieldByName(field)
		if !got.IsValid() || got.Kind() != reflect.String || got.String() != want {
			t.Errorf("%s not retained through API mapping", field)
		}
	}
}

func TestCustomerStockOrderAPIReadsDeliverySnapshot(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS material_source_mode TEXT NOT NULL DEFAULT 'factory';
 ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_reference_id BIGINT NOT NULL DEFAULT 0;
 CREATE TABLE %[1]s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL DEFAULT 'legacy',configured BOOLEAN NOT NULL DEFAULT false);
 ALTER TABLE %[1]s.orders ADD COLUMN delivery_mode TEXT NOT NULL DEFAULT 'ship';
 ALTER TABLE %[1]s.orders ADD COLUMN delivery_warehouse_code TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.orders ADD COLUMN delivery_status TEXT NOT NULL DEFAULT '';
 ALTER TABLE %[1]s.orders ADD COLUMN fulfillment_mode TEXT NOT NULL DEFAULT '';
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,source_id,order_type_id,pay_status_id,ship_status_id,process_status_id,delivery_mode,delivery_warehouse_code,delivery_status,fulfillment_mode)
 VALUES(703,'SOURCE-703','2026-10-19',3,1,1,2,1,1,'customer_stock','cust_3_finished','partially_received','');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,qty,unit,spec,unit_price,line_total)VALUES(703,1,7,'测试规格',24,'袋','454g',47,1128);
 `, schema))
	e := newOrderAPITestEcho(pool, schema)
	for _, path := range []string{"/api/orders?limit=20", "/api/order/form?edit_id=703"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		for _, fragment := range []string{`"delivery_mode":"customer_stock"`, `"delivery_warehouse_code":"cust_3_finished"`, `"delivery_status":"partially_received"`} {
			if !strings.Contains(rec.Body.String(), fragment) {
				t.Errorf("path=%s missing %s", path, fragment)
			}
		}
	}
}
