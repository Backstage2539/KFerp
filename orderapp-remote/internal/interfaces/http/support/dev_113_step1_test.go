package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPDFRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, needle := range []string{
		"PR-SALES-ORDER-001",
		"DEV-SALES-ORDER-001",
		"UT-SALES-ORDER-001",
		"API-SALES-ORDER-001",
		"REV-SALES-ORDER-001",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("sales order pdf requirement seed missing %q", needle)
		}
	}
}

func TestBeanListPublishAdminOnlyRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-113",
		"DEV-113-01",
		"DEV-113-02",
		"DEV-113-03",
		"UT-113-01",
		"API-113-01",
		"REV-113-01",
		"只有管理员才可以发布豆单",
		"客户登录后只能保存修改和下载豆单",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("admin-only bean-list publishing requirement seed missing %q", want)
		}
	}
}
