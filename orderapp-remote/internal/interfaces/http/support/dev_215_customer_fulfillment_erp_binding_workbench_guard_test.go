package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentERPBindingWorkbenchGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"requireCustomerERPWorkbenchTemplateTx",
		"CustomerCapabilityTemplateByKey",
		"ErrCapabilityTemplateERPWorkbenchUnavailable",
		"customer_portal_profiles",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing ERP binding workbench marker %q", want)
		}
	}
	for _, want := range []string{
		"TestUpsertCustomerERPBindingRejectsTemplateWithoutERPWorkbench",
		"retail_mall",
		"ERP workbench unavailable for capability template",
		"customer_erp_user_bindings",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing ERP binding workbench marker %q", want)
		}
	}
	for _, want := range []string{
		"TestInternalERPBindingAPIWorkbenchUnavailableMapsToBadRequest",
		"ERP workbench unavailable for capability template",
		"erpBindingErr",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("customer fulfillment API test missing ERP binding workbench marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"TestUpsertCustomerERPBindingRejectsTemplateWithoutERPWorkbench",
		"ERP 工作台绑定",
		"CUSTOMER_ERP_BINDING_WORKBENCH_REAL_API_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing ERP binding workbench marker %q", want)
		}
	}
}

func TestCustomerFulfillmentERPBindingWorkbenchGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"DEV-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"UT-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"API-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"REV-215-CUSTOMER-FULFILLMENT-ERP-BINDING-WORKBENCH-GUARD",
		"CUSTOMER_ERP_BINDING_WORKBENCH_REAL_API_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerFulfillmentERPBindingWorkbenchGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"ERP workbench unavailable for capability template",
			"ERP 工作台",
			"零售商城",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing ERP binding workbench marker %q", path, want)
			}
		}
	}
}
