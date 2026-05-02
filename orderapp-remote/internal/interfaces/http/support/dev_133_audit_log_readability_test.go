package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev133AuditLogReadabilityRequirementSeeds(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		"PR-133",
		"DEV-133-01",
		"DEV-133-02",
		"DEV-133-03",
		"UT-133-01",
		"API-133-01",
		"REV-133-01",
		"操作日志可读性修正",
		"菜单与系统左侧菜单一致",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev133AuditLogFilterIncludesReadableEntityTypes(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(content)
	for _, want := range []string{
		`value="material_receipt"`,
		`value="sales_order_document"`,
		`value="cost_parameter"`,
		`value="auth_account"`,
		"原料入库单",
		"销售单文件",
		"成本参数",
		"员工账号",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("AuditView.vue missing %q", want)
		}
	}
}
