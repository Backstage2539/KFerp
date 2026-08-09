package costing

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestResolveProductionBomCostsIncludesProductComponentsAndOperationsWithoutLegacyYield(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		2: {
			ProductID:            2,
			VersionID:            202,
			YieldRate:            0.8,
			OutputUnit:           "kg",
			OperationCostPerUnit: 5,
			Items: []productionBomCostItem{{
				ID:            2001,
				ComponentType: "material",
				ConsumeUnit:   "ratio_pct",
				RatioPct:      100,
				UnitCost:      80,
			}},
		},
		1: {
			ProductID:            1,
			VersionID:            101,
			YieldRate:            1,
			OutputUnit:           "袋",
			OperationCostPerUnit: 0.2,
			Items: []productionBomCostItem{
				{
					ID:                 1001,
					ComponentType:      "product",
					ComponentProductID: 2,
					ConsumeUnit:        "g_per_bag",
					QtyPerUnit:         10,
				},
				{
					ID:            1002,
					ComponentType: "material",
					ConsumeUnit:   "unit_per_bag",
					QtyPerUnit:    1,
					UnitCost:      0.4,
				},
			},
		},
	}

	costs := resolveProductionBomCosts(nodes)
	component := costs[2]
	if !component.Resolved {
		t.Fatal("component product BOM cost was not resolved")
	}
	if diff := math.Abs(component.InputCostPerOutputUnit - 80); diff > 1e-9 {
		t.Fatalf("component input cost = %.4f, want 80 with legacy 0.8 yield ignored", component.InputCostPerOutputUnit)
	}
	if diff := math.Abs(component.TotalCostPerOutputUnit - 85); diff > 1e-9 {
		t.Fatalf("component total cost = %.4f, want 85 including operation", component.TotalCostPerOutputUnit)
	}

	parent := costs[1]
	if !parent.Resolved {
		t.Fatal("parent product BOM cost was not resolved")
	}
	if !parent.HasProductComponent {
		t.Fatal("parent BOM should report its product component")
	}
	if diff := math.Abs(parent.InputCostPerOutputUnit - 1.25); diff > 1e-9 {
		t.Fatalf("parent input cost = %.4f, want 1.25", parent.InputCostPerOutputUnit)
	}
	if diff := math.Abs(parent.TotalCostPerOutputUnit - 1.45); diff > 1e-9 {
		t.Fatalf("parent total cost = %.4f, want 1.45 including parent operation", parent.TotalCostPerOutputUnit)
	}
}

func TestResolveProductionBomCostsAppliesHazelnutBlendMaterialLossOnce(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		658: {
			ProductID:            658,
			VersionID:            1400,
			YieldRate:            1,
			OutputQty:            1,
			OutputUnit:           "kg",
			OperationCostPerUnit: 2.04,
			Items: []productionBomCostItem{
				{ID: 1, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 60, MaterialLossRate: 0.2, UnitCost: 54, UnitCostUnit: "kg"},
				{ID: 2, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 20, MaterialLossRate: 0.2, UnitCost: 78, UnitCostUnit: "kg"},
				{ID: 3, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 20, MaterialLossRate: 0.2, UnitCost: 82, UnitCostUnit: "kg"},
			},
		},
	}

	got := resolveProductionBomCosts(nodes)[658]
	if !got.Resolved {
		t.Fatalf("hazelnut blend BOM was not resolved: %+v", got)
	}
	if diff := math.Abs(got.InputCostPerOutputUnit - 77.28); diff > 1e-9 {
		t.Fatalf("material cost = %.6f, want 77.28 with only 20%% material add-on", got.InputCostPerOutputUnit)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 79.32); diff > 1e-9 {
		t.Fatalf("standard manufacturing cost = %.6f, want 79.32 including 2.04 operation", got.TotalCostPerOutputUnit)
	}
}

func TestResolveProductionBomCostsUsesCookieBomMaterialLossAsTheOnlyLoss(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		643: {
			ProductID:            643,
			VersionID:            1381,
			YieldRate:            0.8, // legacy compatibility field: it must not amplify current BOM cost
			OutputQty:            1,
			OutputUnit:           "kg",
			OperationCostPerUnit: 2.6067,
			Items: []productionBomCostItem{
				{ID: 1, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 75, MaterialLossRate: 0.2, UnitCost: 54, UnitCostUnit: "kg"},
				{ID: 2, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 25, MaterialLossRate: 0.2, UnitCost: 45, UnitCostUnit: "kg"},
			},
		},
	}

	got := resolveProductionBomCosts(nodes)[643]
	if !got.Resolved {
		t.Fatalf("cookie blend BOM was not resolved: %+v", got)
	}
	if diff := math.Abs(got.InputCostPerOutputUnit - 62.10); diff > 1e-9 {
		t.Fatalf("material cost = %.6f, want 62.10 = (54*75%% + 45*25%%) * (1 + 20%%)", got.InputCostPerOutputUnit)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 64.7067); diff > 1e-9 {
		t.Fatalf("standard manufacturing cost = %.6f, want 64.7067 including 2.6067 operation", got.TotalCostPerOutputUnit)
	}
}

func TestResolveProductionBomCostsKeepsLegacyFinishedProductCompatibility(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		2: {
			ProductID:  2,
			VersionID:  202,
			YieldRate:  1,
			OutputUnit: "kg",
			Items: []productionBomCostItem{{
				ID:            2001,
				ComponentType: "material",
				ConsumeUnit:   "ratio_pct",
				RatioPct:      100,
				UnitCost:      90,
			}},
		},
		1: {
			ProductID:  1,
			VersionID:  101,
			YieldRate:  1,
			OutputUnit: "袋",
			Items: []productionBomCostItem{{
				ID:                 1001,
				ComponentType:      "finished_product",
				ComponentProductID: 2,
				ConsumeUnit:        "g_per_bag",
				QtyPerUnit:         10,
			}},
		},
	}

	got := resolveProductionBomCosts(nodes)[1]
	if !got.Resolved || !got.HasProductComponent {
		t.Fatalf("legacy finished_product result = %+v", got)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 0.9); diff > 1e-9 {
		t.Fatalf("legacy finished_product cost = %.4f, want 0.9", got.TotalCostPerOutputUnit)
	}
}

func TestResolveProductionBomTrialItemMatchesPriceListGraphContribution(t *testing.T) {
	componentItem := productionBomCostItem{
		ID:                 1001,
		ComponentType:      "product",
		ComponentProductID: 2,
		ConsumeUnit:        "g_per_bag",
		QtyPerUnit:         10,
	}
	nodes := map[int64]productionBomCostNode{
		2: {
			ProductID:            2,
			VersionID:            202,
			YieldRate:            0.8,
			OutputUnit:           "kg",
			OperationCostPerUnit: 5,
			Items: []productionBomCostItem{{
				ID: 2001, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 80,
			}},
		},
		1: {
			ProductID:  1,
			VersionID:  101,
			YieldRate:  0.98,
			OutputUnit: "袋",
			Items:      []productionBomCostItem{componentItem},
		},
	}
	costs := resolveProductionBomCosts(nodes)
	trialItem, ok, warning := resolveProductionBomTrialItemCost(componentItem, 0, "kg", 0.98, 1, "袋", costs)
	if !ok || warning != "" {
		t.Fatalf("trial item failed: ok=%v warning=%q", ok, warning)
	}
	want := costs[1].ItemCosts[componentItem.ID]
	if diff := math.Abs(trialItem.UnitCost - 85); diff > 1e-9 {
		t.Fatalf("trial component unit cost = %.4f, want 85", trialItem.UnitCost)
	}
	if diff := math.Abs(trialItem.ContributionPerOutputUnit - want.ContributionPerOutputUnit); diff > 1e-9 {
		t.Fatalf("trial contribution = %.6f, price-list graph contribution = %.6f", trialItem.ContributionPerOutputUnit, want.ContributionPerOutputUnit)
	}
}

func TestResolveProductionBomCostsRejectsCyclesAndIgnoresLegacyZeroYield(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		1: {
			ProductID:  1,
			VersionID:  101,
			YieldRate:  1,
			OutputUnit: "unit",
			Items: []productionBomCostItem{{
				ID: 1001, ComponentType: "product", ComponentProductID: 2, ConsumeUnit: "unit", QtyPerUnit: 1,
			}},
		},
		2: {
			ProductID:  2,
			VersionID:  202,
			YieldRate:  1,
			OutputUnit: "unit",
			Items: []productionBomCostItem{{
				ID: 2001, ComponentType: "product", ComponentProductID: 1, ConsumeUnit: "unit", QtyPerUnit: 1,
			}},
		},
		3: {
			ProductID:  3,
			VersionID:  303,
			YieldRate:  0,
			OutputUnit: "kg",
			Items: []productionBomCostItem{{
				ID: 3001, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 100,
			}},
		},
	}

	costs := resolveProductionBomCosts(nodes)
	for _, productID := range []int64{1, 2} {
		got := costs[productID]
		if got.Resolved {
			t.Fatalf("product %d unexpectedly resolved: %+v", productID, got)
		}
		if math.IsNaN(got.TotalCostPerOutputUnit) || math.IsInf(got.TotalCostPerOutputUnit, 0) {
			t.Fatalf("product %d returned invalid total cost: %+v", productID, got)
		}
	}
	if got := costs[3]; !got.Resolved || got.TotalCostPerOutputUnit != 100 {
		t.Fatalf("legacy zero yield must not invalidate current BOM cost: %+v", got)
	}
}

func TestResolveProductionBomCostsRejectsMissingPositiveComponentCost(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		1: {
			ProductID:  1,
			VersionID:  101,
			YieldRate:  1,
			OutputQty:  1,
			OutputUnit: "kg",
			Items: []productionBomCostItem{{
				ID: 1001, ComponentType: "material", ConsumeUnit: "ratio_pct", RatioPct: 100, UnitCost: 0,
			}},
		},
	}

	got := resolveProductionBomCosts(nodes)[1]
	if got.Resolved {
		t.Fatalf("zero-cost material with actual usage must fail closed, got %+v", got)
	}
}

func TestResolveProductionBomCostsNormalizesFixedItemsByOutputBasis(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		1: {
			ProductID:            1,
			VersionID:            101,
			YieldRate:            0.8,
			OutputQty:            10,
			OutputUnit:           "盒",
			OperationCostPerUnit: 0.5,
			Items: []productionBomCostItem{
				{ID: 1001, ComponentType: "material", ConsumeUnit: "unit", QtyPerUnit: 20, UnitCost: 1, UnitCostUnit: "个"},
				{ID: 1002, ComponentType: "material", ConsumeUnit: "unit_per_box", QtyPerUnit: 1, UnitCost: 0.2, UnitCostUnit: "个"},
			},
		},
	}

	got := resolveProductionBomCosts(nodes)[1]
	if !got.Resolved {
		t.Fatalf("fixed-output BOM was not resolved: %+v", got)
	}
	// fixed unit 20 is the amount for the 10-box output basis => 2/box;
	// unit_per_box is already per box and must not be divided by output_qty or yield.
	if diff := math.Abs(got.InputCostPerOutputUnit - 2.2); diff > 1e-9 {
		t.Fatalf("input cost per box = %.4f, want 2.2", got.InputCostPerOutputUnit)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 2.7); diff > 1e-9 {
		t.Fatalf("total cost per box = %.4f, want 2.7", got.TotalCostPerOutputUnit)
	}
}

func TestCostingRepositorySharesResolvedProductionBomCostsBetweenPriceListAndTrial(t *testing.T) {
	repositoryBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	repositorySource := string(repositoryBytes)
	for _, functionName := range []string{
		"func (r Repository) LoadPricingRuleTrialBaseCostDetails",
		"func (r Repository) loadProductInputs",
	} {
		start := strings.Index(repositorySource, functionName)
		if start < 0 {
			t.Fatalf("missing %s", functionName)
		}
		next := strings.Index(repositorySource[start+len(functionName):], "\nfunc ")
		end := len(repositorySource)
		if next >= 0 {
			end = start + len(functionName) + next
		}
		if !strings.Contains(repositorySource[start:end], "loadResolvedProductionBomCosts") {
			t.Fatalf("%s must use the shared production BOM component cost resolver", functionName)
		}
	}

	resolverBytes, err := os.ReadFile("production_bom_cost.go")
	if err != nil {
		t.Fatal(err)
	}
	resolverSource := string(resolverBytes)
	for _, want := range []string{
		"product_production_configs",
		"product_production_bom_bindings",
		"v.status='published'",
		"production_bom_version_operation_costs",
		"material_batch_locations",
		"component_type",
		"finished_product",
	} {
		if !strings.Contains(resolverSource, want) {
			t.Fatalf("shared production BOM cost loader missing %q", want)
		}
	}
}
