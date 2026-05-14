package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRetailTemplateWorkbenchInvariantEvidenceExists(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"templateERPWorkbenchDisallowed",
		"ErrCapabilityTemplateERPWorkbenchUnavailable",
		"defaultTemplate.ExposesERPWorkbench()",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("customerportal service missing retail template workbench invariant marker %q", want)
		}
	}
	for _, want := range []string{
		"TestSaveCapabilityTemplateRejectsRetailMallERPWorkbenchFields",
		"TestUpsertPortalERPBindingRejectsSavedRetailMallTemplateWithERPWorkbench",
		"retail ERP workbench template should not be saved",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("customerportal service test missing retail template workbench invariant marker %q", want)
		}
	}
	if !strings.Contains(apiTest, "TestPortalAdminCapabilityTemplateERPWorkbenchUnavailableMapsToBadRequest") {
		t.Fatal("customerportal API tests missing retail template workbench invariant bad request marker")
	}
	for _, want := range []string{
		"PR-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
		"零售商城模板不能保存 ERP 工作台字段",
		"TestSaveCapabilityTemplateRejectsRetailMallERPWorkbenchFields",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing retail template workbench invariant marker %q", want)
		}
	}
}

func TestRetailTemplateWorkbenchInvariantRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
		"DEV-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
		"UT-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
		"API-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
		"REV-191-RETAIL-TEMPLATE-WORKBENCH-INVARIANT",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestRetailTemplateWorkbenchInvariantManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"零售商城模板不能保存 ERP 工作台字段",
			"ERP 工作台字段会被拒绝",
			"零售商城客户不绑定 ERP 工作台账号",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing retail template workbench invariant marker %q", path, want)
			}
		}
	}
}
