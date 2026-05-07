package support

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestOperationManualGovernanceRequirementSeeds(t *testing.T) {
	b, err := readFirstExistingManual(
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"req_store.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"PR-DOCS-001",
		"PR-DOCS-003",
		"DEV-DOCS-001",
		"DEV-DOCS-002",
		"DEV-DOCS-003",
		"DEV-DOCS-006",
		"UT-DOCS-001",
		"UT-DOCS-003",
		"API-DOCS-001",
		"API-DOCS-003",
		"REV-DOCS-001",
		"REV-DOCS-003",
		"操作手册强制更新机制",
		"操作手册图示化",
		"单个大功能一个手册",
		"OPERATION_MANUALS.md",
		"OP_MANUAL_*.md",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("operation manual requirement seed missing %q", want)
		}
	}
}

func TestOperationManualWorkflowGuard(t *testing.T) {
	b, err := readFirstExistingManual(
		filepath.Join("..", "HOW_TO_WORKFLOW.md"),
		filepath.Join("..", "..", "HOW_TO_WORKFLOW.md"),
		filepath.Join("..", "..", "..", "..", "..", "HOW_TO_WORKFLOW.md"),
	)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("HOW_TO_WORKFLOW.md is outside the orderapp Docker build context")
		}
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"操作手册更新",
		"单个大功能必须有一个独立操作手册",
		"现有操作手册要持续查缺补漏",
		"REV 证据必须包含手册文件路径",
		"OP_MANUAL_<FEATURE>.md",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("workflow manual missing operation manual rule %q", want)
		}
	}
	manualStep := strings.Index(text, "操作手册：新增或更新")
	reviewStep := strings.Index(text, "需求审核：按 PR/ACCEPTANCE_TESTS")
	if manualStep < 0 || reviewStep < 0 {
		t.Fatal("workflow missing manual or review step")
	}
	if manualStep > reviewStep {
		t.Fatal("workflow must update operation manuals before REV")
	}
}

func readFirstExistingManual(paths ...string) ([]byte, error) {
	var lastErr error
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func TestOperationManualDocsAreMirrored(t *testing.T) {
	manuals := []string{
		"OPERATION_MANUALS.md",
		"OP_MANUAL_REQUIREMENTS.md",
		"OP_MANUAL_ORDER_SALES.md",
		"OP_MANUAL_PRODUCTION.md",
		"OP_MANUAL_INVENTORY_MATERIALS.md",
		"OP_MANUAL_COSTING.md",
		"OP_MANUAL_FINANCE.md",
		"OP_MANUAL_SETTINGS_AUDIT.md",
		"OP_MANUAL_CUSTOMER_PORTAL.md",
		"OP_MANUAL_CUSTOMER_FULFILLMENT.md",
	}
	for _, name := range manuals {
		local, err := os.ReadFile(filepath.Join("docs", name))
		if err != nil {
			t.Fatalf("missing orderapp docs manual %s: %v", name, err)
		}
		if !strings.Contains(string(local), "操作手册") {
			t.Fatalf("%s should be an operation manual", name)
		}

		rootPath := filepath.Join("..", name)
		root, err := os.ReadFile(rootPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("reading root manual %s: %v", name, err)
		}
		if string(root) != string(local) {
			t.Fatalf("root manual %s and orderapp docs copy differ", name)
		}
	}
}

func TestOperationManualDocsHaveFlowcharts(t *testing.T) {
	manuals := []string{
		"OP_MANUAL_REQUIREMENTS.md",
		"OP_MANUAL_ORDER_SALES.md",
		"OP_MANUAL_PRODUCTION.md",
		"OP_MANUAL_INVENTORY_MATERIALS.md",
		"OP_MANUAL_COSTING.md",
		"OP_MANUAL_FINANCE.md",
		"OP_MANUAL_SETTINGS_AUDIT.md",
		"OP_MANUAL_CUSTOMER_PORTAL.md",
		"OP_MANUAL_CUSTOMER_FULFILLMENT.md",
	}
	for _, name := range manuals {
		b, err := os.ReadFile(filepath.Join("docs", name))
		if err != nil {
			t.Fatalf("missing orderapp docs manual %s: %v", name, err)
		}
		text := string(b)
		if !strings.Contains(text, "## 流程图") {
			t.Fatalf("%s should include a flowchart section", name)
		}
		if !strings.Contains(text, "```mermaid") || !strings.Contains(text, "flowchart ") {
			t.Fatalf("%s should include a Mermaid flowchart", name)
		}
	}
}

func TestConsolidatedOperationManualsReplaceLegacyDocs(t *testing.T) {
	deprecated := []string{
		"delivery-note-user-manual.md",
		"order-entry-user-manual.md",
		"orders-user-manual.md",
		"production-flow-user-manual.md",
		"purchase-user-manual.md",
		"sales-order-user-manual.md",
		"user-permissions-user-manual.md",
		"finance-monthly-closing-user-manual.md",
		"customer-fulfillment-user-manual.md",
		"customer-portal-mall-user-manual.md",
	}
	for _, name := range deprecated {
		if _, err := os.Stat(filepath.Join("docs", name)); !os.IsNotExist(err) {
			t.Fatalf("legacy manual %s should be consolidated into OP_MANUAL_*.md and removed from docs", name)
		}
	}
}

func TestDocsRawRouteServesOperationManual(t *testing.T) {
	dir := t.TempDir()
	content := "# 操作手册总索引\n\n单个大功能一个独立手册。\n"
	if err := os.WriteFile(filepath.Join(dir, "OPERATION_MANUALS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	h := docsHandler{dir: dir}
	e.GET("/docs/:name", h.view)

	req := httptest.NewRequest(http.MethodGet, "/docs/OPERATION_MANUALS.md?raw=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "单个大功能一个独立手册") {
		t.Fatalf("operation manual raw body missing content: %s", rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderContentType); !strings.Contains(got, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
}
