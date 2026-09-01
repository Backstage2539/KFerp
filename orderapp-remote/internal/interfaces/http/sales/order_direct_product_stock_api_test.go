package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	postgressales "orderapp/internal/infrastructure/postgres/sales"

	"github.com/labstack/echo/v4"
)

func TestOrderAPIDirectProductStockUsesInventoryUnitsThroughShipment(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.stock_batches ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.stock_ledger_entries ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.finished_inventory ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.finished_inventory ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
		ALTER TABLE %[1]s.finished_inventory DROP CONSTRAINT IF EXISTS finished_inventory_pkey;
		ALTER TABLE %[1]s.finished_inventory ADD PRIMARY KEY(product_id,bom_spec_id,spec_g,warehouse);
	`, schema))
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("sales EnsureSchema: %v", err)
	}
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.product_bom_spec_migrations
		SET state='ready', spec_identity_mode='product', legacy_catalog_product=false
		WHERE product_id=7;
		INSERT INTO %[1]s.finished_inventory(
			product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g
		) VALUES (7,0,0,0,'finished_goods',3,0);
		UPDATE %[1]s.bean_list_publications SET status='withdrawn';
	`, schema))

	e := newOrderAPITestEcho(pool, schema)
	identity := map[string]any{
		"product_id":        []string{"7"},
		"parent_product_id": []string{"0"},
		"bom_spec_id":       []string{"0"},
		"bom_variant_id":    []string{"0"},
		"item_name":         []string{"橘皮乌龙盒装"},
		"qty":               []string{"2"},
		"unit":              []string{"盒"},
		"spec":              []string{"0"},
		"product_kind":      []string{"drip_bag"},
		"sales_unit":        []string{"box"},
		"unit_bag_count":    []string{"1"},
		"unit_bean_g":       []string{"1"},
	}
	previewBody, _ := json.Marshal(identity)
	previewReq := httptest.NewRequest(http.MethodPost, "/api/order/stock-batch-preview", bytes.NewReader(previewBody))
	previewReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	previewRec := httptest.NewRecorder()
	e.ServeHTTP(previewRec, previewReq)
	if previewRec.Code != http.StatusOK {
		t.Fatalf("POST stock preview status=%d body=%s", previewRec.Code, previewRec.Body.String())
	}
	for _, want := range []string{
		`"product_id":7`,
		`"spec_g":0`,
		`"need_units":2`,
		`"need_g":0`,
		`"available_units":3`,
		`"total_need_units":2`,
		`"allocated_units":2`,
		`"allocated_g":0`,
		`"batch_code":"PRODUCT-FP-7"`,
	} {
		if !strings.Contains(previewRec.Body.String(), want) {
			t.Fatalf("direct-product stock preview missing %s: %s", want, previewRec.Body.String())
		}
	}

	identity["order_date"] = "2026-09-01"
	identity["customer_id"] = 3
	identity["source_id"] = 1
	identity["order_type_id"] = 1
	identity["pay_status_id"] = 2
	identity["payment_method"] = "微信支付"
	identity["ship_status_id"] = 1
	identity["stock_batch_decision"] = "use_batch"
	identity["tier_id"] = []string{"manual"}
	identity["unit_price"] = []string{"30"}
	saveBody, _ := json.Marshal(identity)
	saveReq := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(saveBody))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("POST order status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}
	var saved struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}

	var productID, bomSpecID, bomVariantID, specG, needG, needUnits, allocatedG, allocatedUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,allocated_g,allocated_units
		FROM %s.order_stock_batch_allocations
		WHERE order_id=$1
	`, schema), saved.OrderID).Scan(
		&productID, &bomSpecID, &bomVariantID, &specG, &needG, &needUnits, &allocatedG, &allocatedUnits,
	); err != nil {
		t.Fatal(err)
	}
	if productID != 7 || bomSpecID != 0 || bomVariantID != 0 || specG != 0 || needG != 0 || needUnits != 2 || allocatedG != 0 || allocatedUnits != 2 {
		t.Fatalf("allocation identity/quantity=%d/%d/%d spec=%d need=%dg/%d allocated=%dg/%d", productID, bomSpecID, bomVariantID, specG, needG, needUnits, allocatedG, allocatedUnits)
	}

	trackingBody, _ := json.Marshal(map[string]any{"tracking_no": "GOAL-DIRECT-PRODUCT-001"})
	trackingReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/orders/%d/shipping-tracking", saved.OrderID), bytes.NewReader(trackingBody))
	trackingReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	trackingRec := httptest.NewRecorder()
	e.ServeHTTP(trackingRec, trackingReq)
	if trackingRec.Code != http.StatusOK {
		t.Fatalf("POST shipping tracking status=%d body=%s", trackingRec.Code, trackingRec.Body.String())
	}

	var inventoryUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=0 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&inventoryUnits); err != nil {
		t.Fatal(err)
	}
	if inventoryUnits != 1 {
		t.Fatalf("direct-product stock after shipment=%d units, want 1", inventoryUnits)
	}

	var deductedSpecID, deductedVariantID, deductedG, deductedUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_spec_id,bom_variant_id,deducted_g,deducted_units
		FROM %s.order_stock_deductions
		WHERE order_id=$1
	`, schema), saved.OrderID).Scan(&deductedSpecID, &deductedVariantID, &deductedG, &deductedUnits); err != nil {
		t.Fatal(err)
	}
	if deductedSpecID != 0 || deductedVariantID != 0 || deductedG != 0 || deductedUnits != 2 {
		t.Fatalf("deduction identity/quantity=%d/%d %dg/%d units", deductedSpecID, deductedVariantID, deductedG, deductedUnits)
	}

	var ledgerSpecID, ledgerVariantID, ledgerSpecG, ledgerChangeG, ledgerChangeUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_spec_id,bom_variant_id,spec_g,qty_change_g,qty_change_units
		FROM %s.stock_ledger_entries
		WHERE source_doc_type='sales_order_shipment' AND source_doc_id=$1
	`, schema), saved.OrderID).Scan(&ledgerSpecID, &ledgerVariantID, &ledgerSpecG, &ledgerChangeG, &ledgerChangeUnits); err != nil {
		t.Fatal(err)
	}
	if ledgerSpecID != 0 || ledgerVariantID != 0 || ledgerSpecG != 0 || ledgerChangeG != 0 || ledgerChangeUnits != -2 {
		t.Fatalf("ledger identity/quantity=%d/%d spec=%d change=%dg/%d units", ledgerSpecID, ledgerVariantID, ledgerSpecG, ledgerChangeG, ledgerChangeUnits)
	}

	// A production-completed direct-product order can ship without an explicit
	// pre-allocation. The fallback path must still deduct exactly one product unit.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.orders(
			id,order_no,order_date,customer_id,order_type_id,pay_status_id,ship_status_id,
			process_status_id,grand_total,is_void
		) VALUES (
			89,'SO-DIRECT-PRODUCT-NO-ALLOC','2026-09-01',3,1,2,1,
			(SELECT id FROM %[1]s.order_process_statuses WHERE name='生产完成' LIMIT 1),30,false
		);
		INSERT INTO %[1]s.order_items(
			order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,qty,unit,spec,unit_price,line_total
		) VALUES (89,1,7,0,0,'橘皮乌龙盒装',1,'盒','盒',30,30);
	`, schema))
	fallbackBody, _ := json.Marshal(map[string]any{"tracking_no": "GOAL-DIRECT-PRODUCT-002"})
	fallbackReq := httptest.NewRequest(http.MethodPost, "/api/orders/89/shipping-tracking", bytes.NewReader(fallbackBody))
	fallbackReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	fallbackRec := httptest.NewRecorder()
	e.ServeHTTP(fallbackRec, fallbackReq)
	if fallbackRec.Code != http.StatusOK {
		t.Fatalf("POST no-allocation shipping tracking status=%d body=%s", fallbackRec.Code, fallbackRec.Body.String())
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=0 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&inventoryUnits); err != nil {
		t.Fatal(err)
	}
	if inventoryUnits != 0 {
		t.Fatalf("no-allocation direct-product stock after shipment=%d units, want 0", inventoryUnits)
	}
	var fallbackDeductedUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT deducted_units FROM %s.order_stock_deductions
		WHERE order_id=89 AND product_id=7 AND bom_spec_id=0
	`, schema)).Scan(&fallbackDeductedUnits); err != nil {
		t.Fatal(err)
	}
	if fallbackDeductedUnits != 1 {
		t.Fatalf("no-allocation direct-product deducted_units=%d, want 1", fallbackDeductedUnits)
	}
}
