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
