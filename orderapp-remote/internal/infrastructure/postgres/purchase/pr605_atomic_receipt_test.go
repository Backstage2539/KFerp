package purchase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR605PurchaseReceiptPostsStockPriceAndOrderInOneTransaction(t *testing.T) {
	pool, schema := setupSemiFinishedAtomicityDB(t)
	ctx := context.Background()
	ensurePR605PurchaseStockSchema(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
	svc := purchaseapp.NewService(repo, stockapp.NewService(stockrepo.NewRepository(pool, schema)))

	order, err := svc.CreatePurchaseOrder(ctx, purchaseapp.CreatePurchaseOrderCommand{
		SupplierID: 1, MaterialID: 1, Qty: 12, UnitCode: "kg", UnitCost: 100,
		TargetWarehouse: "raw_materials", Operator: "buyer",
	})
	if err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	var priceBeforeReceipt float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price::float8 FROM %s.materials WHERE id=1`, schema)).Scan(&priceBeforeReceipt); err != nil {
		t.Fatal(err)
	}
	if priceBeforeReceipt != 288 {
		t.Fatalf("purchase order changed material price to %.2f, want existing 288", priceBeforeReceipt)
	}

	receipt, err := svc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
		PurchaseOrderID: order.ID, SupplierID: 1, MaterialID: 1,
		Qty: 10, UnitCode: "kg", UnitCost: 42.5, TargetWarehouse: "raw_materials", Operator: "receiver",
	})
	if err != nil {
		t.Fatalf("create purchase receipt: %v", err)
	}
	if receipt.Qty != 10 || receipt.QtyG != 10000 || receipt.UnitCode != "kg" || receipt.TargetWarehouse != "raw_materials" || receipt.StockReceiptID <= 0 || receipt.StockBatchCode == "" {
		t.Fatalf("receipt=%+v", receipt)
	}

	var price float64
	var onhandG, locationG int64
	var orderStatus, stockStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT purchase_price::float8,onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&price, &onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.purchase_orders WHERE id=$1`, schema), order.ID).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1`, schema), receipt.StockReceiptID).Scan(&stockStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE batch_code=$1 AND warehouse='raw_materials'`, schema), receipt.StockBatchCode).Scan(&locationG); err != nil {
		t.Fatal(err)
	}
	if price != 42.5 || onhandG != 10000 || locationG != 10000 || orderStatus != "received" || stockStatus != "submitted" {
		t.Fatalf("price/onhand/location/order/stock=%.2f/%d/%d/%s/%s", price, onhandG, locationG, orderStatus, stockStatus)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE (entity_type='purchase_order' AND entity_id=$1)
		   OR (entity_type='purchase_receipt' AND entity_id=$2)
		   OR (entity_type='material' AND entity_id=1 AND action='purchase_receipt_price')
	`, schema), order.ID, receipt.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount < 4 {
		t.Fatalf("purchase lifecycle audit rows=%d, want at least 4", auditCount)
	}
}

func TestPR605PackagingReceiptDefaultsToPackagingWarehouseAndRollsBackOnFailure(t *testing.T) {
	pool, schema := setupSemiFinishedAtomicityDB(t)
	ctx := context.Background()
	ensurePR605PurchaseStockSchema(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,purchase_price)
		VALUES(2,'PACK-PR605','测试包材','pack','袋','袋',2)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	svc := purchaseapp.NewService(repo, stockapp.NewService(stockrepo.NewRepository(pool, schema)))
	order, err := svc.CreatePurchaseOrder(ctx, purchaseapp.CreatePurchaseOrderCommand{
		SupplierID: 1, MaterialID: 2, Qty: 10, UnitCode: "袋", UnitCost: 2.5, Operator: "buyer",
	})
	if err != nil {
		t.Fatalf("create packaging order: %v", err)
	}
	if order.TargetWarehouse != "packaging" || order.QtyUnits != 10 {
		t.Fatalf("packaging order=%+v", order)
	}
	receipt, err := svc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
		PurchaseOrderID: order.ID, SupplierID: 1, MaterialID: 2,
		Qty: 9, UnitCode: "袋", UnitCost: 2.75, Operator: "receiver",
	})
	if err != nil {
		t.Fatalf("receive packaging: %v", err)
	}
	var units int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_units),0)::bigint FROM %s.material_batch_locations WHERE material_id=2 AND warehouse='packaging'`, schema)).Scan(&units); err != nil {
		t.Fatal(err)
	}
	if receipt.TargetWarehouse != "packaging" || receipt.QtyUnits != 9 || units != 9 {
		t.Fatalf("packaging receipt=%+v location units=%d", receipt, units)
	}

	rollbackOrder, err := svc.CreatePurchaseOrder(ctx, purchaseapp.CreatePurchaseOrderCommand{
		SupplierID: 1, MaterialID: 1, Qty: 1, UnitCode: "kg", UnitCost: 99, Operator: "buyer",
	})
	if err != nil {
		t.Fatalf("create rollback order: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %s.pr605_fail_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced receipt failure'; END $$;
		CREATE TRIGGER pr605_fail_receipt BEFORE INSERT ON %s.purchase_receipts
		FOR EACH ROW EXECUTE FUNCTION %s.pr605_fail_receipt();
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
		PurchaseOrderID: rollbackOrder.ID, SupplierID: 1, MaterialID: 1,
		Qty: 1, UnitCode: "kg", UnitCost: 99, Operator: "receiver",
	})
	if err == nil || !strings.Contains(err.Error(), "forced receipt failure") {
		t.Fatalf("forced receipt error=%v", err)
	}
	var onhandG, stockEntries, receipts int64
	var purchasePrice float64
	var status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,purchase_price::float8 FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG, &purchasePrice); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_entries WHERE source_type='purchase_order' AND source_id=$1`, schema), rollbackOrder.ID).Scan(&stockEntries); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.purchase_receipts WHERE purchase_order_id=$1`, schema), rollbackOrder.ID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.purchase_orders WHERE id=$1`, schema), rollbackOrder.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if onhandG != 0 || purchasePrice != 288 || stockEntries != 0 || receipts != 0 || status != "ordered" {
		t.Fatalf("rollback state onhand/price/stock/receipts/status=%d/%.2f/%d/%d/%s", onhandG, purchasePrice, stockEntries, receipts, status)
	}
}

func ensurePR605PurchaseStockSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
		CREATE TABLE %s.work_orders(id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.finished_inventory(
			product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
			onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY(product_id,spec_g)
		);
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if err := stockrepo.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("ensure stock schema: %v", err)
	}
}
