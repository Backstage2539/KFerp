package productcreator

import "sort"

// NodeExecution is derived from graph dependencies and persisted choices, never
// from client-supplied skip flags. It does not remove inactive draft inputs.
type NodeExecution struct {
	Status   string   `json:"status"`
	ReusedBy []string `json:"reused_by_node_ids,omitempty"`
}
type ExecutionPlan map[string]NodeExecution

func IsReusedOutput(workflow Workflow, node Node, inputs map[string]map[string]any) bool {
	if workflowVersion(workflow) < 8 || (node.Kind != ModuleMaterial && node.Kind != ModuleProduct) || stringValue(node.Config["data_role"]) != "output" {
		return false
	}
	action := stringValue(inputs[node.ID]["action"])
	if action == "" {
		action = stringValue(node.Config["object_action"])
	}
	if action == "" {
		action = stringValue(node.Config["action"])
	}
	if action == "" {
		if d, ok := node.Config["defaults"].(map[string]any); ok {
			action = stringValue(d["action"])
		}
	}
	return action == "reuse"
}
func BuildExecutionPlan(workflow Workflow, inputs map[string]map[string]any) ExecutionPlan {
	plan := ExecutionPlan{}
	nodes := map[string]Node{}
	incoming := map[string][]string{}
	outgoing := map[string]int{}
	for _, n := range workflow.Nodes {
		nodes[n.ID] = n
		plan[n.ID] = NodeExecution{Status: "ready"}
	}
	if workflowVersion(workflow) < 8 {
		return plan
	}
	for _, e := range workflow.Edges {
		incoming[e.Target] = append(incoming[e.Target], e.Source)
		outgoing[e.Source]++
	}
	active := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if active[id] {
			return
		}
		n, ok := nodes[id]
		if !ok {
			return
		}
		active[id] = true
		if IsReusedOutput(workflow, n, inputs) {
			return
		}
		for _, parent := range incoming[id] {
			visit(parent)
		}
	}
	for _, n := range workflow.Nodes {
		if outgoing[n.ID] == 0 {
			visit(n.ID)
		}
	}
	for _, n := range workflow.Nodes {
		if !active[n.ID] {
			plan[n.ID] = NodeExecution{Status: "skipped"}
		} else if IsReusedOutput(workflow, n, inputs) {
			plan[n.ID] = NodeExecution{Status: "reused"}
		}
	}
	for _, n := range workflow.Nodes {
		if !active[n.ID] || !IsReusedOutput(workflow, n, inputs) {
			continue
		}
		seen := map[string]bool{}
		var trace func(string)
		trace = func(id string) {
			if seen[id] {
				return
			}
			seen[id] = true
			if !active[id] {
				s := plan[id]
				s.ReusedBy = append(s.ReusedBy, n.ID)
				plan[id] = s
			}
			for _, parent := range incoming[id] {
				trace(parent)
			}
		}
		for _, parent := range incoming[n.ID] {
			trace(parent)
		}
	}
	for id, s := range plan {
		sort.Strings(s.ReusedBy)
		plan[id] = s
	}
	return plan
}
func (p ExecutionPlan) Active(id string) bool { return p[id].Status != "skipped" }
func (s NodeExecution) Details() map[string]any {
	return map[string]any{"skip_reason": "upstream_replaced", "reused_by_node_ids": s.ReusedBy}
}

// ActiveWorkflow is for read-only inspections. Validation and topological order
// always use the full template so disabled paths cannot conceal graph damage.
func ActiveWorkflow(w Workflow, inputs map[string]map[string]any) Workflow {
	p := BuildExecutionPlan(w, inputs)
	original := w.Nodes
	w.Nodes = nil
	for _, n := range original {
		if p.Active(n.ID) {
			w.Nodes = append(w.Nodes, n)
		}
	}
	return w
}
