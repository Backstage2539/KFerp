package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev579580CustomerAddressPortalExternalUserDocumentationContract(t *testing.T) {
	markers := []string{
		"PR-579-MINIAPP-CUSTOMER-ADDRESS-PASTE",
		"DEV-579-SHARED-RECIPIENT-PARSE-API",
		"DEV-579-MINIAPP-CUSTOMER-PASTE",
		"DEV-579-DOCS-ACCEPTANCE",
		"REV-579-MINIAPP-CUSTOMER-ADDRESS-PASTE",
		"PR-580-CUSTOMER-PORTAL-EXTERNAL-USER-CAPABILITY-TEMPLATE",
		"DEV-580-EXTERNAL-ACCOUNT-WORKBENCH-SEPARATION",
		"DEV-580-ERP-SESSION-GATE",
		"DEV-580-AUTH-BOOTSTRAP-HARDENING",
		"DEV-580-EXTERNAL-USER-PERMISSION",
		"DEV-580-EXTERNAL-USER-AUDIT",
		"DEV-580-MINI-SESSION-REVOCATION",
		"DEV-580-ACTIVE-BINDING-MUTATION",
		"DEV-580-DOCS-ACCEPTANCE",
		"REV-580-CUSTOMER-PORTAL-EXTERNAL-USER-CAPABILITY-TEMPLATE",
	}

	for name, rel := range map[string]string{
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"requirement store": filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-08-04-miniapp-customer-address-portal-external-user.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", name, marker)
			}
		}
	}

	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)
	for _, rel := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		contents, err := os.ReadFile(filepath.Join(workspaceRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(contents), marker) {
				t.Fatalf("root %s missing %s", rel, marker)
			}
		}
	}

	manuals := map[string][]string{
		filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"): {
			"POST /api/customer-recipient/parse",
			"不记录粘贴原文",
			"操作日志",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"ERP 与员工小程序共用",
			"POST /api/customer-recipient/parse",
			"仍可手工修改",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"外部账号关联与 ERP 工作台授权分离",
			"ERP workbench unavailable for capability template",
			"ERP 网页端的密码登录和已存在的 ERP 登录会话",
			"短信兼容登录只接受既有 active 内部员工的预置有效验证码",
			"不会保存密码明文或密码哈希",
			"customers.read",
			"customers.write",
			"旧 token 不会自动恢复",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"非工作台模板",
			"小程序密码登录",
			"不得进入 ERP 工作台",
			"历史 inactive 绑定只读",
		},
		filepath.Join("docs", "OP_MANUAL_SETTINGS_AUDIT.md"): {
			"真实短信发送通道尚未接入",
			"不会在页面或接口回显验证码",
			"系统 / 员工维护",
		},
	}
	for path, needles := range manuals {
		contents := string(readOrderAppFileForTest(t, path))
		for _, needle := range needles {
			if !strings.Contains(contents, needle) {
				t.Fatalf("%s missing %q", path, needle)
			}
		}
	}
}
