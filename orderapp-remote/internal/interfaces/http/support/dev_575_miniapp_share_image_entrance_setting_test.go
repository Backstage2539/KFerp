package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev575MiniappShareImageEntranceSettingContract(t *testing.T) {
	files := []struct {
		path    string
		needles []string
	}{
		{
			path: filepath.Join("internal", "interfaces", "http", "customerportal", "mini_employee_share_settings_api.go"),
			needles: []string{
				"/api/mini/employee/share-settings",
				"miniapp.share_image.need_show_entrance",
				"settings.write",
				"containsMiniRole(employee.Roles, \"admin\")",
				"miniEmployeeActor(employee)",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "ui_settings.go"),
			needles: []string{
				"pg_advisory_xact_lock",
				"NewAuditService(tx, s.schema).Insert",
				"tx.Commit(ctx)",
			},
		},
		{
			path: filepath.Join("frontend-vue-shell", "src", "views", "AuditView.vue"),
			needles: []string{
				`<option value="ui_setting">系统设置</option>`,
			},
		},
		{
			path: filepath.Join("..", "miniapp", "src", "pages", "profile", "profile.vue"),
			needles: []string{
				"session.accountType === 'employee'",
				"session.roles.includes('admin')",
				"session.permissions.includes('settings.write')",
				"分享图片时携带小程序入口",
				"saveEmployeeShareSettings",
			},
		},
		{
			path: filepath.Join("..", "miniapp", "src", "pages", "employee-order-detail", "employee-order-detail.vue"),
			needles: []string{
				"fetchEmployeeShareSettings",
				"format === 'png'",
				"needShowEntrance: imageNeedShowEntrance",
			},
		},
		{
			path: filepath.Join("..", "miniapp", "src", "utils", "fileOutput.ts"),
			needles: []string{
				"needShowEntrance: options.needShowEntrance === true",
			},
		},
		{
			path: filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
			needles: []string{
				"PR-575-MINIAPP-SHARE-IMAGE-ENTRANCE-SETTING",
				"DEV-575-SHARE-ENTRANCE-SETTING-API-AUDIT",
				"DEV-575-ADMIN-PROFILE-SETTING",
				"DEV-575-IMAGE-SHARE-RUNTIME",
				"REV-575-MINIAPP-SHARE-IMAGE-ENTRANCE-SETTING",
			},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"),
			needles: []string{"全局开关", "所有员工", "低版本微信", "历史消息"},
		},
		{
			path:    filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"),
			needles: []string{"系统 / 小程序设置", "分享图片携带小程序入口", "旧值和新值"},
		},
		{
			path:    filepath.Join("docs", "acceptance", "2026-08-03-miniapp-share-image-entrance-setting.md"),
			needles: []string{"TDD RED 证据", "自动化验收矩阵", "Van 验收清单", "a06aa95ebe38d7b91806cd234032c0cc3bb62a7e", "未上传、未提交审核、未发布"},
		},
	}

	for _, file := range files {
		body := string(readOrderAppFileForTest(t, file.path))
		for _, needle := range file.needles {
			if !strings.Contains(body, needle) {
				t.Fatalf("%s missing %q", file.path, needle)
			}
		}
	}

	fileOutput := string(readOrderAppFileForTest(t, filepath.Join("..", "miniapp", "src", "utils", "fileOutput.ts")))
	if strings.Contains(fileOutput, "needShowEntrance: false") {
		t.Fatal("fileOutput.ts must not hard-code the system setting to false")
	}
}
