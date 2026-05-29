package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappInactiveCustomerBindingGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"JOIN %s.customers c ON c.id=b.customer_id AND c.active=true",
		"b.status='approved' AND c.active=true",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer portal repository missing inactive customer binding marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCurrentContextByTokenSwitchesInactiveCurrentCustomerToFirstActiveBinding",
		"TestSwitchCurrentCustomerRejectsInactiveApprovedCustomer",
		"已停用客户",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer portal repository test missing inactive customer binding marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMiniCurrentCustomerInactiveBindingMapsToForbidden",
		"customer binding not found",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("mini API test missing inactive customer binding marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
		"停用客户",
		"TestCurrentContextByTokenSwitchesInactiveCurrentCustomerToFirstActiveBinding",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing inactive customer binding marker %q", want)
		}
	}
}

func TestMiniappInactiveCustomerBindingGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
		"DEV-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
		"UT-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
		"API-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
		"REV-210-MINIAPP-INACTIVE-CUSTOMER-BINDING-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMiniappInactiveCustomerBindingGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"停用客户",
			"小程序当前客户",
			"不能切换",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing inactive customer binding marker %q", path, want)
			}
		}
	}
}
