package main

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowRequiresTestsBeforeImplementation(t *testing.T) {
	body, err := os.ReadFile("../HOW_TO_WORKFLOW.md")
	if err != nil {
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
		"produceLogs: { title: '生产日志', url: '/vue-shell?view=produceLogs', internal: true }",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing Vue internal production logs wiring %q", want)
		}
	}
}

func TestProductionFlowRoutesAndSchemaAreSplitOut(t *testing.T) {
	body, err := os.ReadFile("production_flow.go")
	if err != nil {
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

	for _, path := range []string{"production_flow_routes.go", "production_flow_schema.go"} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing split-out production flow file %s: %v", path, err)
		}
	}
}

func TestProductionDomainRulesAreNotImplementedInMainFlow(t *testing.T) {
	body, err := os.ReadFile("production_flow.go")
	if err != nil {
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

func TestProductionRoutesCallApplicationService(t *testing.T) {
	body, err := os.ReadFile("production_flow_routes.go")
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
