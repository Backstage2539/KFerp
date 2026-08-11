package authz

import "testing"

func TestDefaultRoleSeedsIncludeCoreInternalRoles(t *testing.T) {
	roles := defaultRoles()
	want := map[string]bool{
		"admin":                         false,
		"sales":                         false,
		"production":                    false,
		"warehouse":                     false,
		"finance":                       false,
		"product":                       false,
		"system":                        false,
		"customer_direct_ship_customer": false,
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
		"order", "orders", "orderSalesManual", "orderInvoice", "salesOrder", "deliveryNote", "contracts", "customers",
		"producePlan", "productionFlow", "productionConfig", "productionAcceptance", "produceRunning", "workOrders", "jobCards", "qualityInspections", "produceLogs", "productionCosts", "productionManual",
		"warehouseInventory", "stockOperations", "stockOutboundLogs", "inventoryMaterialsManual", "stockManual", "materials", "materialReceipts", "materialBatches", "wipMaterials", "stockLedger", "stockBatches", "stockAdjustments", "inventory", "allocationLogs",
		"productSettings", "mallSettings", "bom", "products", "costing", "costingManual",
		"costingSettings", "machines", "companyProfile", "salesOrderSettings", "senderSettings", "outsourceSettings", "processingBilling", "uiSettings", "customerCapabilityTemplates", "customerPortalSettings", "customerPortalManual", "customerFulfillment", "customerFulfillmentManual", "settingsAuditManual",
		"departments", "employees", "audit",
		"reqProduct", "reqDev", "reqUnit", "reqApi", "reqReview", "requirementsManual",
	} {
		if views[key] == "" {
			t.Fatalf("missing permission for view %s", key)
		}
	}
	if views["userPermissions"] != "" {
		t.Fatal("userPermissions should be merged into employees and should not keep a standalone view permission")
	}
	if views["quotePrint"] != "" {
		t.Fatal("removed quote export page should not keep a view permission")
	}
	if views["productionConfig"] != "bom.read" {
		t.Fatalf("productionConfig permission=%q, want bom.read so BOM readers can open the consolidated page", views["productionConfig"])
	}
	if views["processingBilling"] != "finance.read" || views["outsourceSettings"] != "settings.write" {
		t.Fatalf("processing billing/template view permissions=%q/%q, want finance.read/settings.write", views["processingBilling"], views["outsourceSettings"])
	}
}
