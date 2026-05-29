package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerAccountIsolationGuardEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"PR-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
		"REAL_PG_CAPABILITY_GATE_OK pg=55450",
		"CUSTOMER_ACCOUNT_ISOLATION_SMOKE_OK app=http://127.0.0.1:18091 pg=55451",
		"customer capability processing unavailable",
		"E2E隔离代加工客户",
		"E2E隔离公共SKU客户",
		"a_direct_ship_rows=1",
		"b_processing_rows=0",
		"当前结论：未完成",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestCustomerAccountIsolationGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
		"DEV-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
		"UT-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
		"API-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
		"REV-181-CUSTOMER-ACCOUNT-ISOLATION-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestCustomerAccountIsolationGuardSourceChecksCapabilities(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))

	for _, want := range []string{
		"requireCustomerCapability(ctx, customerID, \"processing\")",
		"requireCustomerCapability(ctx, customerID, \"direct_ship\")",
		"customer capability %s unavailable",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customerfulfillment repository missing marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCustomerPortalSubmitRequiresBoundCustomerCapability",
		"customer capability processing unavailable",
		"customer capability direct_ship unavailable",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customerfulfillment repository test missing marker %q", want)
		}
	}
}

func TestCustomerAccountIsolationGuardManualDocumentsFailureHandling(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		manual := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"代加工模板账号才会显示和提交加工工单",
			"公共 SKU 小批量模板账号在工作台显示“提交订单信息”",
			"customer capability processing unavailable",
		} {
			if !strings.Contains(manual, want) {
				t.Fatalf("%s missing customer account isolation manual marker %q", path, want)
			}
		}
	}
}
