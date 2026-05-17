package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemSettingsMenuClickMatrixEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))
	for _, want := range []string{
		"SYSTEM_SETTINGS_MENU_CLICK_MATRIX_SMOKE_OK",
		"views=5",
		"companyProfile",
		"machines",
		"employees",
		"departments",
		"audit",
		"save_company_profile",
		"create_machine",
		"create_department",
		"create_employee",
		"save_roles",
		"audit_filter",
		"port_18163_free",
		"port_9242_free",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing system/settings click matrix marker %q", want)
		}
	}

	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-286-SYSTEM-SETTINGS-MENU-CLICK-MATRIX",
		"DEV-286-SYSTEM-SETTINGS-MENU-CLICK-MATRIX",
		"SYSTEM_SETTINGS_MENU_CLICK_MATRIX_SMOKE_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing system/settings click matrix marker %q", want)
		}
	}
}

func TestSystemSettingsMenuClickMatrixViewsExposeActions(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "CompanyProfileView.vue"): {
			"/api/company/profile",
			"保存公司设置",
			"copyAccountInfo",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "MachinesView.vue"): {
			"/api/produce/machines",
			"saveNew",
			"allowed_specs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CompanyStaffView.vue"): {
			"/api/company/departments",
			"/api/company/employees",
			"fetchRoles",
			"saveEmployeeRoles",
			"resetEmployeePassword",
			"setAccountState",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"): {
			"/api/audit",
			"filters.type",
			"replaceHistoryURL",
			"筛选",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing system/settings click matrix marker %q", rel, want)
			}
		}
	}
}
