package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSkuBomBeanListRequirementSeeds(t *testing.T) {
	src := string(readDev152File(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-152",
		"DEV-152-01",
		"DEV-152-02",
		"DEV-152-03",
		"UT-152-01",
		"API-152-01",
		"REV-152-01",
		"BOM 增删查",
		"客户专属 SKU 列表",
		"客户豆单只能选择公共 SKU 和对应客户 SKU",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer SKU BOM bean list requirement seed missing %q", want)
		}
	}
}

func TestCustomerSkuBomBeanListWiring(t *testing.T) {
	productSettings := string(readDev152File(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"customerSkuRows",
		"selectedCustomerSkuCustomerID",
		"bom_item_count",
		"商品档案配置",
		"维护当前 BOM 明细",
	} {
		if !strings.Contains(productSettings, want) {
			t.Fatalf("ProductSettingsView.vue missing customer SKU list wiring %q", want)
		}
	}

	bomView := string(readDev152File(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{
		"SearchableSelect",
		"deactivateSelectedProductionBoms",
		"deactivateProductionBomRecord",
		"apiSend(`/api/production-boms/${bom.id}`",
	} {
		if !strings.Contains(bomView, want) {
			t.Fatalf("BomView.vue missing BOM maintenance wiring %q", want)
		}
	}

	costingView := string(readDev152File(t, filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue")))
	for _, want := range []string{
		"publicationScope === 'customer'",
		"selectedBeanListCustomerID",
		"customerBeanListItems",
		"customer_id",
	} {
		if !strings.Contains(costingView, want) {
			t.Fatalf("CostingView.vue missing customer bean list wiring %q", want)
		}
	}

	costingAPI := string(readDev152File(t, filepath.Join("internal", "interfaces", "http", "costing", "costing_api.go")))
	for _, want := range []string{
		`case "customer":`,
		"customer_id",
		`return "customer", strconv.FormatInt(customerID, 10)`,
	} {
		if !strings.Contains(costingAPI, want) {
			t.Fatalf("costing_api.go missing customer bean list API wiring %q", want)
		}
	}
}

func readDev152File(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
