package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev350BeanListPublishManualWithdraw(t *testing.T) {
	reqStore := readDev350Text(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	for _, want := range []string{
		"PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"DEV-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"UT-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"API-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"REV-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"2026-05-24-bean-list-publish-manual-withdraw.md",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("dev 350 req seed missing %q", want)
		}
	}

	requireDev350Contains(t, "docs/REQUIREMENTS.md",
		"PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"发布新版豆单不得自动撤回旧版",
		"录单只能选择已发布豆单",
	)
	requireDev350Contains(t, "docs/ACCEPTANCE_TESTS.md",
		"PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"旧版仍保持已发布",
		"已撤回豆单不出现在录单豆单选择器",
	)
	requireDev350Contains(t, "docs/OP_MANUAL_ORDER_SALES.md",
		"PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"录单只能选择状态为已发布的豆单",
	)
	requireDev350Contains(t, "docs/acceptance/2026-05-24-bean-list-publish-manual-withdraw.md",
		"发布新版豆单后旧版仍保持已发布",
		"撤回只能通过版本列表手动撤回",
	)
	requireDev350Contains(t, "docs/OP_MANUAL_COSTING.md",
		"PR-350-BEAN-LIST-PUBLISH-MANUAL-WITHDRAW",
		"发布新版不会自动撤回旧版",
	)

	requireDev350Contains(t, "internal/infrastructure/postgres/costing/repository_test.go",
		"TestPublishBeanListDoesNotWithdrawExistingPublishedSnapshots",
	)
	requireDev350Contains(t, "internal/infrastructure/postgres/sales/order_form_queries_static_test.go",
		"TestOrderFormBeanListVersionOptionsUseOnlyPublishedSnapshots",
		"TestOrderSaveExplicitBeanListPublicationRequiresPublishedSnapshot",
	)
	requireDev350Contains(t, "internal/infrastructure/postgres/orderbeans/usage_test.go",
		"TestExplicitPublicationSelectionRequiresPublishedSnapshots",
	)
	requireDev350Contains(t, "internal/interfaces/http/sales/order_api_test.go",
		"TestOrderAPIFormHidesWithdrawnPublicBeanListVersionsForFallbackCustomer",
		"TestOrderAPIRejectsWithdrawnPublicBeanListPublicationVersion",
	)
}

func requireDev350Contains(t *testing.T, path string, wants ...string) {
	t.Helper()
	text := readDev350Text(t, path)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q", path, want)
		}
	}
}

func readDev350Text(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
