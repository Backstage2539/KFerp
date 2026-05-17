package sales

import (
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestOrderListWhereSearchMatchesResponsibleName(t *testing.T) {
	where, args, nextArg := orderListWhere("test_schema", salesapp.OrderListQuery{Q: "销售小王"})
	joined := strings.Join(where, " AND ")

	for _, want := range []string{
		"o.order_no ILIKE $1",
		"c.name ILIKE $1",
		"o.responsible_party_name ILIKE $1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("order list q search missing %q in %q", want, joined)
		}
	}
	if len(args) != 1 || args[0] != "%销售小王%" {
		t.Fatalf("search args = %#v, want %%销售小王%%", args)
	}
	if nextArg != 2 {
		t.Fatalf("nextArg = %d, want 2", nextArg)
	}
}

func TestOrderListWhereSupportsOrderIDForScopedDetailAccess(t *testing.T) {
	where, args, nextArg := orderListWhere("test_schema", salesapp.OrderListQuery{OrderID: 88, Scope: "fulfillment", FulfillmentEmployeeID: 7, Void: "all"})
	joined := strings.Join(where, " AND ")

	for _, want := range []string{
		"o.id = $1",
		"b.employee_id=$2",
		"portal_service_code IN ('direct_ship','processing_ship','product_order')",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("scoped detail where missing %q in %q", want, joined)
		}
	}
	if len(args) != 2 || args[0] != int64(88) || args[1] != int64(7) {
		t.Fatalf("scoped detail args = %#v, want order id 88 and employee id 7", args)
	}
	if nextArg != 3 {
		t.Fatalf("nextArg = %d, want 3", nextArg)
	}
}

func TestOrderListWhereSupportsMineAndFulfillmentScopes(t *testing.T) {
	where, args, _ := orderListWhere("test_schema", salesapp.OrderListQuery{Scope: "mine", EmployeeID: 7})
	joined := strings.Join(where, " AND ")
	if !strings.Contains(joined, "o.responsible_party_type='employee'") || !strings.Contains(joined, "o.responsible_party_id=$1") {
		t.Fatalf("mine scope where = %q", joined)
	}
	if len(args) != 1 || args[0] != int64(7) {
		t.Fatalf("mine scope args = %#v, want employee id 7", args)
	}

	where, args, _ = orderListWhere("test_schema", salesapp.OrderListQuery{Scope: "fulfillment"})
	joined = strings.Join(where, " AND ")
	for _, want := range []string{
		"customer_type",
		"portal_service_code IN ('direct_ship','processing_ship','product_order')",
		"test_schema.customer_erp_user_bindings",
		"test_schema.customer_portal_profiles",
		"test_schema.customer_capability_templates",
		"test_schema.company_employees",
		"test_schema.employee_login_passwords",
		"b.status='active'",
		"e.account_type='channel_customer'",
		"COALESCE(lp.login_disabled,false)=false",
		"capability_template_key",
		"active_template.active=true",
		"inactive_template.active=false",
		"processing_fulfillment",
		"public_sku_direct_ship",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fulfillment scope missing %q in %q", want, joined)
		}
	}
	if len(args) != 0 {
		t.Fatalf("fulfillment scope args = %#v, want none", args)
	}

	where, args, _ = orderListWhere("test_schema", salesapp.OrderListQuery{Scope: "fulfillment", FulfillmentEmployeeID: 7})
	joined = strings.Join(where, " AND ")
	if !strings.Contains(joined, "b.employee_id=$1") {
		t.Fatalf("customer workbench fulfillment scope must be limited to the bound employee, got %q", joined)
	}
	if len(args) != 1 || args[0] != int64(7) {
		t.Fatalf("customer workbench fulfillment scope args = %#v, want employee id 7", args)
	}
}

func TestResolveOrderFulfillmentMarkersPreservesExistingGeneratedOrderScope(t *testing.T) {
	portalServiceCode, sourceWarehouse := resolveOrderFulfillmentMarkers("direct_ship", "finished_goods", "product_order", "finished_goods")
	if portalServiceCode != "direct_ship" || sourceWarehouse != "finished_goods" {
		t.Fatalf("existing generated scope should be preserved, got portal=%q warehouse=%q", portalServiceCode, sourceWarehouse)
	}

	portalServiceCode, sourceWarehouse = resolveOrderFulfillmentMarkers("", "", "product_order", "finished_goods")
	if portalServiceCode != "product_order" || sourceWarehouse != "finished_goods" {
		t.Fatalf("new ERP fulfillment order should be marked as product_order, got portal=%q warehouse=%q", portalServiceCode, sourceWarehouse)
	}

	portalServiceCode, _ = resolveOrderFulfillmentMarkers("direct_ship", "finished_goods", "", "finished_goods")
	if portalServiceCode != "" {
		t.Fatalf("non-fulfillment customer selection should clear fulfillment scope, got %q", portalServiceCode)
	}
}

func TestWholesaleTierQuantityForSpecUsesKilogramsForKgSpecs(t *testing.T) {
	if got := wholesaleTierQuantityForSpec(2500, 10); got != 25 {
		t.Fatalf("2500g x 10 tier quantity = %.2f, want 25kg", got)
	}
	if got := wholesaleTierQuantityForSpec(454, 10); got != 10 {
		t.Fatalf("454g x 10 tier quantity = %.2f, want 10 packages", got)
	}
}
