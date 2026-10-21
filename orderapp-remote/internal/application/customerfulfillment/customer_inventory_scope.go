package customerfulfillment

import (
	"context"
	"fmt"
)

type CustomerInventoryWarehouse struct {
	WarehouseCode string `json:"warehouse_code"`
	WarehouseName string `json:"warehouse_name"`
	Kind          string `json:"kind"`
}

type CustomerInventoryScope struct {
	InventoryTypes []string                     `json:"inventory_types"`
	Warehouses     []CustomerInventoryWarehouse `json:"warehouses"`
}

type CustomerInventoryScopeRepository interface {
	CustomerAssetInventoryScope(context.Context, int64) (CustomerInventoryScope, error)
}

type CustomerBoundInventoryRepository interface {
	ListCustomerBoundVirtualInventory(context.Context, int64) ([]CustomerInventorySummary, error)
}

func (s *Service) CustomerAssetInventoryScope(ctx context.Context, customerID int64) (CustomerInventoryScope, error) {
	if customerID <= 0 {
		return CustomerInventoryScope{}, fmt.Errorf("customer required")
	}
	repo, err := s.miniDirectShipRepository()
	if err != nil {
		return CustomerInventoryScope{}, err
	}
	provider, ok := repo.(CustomerInventoryScopeRepository)
	if !ok {
		return CustomerInventoryScope{}, fmt.Errorf("customer inventory scope unavailable")
	}
	return provider.CustomerAssetInventoryScope(ctx, customerID)
}
