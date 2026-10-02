package productcreator

import "testing"

func TestV3WorkflowRejectsDuplicateAndUnknownNameVariables(t *testing.T) {
	workflow := Workflow{
		Version:   3,
		Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称"}, {ID: "product-name", Name: "另一个名称"}},
		Nodes: []Node{{ID: "output", Kind: ModuleMaterial, Config: map[string]any{
			"data_role": "output", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "missing"}},
		}}},
	}

	issues := ValidateWorkflowVariableDefinitions(workflow)
	if !hasValidationCode(issues, "duplicate_variable_id") || !hasValidationCode(issues, "unknown_name_variable") {
		t.Fatalf("V3 graph should reject duplicate IDs and unknown variable references: %+v", issues)
	}
}

func TestWorkflowVariableNamesApplyDefaultsAndRespectManualOverrides(t *testing.T) {
	workflow := Workflow{
		Version:   3,
		Variables: []WorkflowVariable{{ID: "product-name", Name: "商品名称", DefaultValue: "云南日晒豆"}},
		Nodes: []Node{
			{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "name_parts": []any{
				map[string]any{"type": "variable", "variable_id": "product-name"}, map[string]any{"type": "text", "value": "_半成品"},
			}}},
			{ID: "manual", Kind: ModuleProduct, Config: map[string]any{"data_role": "output", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product-name"}}}},
		},
	}
	inputs := map[string]map[string]any{"semi": {"name": "旧的自动值", "name_mode": "automatic"}, "manual": {"name": "手动名称", "name_mode": "manual"}}

	resolved := ResolveWorkflowVariableNames(workflow, inputs, map[string]string{})
	if got := resolved["semi"]["name"]; got != "云南日晒豆_半成品" {
		t.Fatalf("automatic variable name = %#v", got)
	}
	if got := resolved["manual"]["name"]; got != "手动名称" {
		t.Fatalf("manual override was replaced: %#v", got)
	}

	issues := ValidateWorkflowVariableValues(workflow, resolved, map[string]string{"product-name": ""})
	if !hasIssue(issues, "required_name_variable") {
		t.Fatalf("cleared required variable should be reported, got %+v", issues)
	}
}

func TestV3NameVariablesOnlyApplyToGeneratedOutputObjects(t *testing.T) {
	workflow := Workflow{Version: 3, Variables: []WorkflowVariable{{ID: "name", Name: "名称"}}, Nodes: []Node{
		{ID: "recipe", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "name"}}}},
	}}
	if issues := ValidateWorkflow(workflow); !hasValidationCode(issues, "invalid_name_target") {
		t.Fatalf("recipe inputs must not define generated object names: %+v", issues)
	}
}

func TestV6MaterialDefaultsAndVariableNamesAreResolvedPerStableRow(t *testing.T) {
	workflow := Workflow{Version: 6, Variables: []WorkflowVariable{{ID: "product", Name: "商品名", DefaultValue: "悲伤樱桃"}}, Nodes: []Node{{ID: "beans", Kind: ModuleMaterial, Config: map[string]any{
		"data_role":    "input",
		"default_rows": []any{map[string]any{"row_id": "green-bean", "action": "create", "supply_mode": "purchase", "unit": "kg", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product"}, map[string]any{"type": "text", "value": "-生豆"}}}},
	}}}}
	inputs := ResolveWorkflowInputDefaults(workflow, nil)
	rows := rowValues(inputs["beans"]["rows"])
	if len(rows) != 1 || stringValue(rows[0]["row_id"]) != "green-bean" {
		t.Fatalf("V6 material defaults should initialize stable rows: %#v", rows)
	}
	inputs = ResolveWorkflowVariableNames(workflow, inputs, nil)
	rows = rowValues(inputs["beans"]["rows"])
	if got := stringValue(rows[0]["name"]); got != "悲伤樱桃-生豆" {
		t.Fatalf("default material name = %q", got)
	}
}

func TestV6MaterialRunRowsCanOverrideNamePartsAndDeletedDefaultsStayDeleted(t *testing.T) {
	workflow := Workflow{Version: 6, Variables: []WorkflowVariable{{ID: "product", Name: "商品名", DefaultValue: "悲伤樱桃"}}, Nodes: []Node{{ID: "beans", Kind: ModuleMaterial, Config: map[string]any{
		"data_role":    "input",
		"default_rows": []any{map[string]any{"row_id": "green-bean", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product"}}}},
	}}}}
	inputs := ResolveWorkflowInputDefaults(workflow, map[string]map[string]any{"beans": {"rows": []any{}}})
	if rows := rowValues(inputs["beans"]["rows"]); len(rows) != 0 {
		t.Fatalf("deleted default rows must not be reinserted: %#v", rows)
	}
	inputs = ResolveWorkflowInputDefaults(workflow, map[string]map[string]any{"beans": {"rows": []any{map[string]any{"row_id": "green-bean", "name_parts": []any{map[string]any{"type": "text", "value": "手动前缀-"}, map[string]any{"type": "variable", "variable_id": "product"}}, "name_mode": "automatic"}}}})
	inputs = ResolveWorkflowVariableNames(workflow, inputs, nil)
	if got := stringValue(rowValues(inputs["beans"]["rows"])[0]["name"]); got != "手动前缀-悲伤樱桃" {
		t.Fatalf("runtime expression should override template expression: %q", got)
	}
	inputs = ResolveWorkflowVariableNames(workflow, map[string]map[string]any{"beans": {"rows": []any{map[string]any{"row_id": "green-bean", "name": "手动名称", "name_mode": "manual"}}}}, nil)
	if got := stringValue(rowValues(inputs["beans"]["rows"])[0]["name"]); got != "手动名称" {
		t.Fatalf("manual material name should be preserved: %q", got)
	}
}

func TestV6EstimatedPurchasePriceAllowsZeroButRejectsNegativeOrInvalid(t *testing.T) {
	workflow := Workflow{Version: 6, Nodes: []Node{{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input", "default_rows": []any{}}}}}
	for _, test := range []struct {
		name  string
		price any
		want  bool
	}{
		{name: "zero", price: 0, want: false},
		{name: "positive", price: 12.5, want: false},
		{name: "negative", price: -1, want: true},
		{name: "invalid", price: "abc", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			inputs := map[string]map[string]any{"raw": {"rows": []any{map[string]any{"row_id": "a", "action": "create", "supply_mode": "purchase", "name": "豆", "unit": "kg", "estimated_unit_price": test.price}}}}
			got := hasIssue(validateBOMRunInputs(workflow, inputs), "invalid_estimated_purchase_price")
			if got != test.want {
				t.Fatalf("invalid issue=%v want %v", got, test.want)
			}
		})
	}
}

func TestV6RuntimeMaterialNamePartsRejectUnknownVariables(t *testing.T) {
	workflow := Workflow{Version: 6, Variables: []WorkflowVariable{{ID: "product", Name: "商品名", DefaultValue: ""}}, Nodes: []Node{{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input", "default_rows": []any{}}}}}
	inputs := map[string]map[string]any{"raw": {"rows": []any{map[string]any{"row_id": "r1", "action": "create", "supply_mode": "purchase", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "not-defined"}}}}}}
	issues := ValidateWorkflowVariableValues(workflow, inputs, map[string]string{"product": "樱桃"})
	if !hasValidationCode(issues, "unknown_name_variable") {
		t.Fatalf("runtime name-part spoofing should be rejected: %+v", issues)
	}
}

func TestV6RuntimeNameCompositionOverridesTemplateWithoutChangingIt(t *testing.T) {
	workflow := Workflow{Version: 6, Variables: []WorkflowVariable{{ID: "product", Name: "商品名", DefaultValue: "悲伤樱桃"}}, Nodes: []Node{{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "name_parts": []any{map[string]any{"type": "variable", "variable_id": "product"}, map[string]any{"type": "text", "value": "-半成品"}}}}}}
	inputs := map[string]map[string]any{"semi": {"name_mode": "automatic", "name_parts": []any{map[string]any{"type": "text", "value": "新组合-"}, map[string]any{"type": "variable", "variable_id": "product"}}}}
	resolved := ResolveWorkflowVariableNames(workflow, inputs, nil)
	if got := stringValue(resolved["semi"]["name"]); got != "新组合-悲伤樱桃" {
		t.Fatalf("runtime name composition = %q", got)
	}
	if got := stringValue(workflow.Nodes[0].Config["name_parts"].([]any)[1].(map[string]any)["value"]); got != "-半成品" {
		t.Fatalf("runtime composition mutated template: %q", got)
	}
}
