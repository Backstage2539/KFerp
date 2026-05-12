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
		"portal_service_code IN ('direct_ship','processing_ship')",
		"test_schema.customer_erp_user_bindings",
		"b.status='active'",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fulfillment scope missing %q in %q", want, joined)
		}
	}
	if len(args) != 0 {
		t.Fatalf("fulfillment scope args = %#v, want none", args)
	}
}
