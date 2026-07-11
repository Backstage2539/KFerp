package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev528GroupSettingsSeparationContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "App.vue"): {
			"groupTemplates: GroupTemplatesView",
			"uiSettings: UISettingsView",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "GroupTemplatesView.vue"): {
			"<h2>分组模板</h2>",
			"/api/business-groups",
			"/api/business-group-items",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-528-SEPARATE-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"两个独立 Vue 页面",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-528-SEPARATE-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"系统设置页面不显示分组模板",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"设置 / 分组模板",
			"设置 / 系统设置",
		},
		"internal/interfaces/http/support/req_store.go": {
			"PR-528-SEPARATE-GROUP-TEMPLATE-SYSTEM-SETTINGS",
			"DEV-528-INDEPENDENT-VIEWS",
			"DEV-528-ROUTE-COMPATIBILITY",
		},
	}
	for path, markers := range checks {
		body := string(readOrderAppFileForTest(t, path))
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Fatalf("%s missing PR-528 marker %q", path, marker)
			}
		}
	}

	systemSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	for _, forbidden := range []string{"<h2>分组模板</h2>", "/api/business-groups", "/api/business-group-items"} {
		if strings.Contains(systemSettings, forbidden) {
			t.Fatalf("system settings still contains group-template marker %q", forbidden)
		}
	}
}
