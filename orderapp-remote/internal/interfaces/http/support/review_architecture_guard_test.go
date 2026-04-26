package support

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowRequiresTestsBeforeImplementation(t *testing.T) {
	body, err := os.ReadFile("../HOW_TO_WORKFLOW.md")
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("HOW_TO_WORKFLOW.md is outside the orderapp Docker build context")
		}
		t.Fatal(err)
	}
	content := string(body)
	testFirst := strings.Index(content, "先编写单元测试代码")
	implementation := strings.Index(content, "实现代码")
	if testFirst < 0 {
		t.Fatal("HOW_TO_WORKFLOW.md must explicitly require writing unit/API test code before implementation")
	}
	if implementation < 0 {
		t.Fatal("HOW_TO_WORKFLOW.md missing implementation step")
	}
	if testFirst > implementation {
		t.Fatal("HOW_TO_WORKFLOW.md lists implementation before test-first work")
	}
	if strings.Contains(content, "3. 实现代码\n4. 单元测试") {
		t.Fatal("HOW_TO_WORKFLOW.md still recommends implementing before recording tests")
	}
}

func TestProductionLogPlanDoesNotExtendLegacyTemplates(t *testing.T) {
	body, err := os.ReadFile("docs/superpowers/plans/2026-04-25-production-log-yield.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"Extend the existing server-rendered production workflow",
		"server-rendered HTML templates",
		"templates/production_logs.html",
		"templates/unprod_summary.html",
		"templates/produce_running.html",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("production-log-yield plan still directs legacy template work: %q", forbidden)
		}
	}
	for _, required := range []string{"Vue/Vite", "JSON API", "frontend-vue-shell"} {
		if !strings.Contains(content, required) {
			t.Fatalf("production-log-yield plan missing architecture requirement %q", required)
		}
	}
}

func TestProductionLogsAreVueInternalView(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"ProductionLogsView",
		"produceLogs: ProductionLogsView",
		"produceLogs: { title: '生产日志'",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing Vue internal production logs wiring %q", want)
		}
	}
}

func TestVueShellDoesNotEmbedLegacyTemplatesInIframe(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"<iframe",
		"frameRef",
		"onFrameLoad",
		"vue-shell-embed-style",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("Vue shell still embeds legacy pages through iframe concern %q", forbidden)
		}
	}
	if strings.Contains(content, "LegacyMigrationView") || strings.Contains(content, "legacyUrl") {
		t.Fatal("Vue shell should no longer expose legacy fallback pages after migrated pages were deleted")
	}
}

func TestProductionFlowRoutesAndSchemaAreSplitOut(t *testing.T) {
	body, err := os.ReadFile("production_flow.go")
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"func registerProductionFlowPages",
		"func ensureProductionRunTable",
		"e.POST(\"/produce/start\"",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("production_flow.go still owns split-out concern %q", forbidden)
		}
	}

	for _, path := range []string{"internal/interfaces/http/production/production_flow_routes.go", "internal/interfaces/http/production/production_flow_schema.go"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing split-out production flow file %s: %v", path, err)
		}
	}
}

func TestProductionDomainRulesAreNotImplementedInMainFlow(t *testing.T) {
	body, err := os.ReadFile("production_flow.go")
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"func normalizeYieldRate(",
		"func defaultProductionInputG(",
		"func finishedTotalG(",
		"func actualYieldRate(",
		"func plannedFinishedInventoryByInput(",
		"func runningInventoryPlan(",
		"func plannedFinishedInventoryAddition(",
		"func normalizeFinishedInventoryAddition(",
		"func restoreAllocatedInventory(",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("production_flow.go still implements domain rule %q", forbidden)
		}
	}
	if _, err := os.Stat("internal/domain/production/yield.go"); err != nil {
		t.Fatalf("missing production domain rules file: %v", err)
	}
}

func TestProductionFlowMonolithIsRemoved(t *testing.T) {
	if _, err := os.Stat("production_flow.go"); err == nil {
		t.Fatal("production_flow.go should be removed; split production concerns into routes, schema, domain, application, and repository files")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := os.Stat("internal/interfaces/http/production/production_running_repository.go"); err != nil {
		t.Fatalf("missing production running repository split file: %v", err)
	}
}

func TestProductionRoutesCallApplicationService(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/production/production_flow_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if !strings.Contains(content, "productionapp.NewService") {
		t.Fatal("production_flow_routes.go should construct the production application service")
	}
	for _, forbidden := range []string{
		"startProductionWithInputs(",
		"listRunningItems(",
		"finishRunningItem(",
		"cancelRunningItem(",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("production_flow_routes.go still calls legacy repository function %q directly", forbidden)
		}
	}
}

func TestSalesSaveOrderCommandIsTyped(t *testing.T) {
	body, err := os.ReadFile("internal/application/sales/service.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	start := strings.Index(content, "type SaveOrderCommand struct")
	end := strings.Index(content, "type OrderItemCommand struct")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("sales service must define SaveOrderCommand followed by OrderItemCommand")
	}
	saveOrderCommand := content[start:end]
	for _, forbidden := range []string{
		"OrderDate             string",
		"ShippingAmount        string",
		"DiscountAmount        string",
		"RoundToInt            string",
		"ProductID             []string",
		"TierID                []string",
		"UnitPrice             []string",
		"ItemName              []string",
		"Qty                   []string",
		"Spec                  []string",
		"func (c SaveOrderCommand) GetMaterial()",
	} {
		if strings.Contains(saveOrderCommand, forbidden) {
			t.Fatalf("SaveOrderCommand still carries HTTP/form-shaped field %q", forbidden)
		}
	}
	for _, want := range []string{"OrderDate             time.Time", "Items                 []OrderItemCommand", "type OrderItemCommand struct"} {
		if !strings.Contains(content, want) {
			t.Fatalf("SaveOrderCommand missing typed field %q", want)
		}
	}
}

func TestSalesRepositoryDoesNotParseSaveOrderFormArrays(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/sales/sales_order_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"maxLen(cmd.ItemName",
		"getStr(cmd.ProductID",
		"getStr(cmd.TierID",
		"getStr(cmd.UnitPrice",
		"getStr(cmd.Qty",
		"getStr(cmd.Spec",
		"strconv.ParseFloat(v, 64)",
		"strconv.ParseInt(pidStr",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("sales_order_repository.go still parses HTTP/form-shaped command data: %q", forbidden)
		}
	}
}

func TestSalesServiceOwnsSaveOrderValidation(t *testing.T) {
	body, err := os.ReadFile("internal/application/sales/service.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if strings.Contains(content, "func (s *Service) SaveOrder(ctx context.Context, cmd SaveOrderCommand) (SaveOrderResult, error) {\n\treturn s.repo.SaveOrder(ctx, cmd)\n}") {
		t.Fatal("sales Service.SaveOrder still directly passes through to repository")
	}
	for _, want := range []string{"func validateSaveOrderCommand", "at least one item required", "customer required"} {
		if !strings.Contains(content, want) {
			t.Fatalf("sales service missing application validation %q", want)
		}
	}
}

func TestMaterialsAPIUsesApplicationService(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/production/materials_api.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if !strings.Contains(content, "materialsapp.NewService") {
		t.Fatal("materials_api.go should construct the materials application service")
	}
	for _, forbidden := range []string{
		"listMaterials(c.Request().Context(), pool, schema",
		"updateMaterialInline(c.Request().Context(), pool, schema",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("materials_api.go still calls repository helper directly: %q", forbidden)
		}
	}
}
