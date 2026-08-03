package sales

import (
	"context"
	"fmt"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestCurrentOrderEditRevisionChangesWithHeaderOrItemMutation(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.orders (id BIGINT PRIMARY KEY, notes TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.order_items (id BIGINT PRIMARY KEY, order_id BIGINT NOT NULL, item_note TEXT NOT NULL DEFAULT '');
		INSERT INTO %s.orders(id,notes) VALUES(42,'first');
		INSERT INTO %s.order_items(id,order_id,item_note) VALUES(51,42,'line one');
	`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	load := func() string {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		revision, err := currentOrderEditRevisionTx(ctx, tx, schema, 42)
		if err != nil {
			t.Fatal(err)
		}
		return revision
	}
	initial := load()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET item_note='line two' WHERE id=51`, schema)); err != nil {
		t.Fatal(err)
	}
	itemChanged := load()
	if itemChanged == initial {
		t.Fatal("item mutation did not change order edit revision")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes='second' WHERE id=42`, schema)); err != nil {
		t.Fatal(err)
	}
	if headerChanged := load(); headerChanged == itemChanged {
		t.Fatal("header mutation did not change order edit revision")
	}
}

func TestShipmentRevisionVerificationRejectsAnExportedOrderChangedBeforeShipmentCreation(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.orders (id BIGINT PRIMARY KEY, notes TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.order_items (id BIGINT PRIMARY KEY, order_id BIGINT NOT NULL, item_note TEXT NOT NULL DEFAULT '');
		INSERT INTO %s.orders(id,notes) VALUES(42,'exported');
		INSERT INTO %s.order_items(id,order_id,item_note) VALUES(51,42,'exported line');
	`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}

	exportTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	exportedRevision, err := currentOrderEditRevisionTx(ctx, exportTx, schema, 42)
	if err != nil {
		_ = exportTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := exportTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET item_note='edited line' WHERE id=51`, schema)); err != nil {
		t.Fatal(err)
	}

	shipmentTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = shipmentTx.Rollback(ctx) }()
	if err := lockShipmentOrdersTx(ctx, shipmentTx, schema, []int64{42}); err != nil {
		t.Fatal(err)
	}
	err = verifyShipmentOrderRevisionsTx(ctx, shipmentTx, schema, []salesapp.OrderShipmentOrderCommand{{
		OrderID:          42,
		ExpectedRevision: exportedRevision,
	}})
	message, ok := salesapp.OrderEditConflictMessage(err)
	if !ok || message != "订单已被其他操作修改，请重新生成发货单" {
		t.Fatalf("revision verification error = %v, message = %q", err, message)
	}
}
