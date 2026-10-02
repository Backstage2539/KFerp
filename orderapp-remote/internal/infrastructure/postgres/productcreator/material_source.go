package productcreator

import (
	"context"
	"fmt"
	app "orderapp/internal/application/productcreator"
	"strings"
)

// Only rows actually consumed by a BOM require manufacturing readiness.
// Unselected candidates remain reusable data inputs, not implicit ingredients.
func (e BusinessExecutor) inspectMaterialSources(ctx context.Context, run app.Run) (map[string]map[string]any, []app.ValidationIssue) {
	details := map[string]map[string]any{}
	issues := []app.ValidationIssue{}
	if run.Workflow.Version < 2 {
		return details, issues
	}
	nodes := map[string]app.Node{}
	producers := map[string]string{}
	for _, n := range run.Workflow.Nodes {
		nodes[n.ID] = n
	}
	for _, edge := range run.Workflow.Edges {
		if edge.Kind == app.EdgeData && edge.SourceHandle == "assembly" && edge.TargetHandle == "from_bom" && nodes[edge.Source].Kind == app.ModuleBOM {
			producers[edge.Target] = edge.Source
		}
	}
	used := map[string]map[string]bool{}
	add := func(nodeID, rowID string) {
		if used[nodeID] == nil {
			used[nodeID] = map[string]bool{}
		}
		used[nodeID][rowID] = true
	}
	for _, node := range run.Workflow.Nodes {
		if node.Kind != app.ModuleBOM {
			continue
		}
		values := run.Inputs[node.ID]
		if run.Workflow.Version >= 4 && stringValue(node.Config["output_type"]) == "product" {
			add(stringValue(values["main_input_source_node_id"]), stringValue(values["main_input_source_row_id"]))
		} else {
			for _, row := range mapRows(values["components"]) {
				add(stringValue(row["source_node_id"]), stringValue(row["source_row_id"]))
			}
		}
	}
	for _, node := range run.Workflow.Nodes {
		if node.Kind != app.ModuleMaterial {
			continue
		}
		output := stringValue(node.Config["data_role"]) == "output"
		rows := mapRows(run.Inputs[node.ID]["rows"])
		if output {
			row := cloneJSONMap(run.Inputs[node.ID])
			row["row_id"] = "output"
			rows = []map[string]any{row}
		}
		summaries := []map[string]any{}
		for _, row := range rows {
			rowID := stringValue(row["row_id"])
			name := stringValue(row["name"])
			mode := stringValue(row["supply_mode"])
			if mode == "" {
				if output {
					mode = "manufacture"
				} else {
					mode = "purchase"
				}
			}
			validProducer := producers[node.ID] != ""
			field := "supply_mode"
			if !output {
				field = "rows." + rowID + ".supply_mode"
			}
			consumed := used[node.ID][rowID]
			if stringValue(row["action"]) == "reuse" && (consumed || run.Workflow.Version >= 7) {
				id := int64(positiveNumber(row["material_id"]))
				var semi, hasBOM bool
				q := fmt.Sprintf(`SELECT m.name,COALESCE(m.is_semi_finished,false),EXISTS(
     SELECT 1 FROM %s.production_bom_output_bindings b
     JOIN %s.production_boms pb ON pb.id=b.bom_id AND pb.output_type='material' AND pb.output_material_id=m.id AND pb.status='active'
     JOIN %s.production_bom_versions v ON v.id=b.bom_version_id AND v.bom_id=pb.id AND v.status='published'
     WHERE b.output_type='material' AND b.output_id=m.id AND b.is_default=true)
     FROM %s.materials m WHERE m.id=$1 AND m.deprecated_at IS NULL`, e.schema, e.schema, e.schema, e.schema)
				var err error
				if tx := queryWithTransaction(ctx); tx != nil {
					err = tx.QueryRow(ctx, q+" FOR SHARE OF m", id).Scan(&name, &semi, &hasBOM)
				} else if e.pool != nil {
					err = e.pool.QueryRow(ctx, q, id).Scan(&name, &semi, &hasBOM)
				} else {
					err = fmt.Errorf("material lookup unavailable")
				}
				if err != nil {
					issues = append(issues, app.ValidationIssue{NodeID: node.ID, Field: strings.TrimSuffix(field, "supply_mode") + "material_id", Code: "material_unavailable", Message: "投入配方物料不存在、已停用或暂时无法读取"})
					continue
				}
				mode = "purchase"
				if semi {
					mode = "manufacture"
				}
				if app.IsReusedOutput(run.Workflow, node, run.Inputs) {
					validProducer = hasBOM
				} else {
					validProducer = validProducer || hasBOM
				}
			}
			if mode == "external" {
				mode = "purchase"
			}
			if mode == "manufactured" {
				mode = "manufacture"
			}
			if run.Workflow.Version >= 7 && mode != app.MaterialSupplyMode(node) {
				issues = append(issues, app.ValidationIssue{NodeID: node.ID, Field: field, Code: "material_supply_mode_mismatch", Message: fmt.Sprintf("%s：所选物料“%s”的真实取得方式与节点不符。外购节点只能选择外购物料，自制物料需使用连接了 BOM 的自制节点", node.Name, name)})
			}
			role := "投入配方物料"
			if output {
				role = "BOM 产出物料"
			}
			summaries = append(summaries, map[string]any{"row_id": rowID, "name": name, "supply_mode": mode, "role": role, "producer_node_id": producers[node.ID], "consumed": consumed})
			if (consumed || app.IsReusedOutput(run.Workflow, node, run.Inputs)) && (mode == "manufacture" || mode == "manufactured") && !validProducer {
				issues = append(issues, app.ValidationIssue{NodeID: node.ID, Field: field, Code: "manufacturing_source_missing", Message: fmt.Sprintf("%s：投入配方的自制物料“%s”缺少制造来源，请连接上游 BOM 产出，或引用具有默认已发布制造 BOM 的物料；外购原料请选择“外购”", node.Name, name)})
			}
		}
		details[node.ID] = map[string]any{"material_objects": summaries}
	}
	return details, issues
}
