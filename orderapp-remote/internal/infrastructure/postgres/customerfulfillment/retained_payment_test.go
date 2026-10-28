package customerfulfillment

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"
	"time"
)

func TestNewRetainedOrderAlwaysStartsUnpaidAndRejectsReceipts(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	repo := salespg.NewRepository(pool, schema)
	product, price := int64(910), 30.0
	cmd := salesapp.SaveOrderCommand{CustomerID: 501, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", Items: []salesapp.OrderItemCommand{{ProductID: &product, Name: "付款测试", Units: 4, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}}
	result, err := repo.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	var prepay, received float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.name,COALESCE(o.prepayment_amount,0),COALESCE(o.payment_goods_amount,0) FROM %s.orders o JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, schema, schema), result.OrderID).Scan(&status, &prepay, &received); err != nil {
		t.Fatal(err)
	}
	if status != "未付款" && status != "未收款" || prepay != 0 || received != 0 {
		t.Fatalf("new retained status=%q prepay=%v received=%v", status, prepay, received)
	}
	paid := int64(0)
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name IN ('已付款','已收款') LIMIT 1`, schema)).Scan(&paid); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []salesapp.SaveOrderCommand{func() salesapp.SaveOrderCommand { c := cmd; c.PayStatusID = paid; return c }(), func() salesapp.SaveOrderCommand { c := cmd; c.PaymentGoodsAmount = 5; return c }()} {
		if _, err = repo.SaveOrder(ctx, bad); err == nil {
			t.Fatal("conflicting receipt accepted")
		}
	}
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.orders WHERE customer_id=501`, schema)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rejected writes changed orders count=%d err=%v", count, err)
	}
}
