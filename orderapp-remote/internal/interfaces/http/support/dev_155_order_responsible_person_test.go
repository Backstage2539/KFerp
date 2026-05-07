package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderResponsiblePersonRequirementSeeds(t *testing.T) {
	src := string(readDev155File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-155",
		"DEV-155-01",
		"DEV-155-02",
		"UT-155-01",
		"API-155-01",
		"REV-155-01",
		"订单负责人",
		"售前售后",
		"提成结算",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("order responsible person requirement seed missing %q", want)
		}
	}
}

func TestOrderResponsiblePersonFrontendAndAPIWiring(t *testing.T) {
	orderEntry := string(readDev155File(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"订单负责人",
		"responsibleOptions",
		"chooseResponsible",
		"filteredResponsibleOptions",
		"responsible_type",
		"responsible_id",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing responsible person wiring %q", want)
		}
	}

	orders := string(readDev155File(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	for _, want := range []string{
		"负责人",
		"responsible_name",
	} {
		if !strings.Contains(orders, want) {
			t.Fatalf("OrdersView.vue missing responsible person display %q", want)
		}
	}

	api := string(readDev155File(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	for _, want := range []string{
		`Employees`,
		`responsible_type`,
		`responsible_id`,
		`responsible_name`,
	} {
		if !strings.Contains(api, want) {
			t.Fatalf("order_api.go missing responsible person API marker %q", want)
		}
	}
}

func readDev155File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
