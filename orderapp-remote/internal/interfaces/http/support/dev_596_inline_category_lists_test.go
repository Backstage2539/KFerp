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
		{table: "req_dev", code: "DEV-596-DOCS-DEVELOPMENT-DELIVERY", status: "done", assignee: "Codex"},
		{table: "req_review", code: "REV-596-INLINE-CATEGORY-LISTS", status: "todo", assignee: "VA"},
	} {
		requireDev596SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}
	for _, want := range []string{
		"docs/acceptance/2026-08-10-inline-category-lists.md",
		"8e0aa8bfe86e26e4d0009603231752afb95d4ef2",
		"frontend server 964/964",
		"miniapp 205/205",
		"development HTTP 200",
		"eac3d213",
		"8b1d187",
		"production not deployed",
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
			"Branch: codex/pr596-final-evidence-20260810",
			"browser read-only QA and development delivery complete at `8e0aa8bf`",
			"frontend server 964/964",
			"miniapp 205/205",
			"DEV-596-DOCS-DEVELOPMENT-DELIVERY（done）",
			"REV-596-INLINE-CATEGORY-LISTS（todo）",
			"design-qa.md",
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
		"## 最终跟踪 RED 证据",
		"## 最终 GREEN 证据",
		"## development 最终部署证据",
		"8e0aa8bfe86e26e4d0009603231752afb95d4ef2",
		"frontend server 964/964",
		"miniapp 205/205",
		"Go 全包通过",
		"浏览器只读 QA 已完成",
		"development HTTP 200",
		"eac3d213",
		"容器未替换",
		"8b1d187",
		"design-qa.md",
		"production 未部署",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("PR-596 acceptance record missing delivery marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"rendered visual QA pending",
		"final tracking patch pending",
		"视觉 QA 待完成",
		"最终跟踪补丁待收尾",
	} {
		if strings.Contains(reqStore, forbidden) || strings.Contains(string(active), forbidden) || strings.Contains(acceptance, forbidden) {
			t.Fatalf("PR-596 final delivery tracking retains pending marker %q", forbidden)
		}
	}

	deployScript, err := os.ReadFile(filepath.Join(repoRoot, "deploy_orderapp.sh"))
	if err != nil {
		if !os.IsNotExist(err) || repoRoot != string(filepath.Separator) {
			t.Fatal(err)
		}
	} else if !strings.Contains(string(deployScript), "  design-qa.md \\") {
		t.Fatal("deploy_orderapp.sh must archive design-qa.md for the server-side PR-596 delivery contract")
	}

	designQAPath := filepath.Join(repoRoot, "design-qa.md")
	designQA, err := os.ReadFile(designQAPath)
	if err != nil {
		if !os.IsNotExist(err) || repoRoot != string(filepath.Separator) {
			t.Fatal(err)
		}
	} else {
		for _, want := range []string{
			"1536×1024",
			"system Chinese sans",
			"截图像素/1x 密度归一",
			"## 测试 state",
			"## Fidelity surfaces",
			"typography",
			"spacing/layout rhythm",
			"colors/tokens",
			"assets",
			"copy/content",
			"开发数据差异属于预期",
			"/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-8766b84c-1c9f-40d0-955b-be61088d973f.png",
			"/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-785663f3-5d66-414a-9ed2-1193b3e235f6.png",
			"/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-9963bf95-ebb8-4fa9-9f53-823c0de1a936.png",
			"/Users/yiiiple-work/.codex/generated_images/019fea4c-c092-7901-a89f-6a80d66cb9c8/exec-10ff9690-6394-46d5-a24f-1f352ac4146b.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/production-bom-drawer.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/materials-drawer.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/products-drawer.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/warehouse.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-bom.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-materials.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-products.png",
			"/Users/yiiiple-work/Documents/Codex/2026-08-10/referenced-chatgpt-conversation-this-is-an/outputs/pr596-design-qa/comparison-warehouse.png",
			"迭代 1",
			"P2",
			"文本 +/-",
			"无文件夹图标",
			"Tabler chevron/folder/folder-off",
			"迭代 2：无 P0/P1/P2",
			"交互只读检查",
			"最终结论：`passed`",
		} {
			if !strings.Contains(string(designQA), want) {
				t.Fatalf("design-qa.md missing PR-596 final QA marker %q", want)
			}
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
