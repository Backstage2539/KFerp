package main

import "testing"

func TestProductKeyPrefersSKU(t *testing.T) {
	row := map[string]any{"sku_code": " SKU-1 ", "name": "ignored"}
	if got := productKey(row); got != "sku:sku-1" {
		t.Fatalf("productKey=%q", got)
	}
}

func TestProductKeyFallsBackToBusinessIdentity(t *testing.T) {
	row := map[string]any{
		"customer_id": 12, "name": "House Blend", "spec_label": "454g",
		"net_content_qty": 454, "net_content_unit": "g", "product_kind": "roasted_bean",
	}
	got := productKey(row)
	want := "legacy:12\x1fhouse blend\x1f454g\x1f454\x1fg\x1froasted_bean\x1f\x1f\x1f\x1f\x1f\x1f"
	if got != want {
		t.Fatalf("productKey=%q want %q", got, want)
	}
}

func TestSpecsExcludeTransactionalAndIdentityTables(t *testing.T) {
	forbidden := map[string]bool{
		"orders": true, "order_items": true, "stock_batches": true, "stock_entries": true,
		"work_orders": true, "production_plans": true, "finance_expenses": true,
		"company_employees": true, "employee_roles": true, "customer_portal_user_bindings": true,
		"login_sessions": true, "login_sms_codes": true,
	}
	for _, batch := range specs() {
		for _, table := range batch.tables {
			if forbidden[table.name] {
				t.Fatalf("forbidden table %s included", table.name)
			}
		}
	}
}

func TestSpecsHaveStableKeys(t *testing.T) {
	for _, batch := range specs() {
		for _, table := range batch.tables {
			if len(table.keys) == 0 && table.customKey == nil {
				t.Fatalf("%s has no business key", table.name)
			}
		}
	}
}
