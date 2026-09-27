package productcreator

import (
	"strings"
	"testing"
)

func TestValidateWorkflowAcceptsTypedMultiLevelBranchAndJoin(t *testing.T) {
	workflow := Workflow{Nodes: []Node{
		{ID: "green", Kind: ModuleMaterial},
		{ID: "roasted", Kind: ModuleMaterial},
		{ID: "roast-bom", Kind: ModuleBOM},
		{ID: "roast-publish", Kind: ModulePublish},
		{ID: "product", Kind: ModuleProduct},
		{ID: "packaging", Kind: ModuleMaterial},
		{ID: "product-bom", Kind: ModuleBOM},
		{ID: "publish", Kind: ModulePublish},
	}, Edges: []Edge{
		{ID: "e1", Source: "green", SourceHandle: "material", Target: "roast-bom", TargetHandle: "components", Kind: EdgeData},
		{ID: "e2", Source: "roasted", SourceHandle: "material", Target: "roast-bom", TargetHandle: "output", Kind: EdgeData},
		{ID: "e3", Source: "product", SourceHandle: "product", Target: "product-bom", TargetHandle: "output", Kind: EdgeData},
		{ID: "e4", Source: "roast-bom", SourceHandle: "bom", Target: "roast-publish", TargetHandle: "bom", Kind: EdgeData},
		{ID: "e4b", Source: "roast-publish", SourceHandle: "specs", Target: "product-bom", TargetHandle: "components", Kind: EdgeData},
		{ID: "e5", Source: "packaging", SourceHandle: "material", Target: "product-bom", TargetHandle: "components", Kind: EdgeData},
		{ID: "e6", Source: "product-bom", SourceHandle: "bom", Target: "publish", TargetHandle: "bom", Kind: EdgeData},
	}}

	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("ValidateWorkflow() returned issues: %+v", issues)
	}
	order, err := TopologicalOrder(workflow)
	if err != nil {
		t.Fatal(err)
	}
	positions := make(map[string]int, len(order))
	for i, id := range order {
		positions[id] = i
	}
	if !(positions["green"] < positions["roast-bom"] && positions["roasted"] < positions["roast-bom"] && positions["roast-bom"] < positions["product-bom"] && positions["packaging"] < positions["product-bom"] && positions["product-bom"] < positions["publish"]) {
		t.Fatalf("unexpected dependency order: %v", order)
	}
}

func TestValidateWorkflowRejectsInvalidHandleAndType(t *testing.T) {
	issues := ValidateWorkflow(Workflow{Nodes: []Node{
		{ID: "product", Kind: ModuleProduct},
		{ID: "bom", Kind: ModuleBOM},
	}, Edges: []Edge{{ID: "e1", Source: "product", SourceHandle: "product", Target: "bom", TargetHandle: "components", Kind: EdgeData}}})

	if !hasIssue(issues, "incompatible_data_type") {
		t.Fatalf("expected incompatible_data_type issue, got %+v", issues)
	}
}

func TestValidateWorkflowRejectsCyclesAndDanglingReferences(t *testing.T) {
	issues := ValidateWorkflow(Workflow{Nodes: []Node{
		{ID: "a", Kind: ModuleMaterial},
		{ID: "b", Kind: ModuleMaterial},
	}, Edges: []Edge{
		{ID: "ab", Source: "a", SourceHandle: "material", Target: "b", TargetHandle: "material", Kind: EdgeData},
		{ID: "ba", Source: "b", SourceHandle: "material", Target: "a", TargetHandle: "material", Kind: EdgeData},
		{ID: "missing", Source: "ghost", SourceHandle: "material", Target: "a", TargetHandle: "material", Kind: EdgeData},
	}})

	if !hasIssue(issues, "cycle") || !hasIssue(issues, "missing_node") {
		t.Fatalf("expected cycle and missing_node issues, got %+v", issues)
	}
}

func TestValidateWorkflowRejectsUnstableAndDuplicateIDs(t *testing.T) {
	issues := ValidateWorkflow(Workflow{Nodes: []Node{
		{ID: "same", Kind: ModuleMaterial},
		{ID: "same", Kind: ModuleMaterial},
		{ID: " ", Kind: ModuleMaterial},
	}})
	if !hasIssue(issues, "duplicate_node_id") || !hasIssue(issues, "missing_node_id") {
		t.Fatalf("expected stable unique node ID issues, got %+v", issues)
	}
}

func hasIssue(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestTopologicalOrderRejectsInvalidGraph(t *testing.T) {
	_, err := TopologicalOrder(Workflow{Nodes: []Node{{ID: "x", Kind: ModuleProduct}}, Edges: []Edge{{ID: "e", Source: "x", SourceHandle: "bad", Target: "x", TargetHandle: "bad", Kind: EdgeData}}})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("TopologicalOrder() error = %v, want cycle error", err)
	}
}

func TestRunDraftCanBeSavedIncompleteButPreviewRequiresRequiredFields(t *testing.T) {
	workflow := Workflow{Nodes: []Node{{ID: "product", Kind: ModuleProduct}}}
	if issues := ValidateRunDraft(workflow, map[string]map[string]any{"product": {"name": "待补充"}}); len(issues) != 0 {
		t.Fatalf("incomplete draft should be resumable, got %+v", issues)
	}
	preview := BuildRunPreview(workflow, map[string]map[string]any{"product": {"name": "待补充"}})
	if preview.Valid || !hasIssue(preview.Issues, "required_field") {
		t.Fatalf("preview should report missing required inputs, got %+v", preview)
	}
}

func TestRunDraftRejectsRepeatedRowIdentity(t *testing.T) {
	workflow := Workflow{Nodes: []Node{{ID: "materials", Kind: ModuleMaterial}}}
	inputs := map[string]map[string]any{"materials": {"rows": []any{
		map[string]any{"row_id": "row-1"},
		map[string]any{"row_id": "row-1"},
	}}}
	if issues := ValidateRunDraft(workflow, inputs); !hasIssue(issues, "duplicate_row_id") {
		t.Fatalf("expected duplicate row identity issue, got %+v", issues)
	}
}

func TestValidateWorkflowRequiresTypedSourcesForRequiredPorts(t *testing.T) {
	issues := ValidateWorkflow(Workflow{Nodes: []Node{
		{ID: "material", Kind: ModuleMaterial},
		{ID: "bom", Kind: ModuleBOM},
	}})
	if !hasIssue(issues, "missing_data_source") {
		t.Fatalf("unconnected required BOM ports must be reported, got %+v", issues)
	}
}

func TestPreviewValidatesMaterialAndBOMRowsAgainstTypedConnections(t *testing.T) {
	workflow := Workflow{Nodes: []Node{{ID: "raw", Kind: ModuleMaterial}, {ID: "bom", Kind: ModuleBOM}}, Edges: []Edge{
		{ID: "e1", Source: "raw", SourceHandle: "material", Target: "bom", TargetHandle: "components", Kind: EdgeData},
		{ID: "e2", Source: "raw", SourceHandle: "material", Target: "bom", TargetHandle: "output", Kind: EdgeData},
	}}
	inputs := map[string]map[string]any{
		"raw": {"rows": []any{map[string]any{"row_id": "mat-1", "name": "生料", "unit": "kg", "action": "create"}}},
		"bom": {"action": "create", "variants": []any{map[string]any{"row_id": "spec-1", "name": "默认规格", "unit": "件", "is_default": true}}, "components": []any{map[string]any{"row_id": "comp-1", "source_node_id": "missing", "quantity": 0, "unit": "kg"}}},
	}
	preview := BuildRunPreview(workflow, inputs)
	if preview.Valid || !hasIssue(preview.Issues, "missing_component_source") || !hasIssue(preview.Issues, "invalid_component_quantity") {
		t.Fatalf("preview must reject non-connected or zero-quantity BOM rows, got %+v", preview)
	}
}

func TestPreviewMarksConditionallySkippedStepAndDoesNotRequireItsFields(t *testing.T) {
	workflow := Workflow{Nodes: []Node{
		{ID: "choice", Kind: ModuleProduct},
		{ID: "optional", Kind: ModuleMaterial, Condition: &Condition{NodeID: "choice", Field: "action", Operator: "equals", Value: "create"}},
	}}
	inputs := map[string]map[string]any{"choice": {"name": "已有商品", "action": "reuse", "owner": "factory", "product_id": 7}}
	preview := BuildRunPreview(workflow, inputs)
	if !preview.Valid || len(preview.Steps) != 2 || preview.Steps[1].Status != "skipped" {
		t.Fatalf("conditionally skipped step should not require its own fields: %+v", preview)
	}
}

func TestPreviewPreventsDefaultBindingChangesForReusedSharedBOM(t *testing.T) {
	workflow := Workflow{Nodes: []Node{
		{ID: "product", Kind: ModuleProduct}, {ID: "materials", Kind: ModuleMaterial},
		{ID: "bom", Kind: ModuleBOM}, {ID: "publish", Kind: ModulePublish},
	}, Edges: []Edge{
		dataEdge("product-output", "product", "product", "bom", "output"),
		dataEdge("material-component", "materials", "material", "bom", "components"),
		dataEdge("publish-bom", "bom", "bom", "publish", "bom"),
	}}
	inputs := map[string]map[string]any{
		"product":   {"name": "装配商品", "action": "create", "owner": "factory"},
		"materials": {"rows": []any{materialRow("part", "零件", "purchase", "个")}},
		"bom":       {"action": "reuse", "bom_id": 91, "output_source_row_id": "product"},
		"publish":   {"set_default": true},
	}
	preview := BuildRunPreview(workflow, inputs)
	if preview.Valid || !hasIssue(preview.Issues, "shared_bom_default_change") {
		t.Fatalf("shared BOM default must be read-only when reusing it, got %+v", preview.Issues)
	}
	inputs["publish"]["set_default"] = false
	preview = BuildRunPreview(workflow, inputs)
	if !preview.Valid {
		t.Fatalf("reusing an existing BOM without changing its default should be valid: %+v", preview.Issues)
	}
}

func TestAcceptanceTemplatesPreviewForNewProductReuseAndNonCoffeeAssembly(t *testing.T) {
	for _, fixture := range []struct {
		name string
		make func() (Workflow, map[string]map[string]any)
	}{
		{name: "new product with purchased raw material and two specs", make: newProductAcceptanceFixture},
		{name: "reuse published semi-finished material", make: reuseSemiFinishedAcceptanceFixture},
		{name: "three-level non-coffee assembly", make: nonCoffeeAssemblyAcceptanceFixture},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			workflow, inputs := fixture.make()
			preview := BuildRunPreview(workflow, inputs)
			if !preview.Valid {
				t.Fatalf("acceptance template preview failed: %+v", preview.Issues)
			}
			if len(preview.Steps) != len(workflow.Nodes) {
				t.Fatalf("preview step count=%d, want %d", len(preview.Steps), len(workflow.Nodes))
			}
		})
	}
}

func newProductAcceptanceFixture() (Workflow, map[string]map[string]any) {
	w := Workflow{Nodes: []Node{
		{ID: "product", Kind: ModuleProduct}, {ID: "materials", Kind: ModuleMaterial}, {ID: "semi-bom", Kind: ModuleBOM},
		{ID: "semi-publish", Kind: ModulePublish}, {ID: "finished-bom", Kind: ModuleBOM}, {ID: "publish", Kind: ModulePublish},
		{ID: "purchase", Kind: ModulePurchase}, {ID: "pricing", Kind: ModulePricing},
	}, Edges: []Edge{
		dataEdge("e1", "materials", "material", "semi-bom", "components"), dataEdge("e2", "materials", "material", "semi-bom", "output"),
		dataEdge("e3", "semi-bom", "bom", "semi-publish", "bom"), dataEdge("e4", "semi-publish", "specs", "finished-bom", "components"),
		dataEdge("e5", "materials", "material", "finished-bom", "components"), dataEdge("e6", "product", "product", "finished-bom", "output"),
		dataEdge("e7", "finished-bom", "bom", "publish", "bom"), dataEdge("e8", "materials", "material", "purchase", "material"),
		dataEdge("e9", "publish", "specs", "pricing", "product"), dataEdge("e10", "purchase", "receipt", "pricing", "receipt"),
	}}
	return w, map[string]map[string]any{
		"product":      {"name": "拼配系列商品", "action": "create", "owner": "factory"},
		"materials":    {"rows": []any{materialRow("green", "采购原料", "purchase", "kg"), materialRow("semi", "自制半成品", "manufacture", "kg"), materialRow("pack", "纸盒", "purchase", "个")}},
		"semi-bom":     {"action": "create", "output_source_row_id": "semi", "variants": []any{variantRow("semi-spec", "半成品", "kg", true)}, "components": []any{componentRow("raw-use", "materials", "green", "", 0.82, "kg")}},
		"semi-publish": {"set_default": true},
		"finished-bom": {"action": "create", "output_source_row_id": "product", "variants": []any{variantRow("250", "250g", "袋", true), variantRow("1000", "1kg", "袋", false)}, "components": []any{componentRow("semi-use", "semi-publish", "semi-spec", "", 0.25, "kg"), componentRow("pack-250", "materials", "pack", "250", 1, "个"), componentRow("pack-1000", "materials", "pack", "1000", 1, "个")}},
		"publish":      {"set_default": true},
		"purchase":     {"material_source_row_id": "green", "supplier_id": 9, "quantity": 20, "warehouse": "raw", "unit_price": 28.5},
		"pricing":      {"price_list_id": 42, "prices": []any{priceRow("p250", "250", 88), priceRow("p1000", "1000", 288)}},
	}
}

func reuseSemiFinishedAcceptanceFixture() (Workflow, map[string]map[string]any) {
	w := Workflow{Nodes: []Node{{ID: "product", Kind: ModuleProduct}, {ID: "materials", Kind: ModuleMaterial}, {ID: "bom", Kind: ModuleBOM}, {ID: "publish", Kind: ModulePublish}}, Edges: []Edge{
		dataEdge("e1", "product", "product", "bom", "output"), dataEdge("e2", "materials", "material", "bom", "components"), dataEdge("e3", "bom", "bom", "publish", "bom"),
	}}
	return w, map[string]map[string]any{
		"product":   {"name": "装饰礼盒", "action": "create", "owner": "factory"},
		"materials": {"rows": []any{map[string]any{"row_id": "semi", "name": "已发布半成品", "action": "reuse", "material_id": 340, "owner_type": "factory", "owner_customer_id": 0, "unit": "件"}, map[string]any{"row_id": "box", "name": "现有礼盒", "action": "reuse", "material_id": 341, "owner_type": "factory", "owner_customer_id": 0, "unit": "个"}}},
		"bom":       {"action": "create", "output_source_row_id": "product", "variants": []any{variantRow("gift", "单盒", "盒", true)}, "components": []any{componentRow("semi-use", "materials", "semi", "", 1, "件"), componentRow("box-use", "materials", "box", "", 1, "个")}},
		"publish":   {"set_default": true},
	}
}

func nonCoffeeAssemblyAcceptanceFixture() (Workflow, map[string]map[string]any) {
	w := Workflow{Nodes: []Node{
		{ID: "product", Kind: ModuleProduct}, {ID: "parts", Kind: ModuleMaterial}, {ID: "housing-bom", Kind: ModuleBOM}, {ID: "housing-publish", Kind: ModulePublish},
		{ID: "assembly-bom", Kind: ModuleBOM}, {ID: "assembly-publish", Kind: ModulePublish}, {ID: "finished-bom", Kind: ModuleBOM}, {ID: "publish", Kind: ModulePublish},
	}, Edges: []Edge{
		dataEdge("e1", "parts", "material", "housing-bom", "components"), dataEdge("e2", "parts", "material", "housing-bom", "output"), dataEdge("e3", "housing-bom", "bom", "housing-publish", "bom"),
		dataEdge("e4", "housing-publish", "specs", "assembly-bom", "components"), dataEdge("e5", "parts", "material", "assembly-bom", "components"), dataEdge("e6", "parts", "material", "assembly-bom", "output"), dataEdge("e7", "assembly-bom", "bom", "assembly-publish", "bom"),
		dataEdge("e8", "assembly-publish", "specs", "finished-bom", "components"), dataEdge("e9", "parts", "material", "finished-bom", "components"), dataEdge("e10", "product", "product", "finished-bom", "output"), dataEdge("e11", "finished-bom", "bom", "publish", "bom"),
	}}
	return w, map[string]map[string]any{
		"product":          {"name": "桌面控制器", "action": "create", "product_kind": "generic", "owner": "factory"},
		"parts":            {"rows": []any{materialRow("plastic", "外壳粒子", "purchase", "kg"), materialRow("pcb", "控制板", "purchase", "个"), materialRow("housing", "外壳组件", "manufacture", "件"), materialRow("assembly", "驱动组件", "manufacture", "件"), materialRow("motor", "微型电机", "purchase", "个"), materialRow("label", "铭牌", "purchase", "张")}},
		"housing-bom":      {"action": "create", "output_source_row_id": "housing", "variants": []any{variantRow("housing-spec", "外壳", "件", true)}, "components": []any{componentRow("plastic-use", "parts", "plastic", "", 0.12, "kg"), componentRow("pcb-use", "parts", "pcb", "", 1, "个")}},
		"housing-publish":  {"set_default": true},
		"assembly-bom":     {"action": "create", "output_source_row_id": "assembly", "variants": []any{variantRow("assembly-spec", "驱动组件", "件", true)}, "components": []any{componentRow("housing-use", "housing-publish", "housing-spec", "", 1, "件"), componentRow("motor-use", "parts", "motor", "", 1, "个")}},
		"assembly-publish": {"set_default": true},
		"finished-bom":     {"action": "create", "output_source_row_id": "product", "variants": []any{variantRow("controller-spec", "控制器", "台", true)}, "components": []any{componentRow("assembly-use", "assembly-publish", "assembly-spec", "", 1, "件"), componentRow("label-use", "parts", "label", "", 1, "张")}},
		"publish":          {"set_default": true},
	}
}

func dataEdge(id, source, sourceHandle, target, targetHandle string) Edge {
	return Edge{ID: id, Source: source, SourceHandle: sourceHandle, Target: target, TargetHandle: targetHandle, Kind: EdgeData}
}

func materialRow(id, name, supplyMode, unit string) map[string]any {
	return map[string]any{"row_id": id, "name": name, "action": "create", "kind": "other", "supply_mode": supplyMode, "unit": unit, "owner_type": "factory", "owner_customer_id": 0}
}

func variantRow(id, name, unit string, isDefault bool) map[string]any {
	return map[string]any{"row_id": id, "name": name, "unit": unit, "is_default": isDefault}
}

func componentRow(id, source, sourceRow, variant string, quantity float64, unit string) map[string]any {
	return map[string]any{"row_id": id, "source_node_id": source, "source_row_id": sourceRow, "variant_row_id": variant, "quantity": quantity, "unit": unit, "loss_rate": 0}
}

func priceRow(id, specID string, price float64) map[string]any {
	return map[string]any{"row_id": id, "spec_row_id": specID, "pricing_mode": "fixed", "price": price}
}
