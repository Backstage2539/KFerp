package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
)

func (r *Repository) CustomerAssetInventoryScope(ctx context.Context, customerID int64) (app.CustomerInventoryScope, error) {
	scope := app.CustomerInventoryScope{InventoryTypes: []string{}, Warehouses: []app.CustomerInventoryWarehouse{}}
	if customerID <= 0 {
		return scope, fmt.Errorf("customer required")
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT code,name,kind FROM %s.warehouses WHERE customer_id=$1 AND is_virtual AND active ORDER BY sort_order,code`, r.schema), customerID)
	if err != nil {
		return scope, err
	}
	defer rows.Close()
	types := map[string]bool{}
	for rows.Next() {
		var w app.CustomerInventoryWarehouse
		if err := rows.Scan(&w.WarehouseCode, &w.WarehouseName, &w.Kind); err != nil {
			return scope, err
		}
		eligible := false
		switch w.Kind {
		case "finished", "customer_finished", "customer_processing", "customer", "shared_finished":
			types["finished_product"] = true
			eligible = true
		case "raw", "raw_materials":
			types["green_bean"] = true
			types["semi_finished"] = true
			eligible = true
		case "packaging":
			types["packaging"] = true
			eligible = true
		}
		if eligible {
			scope.Warehouses = append(scope.Warehouses, w)
		}
	}
	for _, key := range []string{"finished_product", "green_bean", "packaging", "semi_finished"} {
		if types[key] {
			scope.InventoryTypes = append(scope.InventoryTypes, key)
		}
	}
	return scope, rows.Err()
}

func customerInventoryWarehouseCodes(scope app.CustomerInventoryScope) []string {
	codes := make([]string, 0, len(scope.Warehouses))
	for _, w := range scope.Warehouses {
		codes = append(codes, w.WarehouseCode)
	}
	return codes
}
func customerInventoryScopeAllowsType(scope app.CustomerInventoryScope, key string) bool {
	for _, value := range scope.InventoryTypes {
		if value == key {
			return true
		}
	}
	return false
}
func customerInventoryWarehouseSet(codes []string) map[string]bool {
	set := map[string]bool{}
	for _, code := range codes {
		set[code] = true
	}
	return set
}
func (r *Repository) ListCustomerBoundVirtualInventory(ctx context.Context, id int64) ([]app.CustomerInventorySummary, error) {
	scope, err := r.CustomerAssetInventoryScope(ctx, id)
	if err != nil {
		return nil, err
	}
	if !customerInventoryScopeAllowsType(scope, "finished_product") {
		return []app.CustomerInventorySummary{}, nil
	}
	return r.listCustomerCentralInventoryScoped(ctx, id, "", customerInventoryWarehouseSet(customerInventoryWarehouseCodes(scope)))
}
