package authz

import "testing"

func TestDefaultRoleSeedsIncludeCoreInternalRoles(t *testing.T) {
	roles := defaultRoles()
	want := map[string]bool{
		"admin":      false,
		"sales":      false,
		"production": false,
		"warehouse":  false,
		"finance":    false,
		"product":    false,
		"system":     false,
	}
	for _, role := range roles {
		if _, ok := want[role.Code]; ok {
			want[role.Code] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("missing default role %s in %+v", code, roles)
		}
	}
}

func TestDefaultViewPermissionsCoverVueShellMenuKeys(t *testing.T) {
	views := defaultViewPermissions()
	for _, key := range []string{
		"order", "orders", "orderSalesManual", "orderInvoice", "salesOrder", "deliveryNote", "customers", "quotePrint",
		"producePlan", "productionAcceptance", "produceRunning", "workOrders", "jobCards", "qualityInspections", "produceLogs", "productionCosts", "productionManual",
		"warehouseInventory", "stockOperations", "stockOutboundLogs", "inventoryMaterialsManual", "materials", "materialReceipts", "materialBatches", "wipMaterials", "stockLedger", "stockBatches", "stockAdjustments", "inventory", "allocationLogs",
		"productSettings", "bom", "products", "costing", "costingManual",
		"costingSettings", "machines", "companyProfile", "salesOrderSettings", "senderSettings", "outsourceSettings", "customerPortalSettings", "customerFulfillment", "customerFulfillmentManual", "settingsAuditManual",
		"departments", "employees", "audit", "userPermissions",
		"reqProduct", "reqDev", "reqUnit", "reqApi", "reqReview", "requirementsManual",
	} {
		if views[key] == "" {
			t.Fatalf("missing permission for view %s", key)
		}
	}
}
