package support

import (
	"os"
	"strings"
	"testing"
)

func TestOrderResponsibleSearchRequirementSeeds(t *testing.T) {
	src := string(readDev156File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-156",
		"DEV-156-01",
		"DEV-156-02",
		"UT-156-01",
		"API-156-01",
		"REV-156-01",
		"负责人搜索订单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("order responsible search requirement seed missing %q", want)
		}
	}
}

func TestOrderResponsibleSearchFrontendAndQueryWiring(t *testing.T) {
	query := string(readDev156File(t, "internal/infrastructure/postgres/sales/order_queries.go"))
	for _, want := range []string{
		"o.responsible_party_name ILIKE",
		"o.order_no ILIKE",
		"c.name ILIKE",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("order query missing search marker %q", want)
		}
	}

	view := string(readDev156File(t, "frontend-vue-shell/src/views/OrdersView.vue"))
	if !strings.Contains(view, `placeholder="订单号/客户/负责人"`) {
		t.Fatalf("OrdersView.vue search placeholder must mention responsible person")
	}
}

func readDev156File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
