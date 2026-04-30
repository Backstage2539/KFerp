package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCompanySalesOrderRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-117",
		"DEV-117-01",
		"DEV-117-02",
		"DEV-117-03",
		"UT-117-01",
		"API-117-01",
		"REV-117-01",
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

func TestCustomerCompanyFieldsRemainOptionalAtRepositoryWrite(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customer", "repository.go")))
	for _, forbidden := range []string{
		"nullText(companyName)",
		"nullText(companyAddress)",
		"nullText(companyPhone)",
		"nullText(next.companyName)",
		"nullText(next.companyAddress)",
		"nullText(next.companyPhone)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("customer company optional field must not be written as NULL: found %q", forbidden)
		}
	}
}
