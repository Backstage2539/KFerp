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

func TestResolveTypedProductionBomCostsUsesManufacturedMaterialRecursively(t *testing.T) {
	nodes := map[string]productionBomCostNode{
		"material:20": {
			OutputType:           "material",
			OutputID:             20,
			VersionID:            202,
			OutputQty:            1,
			OutputUnit:           "kg",
			OperationCostPerUnit: 5,
			Items: []productionBomCostItem{{
				ID:                  2001,
				ComponentType:       "material",
				ComponentMaterialID: 10,
				ConsumeUnit:         "kg",
				QtyPerUnit:          1.25,
				UnitCost:            78,
				UnitCostUnit:        "kg",
			}},
		},
		"product:30": {
			OutputType:           "product",
			OutputID:             30,
			VersionID:            303,
			OutputQty:            1,
			OutputUnit:           "袋",
			OperationCostPerUnit: 1,
			Items: []productionBomCostItem{
				{
					ID:                  3001,
					ComponentType:       "material",
					ComponentMaterialID: 20,
					ConsumeUnit:         "g_per_bag",
					QtyPerUnit:          227,
					// This purchase price must be ignored because material:20 has
					// a default published manufacturing BOM.
					UnitCost:     999,
					UnitCostUnit: "kg",
				},
				{
					ID:                  3002,
					ComponentType:       "material",
					ComponentMaterialID: 21,
					ConsumeUnit:         "unit_per_bag",
					QtyPerUnit:          1,
					UnitCost:            0.5,
					UnitCostUnit:        "个",
				},
			},
		},
	}

	resolved := resolveTypedProductionBomCosts(nodes)
	roasted := resolved["material:20"]
	if !roasted.Resolved || math.Abs(roasted.TotalCostPerOutputUnit-102.5) > 1e-9 {
		t.Fatalf("manufactured material cost = %+v, want 102.5/kg", roasted)
	}
	finished := resolved["product:30"]
	want := 227.0/1000*102.5 + 0.5 + 1
	if !finished.Resolved || math.Abs(finished.TotalCostPerOutputUnit-want) > 1e-9 {
		t.Fatalf("finished SKU cost = %+v, want %.6f", finished, want)
	}
	if !finished.HasManufacturedMaterialComponent {
		t.Fatal("finished SKU must report recursive manufactured-material costing")
	}
	if item := finished.ItemCosts[3001]; math.Abs(item.UnitCost-102.5) > 1e-9 || item.CostUnit != "kg" {
		t.Fatalf("finished roasted component snapshot = %+v, want recursive 102.5/kg", item)
	}
}

func TestResolveTypedProductionBomCostsConvertsManufacturedRatioCostAcrossMassUnits(t *testing.T) {
	nodes := map[string]productionBomCostNode{
		"material:20": {
			OutputType:           "material",
			OutputID:             20,
			VersionID:            202,
			OutputQty:            1,
			OutputUnit:           "g",
			OperationCostPerUnit: 0.1,
		},
		"material:30": {
			OutputType: "material",
			OutputID:   30,
			VersionID:  303,
			OutputQty:  1,
			OutputUnit: "kg",
			Items: []productionBomCostItem{{
				ID:                  3001,
				ComponentType:       "material",
				ComponentMaterialID: 20,
				ConsumeUnit:         "ratio_pct",
				RatioPct:            100,
			}},
		},
	}

	resolved := resolveTypedProductionBomCosts(nodes)
	child := resolved["material:20"]
	if !child.Resolved || math.Abs(child.TotalCostPerOutputUnit-0.1) > 1e-9 || child.OutputUnit != "g" {
		t.Fatalf("child cost = %+v, want 0.1/g", child)
	}
	parent := resolved["material:30"]
	if !parent.Resolved || math.Abs(parent.TotalCostPerOutputUnit-100) > 1e-9 {
		t.Fatalf("parent ratio cost = %+v, want 100/kg after g-to-kg conversion", parent)
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
	if diff := math.Abs(got.InputCostPerOutputUnit - 80.50); diff > 1e-9 {
		t.Fatalf("material cost = %.6f, want 80.50 with BOM loss treated as gross-input fraction", got.InputCostPerOutputUnit)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 82.54); diff > 1e-9 {
		t.Fatalf("standard manufacturing cost = %.6f, want 82.54 including 2.04 operation", got.TotalCostPerOutputUnit)
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
	if diff := math.Abs(got.InputCostPerOutputUnit - 64.6875); diff > 1e-9 {
		t.Fatalf("material cost = %.6f, want 64.6875 = (54*75%% + 45*25%%) / (1 - 20%%)", got.InputCostPerOutputUnit)
	}
	if diff := math.Abs(got.TotalCostPerOutputUnit - 67.2942); diff > 1e-9 {
		t.Fatalf("standard manufacturing cost = %.6f, want 67.2942 including 2.6067 operation", got.TotalCostPerOutputUnit)
	}
}

func TestResolveProductionBomCostsUsesChuxiaoLossAsGrossInputFraction(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		659: {
			ProductID:  659,
			VersionID:  1595,
			YieldRate:  1,
			OutputQty:  1,
			OutputUnit: "kg",
			Items: []productionBomCostItem{{
				ID: 1, ComponentType: "material", ConsumeUnit: "ratio_pct",
				RatioPct: 25, MaterialLossRate: 0.195, UnitCost: 78, UnitCostUnit: "kg",
			}},
		},
	}

	got := resolveProductionBomCosts(nodes)[659]
	want := 0.25 / (1 - 0.195) * 78
	if !got.Resolved || math.Abs(got.InputCostPerOutputUnit-want) > 1e-9 {
		t.Fatalf("Colombia cost = %.6f, want %.6f = 25%% / 80.5%% * 78", got.InputCostPerOutputUnit, want)
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

func TestResolveProductionBomTrialItemUsesExplicitComponentSpecificationWithoutParentFallback(t *testing.T) {
	item := productionBomCostItem{
		ComponentType:      "product",
		ComponentProductID: 600,
		ComponentBomSpecID: 702,
		ConsumeUnit:        "unit",
		QtyPerUnit:         1,
	}
	costs := map[int64]productionBomResolvedCost{
		600:                              {Resolved: true, TotalCostPerOutputUnit: 999, OutputUnit: "袋"},
		productionBomSpecCostMapKey(702): {Resolved: true, TotalCostPerOutputUnit: 23.5, OutputUnit: "袋"},
	}
	got, ok, warning := resolveProductionBomTrialItemCost(item, 0, "", 1, 1, "袋", costs)
	if !ok || warning != "" || math.Abs(got.UnitCost-23.5) > 1e-9 {
		t.Fatalf("explicit component specification cost = %+v ok=%v warning=%q", got, ok, warning)
	}

	item.ComponentBomSpecID = 703
	if _, ok, _ := resolveProductionBomTrialItemCost(item, 0, "", 1, 1, "袋", costs); ok {
		t.Fatal("missing explicit specification must fail instead of falling back to the parent product cost")
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

func TestResolveProductionBomCostsKeepsPartialCostAndAllMissingComponents(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		1: {
			ProductID: 1, VersionID: 101, BomID: 9001, BomName: "初晓制造 BOM", VersionNo: "V001",
			OutputQty: 1, OutputUnit: "kg",
			Items: []productionBomCostItem{
				{ID: 1001, ComponentType: "material", ComponentName: "已维护原料", ConsumeUnit: "kg", QtyPerUnit: 1, UnitCost: 10, UnitCostUnit: "kg"},
				{ID: 1002, ComponentType: "material", ComponentName: "孟连水洗5T批次", ConsumeUnit: "kg", QtyPerUnit: 1, UnitCost: 0, UnitCostUnit: "kg"},
				{ID: 1003, ComponentType: "material", ComponentName: "另一个缺口", ConsumeUnit: "kg", QtyPerUnit: 1, UnitCost: 0, UnitCostUnit: "kg"},
			},
		},
	}

	got := resolveProductionBomCosts(nodes)[1]
	if got.Resolved || got.CostStatus != "incomplete" {
		t.Fatalf("BOM with missing components must be incomplete: %+v", got)
	}
	if math.Abs(got.PartialTotalCostPerOutputUnit-10) > 1e-9 {
		t.Fatalf("partial cost = %.4f, want 10", got.PartialTotalCostPerOutputUnit)
	}
	if len(got.ItemCosts) != 1 {
		t.Fatalf("valid item contribution was discarded: %+v", got.ItemCosts)
	}
	if len(got.UnresolvedIssues) != 2 {
		t.Fatalf("all missing components must be returned: %+v", got.UnresolvedIssues)
	}
	for _, issue := range got.UnresolvedIssues {
		if issue.BomID != 9001 || issue.VersionNo != "V001" || len(issue.Path) != 1 {
			t.Fatalf("issue lost BOM context/path: %+v", issue)
		}
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

func TestResolveProductionBomCostsConvertsGramConsumptionFromKgInventoryPrice(t *testing.T) {
	nodes := map[int64]productionBomCostNode{
		1: {
			ProductID:  1,
			VersionID:  101,
			YieldRate:  1,
			OutputQty:  1,
			OutputUnit: "个",
			Items: []productionBomCostItem{{
				ID:            1001,
				ComponentType: "material",
				ConsumeUnit:   "g",
				QtyPerUnit:    227,
				UnitCost:      288,
				UnitCostUnit:  "kg",
			}},
		},
	}

	got := resolveProductionBomCosts(nodes)[1]
	if !got.Resolved {
		t.Fatalf("227g component cost was not resolved: %+v", got)
	}
	if diff := math.Abs(got.InputCostPerOutputUnit - 65.376); diff > 1e-9 {
		t.Fatalf("227g at 288 yuan/kg = %.6f, want 65.376", got.InputCostPerOutputUnit)
	}
	if item := got.ItemCosts[1001]; math.Abs(item.ContributionPerOutputUnit-65.376) > 1e-9 || item.CostUnit != "kg" {
		t.Fatalf("resolved item = %+v, want 65.376 yuan with inventory unit kg", item)
	}
}

func TestProductionBomCostReadsPurchasePriceInMaterialInventoryUnit(t *testing.T) {
	b, err := os.ReadFile("production_bom_cost.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "COALESCE(NULLIF(m.unit,''), 'kg') AS unit_cost_unit") {
		t.Fatal("production BOM costing must expose the material inventory unit as purchase-price unit")
	}
	if strings.Contains(src, "COALESCE(NULLIF(m.cost_unit,''), 'kg') AS unit_cost_unit") {
		t.Fatal("production BOM costing must not use an independent material cost unit")
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
		"production_bom_output_bindings",
		"output_type",
		"output_material_id",
		"version.status='published'",
		"production_bom_version_operation_costs",
		"material_batch_locations",
		"material_id",
		"component_type",
		"finished_product",
	} {
		if !strings.Contains(resolverSource, want) {
			t.Fatalf("shared production BOM cost loader missing %q", want)
		}
	}
}
