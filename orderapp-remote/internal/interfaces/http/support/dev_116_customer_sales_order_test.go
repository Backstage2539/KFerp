package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCompanySalesOrderRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-116",
		"DEV-116-01",
		"DEV-116-02",
		"DEV-116-03",
		"UT-116-01",
		"API-116-01",
		"REV-116-01",
		"客户公司名称",
		"销售单页面右侧抽屉",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer company sales order requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderVueExposesCustomerInfoDrawer(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"客户信息",
		"drawer",
		"company_name",
		"company_address",
		"company_phone",
		"/api/customers/",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing customer drawer marker %q", want)
		}
	}
}
