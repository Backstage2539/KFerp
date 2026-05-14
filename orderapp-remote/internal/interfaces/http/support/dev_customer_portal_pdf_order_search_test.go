package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalPDFOrderSearchRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-PDF-ORDER-SEARCH",
		"DEV-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-01",
		"DEV-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-02",
		"DEV-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-03",
		"UT-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-01",
		"API-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-01",
		"REV-CUSTOMER-PORTAL-PDF-ORDER-SEARCH-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal PDF/order search seed missing %q", want)
		}
	}
}

func TestMiniappServicePageSupportsNativeBeanListCacheAndOrderSearch(t *testing.T) {
	miniRoot := filepath.Join("..", "miniapp", "src")
	servicePath := filepath.Join(miniRoot, "pages", "service", "service.vue")
	if _, err := os.Stat(servicePath); err != nil {
		if os.IsNotExist(err) {
			t.Skip("miniapp source is not present in the orderapp-only Docker build context")
		}
		t.Fatalf("stat miniapp service page: %v", err)
	}
	servicePage, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read miniapp service page: %v", err)
	}
	serviceSrc := string(servicePage)
	for _, want := range []string{
		"cachedBeanListPage",
		"cacheBeanListPage",
		"beanListDisplayStyle",
		"beanListPageCacheStorageKey",
		"beanListPageCacheChanged",
		"bean-list-surface",
		"bean-list-cover",
		"bean-list-table",
		"今天",
		"最近三天",
		"最近7天",
		"本月",
		"收件人/地址/产品",
		"生产状态",
		"收款状态",
		"发货状态",
	} {
		if !strings.Contains(serviceSrc, want) {
			t.Fatalf("miniapp native bean list cache/order search source missing %q", want)
		}
	}
	for _, unwanted := range []string{"打开 PDF", "豆单 PDF", "openBeanListPDF", "openBeanListDocument", "saveBeanListPDF"} {
		if strings.Contains(serviceSrc, unwanted) {
			t.Fatalf("miniapp bean list page must render native content instead of PDF flow %q", unwanted)
		}
	}
}

func TestCustomerPortalInlinePDFStatusFilterRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER",
		"DEV-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER-01",
		"DEV-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER-02",
		"UT-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER-01",
		"API-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER-01",
		"REV-CUSTOMER-PORTAL-INLINE-PDF-STATUS-FILTER-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal inline PDF/status filter seed missing %q", want)
		}
	}
}

func TestCustomerPortalNativeBeanListCacheRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE",
		"DEV-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE-01",
		"DEV-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE-02",
		"UT-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE-01",
		"API-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE-01",
		"REV-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal native bean list cache seed missing %q", want)
		}
	}
}
