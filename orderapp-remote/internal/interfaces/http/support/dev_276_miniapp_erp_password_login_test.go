package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappERPPasswordLoginRequirementRecords(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-276-MINIAPP-ERP-PASSWORD-LOGIN",
		"DEV-276-MINIAPP-ERP-PASSWORD-LOGIN",
		"UT-276-MINIAPP-ERP-PASSWORD-LOGIN",
		"API-276-MINIAPP-ERP-PASSWORD-LOGIN",
		"REV-276-MINIAPP-ERP-PASSWORD-LOGIN",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %s", want)
		}
	}

	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md")))
	for _, want := range []string{"微信手机号", "客户绑定", "个人中心", "退出登录", "旧版兼容"} {
		if !strings.Contains(manual, want) {
			t.Fatalf("OP_MANUAL_CUSTOMER_PORTAL.md missing %q", want)
		}
	}

	miniappTestManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "customer-portal-miniapp-test.md")))
	for _, want := range []string{"微信手机号", "旧版兼容", "ERP 桌面登录", "真机验收"} {
		if !strings.Contains(miniappTestManual, want) {
			t.Fatalf("customer-portal-miniapp-test.md missing %q", want)
		}
	}

	requirements := string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md")))
	if !strings.Contains(requirements, "PR-276-MINIAPP-ERP-PASSWORD-LOGIN") || !strings.Contains(requirements, "ERP账号密码登录") {
		t.Fatalf("REQUIREMENTS.md missing PR-274 miniapp ERP password login requirement")
	}
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md")))
	if !strings.Contains(acceptance, "PR-276-MINIAPP-ERP-PASSWORD-LOGIN") || !strings.Contains(acceptance, "/api/mini/login/password") {
		t.Fatalf("ACCEPTANCE_TESTS.md missing PR-274 miniapp ERP password login acceptance")
	}
}
