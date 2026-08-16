package purchase

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	purchaseapp "orderapp/internal/application/purchase"
	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPurchaseWritesRejectSemiFinishedMaterialPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr600_purchase_semi_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(
			id BIGINT PRIMARY KEY, code TEXT NOT NULL, name TEXT NOT NULL,
			is_semi_finished BOOLEAN NOT NULL DEFAULT false,
			purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.materials(id,code,name,is_semi_finished) VALUES(1,'WIP-1','半成品',true);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.purchase_suppliers(id,name) VALUES(1,'测试供应商')`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	if _, err := repo.CreatePurchaseOrder(ctx, purchaseapp.CreatePurchaseOrderCommand{
		SupplierID: 1, MaterialID: 1, QtyG: 1000, UnitCost: 288, Operator: "buyer",
	}); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("CreatePurchaseOrder error=%v, want manufacture-only guard", err)
	}
	if _, err := repo.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
		SupplierID: 1, SupplierName: "测试供应商", MaterialID: 1, QtyG: 1000, UnitCost: 288, Operator: "buyer",
	}, stockapp.MaterialReceiptResult{ReceiptID: 9, BatchCode: "MB-9"}); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("CreatePurchaseReceipt error=%v, want manufacture-only guard", err)
	}
	if err := repo.UpdateMaterialPurchasePrice(ctx, 1, 288); err == nil || !strings.Contains(err.Error(), "半成品只能通过生产入库") {
		t.Fatalf("UpdateMaterialPurchasePrice error=%v, want manufacture-only guard", err)
	}
	var orders, receipts int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %s.purchase_orders),(SELECT count(*) FROM %s.purchase_receipts)`, schema, schema)).Scan(&orders, &receipts); err != nil {
		t.Fatal(err)
	}
	if orders != 0 || receipts != 0 {
		t.Fatalf("rejected purchase rows orders/receipts=%d/%d, want 0/0", orders, receipts)
	}
}
