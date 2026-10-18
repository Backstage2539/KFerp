package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	productionapp "orderapp/internal/application/production"
	salesapp "orderapp/internal/application/sales"
	postproduction "orderapp/internal/infrastructure/postgres/production"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
)

func TestMiniRetainedPurchaseKeepsOriginalProductionDemand(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	productID, price := int64(910), 30.0
	sale, err := postgressales.NewRepository(pool, schema).SaveOrder(ctx, salesapp.SaveOrderCommand{
		CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(),
		DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "mini_user:703",
		Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "留厂24包", Units: 24, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	cmd := app.MiniDirectShipCommand{CustomerID: 501, UsageCode: "product_order", DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", IdempotencyKey: "MINI-RETAINED24", Actor: "mini_user:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 24, InventoryUnit: "袋"}}}
	request, err := NewRepository(pool, schema).recordMiniDirectShipOrderTx(ctx, tx, app.PreparedMiniDirectShipOrder{Command: cmd, RequestHash: "MINI-RETAINED24"}, app.DirectShipOrderSummary{OrderID: sale.OrderID, OrderNo: sale.OrderNo})
	if err != nil {
		t.Fatal(err)
	}
	if request.OrderID != sale.OrderID {
		t.Fatalf("unexpected order %+v", request)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	data, err := postproduction.NewRepository(pool, schema).PlanSummary(ctx, productionapp.PlanSummaryQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	var demand float64
	for _, row := range data.Rows {
		if row.ProductID == productID {
			demand += row.SalesSpecCount
		}
	}
	if demand != 24 {
		t.Fatalf("mini retained purchase production demand=%v, want original purchase24; rows=%+v", demand, data.Rows)
	}
	var status string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT s.name FROM %s.orders o JOIN %s.order_process_statuses s ON s.id=o.process_status_id WHERE o.id=$1`, schema, schema), sale.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "待生产" {
		t.Fatalf("retained purchase status=%q", status)
	}
}
