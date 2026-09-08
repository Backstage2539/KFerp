package customerfulfillment

import (
	"context"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

type sharedOrderTestRepository struct {
	fakeCustomerFulfillmentRepository
}

func (r *sharedOrderTestRepository) CustomerWorkspace(context.Context, int64, string, int64) (map[string]any, error) {
	return map[string]any{"customer_id": int64(14), "capabilities": []string{"direct_ship"}}, nil
}

type sharedOrderTestSales struct{ command salesapp.SaveOrderCommand }

func (s *sharedOrderTestSales) SaveOrder(_ context.Context, c salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	s.command = c
	return salesapp.SaveOrderResult{OrderID: 42, OrderNo: "SO-test"}, nil
}
func TestLegacyPortalSubmitUsesSharedSalesAndBoundCustomer(t *testing.T) {
	repo := &sharedOrderTestRepository{}
	repo.customerContextResult = CustomerERPContext{CustomerID: 14}
	repo.optionsResult = CustomerFulfillmentOptions{CustomerSKUs: []CustomerSKUOption{{ProductID: 7, ProductName: "测试商品", Spec: "227g"}}}
	sales := &sharedOrderTestSales{}
	svc := NewService(repo)
	svc.UseSalesOrderService(sales)
	result, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{EmployeeID: 13, Actor: "测试客户", ReceiverName: "收件人", ReceiverPhone: "13800000000", ReceiverAddress: "测试地址", ProductName: "测试商品", Spec: "227g", QuantityUnits: 2, ShippingAmount: 10})
	if err != nil {
		t.Fatal(err)
	}
	c := sales.command
	if result.OrderID != 42 || result.OrderNo != "SO-test" || !c.CustomerSubmission || c.CustomerID != 14 || c.ShippingAmount != 0 || c.PortalServiceCode != "direct_ship" || len(c.Items) != 1 || *c.Items[0].ProductID != 7 || c.Items[0].SpecG != 227 || c.Items[0].Units != 2 {
		t.Fatalf("legacy adapter did not preserve shared contract: %+v", c)
	}
	if repo.customerDirectShipCmd.EmployeeID != 0 {
		t.Fatal("legacy save repository used")
	}
}
