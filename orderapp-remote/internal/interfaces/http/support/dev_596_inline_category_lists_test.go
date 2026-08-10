package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev596InlineCategoryListsDeliveryContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-596-INLINE-CATEGORY-LISTS", status: "review", assignee: "VA"},
		{table: "req_dev", code: "DEV-596-SHARED-INLINE-GROUP-WORKSPACE", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-596-MATERIAL-PRODUCT-LISTS", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-596-BOM-SETTINGS-DRAWER", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-596-WAREHOUSE-INVENTORY-LISTS", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-596-DOCS-DEVELOPMENT-DELIVERY", status: "doing", assignee: "Codex"},
		{table: "req_review", code: "REV-596-INLINE-CATEGORY-LISTS", status: "todo", assignee: "VA"},
	} {
		requireDev596SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}
	for _, want := range []string{
		"docs/acceptance/2026-08-10-inline-category-lists.md",
		"cfc781df3e8cb540ec4d853bdd30ebf108caa26b",
		"rendered visual QA pending",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-596 delivery evidence %q", want)
		}
	}

	repoRoot := filepath.Dir(findAncestorForTest(t, "go.mod"))
	active, err := os.ReadFile(filepath.Join(repoRoot, "ACTIVE_REQUIREMENTS.md"))
	if err != nil {
		// The release Dockerfile deliberately copies only durable root
		// governance documents into its isolated /src build context. Keep ACTIVE
		// mandatory in a real checkout without widening that Docker contract.
		if !os.IsNotExist(err) || repoRoot != string(filepath.Separator) {
			t.Fatal(err)
		}
	} else {
		for _, want := range []string{
			"### PR-596-INLINE-CATEGORY-LISTS",
			"Branch: codex/pr596-delivery-evidence-20260810",
			"first development deployment complete at `cfc781df`",
			"rendered visual QA and final tracking patch pending",
			"DEV-596-DOCS-DEVELOPMENT-DELIVERY（in progress）",
			"REV-596-INLINE-CATEGORY-LISTS（todo）",
		} {
			if !strings.Contains(string(active), want) {
				t.Fatalf("ACTIVE_REQUIREMENTS.md missing PR-596 delivery marker %q", want)
			}
		}
	}

	acceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "acceptance", "2026-08-10-inline-category-lists.md")))
	for _, want := range []string{
		"## RED 证据",
		"go test ./internal/interfaces/http/support -run TestDev596InlineCategoryListsDeliveryContracts -count=1",
		"## GREEN 证据",
		"## development 首次部署证据",
		"cfc781df3e8cb540ec4d853bdd30ebf108caa26b",
		"development 首次部署已完成",
		"视觉 QA 待完成",
		"最终跟踪补丁待收尾",
		"production 未部署",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("PR-596 acceptance record missing delivery marker %q", want)
		}
	}
}

func requireDev596SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
