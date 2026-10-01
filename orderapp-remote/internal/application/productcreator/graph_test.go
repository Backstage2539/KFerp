package productcreator

import (
	"fmt"
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

func TestV4SpecTemplateFieldIsRequiredOnlyForProductOutputBOM(t *testing.T) {
	fieldFor := func(outputType, key string) (Field, bool) {
		module := moduleForNode(Node{Kind: ModuleBOM, Config: map[string]any{"output_type": outputType}}, 4)
		for _, field := range module.Fields {
			if field.Key == key {
				return field, true
			}
		}
		return Field{}, false
	}

	productTemplate, found := fieldFor("product", "spec_template_version_id")
	if !found || !productTemplate.Required {
		t.Fatalf("product output BOM must expose a required template selector, got found=%t field=%+v", found, productTemplate)
	}
	if materialTemplate, found := fieldFor("material", "spec_template_version_id"); found {
		t.Fatalf("material output BOM must not expose a product specification template selector, got %+v", materialTemplate)
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

func TestBOMTemplateCatalogSeparatesReusableDataAndActions(t *testing.T) {
	counts := map[string]int{}
	for _, module := range ModuleCatalog() {
		if module.PaletteVisible {
			counts[module.Category]++
			if module.Kind == ModulePricing || module.Kind == ModulePublish {
				t.Fatalf("legacy module %q must not be visible for new templates", module.Kind)
			}
		}
	}
	if counts["数据类型"] != 15 || counts["动作"] != 9 {
		t.Fatalf("visible module groups=%v, want three data types and one action for workflow versions 2 through 6", counts)
	}
	for _, module := range ModuleCatalog() {
		if module.WorkflowVersion >= 3 {
			for _, field := range module.Fields {
				if field.Key == "kind" || field.Key == "product_kind" {
					t.Fatalf("V3 module %q still exposes industry category field %q", module.Kind, field.Key)
				}
			}
		}
	}
}

func TestV6CatalogRemovesPurchaseAndKeepsMaterialBOMPortAvailable(t *testing.T) {
	visible := map[ModuleKind]Module{}
	for _, module := range ModuleCatalog() {
		if module.PaletteVisible && module.WorkflowVersion == 6 {
			visible[module.Kind] = module
		}
	}
	if len(visible) != 4 {
		t.Fatalf("V6 palette modules=%v, want material, product, process and BOM only", visible)
	}
	if _, ok := visible[ModulePurchase]; ok {
		t.Fatal("purchase action must not be available in V6 templates")
	}
	input := moduleForNode(Node{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}}, 6)
	if port, ok := findPort(input.Inputs, "from_bom"); !ok || port.Label != "BOM产出" {
		t.Fatalf("V6 input material must expose the BOM output input for validation and connection hints: %+v", input.Inputs)
	}
}

func TestV6OnlyManufacturedSingleRowMaterialCanReceiveBOMOutput(t *testing.T) {
	workflow := Workflow{Version: 6, Nodes: []Node{
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "material", "output_qty": 1, "output_unit": "kg", "route_id": 4}},
		{ID: "material", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "purchase", "default_rows": []any{map[string]any{"row_id": "one"}}}},
	}, Edges: []Edge{dataEdge("output", "bom", "assembly", "material", "from_bom")}}
	issues := ValidateWorkflow(workflow)
	if !hasValidationCode(issues, "bom_output_requires_manufacture") {
		t.Fatalf("purchased material must reject BOM output with a specific issue: %+v", issues)
	}

	workflow.Nodes[1].Config["supply_mode"] = "manufacture"
	workflow.Nodes[1].Config["default_rows"] = []any{map[string]any{"row_id": "one"}, map[string]any{"row_id": "two"}}
	issues = ValidateWorkflow(workflow)
	if !hasValidationCode(issues, "multirow_material_output") {
		t.Fatalf("a multi-row input preset cannot silently become a single BOM output archive: %+v", issues)
	}
}

func TestV6InputMaterialDefaultRowsMayUseWorkflowVariables(t *testing.T) {
	workflow := Workflow{Version: 6, Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名"}}, Nodes: []Node{
		{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input", "default_rows": []any{
			map[string]any{"row_id": "bean", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product-name"}, map[string]any{"type": "text", "value": "-生豆"}}},
		}}},
	}}
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("V6 input material row names should accept template variables: %+v", issues)
	}
}

func TestV5ProductAndMaterialBOMsAcceptOneConnectedProcessRoute(t *testing.T) {
	workflow := Workflow{Version: 5, Nodes: []Node{
		{ID: "source", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "route", Kind: ModuleProcess, Config: map[string]any{"route_id": 4}},
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "spec_template_version_id": 53}},
		{ID: "product", Kind: ModuleProduct, Config: map[string]any{"data_role": "output"}},
	}, Edges: []Edge{
		dataEdge("ingredient", "source", "material", "bom", "components"),
		dataEdge("process", "route", "route", "bom", "route"),
		dataEdge("output", "bom", "assembly", "product", "from_bom"),
	}}
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("V5 product BOM must accept a connected route alongside its specification template: %+v", issues)
	}

	workflow.Nodes[2].Config["output_type"] = "material"
	workflow.Nodes[3].Kind = ModuleMaterial
	delete(workflow.Nodes[2].Config, "spec_template_version_id")
	workflow.Nodes[2].Config["output_qty"] = 1
	workflow.Nodes[2].Config["output_unit"] = "kg"
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("V5 material BOM must accept a connected route: %+v", issues)
	}

	workflow.Nodes = append(workflow.Nodes, Node{ID: "route-2", Kind: ModuleProcess, Config: map[string]any{"route_id": 5}})
	workflow.Edges = append(workflow.Edges, dataEdge("process-2", "route-2", "route", "bom", "route"))
	if issues := ValidateWorkflow(workflow); !hasValidationCode(issues, "multiple_routes") {
		t.Fatalf("a BOM must reject a second process route: %+v", issues)
	}

	workflow.Edges = workflow.Edges[:len(workflow.Edges)-1]
	workflow.Nodes[1].Config["route_id"] = 0
	if issues := ValidateWorkflow(workflow); !hasValidationCode(issues, "route_required") {
		t.Fatalf("a connected process node must select an active route before the template can publish: %+v", issues)
	}
}

func TestResolveBOMProcessRouteV5PriorityAndLegacyBehavior(t *testing.T) {
	workflow := Workflow{Version: 5, Edges: []Edge{dataEdge("route-edge", "process", "route", "bom", "route")}}
	node := Node{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "route_id": 7}}
	if got := ResolveBOMProcessRoute(workflow, node, map[string]any{"route_override_id": float64(9)}, map[string]int64{"process": 8}); got.ID != 9 || got.Source != "run_override" {
		t.Fatalf("run override=%+v, want route 9 from run_override", got)
	}
	if got := ResolveBOMProcessRoute(workflow, node, map[string]any{"route_override_id": float64(0)}, map[string]int64{"process": 8}); got.ID != 8 || got.Source != "connected_node" || got.ProcessNodeID != "process" {
		t.Fatalf("connected route=%+v, want route 8 from process node", got)
	}
	if got := ResolveBOMProcessRoute(Workflow{Version: 5}, node, map[string]any{}, nil); got.ID != 0 || got.Source != "specification_template" {
		t.Fatalf("template route=%+v, want per-spec template defaults", got)
	}
	node.Config["output_type"] = "material"
	if got := ResolveBOMProcessRoute(Workflow{Version: 5}, node, map[string]any{}, nil); got.ID != 7 || got.Source != "bom_default" {
		t.Fatalf("material default route=%+v, want route 7 from BOM default", got)
	}
	if got := ResolveBOMProcessRoute(Workflow{Version: 4}, node, map[string]any{"route_override_id": float64(9)}, nil); got.ID != 7 || got.Source != "bom_default" {
		t.Fatalf("legacy route=%+v, V4 must ignore V5 route overrides", got)
	}
}

func TestBOMCentricWorkflowSupportsInputToAssemblyToGeneratedMaterial(t *testing.T) {
	workflow := Workflow{Version: 2, Nodes: []Node{
		{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "roast", Kind: ModuleProcess, Config: map[string]any{"route_id": 4}},
		{ID: "semi-bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "material", "output_qty": 1, "output_unit": "kg", "route_id": 4, "material_loss_rate": 0}},
		{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "object_action": "create"}},
		{ID: "pack", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "pack-route", Kind: ModuleProcess, Config: map[string]any{"route_id": 5}},
		{ID: "finish", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "output_qty": 1, "variants": []any{variantRow("200g", "200g", "袋", true)}}},
		{ID: "product", Kind: ModuleProduct, Config: map[string]any{"data_role": "output", "object_action": "create"}},
	}, Edges: []Edge{
		dataEdge("e1", "raw", "material", "semi-bom", "components"),
		dataEdge("e2", "roast", "route", "semi-bom", "route"),
		dataEdge("e3", "semi-bom", "assembly", "semi", "from_bom"),
		dataEdge("e4", "semi", "material", "finish", "components"),
		dataEdge("e5", "pack", "material", "finish", "components"),
		dataEdge("e6", "pack-route", "route", "finish", "route"),
		dataEdge("e7", "finish", "assembly", "product", "from_bom"),
	}}
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("BOM-centred graph should validate: %+v", issues)
	}
	order, err := TopologicalOrder(workflow)
	if err != nil || len(order) != len(workflow.Nodes) {
		t.Fatalf("topological order = %v, %v", order, err)
	}
}

func TestV3BOMWorkflowSupportsManyStableRecipeInputsAndOneRoute(t *testing.T) {
	nodes := []Node{
		{ID: "route", Kind: ModuleProcess, Config: map[string]any{"route_id": 7}},
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "material", "output_qty": 1, "output_unit": "kg"}},
		{ID: "output", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "object_action": "create", "unit": "kg"}},
	}
	edges := []Edge{dataEdge("route-edge", "route", "route", "bom", "route"), dataEdge("output-edge", "bom", "assembly", "output", "from_bom")}
	for index := 1; index <= 4; index++ {
		sourceID := fmt.Sprintf("input-%d", index)
		nodes = append(nodes, Node{ID: sourceID, Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}})
		edges = append(edges, dataEdge(fmt.Sprintf("recipe-%d", index), sourceID, "material", "bom", "components"))
	}
	workflow := Workflow{Version: 3, Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称"}}, Nodes: nodes, Edges: edges}
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("V3 BOM with four distinct recipe inputs and one route should validate: %+v", issues)
	}
	workflow.Edges = append(workflow.Edges, dataEdge("duplicate-recipe", "input-1", "material", "bom", "components"))
	if issues := ValidateWorkflow(workflow); !hasIssue(issues, "duplicate_recipe_source") {
		t.Fatalf("duplicate source connections should be rejected: %+v", issues)
	}
	workflow.Edges[len(workflow.Edges)-1].ID = ""
	if issues := ValidateWorkflow(workflow); !hasIssue(issues, "missing_edge_id") {
		t.Fatalf("V3 edge identity must be stable: %+v", issues)
	}
}

func TestBOMCentricWorkflowRejectsHiddenConditionsAndWrongOutputObject(t *testing.T) {
	workflow := Workflow{Version: 2, Nodes: []Node{
		{ID: "source", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product"}},
		{ID: "material", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output"}},
	}, Edges: []Edge{
		dataEdge("e1", "source", "material", "bom", "components"),
		dataEdge("e2", "bom", "assembly", "material", "from_bom"),
	}}
	if issues := ValidateWorkflow(workflow); !hasIssue(issues, "output_type_mismatch") {
		t.Fatalf("product BOM must not target material data node, got %+v", issues)
	}
	workflow.Nodes[0].Condition = &Condition{NodeID: "source", Field: "action", Operator: "equals", Value: "create"}
	if issues := ValidateWorkflow(workflow); !hasIssue(issues, "conditions_disabled") {
		t.Fatalf("new workflow must reject hidden conditions, got %+v", issues)
	}
}

func TestV4ProductBOMUsesSpecificationTemplateAndConnectedMainInputCandidates(t *testing.T) {
	workflow := Workflow{Version: 4, Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称"}}, Nodes: []Node{
		{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "pack", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "finished-bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "spec_template_version_id": 53}},
		{ID: "product", Kind: ModuleProduct, Config: map[string]any{"data_role": "output", "object_action": "create", "owner": "factory"}},
	}, Edges: []Edge{
		dataEdge("semi-input", "semi", "material", "finished-bom", "components"),
		dataEdge("pack-input", "pack", "material", "finished-bom", "components"),
		dataEdge("finished-output", "finished-bom", "assembly", "product", "from_bom"),
	}}
	if issues := ValidateWorkflow(workflow); len(issues) != 0 {
		t.Fatalf("V4 product BOM with a template and multiple connected main-input candidates should validate: %+v", issues)
	}

	workflow.Nodes[2].Config["spec_template_version_id"] = 0
	if issues := ValidateWorkflow(workflow); !hasValidationCode(issues, "spec_template_required") {
		t.Fatalf("product-output BOM must require a specification template: %+v", issues)
	}

	workflow.Nodes[2].Config["spec_template_version_id"] = 53
	workflow.Nodes = append(workflow.Nodes, Node{ID: "route", Kind: ModuleProcess, Config: map[string]any{"route_id": 9}})
	workflow.Edges = append(workflow.Edges, dataEdge("separate-route", "route", "route", "finished-bom", "route"))
	if issues := ValidateWorkflow(workflow); !hasValidationCode(issues, "template_route_conflict") {
		t.Fatalf("V4 product BOM must inherit process route from the specification template: %+v", issues)
	}
}

func TestV4SpecificationTemplateFieldLivesOnBOMOnly(t *testing.T) {
	var bomModule, productModule *Module
	for _, module := range ModuleCatalog() {
		if module.WorkflowVersion != 4 {
			continue
		}
		switch module.Kind {
		case ModuleBOM:
			copy := module
			bomModule = &copy
		case ModuleProduct:
			copy := module
			productModule = &copy
		}
	}
	if bomModule == nil || productModule == nil {
		t.Fatal("expected V4 product and BOM module definitions")
	}
	containsField := func(module *Module, key string) bool {
		for _, field := range module.Fields {
			if field.Key == key {
				return true
			}
		}
		return false
	}
	if !containsField(bomModule, "spec_template_version_id") {
		t.Fatal("the V4 BOM module must expose the specification-template reference")
	}
	if containsField(productModule, "spec_template_version_id") || containsField(productModule, "variants") {
		t.Fatal("product master nodes must not store a specification template or manual variants")
	}
}

func TestV4ProductBOMBlocksUnreviewedLegacyManualConfiguration(t *testing.T) {
	workflow := Workflow{Version: 4, Nodes: []Node{
		{ID: "material", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "spec_template_version_id": 53, "legacy_spec_configuration_pending": true}},
		{ID: "product", Kind: ModuleProduct, Config: map[string]any{"data_role": "output"}},
	}, Edges: []Edge{
		dataEdge("component", "material", "material", "bom", "components"),
		dataEdge("output", "bom", "assembly", "product", "from_bom"),
	}}
	issues := ValidateWorkflow(workflow)
	if !hasValidationCode(issues, "legacy_product_bom_conflict") {
		t.Fatalf("expected unresolved V3 product BOM settings to block V4 publication, got %+v", issues)
	}
	workflow.Nodes[1].Config["legacy_spec_configuration_pending"] = false
	if issues := ValidateWorkflow(workflow); hasValidationCode(issues, "legacy_product_bom_conflict") {
		t.Fatalf("explicitly reviewed legacy settings should no longer block publication: %+v", issues)
	}
}

func TestV4ProductBOMRunRequiresOneConnectedMainInputReference(t *testing.T) {
	workflow := Workflow{Version: 4, Nodes: []Node{
		{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "pack", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "finished-bom", Kind: ModuleBOM, Config: map[string]any{"output_type": "product", "spec_template_version_id": 53}},
		{ID: "product", Kind: ModuleProduct, Config: map[string]any{"data_role": "output"}},
	}, Edges: []Edge{
		dataEdge("semi-input", "semi", "material", "finished-bom", "components"),
		dataEdge("pack-input", "pack", "material", "finished-bom", "components"),
		dataEdge("finished-output", "finished-bom", "assembly", "product", "from_bom"),
	}}
	inputs := map[string]map[string]any{
		"semi":         {"rows": []any{materialRow("semi-row", "半成品", "manufacture", "kg")}},
		"pack":         {"rows": []any{materialRow("pack-row", "包装", "purchase", "袋")}},
		"product":      {"action": "create", "name": "新商品", "owner": "factory"},
		"finished-bom": {"main_input_source_node_id": "semi", "main_input_source_row_id": "semi-row"},
	}
	if issues := ValidateRunInputs(workflow, inputs); len(issues) != 0 {
		t.Fatalf("connected, selected product main-input candidate should validate: %+v", issues)
	}

	delete(inputs["finished-bom"], "main_input_source_row_id")
	if issues := ValidateRunInputs(workflow, inputs); !hasIssue(issues, "main_input_required") {
		t.Fatalf("product BOM must require a selected main-input candidate: %+v", issues)
	}

	inputs["finished-bom"]["main_input_source_node_id"] = "unconnected"
	inputs["finished-bom"]["main_input_source_row_id"] = "row"
	if issues := ValidateRunInputs(workflow, inputs); !hasIssue(issues, "main_input_not_connected") {
		t.Fatalf("product BOM must reject a main input outside its connected sources: %+v", issues)
	}
}

func TestBOMCentricDefaultsFillBlankInputsAndProtectFixedFields(t *testing.T) {
	workflow := Workflow{Version: 2, Nodes: []Node{
		{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input"}},
		{ID: "route", Kind: ModuleProcess, Config: map[string]any{"route_id": 4}},
		{ID: "bom", Kind: ModuleBOM, Config: map[string]any{
			"output_type": "material", "output_qty": 0.5, "output_unit": "kg", "route_id": 4,
			"material_loss_rate": 0.04, "fixed_fields": []any{"output_qty"},
			"components": []any{map[string]any{"row_id": "raw-use", "source_node_id": "raw", "source_row_id": "raw", "quantity": 0.25, "unit": "kg"}},
		}},
		{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "object_action": "create", "unit": "kg", "name_pattern": "半成品"}},
	}, Edges: []Edge{
		dataEdge("ingredient", "raw", "material", "bom", "components"),
		dataEdge("route-edge", "route", "route", "bom", "route"),
		dataEdge("bom-output", "bom", "assembly", "semi", "from_bom"),
	}}
	inputs := map[string]map[string]any{
		"raw":   {"rows": []any{map[string]any{"row_id": "raw", "action": "create", "name": "生料", "unit": "kg"}}},
		"route": {"route_id": 4},
		"bom":   {"output_qty": 3.0, "components": []any{}},
		"semi":  {"action": "create", "name": "半成品A", "unit": "kg"},
	}
	resolved := ResolveWorkflowInputDefaults(workflow, inputs)
	bomValues := resolved["bom"]
	if numericValue(bomValues["output_qty"]) != 0.5 || stringValue(bomValues["output_unit"]) != "kg" || numericValue(bomValues["material_loss_rate"]) != 0.04 {
		t.Fatalf("BOM defaults were not applied or fixed values were overridden: %#v", bomValues)
	}
	components := rowValues(bomValues["components"])
	if len(components) != 1 || numericValue(components[0]["quantity"]) != 0.25 {
		t.Fatalf("blank recipe defaults were not applied: %#v", bomValues["components"])
	}
	issues := ValidateRunInputs(workflow, resolved)
	if len(issues) != 0 {
		t.Fatalf("fractional quantities and configured defaults should validate: %+v", issues)
	}
}

func TestFixedBOMComponentDefaultsPreserveRuntimeSourceIdentity(t *testing.T) {
	workflow := Workflow{Version: 2, Nodes: []Node{{ID: "bom", Kind: ModuleBOM, Config: map[string]any{
		"fixed_fields": []any{"components"},
		"components":   []any{map[string]any{"row_id": "template-component", "source_node_id": "materials", "quantity": 2.5, "unit": "kg"}},
	}}}}
	inputs := map[string]map[string]any{"bom": {"components": []any{map[string]any{
		"row_id": "run-component", "source_node_id": "materials", "source_row_id": "material-row-7", "quantity": 99.0, "unit": "g",
	}}}}
	resolved := ResolveWorkflowInputDefaults(workflow, inputs)
	rows := rowValues(resolved["bom"]["components"])
	if len(rows) != 1 {
		t.Fatalf("component rows = %#v", rows)
	}
	if rows[0]["row_id"] != "run-component" || rows[0]["source_row_id"] != "material-row-7" {
		t.Fatalf("fixed defaults must preserve the runtime-selected source identity: %#v", rows[0])
	}
	if numericValue(rows[0]["quantity"]) != 2.5 || rows[0]["unit"] != "kg" {
		t.Fatalf("fixed recipe defaults = %#v, want 2.5 kg", rows[0])
	}
}
