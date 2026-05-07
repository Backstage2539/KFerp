package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionDirectShipFlowRequirementSeeds(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"PR-144",
		"DEV-144-01",
		"DEV-144-02",
		"DEV-144-03",
		"UT-144-01",
		"API-144-01",
		"REV-144-01",
		"库存充足",
		"无需生产",
		"生产流程",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("production direct ship requirement seed missing %q", want)
		}
	}
}

func TestProductionPlanRemovesMaterialPlanAndGuidesDirectShipping(t *testing.T) {
	plan, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProducePlanView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(plan)
	for _, forbidden := range []string{"物料需求计划", "loadMaterialPlan", "materialPlanRows", "/api/produce/material-plan"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("ProducePlanView.vue should remove %q", forbidden)
		}
	}
	for _, want := range []string{"库存充足", "库存待发货", "直接发货", "openShipReadyOrders"} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProducePlanView.vue missing direct shipping guidance %q", want)
		}
	}
}

func TestOrdersViewTreatsNoProductionAsShipReady(t *testing.T) {
	orders, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(orders)
	for _, want := range []string{"isShipReady", "无需生产", "库存待发货", "只看可发货", "ship_ready"} {
		if !strings.Contains(src, want) {
			t.Fatalf("OrdersView.vue missing ship-ready source %q", want)
		}
	}
	if strings.Contains(src, "只看生产完成") {
		t.Fatal("OrdersView.vue should rename the shipping preset away from only production-complete wording")
	}
}

func TestProductionFlowPageIsPrimaryNewEmployeeEntry(t *testing.T) {
	menu, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"))
	if err != nil {
		t.Fatal(err)
	}
	menuSrc := string(menu)
	flowIndex := strings.Index(menuSrc, "{ key: 'productionManual', label: '生产流程'")
	planIndex := strings.Index(menuSrc, "{ key: 'producePlan'")
	if flowIndex < 0 {
		t.Fatal("production menu must expose productionManual as label 生产流程")
	}
	if planIndex < 0 {
		t.Fatal("production menu missing producePlan")
	}
	if flowIndex > planIndex {
		t.Fatal("production flow page should appear before production plan for new operators")
	}

	manual, err := os.ReadFile(filepath.Join("frontend-vue-shell", "src", "views", "ProductionManualView.vue"))
	if err != nil {
		t.Fatal(err)
	}
	manualSrc := string(manual)
	for _, want := range []string{"生产流程用户手册", "流程导航", "库存待发货", "直接发货"} {
		if !strings.Contains(manualSrc, want) {
			t.Fatalf("ProductionManualView.vue missing %q", want)
		}
	}
	if strings.Contains(manualSrc, "物料需求计划") {
		t.Fatal("ProductionManualView.vue should not refer operators to the removed material plan page")
	}
}
