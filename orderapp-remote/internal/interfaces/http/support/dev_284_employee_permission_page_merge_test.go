package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEmployeePermissionPageMergeWiring(t *testing.T) {
	companyStaff := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyStaffView.vue")))
	userPermissions := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UserPermissionsView.vue")))
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))

	for _, want := range []string{
		"fetchInternalAuthAccounts",
		"resetEmployeePassword",
		"saveEmployeeRoles",
		"内部权限",
		"设置密码",
		"重置密码",
	} {
		if !strings.Contains(companyStaff, want) {
			t.Fatalf("CompanyStaffView.vue should own merged employee permission behavior %q", want)
		}
	}
	for _, forbidden := range []string{"account_type", "渠道客户", "setAccountType", "isChannelCustomer", "公共SKU代发客户", "代加工客户"} {
		if strings.Contains(companyStaff, forbidden) {
			t.Fatalf("merged employee page must not expose external account behavior %q", forbidden)
		}
	}
	if strings.Contains(app, "UserPermissionsView") {
		t.Fatal("App.vue should not import or render a standalone UserPermissionsView")
	}
	if !strings.Contains(app, "userPermissions: 'employees'") {
		t.Fatal("App.vue should keep view=userPermissions as a legacy alias to employees")
	}
	if strings.Contains(menu, "label: '用户权限'") || strings.Contains(menu, "key: 'userPermissions'") {
		t.Fatal("menu-ia.js should not expose userPermissions as a standalone menu/config item")
	}
	if strings.TrimSpace(userPermissions) != "" && !strings.Contains(userPermissions, "view=userPermissions 已合并到员工维护") {
		t.Fatal("UserPermissionsView.vue should be removed or reduced to a compatibility note")
	}
}
