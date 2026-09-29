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
