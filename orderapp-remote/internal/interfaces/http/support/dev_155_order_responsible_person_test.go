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
		"PR-327",
		"DEV-155-01",
		"DEV-155-02",
		"UT-155-01",
		"API-155-01",
		"REV-155-01",
		"订单负责人",
		"客户资料",
		"内部员工",
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
		"客户负责人",
		"selectedCustomerResponsibleLabel",
		"responsible_employee_id",
		"请选择客户负责人",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing responsible person wiring %q", want)
		}
	}
	for _, removed := range []string{
		"订单负责人",
		"chooseResponsible",
		"filteredResponsibleOptions",
		"responsible_type",
		"responsible_id",
	} {
		if strings.Contains(orderEntry, removed) {
			t.Fatalf("OrderEntryView.vue still contains obsolete order responsible picker marker %q", removed)
		}
	}

	orderEntryLib := string(readDev155File(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"responsibleOptions({ employees = [] }",
		"type: 'employee'",
	} {
		if !strings.Contains(orderEntryLib, want) {
			t.Fatalf("order-entry.js missing employee-only responsible helper marker %q", want)
		}
	}
	for _, removed := range []string{
		"type: 'customer'",
		"responsible_type",
		"responsible_id",
	} {
		if strings.Contains(orderEntryLib, removed) {
			t.Fatalf("order-entry.js still contains obsolete order responsible payload marker %q", removed)
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
		`responsible_employee_id`,
		`responsible_employee_name`,
		`responsible_type`,
		`responsible_id`,
		`responsible_name`,
	} {
		if !strings.Contains(api, want) {
			t.Fatalf("order_api.go missing responsible person API marker %q", want)
		}
	}

	repository := string(readDev155File(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		"resolveOrderResponsibleParty(ctx, tx, r.schema, cmd.CustomerID)",
		"customer responsible employee required",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("sales repository missing customer-derived responsible marker %q", want)
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
