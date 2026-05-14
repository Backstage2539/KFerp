package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappCurrentCustomerOrderIsolationEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"SwitchCurrentCustomer(ctx context.Context, token string, customerID int64)",
		"WHERE b.mini_user_id=$1 AND b.customer_id=$2 AND b.status='approved'",
		"return r.CurrentContextByToken(ctx, token)",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing current-customer switch marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMiniappCurrentCustomerSwitchScopesOrderServicePage",
		"TestMiniappCurrentCustomerSwitchRejectsUnapprovedCustomerWithoutChangingSession",
		"SO-CURRENT-A",
		"SO-CURRENT-B",
		"customer A order leaked after switch",
		"unapproved switch changed current customer",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing current-customer order isolation marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
		"小程序当前客户订单隔离",
		"TestMiniappCurrentCustomerSwitchScopesOrderServicePage",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing current-customer order isolation marker %q", want)
		}
	}
}

func TestMiniappCurrentCustomerOrderIsolationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
		"DEV-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
		"UT-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
		"API-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
		"REV-186-MINIAPP-CURRENT-CUSTOMER-ORDER-ISOLATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestMiniappCurrentCustomerOrderIsolationManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"切换当前客户",
			"我的订单",
			"不能显示切换前客户",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing current-customer order isolation marker %q", path, want)
			}
		}
	}
}
