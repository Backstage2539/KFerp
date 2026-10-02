package productcreator

import "testing"

func reuseFixture() (Workflow, map[string]map[string]any) {
	w := Workflow{Version: 8, Nodes: []Node{
		{ID: "raw", Kind: ModuleMaterial, Config: map[string]any{"data_role": "input", "supply_mode": "purchase"}},
		{ID: "route", Kind: ModuleProcess, Config: map[string]any{"route_id": 1}},
		{ID: "roast", Kind: ModuleBOM, Config: map[string]any{"output_type": "material"}},
		{ID: "semi", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}},
		{ID: "grind", Kind: ModuleBOM, Config: map[string]any{"output_type": "material", "route_id": 2}},
		{ID: "flour", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}},
	}, Edges: []Edge{dataEdge("a", "raw", "material", "roast", "components"), dataEdge("b", "route", "route", "roast", "route"), dataEdge("c", "roast", "assembly", "semi", "from_bom"), dataEdge("d", "semi", "material", "grind", "components"), dataEdge("e", "grind", "assembly", "flour", "from_bom")}}
	in := map[string]map[string]any{"semi": {"action": "reuse", "material_id": 42}, "grind": {"output_qty": 1, "output_unit": "kg", "components": []any{map[string]any{"row_id": "r", "source_node_id": "semi", "source_row_id": "output", "quantity": 1, "unit": "kg"}}}, "flour": {"action": "create", "name": "粉", "unit": "kg"}}
	return w, in
}
func TestV8ReuseStopsUpstreamValidationAndPreview(t *testing.T) {
	w, in := reuseFixture()
	p := BuildRunPreview(w, in)
	if !p.Valid {
		t.Fatalf("upstream empty fields must not block reuse: %+v", p.Issues)
	}
	for _, s := range p.Steps {
		if s.NodeID == "raw" || s.NodeID == "roast" || s.NodeID == "route" {
			if s.Status != "skipped" {
				t.Fatalf("upstream step executed: %+v", s)
			}
		}
	}
	w.Version = 7
	if BuildRunPreview(w, in).Valid {
		t.Fatal("V7 semantics changed")
	}
}
func TestV8ReuseKeepsSharedDependenciesAndRestoresCreate(t *testing.T) {
	w, in := reuseFixture()
	w.Edges = append(w.Edges, dataEdge("shared", "route", "route", "grind", "route"))
	p := BuildRunPreview(w, in)
	found := false
	for _, i := range p.Issues {
		if i.NodeID == "route" {
			found = true
		}
		if i.NodeID == "raw" || i.NodeID == "roast" {
			t.Fatalf("inactive issue: %+v", i)
		}
	}
	if !found {
		t.Fatal("shared route must still be required")
	}
	in["semi"]["action"] = "create"
	found = false
	for _, i := range BuildRunPreview(w, in).Issues {
		if i.NodeID == "raw" {
			found = true
		}
	}
	if !found {
		t.Fatal("new mode did not restore upstream")
	}
}
func TestV8ReuseMissingSelectionAndCycleStillFail(t *testing.T) {
	w, in := reuseFixture()
	delete(in["semi"], "material_id")
	for _, i := range BuildRunPreview(w, in).Issues {
		if i.NodeID != "semi" {
			t.Fatalf("only selection should fail: %+v", i)
		}
	}
	if BuildRunPreview(w, in).Valid {
		t.Fatal("missing reference accepted")
	}
	w.Edges = append(w.Edges, dataEdge("cycle", "flour", "material", "roast", "components"))
	if BuildRunPreview(w, in).Valid {
		t.Fatal("inactive cycle accepted")
	}
}

func TestV8ReuseMultipleEndsConsecutiveBoundariesAndVariables(t *testing.T) {
	w, in := reuseFixture()
	// A second sink still requires the original raw input, but not roasting.
	w.Nodes = append(w.Nodes, Node{ID: "other", Kind: ModuleBOM, Config: map[string]any{"output_type": "material"}}, Node{ID: "other-output", Kind: ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}})
	w.Edges = append(w.Edges, dataEdge("f", "raw", "material", "other", "components"), dataEdge("g", "other", "assembly", "other-output", "from_bom"))
	p := BuildExecutionPlan(w, in)
	if !p.Active("raw") || p.Active("roast") {
		t.Fatalf("shared input broken: %+v", p)
	}
	in["flour"]["action"] = "reuse"
	in["flour"]["material_id"] = 43
	p = BuildExecutionPlan(w, in)
	if p.Active("semi") || p.Active("grind") || p["flour"].Status != "reused" {
		t.Fatalf("consecutive boundary: %+v", p)
	}
	w, in = reuseFixture()
	w.Variables = []WorkflowVariable{{ID: "up", Name: "上游名"}, {ID: "down", Name: "下游名"}}
	w.Nodes[3].Config["name_parts"] = []any{map[string]any{"type": "variable", "variable_id": "up"}}
	w.Nodes[5].Config["name_parts"] = []any{map[string]any{"type": "variable", "variable_id": "down"}}
	issues := ValidateWorkflowVariableValues(w, in, nil)
	if len(issues) != 1 || issues[0].NodeID != "flour" {
		t.Fatalf("variable filtering: %+v", issues)
	}
}
