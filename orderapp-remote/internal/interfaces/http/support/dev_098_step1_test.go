package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderEntryPolishRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, needle := range []string{
		"PR-100",
		"DEV-100-01",
		"DEV-100-02",
		"DEV-100-03",
		"UT-100-01",
		"API-100-01",
		"REV-100-01",
		"客户模糊搜索",
		"付款状态默认已付款",
		"商品下拉直接搜索",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("req_store.go missing %q", needle)
		}
	}
}

func TestOrderEntryVueUsesComboboxesAndManualPrice(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, needle := range []string{
		"customer-combobox",
		"product-combobox",
		"chooseCustomer",
		"chooseProduct",
		"markManualPrice",
		"2.5kg",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("OrderEntryView.vue missing %q", needle)
		}
	}
	if strings.Contains(src, "product-filter") {
		t.Fatal("OrderEntryView.vue should not keep a separate product-filter input")
	}
}
