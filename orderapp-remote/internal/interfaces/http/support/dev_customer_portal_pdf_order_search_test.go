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

func TestMiniappServicePageSupportsBeanListPDFCacheAndOrderSearch(t *testing.T) {
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
		"openBeanListPDF",
		"uni.downloadFile",
		"uni.saveFile",
		"uni.openDocument",
		"今天",
		"最近三天",
		"最近7天",
		"本月",
		"收件人/地址/产品",
	} {
		if !strings.Contains(serviceSrc, want) {
			t.Fatalf("miniapp PDF cache/order search source missing %q", want)
		}
	}
}
