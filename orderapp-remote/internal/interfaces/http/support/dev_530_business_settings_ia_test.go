package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev530BusinessSettingsIAContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-530-BUSINESS-SETTINGS-IA",
			"DEV-530-BUSINESS-SETTINGS-TABS",
			"DEV-530-SYSTEM-NOTIFICATION-TAB",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"name: '商品'",
			"key: 'businessSettings', label: '业务设置'",
			"machines: '设备产能配置'",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BusinessSettingsView.vue"): {
			"销售单设置", "物流设置", "发货人设置", "分组模板", "全局单位字典",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue"): {
			"系统基础设置", "通知设置", "NotificationSettingsView",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {"PR-530-BUSINESS-SETTINGS-IA"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {"PR-530-BUSINESS-SETTINGS-IA"},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {"设置 / 业务设置", "系统基础设置", "设备产能配置"},
		filepath.Join("docs", "acceptance", "2026-07-12-business-settings-ia.md"): {"PR-530 商品菜单与业务/系统设置整合验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-530 marker %q", rel, want)
			}
		}
	}
}

func TestDev530StandaloneMenuEntriesAreRemoved(t *testing.T) {
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	primary := strings.Split(menu, "export const hiddenViewTitles")[0]
	for _, forbidden := range []string{
		"key: 'machines'", "key: 'salesOrderSettings'", "key: 'logisticsSettings'", "key: 'senderSettings'", "key: 'groupTemplates'", "key: 'notificationSettings'",
	} {
		if strings.Contains(primary, forbidden) {
			t.Fatalf("primary menu still contains consolidated setting %q", forbidden)
		}
	}
}
