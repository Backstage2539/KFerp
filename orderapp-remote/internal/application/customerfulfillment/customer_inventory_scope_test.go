package customerfulfillment

import (
	"context"
	"testing"
)

type boundInventoryApplicationStub struct {
	*miniDirectShipApplicationRepositoryStub
	boundCustomerID int64
}

func (r *boundInventoryApplicationStub) ListCustomerBoundVirtualInventory(_ context.Context, id int64) ([]CustomerInventorySummary, error) {
	r.boundCustomerID = id
	return []CustomerInventorySummary{{ProductID: 501, ProductName: "授权虚拟仓库存", TotalQty: 2}}, nil
}
func (r *boundInventoryApplicationStub) CustomerAssetInventoryScope(_ context.Context, id int64) (CustomerInventoryScope, error) {
	return CustomerInventoryScope{InventoryTypes: []string{"finished_product"}, Warehouses: []CustomerInventoryWarehouse{}}, nil
}

func TestMiniInventoryApplicationNeverFallsBackToWideStockWhenScopeIsRequired(t *testing.T) {
	wide := &miniDirectShipApplicationRepositoryStub{fakeCustomerFulfillmentRepository: &fakeCustomerFulfillmentRepository{}, inventoryRows: []CustomerInventorySummary{{ProductID: 999, ProductName: "非授权公共仓库存"}}}
	if _, err := NewService(wide).ListCustomerCentralInventory(context.Background(), CustomerInventoryListQuery{CustomerID: 9, BoundVirtualOnly: true, LegacyAll: true}); err == nil {
		t.Fatal("missing scoped inventory implementation fell back to broad inventory")
	}
	bound := &boundInventoryApplicationStub{miniDirectShipApplicationRepositoryStub: wide}
	result, err := NewService(bound).ListCustomerCentralInventory(context.Background(), CustomerInventoryListQuery{CustomerID: 9, BoundVirtualOnly: true, LegacyAll: true})
	if err != nil {
		t.Fatal(err)
	}
	if bound.boundCustomerID != 9 || wide.inventoryCustomerID != 0 || len(result.Rows) != 1 || result.Rows[0].ProductID != 501 {
		t.Fatalf("inventory ignored bound scope: %+v repo=%+v", result, bound)
	}
	if _, err := NewService(bound).CustomerAssetInventoryScope(context.Background(), 0); err == nil {
		t.Fatal("scope accepted unbound customer")
	}
}
