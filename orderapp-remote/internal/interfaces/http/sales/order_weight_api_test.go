package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"testing"
)

func TestOrderAPIQuantityEditPreservesFrozenWeightAndHistoryUnknown(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.customer_order_production_demands(id bigserial PRIMARY KEY,order_id bigint,line_no int,customer_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,target_qty numeric,material_source_mode text,status text,migrated_at timestamptz,migration_note text DEFAULT '',created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),UNIQUE(order_id,line_no))`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
 ALTER TABLE %[1]s.production_bom_version_variants ADD COLUMN logistics_weight_g numeric(20,6);
 UPDATE %[1]s.production_bom_version_variants SET logistics_weight_g=250 WHERE id=9101;
 UPDATE %[1]s.production_bom_version_variants SET logistics_weight_g=500 WHERE id=9102;
 CREATE TABLE %[1]s.materials(id bigint PRIMARY KEY,unit text);
 INSERT INTO %[1]s.materials VALUES(7061,'KG'),(7062,'个');
 CREATE TABLE %[1]s.production_bom_version_items(id bigint PRIMARY KEY,version_id bigint,variant_id bigint,material_id bigint,component_type text,consume_unit text,qty_per_unit numeric,ratio_pct numeric,material_loss_rate numeric);
 INSERT INTO %[1]s.production_bom_version_items VALUES(7061,8101,9101,7061,'material','fixed_qty',.227,0,20),(7062,8101,9101,7062,'material','fixed_qty',1,0,0),(7063,8101,9102,7061,'material','fixed_qty',.454,0,20);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{"order_date": "2026-10-20", "customer_id": 3, "pay_status_id": 1, "ship_status_id": 1, "product_id": []string{"7"}, "parent_product_id": []string{"7"}, "bom_spec_id": []string{"9001"}, "bom_variant_id": []string{"9101"}, "item_name": []string{"227g袋装"}, "tier_id": []string{"manual"}, "unit_price": []string{"88"}, "qty": []string{"2"}}
	save := func() int64 {
		t.Helper()
		body, _ := json.Marshal(payload)
		q := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
		q.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, q)
		if rec.Code != http.StatusOK {
			t.Fatalf("save order: %d %s", rec.Code, rec.Body.String())
		}
		var got struct {
			ID int64 `json:"order_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.ID
	}
	id := save()
	var initialMaterial, initialLogistics float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8 FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&initialMaterial, &initialLogistics); err != nil {
		t.Fatal(err)
	}
	if initialMaterial != 454 || initialLogistics != 500 {
		t.Fatalf("initial actual weights %v/%v want454/500", initialMaterial, initialLogistics)
	}

	var itemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET material_unit_weight_g=227,material_item_weight_g=454,logistics_unit_weight_g=250,logistics_item_weight_g=500,weight_snapshot_json='{"rule_version":1,"product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"spec_g":0,"spec":"227g 袋装","unit":"袋","qty":2}'::jsonb WHERE id=$1`, schema), itemID)
	if err != nil {
		t.Fatal(err)
	}
	payload["edit_id"] = id
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["qty"] = []string{"4"}
	save()
	var material, logistics float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8,id FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&material, &logistics, &itemID); err != nil {
		t.Fatal(err)
	}
	if material != 908 || logistics != 1000 {
		t.Fatalf("quantity edit weights %v/%v want 908/1000", material, logistics)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/orders/%d/detail", id), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"logistics_item_weight_g":"1000.000000"`) {
		t.Fatalf("weight detail: %d %s", rec.Code, rec.Body.String())
	}

	// Republishing the same stable specification must not replace frozen source weights.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`
 UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=8101;
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status)VALUES(8102,8001,'V002','published');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,logistics_weight_g)VALUES(9103,8102,9001,'新版227g袋','袋',true,1,600),(9104,8102,9002,'新版454g袋','袋',false,2,500);
 UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=8102 WHERE bom_id=8001;
 INSERT INTO %[1]s.production_bom_version_items VALUES(7064,8102,9103,7061,'material','fixed_qty',.3,0,20),(7065,8102,9104,7061,'material','fixed_qty',.454,0,20);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["bom_variant_id"] = []string{"9103"}
	save()
	var sourceVariant int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8,id,(weight_snapshot_json->>'bom_variant_id')::bigint FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&material, &logistics, &itemID, &sourceVariant); err != nil {
		t.Fatal(err)
	}
	if material != 908 || logistics != 1000 || sourceVariant != 9101 {
		t.Fatalf("same stable spec replaced source weight %v/%v variant%d", material, logistics, sourceVariant)
	}
	// Simulate a pre-feature row: edits must preserve unknown rather than fetch current BOM.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET material_unit_weight_g=NULL,material_item_weight_g=NULL,logistics_unit_weight_g=NULL,logistics_item_weight_g=NULL,weight_snapshot_json='{}'::jsonb WHERE id=$1`, schema), itemID); err != nil {
		t.Fatal(err)
	}
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["qty"] = []string{"5"}
	save()
	var materialUnknown, logisticsUnknown bool
	var snapshot string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g IS NULL,logistics_item_weight_g IS NULL,weight_snapshot_json::text FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&materialUnknown, &logisticsUnknown, &snapshot); err != nil {
		t.Fatal(err)
	}
	if !materialUnknown || !logisticsUnknown || !strings.Contains(snapshot, `"historical_unknown": true`) {
		t.Fatalf("historical weights %v/%v snapshot=%s", materialUnknown, logisticsUnknown, snapshot)
	}
	// A copied order is new business: even supplied old IDs must resolve current weights.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_version_variants SET logistics_weight_g=300 WHERE id=9103`, schema)); err != nil {
		t.Fatal(err)
	}
	delete(payload, "edit_id")
	payload["qty"] = []string{"1"}
	copiedID := save()
	var copiedMaterial, copiedLogistics float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8 FROM %s.order_items WHERE order_id=$1`, schema), copiedID).Scan(&copiedMaterial, &copiedLogistics); err != nil {
		t.Fatal(err)
	}
	if copiedMaterial != 300 || copiedLogistics != 300 {
		t.Fatalf("copied order reused old weights %v/%v", copiedMaterial, copiedLogistics)
	}
	// Changing specification on the same product resolves that specification's own weights.
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	payload["edit_id"] = id
	payload["item_id"] = []string{fmt.Sprint(itemID)}
	payload["bom_spec_id"] = []string{"9002"}
	payload["bom_variant_id"] = []string{"9104"}
	payload["qty"] = []string{"3"}
	save()
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_item_weight_g::float8,logistics_item_weight_g::float8 FROM %s.order_items WHERE order_id=$1`, schema), id).Scan(&material, &logistics); err != nil {
		t.Fatal(err)
	}
	if material != 1362 || logistics != 1500 {
		t.Fatalf("changed spec weights %v/%v want1362/1500", material, logistics)
	}

}
