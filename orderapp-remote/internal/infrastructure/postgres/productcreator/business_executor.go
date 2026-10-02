package productcreator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	costingapp "orderapp/internal/application/costing"
	materialsapp "orderapp/internal/application/materials"
	creatorapp "orderapp/internal/application/productcreator"
	purchaseapp "orderapp/internal/application/purchase"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BusinessExecutor struct {
	schema    string
	pool      *pgxpool.Pool
	catalog   *catalogapp.Service
	materials *materialsapp.Service
	bom       *bomapp.Service
	purchase  *purchaseapp.Service
	costing   *costingapp.Service
}

func NewBusinessExecutor(schema string, pool *pgxpool.Pool, catalog *catalogapp.Service, materials *materialsapp.Service, bom *bomapp.Service, purchase *purchaseapp.Service, costing *costingapp.Service) BusinessExecutor {
	return BusinessExecutor{schema: schema, pool: pool, catalog: catalog, materials: materials, bom: bom, purchase: purchase, costing: costing}
}

func (e BusinessExecutor) InspectConfigurationPreview(ctx context.Context, run creatorapp.Run) (map[string]map[string]any, []creatorapp.ValidationIssue) {
	details, issues := e.inspectMaterialSources(ctx, run)
	if run.Workflow.Version < 4 || e.bom == nil {
		return details, issues
	}
	if run.Workflow.Version >= 5 {
		for _, node := range run.Workflow.Nodes {
			if node.Kind != creatorapp.ModuleBOM || stringValue(node.Config["output_type"]) != "material" {
				continue
			}
			connectedRoutes := map[string]int64{}
			for _, edge := range run.Workflow.Edges {
				if edge.Kind == creatorapp.EdgeData && edge.Target == node.ID && edge.TargetHandle == "route" {
					connectedRoutes[edge.Source] = int64(positiveNumber(run.Inputs[edge.Source]["route_id"]))
				}
			}
			selection := creatorapp.ResolveBOMProcessRoute(run.Workflow, node, run.Inputs[node.ID], connectedRoutes)
			name := ""
			if selection.ID > 0 {
				var status string
				var routeErr error
				name, status, routeErr = e.readProcessRoute(ctx, selection.ID)
				if routeErr != nil || status != "active" {
					field, message := "route_override_id", "本次选择的工艺路线已失效，请重新选择"
					if selection.Source == "connected_node" {
						field, message = "route", "连接的工艺路线已失效，请更换工艺节点路线"
					}
					issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: field, Code: "process_route_unavailable", Message: message})
					selection.ID = 0
				}
			}
			details[node.ID] = map[string]any{"process_route": map[string]any{"id": selection.ID, "name": name, "source": selection.Source, "process_node_id": selection.ProcessNodeID}}
		}
	}
	var templateCatalog []bomapp.ProductionBomSpecTemplate
	loaded := false
	for _, node := range run.Workflow.Nodes {
		if node.Kind != creatorapp.ModuleBOM || stringValue(node.Config["output_type"]) != "product" {
			continue
		}
		versionID := positiveNumber(node.Config["spec_template_version_id"])
		if versionID <= 0 {
			continue // The graph validator reports the required selection.
		}
		if !loaded {
			var err error
			templateCatalog, err = e.bom.ListProductionBomSpecTemplates(ctx)
			if err != nil {
				issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "spec_template_version_id", Code: "spec_template_unavailable", Message: "规格模板暂时无法读取，请稍后重试"})
				return details, issues
			}
			loaded = true
		}
		var selectedTemplate *bomapp.ProductionBomSpecTemplate
		var selectedVersion *bomapp.ProductionBomSpecTemplateVersion
		for index := range templateCatalog {
			candidate := &templateCatalog[index]
			if !candidate.Active {
				continue
			}
			for versionIndex := range candidate.Versions {
				version := &candidate.Versions[versionIndex]
				if version.ID == versionID && version.Status == "published" {
					selectedTemplate, selectedVersion = candidate, version
					break
				}
			}
			if selectedVersion != nil {
				break
			}
		}
		if selectedTemplate == nil || selectedVersion == nil {
			issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "spec_template_version_id", Code: "spec_template_not_published", Message: "所选规格模板版本已停用或不再发布，请重新选择"})
			continue
		}
		templateDetail, err := e.bom.GetProductionBomSpecTemplate(ctx, selectedTemplate.ID, versionID)
		if err != nil {
			issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "spec_template_version_id", Code: "spec_template_unavailable", Message: "所选规格模板详情无法读取"})
			continue
		}
		// Keep run-specific route overrides on this preview copy only.
		templateDetail.Variants = append([]bomapp.ProductionBomSpecTemplateVariant(nil), templateDetail.Variants...)
		if len(templateDetail.Variants) == 0 {
			issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "spec_template_version_id", Code: "spec_template_empty", Message: "所选规格模板没有规格，请先发布有效版本"})
			continue
		}
		inputValues := run.Inputs[node.ID]
		processRoute := creatorapp.BOMProcessRoute{Source: "specification_template"}
		processRouteName := "规格模板各规格默认工艺"
		if run.Workflow.Version >= 5 {
			connectedRoutes := map[string]int64{}
			for _, edge := range run.Workflow.Edges {
				if edge.Kind == creatorapp.EdgeData && edge.Target == node.ID && edge.TargetHandle == "route" {
					connectedRoutes[edge.Source] = int64(positiveNumber(run.Inputs[edge.Source]["route_id"]))
				}
			}
			processRoute = creatorapp.ResolveBOMProcessRoute(run.Workflow, node, inputValues, connectedRoutes)
			if processRoute.ID > 0 {
				var status string
				var routeErr error
				processRouteName, status, routeErr = e.readProcessRoute(ctx, processRoute.ID)
				if routeErr != nil || status != "active" {
					field := "route_override_id"
					message := "本次选择的工艺路线已失效，请重新选择"
					if processRoute.Source == "connected_node" {
						field = "route"
						message = "连接的工艺路线已失效，请更换工艺节点路线"
					}
					issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: field, Code: "process_route_unavailable", Message: message})
					processRoute.ID = 0
				}
			} else if processRoute.Source == "specification_template" {
				processRouteName = "规格模板各规格默认工艺"
			}
			if processRoute.ID > 0 {
				for index := range templateDetail.Variants {
					templateDetail.Variants[index].ProcessRouteID = processRoute.ID
				}
			}
			for index := range templateDetail.Variants {
				variant := &templateDetail.Variants[index]
				variant.ProcessRouteSource = processRoute.Source
				if variant.ProcessRouteID <= 0 || strings.TrimSpace(e.schema) == "" {
					continue
				}
				var status string
				var routeErr error
				variant.ProcessRouteName, status, routeErr = e.readProcessRoute(ctx, variant.ProcessRouteID)
				if routeErr != nil || status != "active" {
					issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "spec_template_version_id", Code: "process_route_unavailable", Message: fmt.Sprintf("规格 %s 使用的工艺路线已失效，请更新规格模板", variant.Name)})
					variant.ProcessRouteName = ""
				}
			}
		}
		mainInputNodeID := strings.TrimSpace(stringValue(inputValues["main_input_source_node_id"]))
		var mainInputNode creatorapp.Node
		mainInputExists := false
		for _, candidate := range run.Workflow.Nodes {
			if candidate.ID == mainInputNodeID {
				mainInputNode, mainInputExists = candidate, true
				break
			}
		}
		if mainInputExists && mainInputNode.Kind == creatorapp.ModuleProduct {
			selectedRowID := strings.TrimSpace(stringValue(inputValues["main_input_source_row_id"]))
			if stringValue(mainInputNode.Config["data_role"]) == "output" {
				var upstreamBOM *creatorapp.Node
				for index := range run.Workflow.Nodes {
					candidate := &run.Workflow.Nodes[index]
					if candidate.Kind != creatorapp.ModuleBOM || stringValue(candidate.Config["output_type"]) != "product" {
						continue
					}
					for _, edge := range run.Workflow.Edges {
						if edge.Source == candidate.ID && edge.Target == mainInputNode.ID && edge.SourceHandle == "assembly" && edge.TargetHandle == "from_bom" {
							upstreamBOM = candidate
							break
						}
					}
					if upstreamBOM != nil {
						break
					}
				}
				validSpecKey := false
				if upstreamBOM != nil {
					upstreamVersionID := positiveNumber(upstreamBOM.Config["spec_template_version_id"])
					for _, sourceTemplate := range templateCatalog {
						if !sourceTemplate.Active {
							continue
						}
						published := false
						for _, sourceVersion := range sourceTemplate.Versions {
							if sourceVersion.ID == int64(upstreamVersionID) && sourceVersion.Status == "published" {
								published = true
								break
							}
						}
						if !published {
							continue
						}
						sourceDetail, detailErr := e.bom.GetProductionBomSpecTemplate(ctx, sourceTemplate.ID, int64(upstreamVersionID))
						if detailErr != nil {
							continue
						}
						for _, variant := range sourceDetail.Variants {
							if variant.SpecKey == selectedRowID {
								validSpecKey = true
								break
							}
						}
						break
					}
				}
				if !validSpecKey {
					issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "main_input_source_row_id", Code: "main_input_row_invalid", Message: "所选主体不属于该上游商品 BOM 生成的已发布规格"})
				}
			} else if selectedSpecID := positiveNumber(run.Inputs[mainInputNode.ID]["bom_spec_id"]); selectedSpecID <= 0 || selectedRowID != fmt.Sprintf("%d", int64(selectedSpecID)) {
				issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "main_input_source_row_id", Code: "main_input_row_invalid", Message: "请从已有商品已发布规格中选择主体"})
			}
			for _, variant := range templateDetail.Variants {
				for _, item := range variant.Items {
					if item.IsMainInput && (strings.EqualFold(strings.TrimSpace(item.ConsumeUnit), "ratio_pct") || item.RatioPct > 0 || variant.MaterialLossRate > 0) {
						issues = append(issues, creatorapp.ValidationIssue{NodeID: node.ID, Field: "main_input_source_row_id", Code: "product_main_input_requires_fixed_template", Message: "当前规格模板含比例配方或原料损耗，主体来源必须选择物料"})
						break
					}
				}
			}
		}
		mainInput := map[string]any{}
		if mainInputExists {
			mainInput = map[string]any{"source_node_name": mainInputNode.Name, "type": mainInputNode.Kind}
		}
		details[node.ID] = map[string]any{
			"specification_template": map[string]any{
				"template_id": selectedTemplate.ID, "name": selectedTemplate.Name,
				"version_id": selectedVersion.ID, "version_no": selectedVersion.VersionNo,
				"variant_count": len(templateDetail.Variants), "variants": templateDetail.Variants,
			},
			"main_input":    mainInput,
			"process_route": map[string]any{"id": processRoute.ID, "name": processRouteName, "source": processRoute.Source, "process_node_id": processRoute.ProcessNodeID},
		}
	}
	return details, issues
}

type createdReference struct {
	Type      string `json:"type"`
	RowID     string `json:"row_id,omitempty"`
	ID        int64  `json:"id"`
	Name      string `json:"name,omitempty"`
	Code      string `json:"code,omitempty"`
	Unit      string `json:"unit,omitempty"`
	OwnerID   int64  `json:"owner_customer_id,omitempty"`
	ProductID int64  `json:"product_id,omitempty"`
	BOMID     int64  `json:"bom_id,omitempty"`
	VersionID int64  `json:"version_id,omitempty"`
	SpecID    int64  `json:"bom_spec_id,omitempty"`
	VariantID int64  `json:"bom_variant_id,omitempty"`
	Published bool   `json:"published,omitempty"`
	New       bool   `json:"new,omitempty"`
}

func (e BusinessExecutor) ExecuteConfiguration(ctx context.Context, run creatorapp.Run, actor string) (map[string]any, error) {
	if run.Workflow.Version >= 2 {
		return e.executeBOMCentricConfiguration(ctx, run, actor)
	}
	order, err := creatorapp.TopologicalOrder(run.Workflow)
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]creatorapp.Node, len(run.Workflow.Nodes))
	for _, node := range run.Workflow.Nodes {
		nodes[node.ID] = node
	}
	previewStatus := map[string]string{}
	if run.Preview != nil {
		for _, step := range run.Preview.Steps {
			previewStatus[step.NodeID] = step.Status
		}
	}
	refs := map[string]map[string]createdReference{}
	for nodeID := range nodes {
		refs[nodeID] = map[string]createdReference{}
	}
	steps := make(map[string]any, len(order))
	needsFollowup := false
	for _, nodeID := range order {
		node := nodes[nodeID]
		if previewStatus[nodeID] == "skipped" {
			steps[nodeID] = map[string]any{"status": "skipped"}
			continue
		}
		stepCtx := postgresinfra.WithBusinessProvenance(ctx, run.ID, nodeID)
		values := run.Inputs[nodeID]
		var stepResult map[string]any
		switch node.Kind {
		case creatorapp.ModuleProduct:
			stepResult, err = e.executeProduct(stepCtx, node, values, actor, refs)
		case creatorapp.ModuleMaterial:
			stepResult, err = e.executeMaterials(stepCtx, node, values, actor, refs)
		case creatorapp.ModuleProcess:
			stepResult, err = e.executeProcess(stepCtx, node, values, refs)
		case creatorapp.ModuleBOM:
			stepResult, err = e.executeBOM(stepCtx, run, node, values, actor, refs)
		case creatorapp.ModulePublish:
			stepResult, err = e.executePublish(stepCtx, run, node, values, actor, refs)
		case creatorapp.ModulePurchase, creatorapp.ModulePricing:
			// Transaction and publication steps run after the configuration
			// transaction. Their inputs are already persisted in the run draft.
			stepResult = map[string]any{"status": "ready", "action": "waiting_for_configuration"}
			needsFollowup = true
		default:
			err = fmt.Errorf("unregistered module %q", node.Kind)
		}
		if err != nil {
			return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: nodeID, Code: "business_validation", Message: err.Error()}}}
		}
		if stepResult == nil {
			stepResult = map[string]any{}
		}
		if _, ok := stepResult["status"]; !ok {
			stepResult["status"] = "succeeded"
		}
		steps[nodeID] = stepResult
	}
	runStatus := "config_committed"
	if needsFollowup {
		runStatus = "in_progress"
	}
	return map[string]any{"run_status": runStatus, "steps": steps, "objects": allReferences(refs)}, nil
}

// executeBOMCentricConfiguration compiles the visible BOM graph into the
// existing domain operations. Data records are prepared first inside the
// caller's transaction; BOMs then execute in dependency order and are
// published before their outputs can feed a downstream BOM.
func (e BusinessExecutor) executeBOMCentricConfiguration(ctx context.Context, run creatorapp.Run, actor string) (map[string]any, error) {
	run.Inputs = creatorapp.ResolveWorkflowInputDefaults(run.Workflow, run.Inputs)
	if _, issues := e.inspectMaterialSources(ctx, run); len(issues) > 0 {
		return nil, creatorapp.ExecutionError{Issues: issues}
	}
	order, err := creatorapp.TopologicalOrder(run.Workflow)
	if err != nil {
		return nil, err
	}
	nodes := make(map[string]creatorapp.Node, len(run.Workflow.Nodes))
	for _, node := range run.Workflow.Nodes {
		nodes[node.ID] = node
	}
	refs := make(map[string]map[string]createdReference, len(nodes))
	for id := range nodes {
		refs[id] = map[string]createdReference{}
	}
	steps := make(map[string]any, len(nodes))
	outputNodeForBOM := make(map[string]string)
	for _, edge := range run.Workflow.Edges {
		if source, ok := nodes[edge.Source]; ok && source.Kind == creatorapp.ModuleBOM && edge.SourceHandle == "assembly" {
			if target, exists := nodes[edge.Target]; exists && (target.Kind == creatorapp.ModuleMaterial || target.Kind == creatorapp.ModuleProduct) && edge.TargetHandle == "from_bom" {
				outputNodeForBOM[source.ID] = target.ID
			}
		}
	}

	// Prepare component inputs and output records first. Output records are
	// created only once and are reused by every downstream edge.
	for _, id := range order {
		node := nodes[id]
		values := cloneJSONMap(run.Inputs[id])
		if run.Workflow.Version >= 3 {
			// V3 intentionally has no industry-category controls. Keep the
			// domain's compatibility fields neutral even if a client forges them.
			switch node.Kind {
			case creatorapp.ModuleMaterial:
				values["kind"] = "other"
				for _, row := range mapRows(values["rows"]) {
					row["kind"] = "other"
				}
			case creatorapp.ModuleProduct:
				values["product_kind"] = "generic"
			}
		}
		stepCtx := postgresinfra.WithBusinessProvenance(ctx, run.ID, id)
		switch node.Kind {
		case creatorapp.ModuleMaterial:
			if stringValue(node.Config["data_role"]) == "output" {
				row := map[string]any{
					"row_id": "output", "action": outputObjectAction(node, values), "material_id": values["material_id"],
					"name": resolvedOutputName(run, node, values), "kind": values["kind"], "supply_mode": values["supply_mode"],
					"unit": resolvedMaterialOutputUnit(run, node, values), "owner_type": materialOwnerType(values), "owner_customer_id": values["owner_customer_id"],
					"industry_fields": values["industry_fields"],
				}
				result, execErr := e.executeMaterials(stepCtx, node, map[string]any{"rows": []any{row}}, actor, refs)
				if execErr != nil {
					return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "name", Code: "business_validation", Message: execErr.Error()}}}
				}
				ref := refs[id]["output"]
				refs[id]["material"] = ref
				steps[id] = result
			} else {
				result, execErr := e.executeMaterials(stepCtx, node, values, actor, refs)
				if execErr != nil {
					return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "rows", Code: "business_validation", Message: execErr.Error()}}}
				}
				steps[id] = result
			}
		case creatorapp.ModuleProduct:
			if stringValue(node.Config["data_role"]) == "output" {
				productValues := cloneJSONMap(values)
				productValues["action"] = outputObjectAction(node, values)
				productValues["name"] = resolvedOutputName(run, node, values)
				if _, exists := productValues["owner"]; !exists {
					productValues["owner"] = node.Config["owner"]
				}
				result, execErr := e.executeProduct(stepCtx, node, productValues, actor, refs)
				if execErr != nil {
					return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "name", Code: "business_validation", Message: execErr.Error()}}}
				}
				ref := refs[id]["product"]
				ref.RowID = "output"
				refs[id]["output"] = ref
				steps[id] = result
			} else {
				result, execErr := e.executeBOMProductInput(stepCtx, node, values, refs)
				if execErr != nil {
					return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "product_id", Code: "business_validation", Message: execErr.Error()}}}
				}
				steps[id] = result
			}
		case creatorapp.ModuleProcess:
			result, execErr := e.executeProcess(stepCtx, node, values, refs)
			if execErr != nil {
				return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "route_id", Code: "business_validation", Message: execErr.Error()}}}
			}
			steps[id] = result
		}
	}

	needsFollowup := false
	for _, id := range order {
		node := nodes[id]
		values := run.Inputs[id]
		switch node.Kind {
		case creatorapp.ModuleBOM:
			outputNodeID := outputNodeForBOM[id]
			if outputNodeID == "" {
				return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "assembly", Code: "bom_output_required", Message: "BOM组装没有连接产出对象"}}}
			}
			output := refs[outputNodeID]["output"]
			if output.ID <= 0 {
				return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: outputNodeID, Code: "output_not_ready", Message: "BOM产出档案尚未准备好"}}}
			}
			bomValues := cloneJSONMap(values)
			bomValues["action"] = "create"
			bomValues["output_source_row_id"] = "output"
			if bomValues["route_id"] == nil || positiveNumber(bomValues["route_id"]) == 0 {
				for _, edge := range run.Workflow.Edges {
					if edge.Target == id && edge.TargetHandle == "route" {
						bomValues["route_id"] = refs[edge.Source]["route"].ID
					}
				}
			}
			executionRun := run
			executionRun.Workflow = run.Workflow
			executionRun.Workflow.Edges = append(append([]creatorapp.Edge(nil), run.Workflow.Edges...), creatorapp.Edge{
				ID: "compiled-output-" + id, Source: outputNodeID, SourceHandle: map[string]string{"product": "product", "material": "material"}[output.Type], Target: id, TargetHandle: "output", Kind: creatorapp.EdgeData,
			})
			executionRun.Inputs = cloneNestedJSONMap(run.Inputs)
			executionRun.Inputs[id] = bomValues
			stepCtx := postgresinfra.WithBusinessProvenance(ctx, run.ID, id)
			created, execErr := e.executeBOM(stepCtx, executionRun, node, bomValues, actor, refs)
			if execErr != nil {
				return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "components", Code: "business_validation", Message: execErr.Error()}}}
			}
			publishID := "compiled-publish-" + id
			publishNode := creatorapp.Node{ID: publishID, Kind: creatorapp.ModulePublish}
			refs[publishID] = map[string]createdReference{}
			publishRun := executionRun
			publishRun.Workflow.Edges = append(publishRun.Workflow.Edges, creatorapp.Edge{ID: "compiled-publish-edge-" + id, Source: id, SourceHandle: "bom", Target: publishID, TargetHandle: "bom", Kind: creatorapp.EdgeData})
			publishRun.Inputs[publishID] = map[string]any{"set_default": output.New}
			published, execErr := e.executePublish(stepCtx, publishRun, publishNode, publishRun.Inputs[publishID], actor, refs)
			if execErr != nil {
				return nil, creatorapp.ExecutionError{Issues: []creatorapp.ValidationIssue{{NodeID: id, Field: "publish", Code: "business_validation", Message: execErr.Error()}}}
			}
			for key, ref := range refs[publishID] {
				if key != "output" && ref.Type != "spec" {
					continue
				}
				refs[outputNodeID][key] = ref
			}
			delete(refs, publishID)
			steps[id] = map[string]any{"status": "succeeded", "bom": created, "publication": published, "default_bound": output.New}
		case creatorapp.ModulePurchase:
			steps[id] = map[string]any{"status": "ready", "action": "waiting_for_configuration"}
			needsFollowup = true
		}
	}

	runStatus := "config_committed"
	if needsFollowup {
		runStatus = "in_progress"
	}
	return map[string]any{"run_status": runStatus, "steps": steps, "objects": allReferences(refs)}, nil
}

func (e BusinessExecutor) executeBOMProductInput(ctx context.Context, node creatorapp.Node, values map[string]any, refs map[string]map[string]createdReference) (map[string]any, error) {
	id := positiveNumber(values["product_id"])
	if id <= 0 {
		return nil, fmt.Errorf("请选择已有商品")
	}
	var name string
	var ownerID int64
	var active bool
	if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(customer_id,0),COALESCE(active,true) FROM %s.products WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&name, &ownerID, &active); err != nil || !active {
		return nil, fmt.Errorf("引用的商品不存在或当前不可见")
	}
	specID := positiveNumber(values["bom_spec_id"])
	rows, err := queryWithTransaction(ctx).Query(ctx, fmt.Sprintf(`
		SELECT spec.id,spec.spec_key,COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),variant.inventory_unit,variant.id
		FROM %s.production_bom_specs spec
		JOIN LATERAL (
			SELECT version.id
			FROM %s.production_bom_versions version
			WHERE version.bom_id=spec.bom_id AND version.status='published'
			ORDER BY version.published_at DESC NULLS LAST,version.created_at DESC,version.id DESC
			LIMIT 1
		) latest ON true
		JOIN %s.production_bom_version_variants variant ON variant.version_id=latest.id AND variant.bom_spec_id=spec.id
		JOIN %s.production_boms bom ON bom.id=spec.bom_id AND bom.output_type='product' AND bom.output_product_id=$1 AND COALESCE(bom.status,'active')='active'
		WHERE ($2=0 OR spec.id=$2) ORDER BY bom.id,variant.sort_order,variant.id`, e.schema, e.schema, e.schema, e.schema), id, specID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ref createdReference
		if err := rows.Scan(&ref.ID, &ref.RowID, &ref.Name, &ref.Unit, &ref.VariantID); err != nil {
			return nil, err
		}
		ref.Type, ref.SpecID, ref.ProductID, ref.OwnerID, ref.Published = "spec", ref.ID, id, ownerID, true
		refs[node.ID][fmt.Sprintf("%d", ref.SpecID)] = ref
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("所选商品没有可用的已发布规格")
	}
	refs[node.ID]["product"] = createdReference{Type: "product", RowID: "product", ID: id, Name: name, OwnerID: ownerID, ProductID: id}
	return map[string]any{"product_id": id, "specifications": refsForNode(refs[node.ID])}, nil
}

func outputObjectAction(node creatorapp.Node, values map[string]any) string {
	action := stringValue(values["action"])
	if action == "" {
		action = stringValue(node.Config["object_action"])
	}
	if action == "" {
		action = stringValue(node.Config["action"])
	}
	if action == "" {
		action = "create"
	}
	return action
}

func materialOwnerType(values map[string]any) string {
	owner := stringValue(values["owner_type"])
	if owner == "" {
		owner = stringValue(values["owner"])
	}
	if owner == "" {
		owner = "factory"
	}
	return owner
}

func resolvedOutputName(run creatorapp.Run, node creatorapp.Node, values map[string]any) string {
	if name := stringValue(values["name"]); name != "" && !workflowFieldIsFixed(node, "name") {
		return name
	}
	pattern := stringValue(node.Config["name_pattern"])
	if pattern == "" {
		pattern = stringValue(values["name_pattern"])
	}
	if pattern == "" {
		return ""
	}
	productName := ""
	for _, candidate := range run.Workflow.Nodes {
		if candidate.Kind == creatorapp.ModuleProduct && stringValue(candidate.Config["data_role"]) == "output" {
			productName = stringValue(run.Inputs[candidate.ID]["name"])
			if productName != "" {
				break
			}
		}
	}
	for _, token := range []string{"{{商品名称}}", "{{商品}}", "{商品名称}", "{商品}"} {
		pattern = strings.ReplaceAll(pattern, token, productName)
	}
	return strings.TrimSpace(pattern)
}

func resolvedMaterialOutputUnit(run creatorapp.Run, node creatorapp.Node, values map[string]any) string {
	for _, edge := range run.Workflow.Edges {
		if edge.Target != node.ID || edge.TargetHandle != "from_bom" || edge.Kind != creatorapp.EdgeData {
			continue
		}
		unit := stringValue(run.Inputs[edge.Source]["output_unit"])
		if unit == "" {
			for _, source := range run.Workflow.Nodes {
				if source.ID == edge.Source {
					unit = stringValue(source.Config["output_unit"])
					break
				}
			}
		}
		if unit != "" {
			return unit
		}
	}
	return defaultString(stringValue(values["unit"]), defaultString(stringValue(node.Config["unit"]), "unit"))
}

func workflowFieldIsFixed(node creatorapp.Node, key string) bool {
	switch fields := node.Config["fixed_fields"].(type) {
	case []any:
		for _, field := range fields {
			if stringValue(field) == key {
				return true
			}
		}
	case []string:
		for _, field := range fields {
			if field == key {
				return true
			}
		}
	}
	return false
}

func cloneNestedJSONMap(values map[string]map[string]any) map[string]map[string]any {
	copy := make(map[string]map[string]any, len(values))
	for key, value := range values {
		copy[key] = cloneJSONMap(value)
	}
	return copy
}

func (e BusinessExecutor) ExecuteRunStep(ctx context.Context, run creatorapp.Run, node creatorapp.Node, action string, stepInputs map[string]any, actor string) (map[string]any, error) {
	if node.Kind == creatorapp.ModulePricing {
		return e.executePricingStep(ctx, run, node, action, actor)
	}
	if node.Kind != creatorapp.ModulePurchase || e.purchase == nil {
		return nil, creatorapp.ErrRunStepExecutorUnavailable
	}
	material, err := resolvePurchaseMaterial(run, node)
	if err != nil {
		return nil, err
	}
	values := run.Inputs[node.ID]
	switch action {
	case "create_purchase_order":
		order, err := e.purchase.CreatePurchaseOrder(ctx, purchaseapp.CreatePurchaseOrderCommand{
			SupplierID: positiveNumber(values["supplier_id"]), MaterialID: material.ID,
			Qty: numberValue(values["quantity"]), UnitCode: material.Unit,
			TargetWarehouse: stringValue(values["warehouse"]), UnitCost: numberValue(values["unit_price"]),
			Note: fmt.Sprintf("商品创建器运行 #%d", run.ID), Operator: actor,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "ready", "purchase_order": map[string]any{"id": order.ID, "order_no": order.OrderNo, "status": order.Status, "quantity": order.Qty, "unit": order.UnitCode, "unit_price": order.UnitCost}, "receipt_status": "waiting"}, nil
	case "confirm_receipt":
		steps, _ := run.BusinessResults["steps"].(map[string]any)
		previous, _ := steps[node.ID].(map[string]any)
		order, _ := previous["purchase_order"].(map[string]any)
		orderID := positiveNumber(order["id"])
		if orderID <= 0 {
			return nil, fmt.Errorf("请先创建并确认采购单")
		}
		quantity := numberValue(stepInputs["quantity"])
		if quantity <= 0 {
			quantity = numberValue(values["quantity"])
		}
		unitPrice := numberValue(stepInputs["unit_price"])
		if unitPrice <= 0 {
			unitPrice = numberValue(values["unit_price"])
		}
		receipt, err := e.purchase.CreatePurchaseReceipt(ctx, purchaseapp.CreatePurchaseReceiptCommand{
			PurchaseOrderID: orderID, SupplierID: positiveNumber(values["supplier_id"]), MaterialID: material.ID,
			Qty: quantity, UnitCode: material.Unit, TargetWarehouse: stringValue(values["warehouse"]),
			UnitCost: unitPrice, Note: fmt.Sprintf("商品创建器运行 #%d 到货确认", run.ID), Operator: actor,
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "succeeded", "purchase_receipt": map[string]any{"id": receipt.ID, "receipt_no": receipt.ReceiptNo, "stock_batch_code": receipt.StockBatchCode, "quantity": receipt.Qty, "unit": receipt.UnitCode, "unit_price": receipt.UnitCost}}, nil
	default:
		return nil, fmt.Errorf("unsupported purchase action")
	}
}

func (e BusinessExecutor) executePricingStep(ctx context.Context, run creatorapp.Run, node creatorapp.Node, action, actor string) (map[string]any, error) {
	if e.costing == nil {
		return nil, creatorapp.ErrRunStepExecutorUnavailable
	}
	if action == "publish_price" {
		steps := resultMap(run.BusinessResults["steps"])
		previous := resultMap(steps[node.ID])
		if resultMap(previous["price_draft"])["id"] == nil {
			return nil, fmt.Errorf("请先保存价格草稿，再发布价格")
		}
	}
	if err := requirePricingReceipts(run, node); err != nil {
		return nil, err
	}
	command, priceResults, err := e.beanListCommandForRun(ctx, run, node, action, actor)
	if err != nil {
		return nil, err
	}
	switch action {
	case "preview_pricing":
		if err := e.costing.ValidateBeanListDraft(ctx, command); err != nil {
			return nil, err
		}
		return map[string]any{"status": "ready", "pricing_preview": map[string]any{"valid": true, "prices": priceResults, "target_table": command.ProductTypeName}}, nil
	case "save_price_draft":
		draft, err := e.costing.SaveBeanListDraft(ctx, command)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "ready", "price_draft": map[string]any{"id": draft.ID, "version": draft.Version, "status": draft.Status, "table_name": draft.TableName}}, nil
	case "publish_price":
		published, err := e.costing.PublishBeanList(ctx, command)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "succeeded", "published_price": map[string]any{"id": published.ID, "version": published.Version, "status": published.Status, "table_name": published.TableName}}, nil
	default:
		return nil, fmt.Errorf("unsupported pricing action")
	}
}

func requirePricingReceipts(run creatorapp.Run, node creatorapp.Node) error {
	steps := resultMap(run.BusinessResults["steps"])
	for _, edge := range run.Workflow.Edges {
		if edge.Target != node.ID || edge.TargetHandle != "receipt" || edge.Kind != creatorapp.EdgeData {
			continue
		}
		if resultMap(steps[edge.Source])["purchase_receipt"] == nil {
			return fmt.Errorf("定价引用了本次采购成本，请先完成收货并重新试算")
		}
	}
	return nil
}

func (e BusinessExecutor) beanListCommandForRun(ctx context.Context, run creatorapp.Run, node creatorapp.Node, action, actor string) (costingapp.PublishBeanListCommand, []map[string]any, error) {
	var specSource creatorapp.Edge
	for _, edge := range run.Workflow.Edges {
		if edge.Target == node.ID && edge.TargetHandle == "product" && edge.Kind == creatorapp.EdgeData {
			if specSource.ID != "" {
				return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("一个价格步骤只能连接一个商品规格来源；请按商品拆分价格步骤")
			}
			specSource = edge
		}
	}
	if specSource.ID == "" {
		return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("请连接一个已发布的商品规格来源")
	}
	objects := resultMap(run.BusinessResults["objects"])
	refs := anySlice(objects[specSource.Source])
	specs := make(map[string]createdReference)
	var product createdReference
	for _, raw := range refs {
		ref := referenceFromAny(raw)
		if ref.Type == "spec" && ref.Published {
			specs[ref.RowID] = ref
		} else if ref.Type == "product" && ref.RowID == "output" {
			product = ref
		}
	}
	if product.ID <= 0 || product.Type != "product" || len(specs) == 0 {
		return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("价格来源必须是已发布商品 BOM 的有效规格")
	}
	ownerType, ownerKey := "official", ""
	if product.OwnerID > 0 {
		ownerType, ownerKey = "customer", fmt.Sprint(product.OwnerID)
	}
	query := costingapp.BeanListPublicationQuery{ListType: "commercial", PublicationPurpose: costingapp.BeanListPublicationPurposeFactorySupply, OwnerType: ownerType, OwnerKey: ownerKey}
	sourceID := positiveNumber(run.Inputs[node.ID]["price_list_id"])
	if sourceID <= 0 {
		return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("请选择目标价格表")
	}
	source, err := e.costing.LoadBeanListPublication(ctx, query, sourceID)
	if err != nil {
		return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("目标价格表不存在或不属于该商品归属范围：%w", err)
	}
	if source.Status != "published" || source.OwnerType != ownerType || source.OwnerKey != ownerKey || source.ListType != "commercial" {
		return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("目标价格表必须是同一商品归属范围内已发布的商品价格表")
	}
	command := costingapp.PublishBeanListCommand{
		ListType: source.ListType, PublicationPurpose: source.PublicationPurpose,
		ProductTypeCategoryID: source.ProductTypeCategoryID, ProductTypeName: source.ProductTypeName,
		ClassificationTemplateID: source.ClassificationTemplateID, ClassificationTemplateName: source.ClassificationTemplateName,
		ClassificationCategoryID: source.ClassificationCategoryID, ClassificationCategoryName: source.ClassificationCategoryName,
		Version: "PC-DRAFT-" + fmt.Sprint(run.ID) + "-" + node.ID, OwnerType: ownerType, OwnerKey: ownerKey,
		PriceSourcePublicationID: source.ID, StyleSourcePublicationID: source.StyleSourcePublicationID, SourceVersion: source.Version,
		Config: cloneJSONMap(source.Config), Content: cloneJSONMap(source.Content), Changelog: fmt.Sprintf("商品创建器运行 #%d · %s", run.ID, node.Name), Actor: actor,
	}
	tableKey := strings.TrimSpace(source.TableKey)
	if tableKey == "" {
		tableKey = fmt.Sprintf("creator-source-%d", source.ID)
	}
	tableName := strings.TrimSpace(source.TableName)
	if tableName == "" {
		tableName = strings.TrimSpace(source.ProductTypeName)
	}
	if tableName == "" {
		tableName = "商品价格表"
	}
	command.WorkflowPublicationMetadata = &costingapp.PublicationTableMetadata{
		ReleaseID: fmt.Sprintf("product-creator-run-%d-%s", run.ID, node.ID),
		TableKey:  tableKey, TableName: tableName,
		IsDefaultTable: source.IsDefaultTable, DirectShipEnabled: source.DirectShipEnabled,
	}
	if command.Config == nil {
		command.Config = map[string]any{}
	}
	if command.Content == nil {
		command.Content = map[string]any{}
	}
	priceInputs := map[string]map[string]any{}
	for _, row := range mapRows(run.Inputs[node.ID]["prices"]) {
		priceInputs[stringValue(row["spec_row_id"])] = row
	}
	newSelections := make([]any, 0, len(specs))
	for _, ref := range specs {
		newSelections = append(newSelections, map[string]any{"parent_product_id": ref.ProductID, "bom_id": ref.BOMID, "bom_version_id": ref.VersionID, "bom_spec_id": ref.SpecID, "bom_variant_id": ref.VariantID, "selection_source": "explicit"})
	}
	selectionRows := anySlice(command.Config["product_spec_selections"])
	keptSelections := make([]any, 0, len(selectionRows)+len(newSelections))
	for _, raw := range selectionRows {
		selection := resultMap(raw)
		if positiveNumber(selection["parent_product_id"]) != product.ID {
			keptSelections = append(keptSelections, raw)
		}
	}
	command.Config["product_spec_selections"] = append(keptSelections, newSelections...)
	oldRows := anySlice(command.Content["price_rows"])
	rowTemplate := map[string]any{}
	for _, raw := range oldRows {
		candidate := resultMap(raw)
		if len(candidate) > 0 && len(rowTemplate) == 0 {
			rowTemplate = candidate
		}
	}
	newPriceRows := make([]any, 0, len(oldRows)+len(specs))
	for _, raw := range oldRows {
		row := resultMap(raw)
		parentID := positiveNumber(row["parent_product_id"])
		if parentID == 0 {
			parentID = positiveNumber(row["product_id"])
		}
		if parentID != product.ID {
			newPriceRows = append(newPriceRows, raw)
		}
	}
	priceResults := make([]map[string]any, 0, len(specs))
	for _, row := range mapRows(run.Inputs[node.ID]["prices"]) {
		ref, ok := specs[stringValue(row["spec_row_id"])]
		if !ok {
			return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("价格行引用的商品规格已失效")
		}
		price, rule, trial, err := e.resolveWorkflowPrice(ctx, row, ref)
		if err != nil {
			return costingapp.PublishBeanListCommand{}, nil, fmt.Errorf("规格 %s：%w", ref.Name, err)
		}
		priceRow := cloneJSONMap(rowTemplate)
		priceRow["product_id"], priceRow["parent_product_id"] = ref.ProductID, ref.ProductID
		priceRow["product_name"], priceRow["product_name_snapshot"] = product.Name, product.Name
		priceRow["name"], priceRow["spec_label"], priceRow["sku_name"] = product.Name+" · "+ref.Name, ref.Name, ref.Name
		priceRow["bom_id"], priceRow["bom_version_id"], priceRow["bom_spec_id"], priceRow["bom_variant_id"] = ref.BOMID, ref.VersionID, ref.SpecID, ref.VariantID
		priceRow["pricing_mode"], priceRow["pricing_mode_source"] = map[bool]string{true: "pricing_rule", false: "fixed_price"}[rule != nil], "product_creator"
		priceRow["fixed_unit_price"], priceRow["final_unit_price"], priceRow["original_final_unit_price"] = price, price, price
		priceRow["price_unit"], priceRow["inventory_unit"] = ref.Unit, ref.Unit
		priceRow["inventory_conversion_json"] = map[string]any{ref.Unit: map[string]any{ref.Unit: float64(1)}}
		priceRow["group_source"] = defaultString(stringValue(priceRow["group_source"]), costingapp.PriceListGroupSourceProductCatalog)
		if _, ok := objectMap(priceRow["group_snapshot"]); !ok {
			priceRow["group_snapshot"] = map[string]any{"source": "product_creator", "product_type_name": source.ProductTypeName}
		}
		priceRow["cost_source_snapshot"] = map[string]any{"source": "manual_price", "mode": stringValue(row["pricing_mode"]), "trial": trial}
		priceRow["customer_reference_snapshot"] = map[string]any{}
		priceRow["manual_adjusted"] = rule == nil
		priceRow["frozen_final_price"] = true
		delete(priceRow, "source_price_record_id")
		if rule != nil {
			priceRow["pricing_rule_id"], priceRow["pricing_rule_source"], priceRow["pricing_rule_version"] = rule.ID, "product_creator", rule.FormulaVersion
		} else {
			delete(priceRow, "pricing_rule_id")
			delete(priceRow, "pricing_rule_source")
			delete(priceRow, "pricing_rule_version")
		}
		newPriceRows = append(newPriceRows, priceRow)
		priceResults = append(priceResults, map[string]any{"spec_row_id": ref.RowID, "spec_name": ref.Name, "unit": ref.Unit, "price": price})
	}
	command.Content["price_rows"] = newPriceRows
	if action == "publish_price" {
		command.Version = source.Version
	}
	return command, priceResults, nil
}

type workflowPriceRule struct {
	ID             int64
	FormulaVersion string
}

func (e BusinessExecutor) resolveWorkflowPrice(ctx context.Context, input map[string]any, spec createdReference) (float64, *workflowPriceRule, map[string]any, error) {
	mode := stringValue(input["pricing_mode"])
	if mode == "" {
		mode = "fixed"
	}
	if mode != "rule" {
		price := numberValue(input["price"])
		if price <= 0 {
			return 0, nil, nil, fmt.Errorf("固定售价必须大于零")
		}
		return price, nil, map[string]any{"source": "manual"}, nil
	}
	ruleID := positiveNumber(input["pricing_rule_id"])
	command := costingapp.PricingRuleTrialCommand{PricingRuleID: ruleID, ProductID: spec.ProductID, CustomerID: spec.OwnerID, BomID: spec.BOMID, BomVersionID: spec.VersionID, BomSpecID: spec.SpecID, BomVariantID: spec.VariantID, QuoteUnit: spec.Name}
	trial, err := e.costing.PricingRuleTrial(ctx, command)
	if err != nil {
		return 0, nil, nil, err
	}
	if trial.FinalUnitPrice <= 0 {
		return 0, nil, nil, fmt.Errorf("定价规则没有返回有效售价")
	}
	trialJSON, err := json.Marshal(trial)
	if err != nil {
		return 0, nil, nil, err
	}
	var snapshot map[string]any
	if err := json.Unmarshal(trialJSON, &snapshot); err != nil {
		return 0, nil, nil, err
	}
	snapshot["pricing_rule_trial_base_cost_details"] = snapshot["base_cost_details"]
	snapshot["pricing_rule_trial_warnings"] = snapshot["warnings"]
	rule := &workflowPriceRule{ID: trial.PricingRuleID, FormulaVersion: trial.FormulaVersion}
	return trial.FinalUnitPrice, rule, snapshot, nil
}

func referenceFromAny(value any) createdReference {
	row := resultMap(value)
	return createdReference{Type: stringValue(row["type"]), RowID: stringValue(row["row_id"]), ID: positiveNumber(row["id"]), Name: stringValue(row["name"]), Unit: stringValue(row["unit"]), OwnerID: positiveNumber(row["owner_customer_id"]), ProductID: positiveNumber(row["product_id"]), BOMID: positiveNumber(row["bom_id"]), VersionID: positiveNumber(row["version_id"]), SpecID: positiveNumber(row["bom_spec_id"]), VariantID: positiveNumber(row["bom_variant_id"]), Published: boolValue(row["published"])}
}

func cloneJSONMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var clone map[string]any
	if err := json.Unmarshal(body, &clone); err != nil {
		return map[string]any{}
	}
	return clone
}

func objectMap(value any) (map[string]any, bool) {
	row, ok := value.(map[string]any)
	return row, ok && len(row) > 0
}

func resolvePurchaseMaterial(run creatorapp.Run, node creatorapp.Node) (createdReference, error) {
	rowID := stringValue(run.Inputs[node.ID]["material_source_row_id"])
	for _, edge := range run.Workflow.Edges {
		if edge.Target != node.ID || edge.TargetHandle != "material" || edge.Kind != creatorapp.EdgeData {
			continue
		}
		objects, _ := run.BusinessResults["objects"].(map[string]any)
		for _, raw := range anySlice(objects[edge.Source]) {
			ref, _ := raw.(map[string]any)
			if stringValue(ref["type"]) == "material" && stringValue(ref["row_id"]) == rowID {
				return createdReference{Type: "material", RowID: rowID, ID: positiveNumber(ref["id"]), Name: stringValue(ref["name"]), Unit: stringValue(ref["unit"]), OwnerID: positiveNumber(ref["owner_customer_id"])}, nil
			}
		}
	}
	return createdReference{}, fmt.Errorf("采购物料引用已失效，请重新检查物料步骤")
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func (e BusinessExecutor) executeProduct(ctx context.Context, node creatorapp.Node, values map[string]any, actor string, refs map[string]map[string]createdReference) (map[string]any, error) {
	if stringValue(values["action"]) == "reuse" {
		id := positiveNumber(values["product_id"])
		var name string
		var ownerID int64
		var active bool
		if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(customer_id,0),COALESCE(active,true) FROM %s.products WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&name, &ownerID, &active); err != nil {
			return nil, fmt.Errorf("引用的商品不存在或当前不可见")
		}
		if !active || ownerID != positiveNumber(values["customer_id"]) {
			return nil, fmt.Errorf("所选商品已失效或归属客户不匹配，请重新选择")
		}
		refs[node.ID]["product"] = createdReference{Type: "product", RowID: "product", ID: id, Name: name, OwnerID: ownerID, ProductID: id}
		return map[string]any{"object": refs[node.ID]["product"]}, nil
	}
	owner := stringValue(values["owner"])
	if owner == "" {
		owner = "factory"
	}
	customerID := int64(0)
	if owner == "customer" {
		customerID = positiveNumber(values["customer_id"])
	}
	productKind := stringValue(values["product_kind"])
	if productKind == "" {
		productKind = "generic"
	}
	specialAttrs, err := productSpecialAttrs(values["industry_fields"])
	if err != nil {
		return nil, fmt.Errorf("行业字段格式不正确：%w", err)
	}
	product, err := e.catalog.CreateProduct(ctx, catalogapp.CreateProductCommand{
		Actor: actor, OwnershipType: owner, Name: stringValue(values["name"]), CustomerID: customerID,
		CustomerDisplayName: stringValue(values["name"]), ProductKind: productKind,
		SpecialAttrsJSON: specialAttrs, IsProcessingProduct: productKind == "generic",
	})
	if err != nil {
		return nil, err
	}
	if err := e.assignBusinessGroup(ctx, actor, "product_catalog", "product", product.ID, positiveNumber(values["classification_group_id"]), positiveNumber(values["classification_item_id"])); err != nil {
		return nil, err
	}
	refs[node.ID]["product"] = createdReference{Type: "product", RowID: "product", ID: product.ID, Name: product.Name, Code: product.SKUCode, OwnerID: customerID, ProductID: product.ID, New: true}
	return map[string]any{"object": refs[node.ID]["product"]}, nil
}

func (e BusinessExecutor) executeMaterials(ctx context.Context, node creatorapp.Node, values map[string]any, actor string, refs map[string]map[string]createdReference) (map[string]any, error) {
	rows := mapRows(values["rows"])
	created := make([]createdReference, 0, len(rows))
	for _, row := range rows {
		rowID := stringValue(row["row_id"])
		var material materialsapp.Material
		if stringValue(row["action"]) == "reuse" {
			id := positiveNumber(row["material_id"])
			var name, unit string
			var ownerID int64
			var deprecated *string
			if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(owner_customer_id,0),deprecated_at::text,COALESCE(NULLIF(unit,''),'unit') FROM %s.materials WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&name, &ownerID, &deprecated, &unit); err != nil {
				return nil, fmt.Errorf("引用的物料不存在或当前不可见")
			}
			if deprecated != nil || ownerID != positiveNumber(row["owner_customer_id"]) {
				return nil, fmt.Errorf("所选物料已失效或归属客户不匹配，请重新选择")
			}
			material = materialsapp.Material{ID: id, Name: name, OwnerCustomerID: ownerID, Unit: unit}
		} else {
			code, err := generatedMaterialCode()
			if err != nil {
				return nil, err
			}
			ownerType := stringValue(row["owner_type"])
			if ownerType == "" {
				ownerType = "factory"
			}
			supplyMode := stringValue(row["supply_mode"])
			if supplyMode == "" || supplyMode == "external" {
				supplyMode = "purchase"
			} else if supplyMode == "manufactured" {
				supplyMode = "manufacture"
			}
			material, err = e.materials.Create(ctx, materialsapp.CreateCommand{Actor: actor, OwnerType: ownerType, OwnerCustomerID: positiveNumber(row["owner_customer_id"]), Input: materialsapp.MaterialInput{
				Code: code, Name: stringValue(row["name"]), Kind: defaultString(stringValue(row["kind"]), "other"),
				IsSemiFinished: supplyMode == "manufacture", IsSemiFinishedSet: true, SupplyMode: supplyMode,
				Unit: stringValue(row["unit"]), CostUnit: stringValue(row["unit"]), IndustryFields: materialIndustryFields(row["industry_fields"]),
			}})
			if err != nil {
				return nil, err
			}
			if estimate, present := row["estimated_unit_price"]; present && estimate != nil && supplyMode == "purchase" {
				price := numberValue(estimate)
				if _, err := queryWithTransaction(ctx).Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET estimated_unit_price=$2,updated_at=now() WHERE id=$1`, e.schema), material.ID, price); err != nil {
					return nil, err
				}
				formatted := fmt.Sprintf("%.2f", price)
				if err := postgresinfra.AuditInsertTx(ctx, queryWithTransaction(ctx), e.schema, actor, "material", &material.ID, "set_estimated_purchase_price", postgresinfra.StrPtr("estimated_unit_price"), nil, postgresinfra.StrPtr(formatted), postgresinfra.AuditMeta{"material_id": material.ID, "unit": material.Unit, "temporary": true}); err != nil {
					return nil, err
				}
			}
			if err := e.assignBusinessGroup(ctx, actor, "material_catalog", "material", material.ID, positiveNumber(row["classification_group_id"]), positiveNumber(row["classification_item_id"])); err != nil {
				return nil, err
			}
		}
		ref := createdReference{Type: "material", RowID: rowID, ID: material.ID, Name: material.Name, Code: material.Code, Unit: material.Unit, OwnerID: material.OwnerCustomerID, New: stringValue(row["action"]) != "reuse"}
		refs[node.ID][rowID] = ref
		created = append(created, ref)
	}
	return map[string]any{"objects": created}, nil
}

func (e BusinessExecutor) assignBusinessGroup(ctx context.Context, actor, usageKey, objectKey string, objectID, groupID, groupItemID int64) error {
	if groupID == 0 && groupItemID == 0 {
		return nil
	}
	if groupID <= 0 || groupItemID <= 0 || objectID <= 0 {
		return fmt.Errorf("分类选择无效，请重新选择分类")
	}
	tx := queryWithTransaction(ctx)
	var groupOK, itemOK, usageOK bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_groups WHERE id=$1 AND active=true)`, e.schema), groupID).Scan(&groupOK); err != nil {
		return err
	}
	if !groupOK {
		return fmt.Errorf("所选分类已停用，请重新选择")
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_group_items WHERE id=$1 AND group_id=$2 AND active=true)`, e.schema), groupItemID, groupID).Scan(&itemOK); err != nil {
		return err
	}
	if !itemOK {
		return fmt.Errorf("所选分类项目已停用或不属于当前分类")
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.business_group_usages WHERE group_id=$1 AND lower(usage_key)=lower($2) AND active=true)`, e.schema), groupID, usageKey).Scan(&usageOK); err != nil {
		return err
	}
	if !usageOK {
		return fmt.Errorf("所选分类未启用于当前业务")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.business_group_assignments WHERE lower(usage_key)=lower($1) AND lower(object_key)=lower($2) AND object_id=$3 AND COALESCE(object_ref,'')=''`, e.schema), usageKey, objectKey, objectID); err != nil {
		return err
	}
	var assignmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.business_group_assignments(group_id,group_item_id,usage_key,object_key,object_id,object_ref,sort_order,created_by,updated_by)
		VALUES($1,$2,$3,$4,$5,'',100,$6,$6) RETURNING id
	`, e.schema), groupID, groupItemID, usageKey, objectKey, objectID, actor).Scan(&assignmentID); err != nil {
		return err
	}
	return postgresinfra.AuditInsertTx(ctx, tx, e.schema, actor, "business_group_assignment", &assignmentID, "assign_product_creator_classification", postgresinfra.StrPtr("group_item_id"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", groupItemID)), postgresinfra.AuditMeta{"group_id": groupID, "group_item_id": groupItemID, "usage_key": usageKey, "object_key": objectKey, "object_id": objectID, "template_run": true})
}

func (e BusinessExecutor) executeProcess(ctx context.Context, node creatorapp.Node, values map[string]any, refs map[string]map[string]createdReference) (map[string]any, error) {
	if id := positiveNumber(values["route_id"]); id > 0 {
		var status string
		if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.process_routes WHERE id=$1 FOR SHARE`, e.schema), id).Scan(&status); err != nil || status != "active" {
			return nil, fmt.Errorf("请选择有效工艺路线")
		}
		refs[node.ID]["route"] = createdReference{Type: "route", ID: id}
		return map[string]any{"route_id": id}, nil
	}
	return nil, fmt.Errorf("请选择有效的工艺路线")
}

func (e BusinessExecutor) executeBOM(ctx context.Context, run creatorapp.Run, node creatorapp.Node, values map[string]any, actor string, refs map[string]map[string]createdReference) (map[string]any, error) {
	if stringValue(values["action"]) == "reuse" {
		id := positiveNumber(values["bom_id"])
		var ref createdReference
		var specificationMode string
		var versionStatus string
		if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`
			SELECT b.output_type,CASE WHEN b.output_type='material' THEN b.output_material_id ELSE b.output_product_id END,
			       COALESCE(CASE WHEN b.output_type='material' THEN m.name ELSE p.name END,''),COALESCE(CASE WHEN b.output_type='material' THEN m.owner_customer_id ELSE p.customer_id END,0),
			       COALESCE(CASE WHEN b.output_type='material' THEN NULLIF(m.unit,'') ELSE '' END,'unit'),
			       b.specification_mode,v.id,v.status
			FROM %s.production_boms b
		JOIN LATERAL (SELECT id,status FROM %s.production_bom_versions WHERE bom_id=b.id AND status='published' ORDER BY published_at DESC NULLS LAST,id DESC LIMIT 1) v ON true
		LEFT JOIN %s.materials m ON b.output_type='material' AND m.id=b.output_material_id
		LEFT JOIN %s.products p ON b.output_type='product' AND p.id=b.output_product_id
		WHERE b.id=$1 AND COALESCE(b.status,'active')='active'`, e.schema, e.schema, e.schema, e.schema), id).Scan(&ref.Type, &ref.ID, &ref.Name, &ref.OwnerID, &ref.Unit, &specificationMode, &ref.VersionID, &versionStatus); err != nil {
			return nil, fmt.Errorf("引用的 BOM 没有有效的已发布版本")
		}
		if ref.Type != "product" && ref.Type != "material" || versionStatus != "published" {
			return nil, fmt.Errorf("只能引用有效的商品或自制物料 BOM")
		}
		selectedOutput, err := resolveBOMOutput(run, node, values, refs)
		if err != nil {
			return nil, err
		}
		if selectedOutput.Type != ref.Type || selectedOutput.ID != ref.ID || selectedOutput.OwnerID != ref.OwnerID {
			return nil, fmt.Errorf("所选 BOM 与产出对象或归属不一致，请选择该对象自己的 BOM")
		}
		outputType := ref.Type
		ref.Type, ref.BOMID, ref.Published = "bom", id, true
		refs[node.ID]["bom"] = ref
		output := createdReference{Type: outputType, RowID: "output", ID: ref.ID, Name: ref.Name, Unit: ref.Unit, OwnerID: ref.OwnerID, BOMID: id, VersionID: ref.VersionID, Published: true}
		if outputType == "product" {
			output.ProductID = ref.ID
		}
		refs[node.ID]["output"] = output
		return map[string]any{"bom_id": id, "version_id": ref.VersionID, "reused": true, "specification_mode": specificationMode}, nil
	}
	output, err := resolveBOMOutput(run, node, values, refs)
	if err != nil {
		return nil, err
	}
	outputType := output.Type
	components, err := resolveBOMComponents(run, node, values, refs)
	if err != nil {
		return nil, err
	}
	variants := make([]bomapp.ProductionBomDraftVariant, 0)
	for i, row := range mapRows(values["variants"]) {
		variant := bomapp.ProductionBomDraftVariant{SpecKey: stringValue(row["row_id"]), Name: stringValue(row["name"]), InventoryUnit: stringValue(row["unit"]), IsDefault: boolValue(row["is_default"]), SortOrder: i + 1}
		for _, component := range components {
			if component.variantRowID == "" || component.variantRowID == stringValue(row["row_id"]) {
				variant.Items = append(variant.Items, component.item)
			}
		}
		variants = append(variants, variant)
	}
	routeID := positiveNumber(values["route_id"])
	routeSelection := creatorapp.BOMProcessRoute{ID: int64(routeID), Source: "bom_default"}
	if run.Workflow.Version >= 5 {
		connectedRoutes := map[string]int64{}
		for _, edge := range run.Workflow.Edges {
			if edge.Kind == creatorapp.EdgeData && edge.Target == node.ID && edge.TargetHandle == "route" {
				if route, ok := refs[edge.Source]["route"]; ok {
					connectedRoutes[edge.Source] = route.ID
				}
			}
		}
		routeSelection = creatorapp.ResolveBOMProcessRoute(run.Workflow, node, values, connectedRoutes)
		routeID = routeSelection.ID
	} else {
		for _, edge := range run.Workflow.Edges {
			if edge.Target == node.ID && edge.TargetHandle == "route" {
				if route, ok := refs[edge.Source]["route"]; ok {
					routeID = route.ID
					routeSelection = creatorapp.BOMProcessRoute{ID: route.ID, Source: "connected_node", ProcessNodeID: edge.Source}
				}
			}
		}
	}
	for index := range variants {
		variants[index].ProcessRouteID = routeID
		variants[index].MaterialLossRate = numberValue(values["material_loss_rate"])
	}
	outputQty := numberValue(values["output_qty"])
	if outputQty <= 0 {
		outputQty = 1
	}
	outputMaterialLossRate := numberValue(values["material_loss_rate"])
	if outputMaterialLossRate < 0 || outputMaterialLossRate >= 1 {
		return nil, fmt.Errorf("物料损耗率必须大于等于 0 且小于 100%%")
	}
	name := strings.TrimSpace(stringValue(values["name"]))
	if name == "" {
		name = strings.TrimSpace(output.Name) + " BOM"
	}
	var summary bomapp.ProductionBomSummary
	outputProductID, outputMaterialID := int64(0), int64(0)
	if outputType == "product" {
		outputProductID = output.ID
	} else {
		outputMaterialID = output.ID
	}
	processRouteSource := ""
	processRouteNodeID := ""
	if run.Workflow.Version >= 5 {
		processRouteSource = routeSelection.Source
		processRouteNodeID = routeSelection.ProcessNodeID
	}
	if run.Workflow.Version >= 4 && outputType == "product" {
		templateVersionID := positiveNumber(node.Config["spec_template_version_id"])
		mainInput, inputErr := resolveProductBOMMainInput(run, node, values, refs)
		if inputErr != nil {
			return nil, inputErr
		}
		command := bomapp.CreateProductionBomCommand{
			Name: name, OutputType: "product", OutputID: output.ID, OutputProductID: output.ID,
			SpecificationMode: bomapp.ProductionBomSpecificationModeSpecGroup,
			OutputQty:         1, SpecTemplateVersionID: templateVersionID, MainInputComponent: mainInput, Actor: actor,
		}
		if run.Workflow.Version >= 5 {
			command.ProcessRouteSource = routeSelection.Source
			command.ProcessRouteNodeID = routeSelection.ProcessNodeID
			if routeSelection.ID > 0 && (routeSelection.Source == "run_override" || routeSelection.Source == "connected_node") {
				command.ProcessRouteOverrideID = routeSelection.ID
			}
		}
		summary, err = e.bom.CreateProductionBom(ctx, command)
	} else if stringValue(values["action"]) == "copy" {
		specificationMode := bomapp.ProductionBomSpecificationModeSingle
		if outputType == "product" {
			specificationMode = bomapp.ProductionBomSpecificationModeSpecGroup
		}
		summary, err = e.bom.CopyProductionBom(ctx, bomapp.CopyProductionBomCommand{ID: positiveNumber(values["bom_id"]), Name: name, OutputType: outputType, OutputID: output.ID, OutputProductID: outputProductID, OutputMaterialID: outputMaterialID, SpecificationMode: specificationMode, Actor: actor})
		if err == nil {
			_, err = e.bom.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: summary.LatestVersionID, MaterialLossRate: &outputMaterialLossRate, OutputQty: outputQty, OutputUnit: defaultString(stringValue(values["output_unit"]), outputUnit(output, variants)), ProcessRouteID: routeID, ProcessRouteSource: processRouteSource, ProcessRouteNodeID: processRouteNodeID, Variants: variants, Items: materialItems(components), Actor: actor})
		}
	} else {
		summary, err = e.bom.CreateProductionBom(ctx, bomapp.CreateProductionBomCommand{Name: name, OutputType: outputType, OutputID: output.ID, OutputProductID: outputProductID, OutputMaterialID: outputMaterialID, SpecificationMode: map[bool]string{true: bomapp.ProductionBomSpecificationModeSpecGroup, false: bomapp.ProductionBomSpecificationModeSingle}[outputType == "product"], OutputQty: outputQty, OutputUnit: defaultString(stringValue(values["output_unit"]), outputUnit(output, variants)), Variants: variants, Actor: actor})
		if err == nil {
			var draftVariants []bomapp.ProductionBomDraftVariant
			if outputType == "product" {
				draftVariants = variants
			}
			_, err = e.bom.UpdateProductionBomVersionDraft(ctx, bomapp.UpdateProductionBomVersionDraftCommand{VersionID: summary.LatestVersionID, MaterialLossRate: &outputMaterialLossRate, OutputQty: outputQty, OutputUnit: defaultString(stringValue(values["output_unit"]), outputUnit(output, variants)), ProcessRouteID: routeID, ProcessRouteSource: processRouteSource, ProcessRouteNodeID: processRouteNodeID, Variants: draftVariants, Items: materialItems(components), Actor: actor})
		}
	}
	if err != nil {
		return nil, err
	}
	if err := e.assignBusinessGroup(ctx, actor, "production_bom", "production_bom", summary.ID, positiveNumber(values["classification_group_id"]), positiveNumber(values["classification_item_id"])); err != nil {
		return nil, err
	}
	processRouteName := ""
	if routeSelection.ID > 0 {
		var status string
		if err := queryWithTransaction(ctx).QueryRow(ctx, fmt.Sprintf(`SELECT name,status FROM %s.process_routes WHERE id=$1 FOR SHARE`, e.schema), routeSelection.ID).Scan(&processRouteName, &status); err != nil || status != "active" {
			return nil, fmt.Errorf("所选工艺路线已失效，请重新预览")
		}
	}
	processRoute := map[string]any{"id": routeSelection.ID, "name": processRouteName, "source": routeSelection.Source, "process_node_id": routeSelection.ProcessNodeID}
	result := map[string]any{"bom_id": summary.ID, "bom_code": summary.Code, "version_id": summary.LatestVersionID, "output": output, "process_route": processRoute}
	if run.Workflow.Version >= 5 && outputType == "product" {
		rows, queryErr := queryWithTransaction(ctx).Query(ctx, fmt.Sprintf(`
			SELECT COALESCE(spec.spec_key,''),COALESCE(variant.process_route_id,0),COALESCE(route.name,'')
			FROM %s.production_bom_version_variants variant
			JOIN %s.production_bom_specs spec ON spec.id=variant.bom_spec_id
			LEFT JOIN %s.process_routes route ON route.id=variant.process_route_id
			WHERE variant.version_id=$1 ORDER BY variant.sort_order,variant.id
		`, e.schema, e.schema, e.schema), summary.LatestVersionID)
		if queryErr != nil {
			return nil, queryErr
		}
		effectiveRoutes := make([]map[string]any, 0)
		for rows.Next() {
			var specKey, routeName string
			var routeID int64
			if err := rows.Scan(&specKey, &routeID, &routeName); err != nil {
				rows.Close()
				return nil, err
			}
			effectiveRoutes = append(effectiveRoutes, map[string]any{"spec_key": specKey, "route_id": routeID, "route_name": routeName})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		result["effective_specification_routes"] = effectiveRoutes
	}
	ref := createdReference{Type: "bom", ID: summary.ID, Name: summary.Name, BOMID: summary.ID, VersionID: summary.LatestVersionID}
	refs[node.ID]["bom"] = ref
	outputRef := createdReference{Type: outputType, RowID: "output", ID: output.ID, Name: output.Name, Unit: defaultString(stringValue(values["output_unit"]), output.Unit), OwnerID: output.OwnerID, New: output.New, BOMID: summary.ID, VersionID: summary.LatestVersionID}
	if outputType == "product" {
		outputRef.ProductID = output.ID
	}
	refs[node.ID]["output"] = outputRef
	return result, nil
}

func resolveProductBOMMainInput(run creatorapp.Run, node creatorapp.Node, values map[string]any, refs map[string]map[string]createdReference) (bomapp.ProductionBomMainInputComponent, error) {
	sourceNodeID := stringValue(values["main_input_source_node_id"])
	sourceRowID := stringValue(values["main_input_source_row_id"])
	var source creatorapp.Edge
	for _, edge := range run.Workflow.Edges {
		if edge.Kind == creatorapp.EdgeData && edge.Target == node.ID && edge.TargetHandle == "components" && edge.Source == sourceNodeID {
			source = edge
			break
		}
	}
	if source.ID == "" || sourceRowID == "" {
		return bomapp.ProductionBomMainInputComponent{}, fmt.Errorf("请选择一个已连接的规格主体来源")
	}
	ref, ok := refs[sourceNodeID][sourceRowID]
	if !ok || ref.ID <= 0 {
		return bomapp.ProductionBomMainInputComponent{}, fmt.Errorf("所选规格主体已失效，请重新选择")
	}
	switch ref.Type {
	case "material":
		return bomapp.ProductionBomMainInputComponent{ComponentType: "material", MaterialID: ref.ID}, nil
	case "spec":
		if source.SourceHandle != "specs" || ref.ProductID <= 0 || ref.SpecID <= 0 || !ref.Published {
			return bomapp.ProductionBomMainInputComponent{}, fmt.Errorf("商品规格主体必须引用已发布的具体商品规格")
		}
		return bomapp.ProductionBomMainInputComponent{ComponentType: "product", ComponentProductID: ref.ProductID, ComponentBomSpecID: ref.SpecID}, nil
	default:
		return bomapp.ProductionBomMainInputComponent{}, fmt.Errorf("规格主体只支持物料或已发布商品规格")
	}
}

func (e BusinessExecutor) executePublish(ctx context.Context, run creatorapp.Run, node creatorapp.Node, values map[string]any, actor string, refs map[string]map[string]createdReference) (map[string]any, error) {
	var source creatorapp.Edge
	for _, edge := range run.Workflow.Edges {
		if edge.Target == node.ID && edge.TargetHandle == "bom" && edge.Kind == creatorapp.EdgeData {
			source = edge
			break
		}
	}
	bomRef, ok := refs[source.Source]["bom"]
	if !ok || bomRef.BOMID <= 0 {
		return nil, fmt.Errorf("BOM 来源步骤尚未生成有效档案")
	}
	if bomRef.Published && boolValue(values["set_default"]) {
		return nil, fmt.Errorf("引用的已有 BOM 是共享档案；本次不能修改它的默认绑定，请关闭“设为默认 BOM”")
	}
	if !bomRef.Published {
		publishCtx := ctx
		if !boolValue(values["set_default"]) {
			publishCtx = postgresinfra.WithoutAutomaticMaterialBomDefault(publishCtx)
		}
		if err := e.bom.PublishProductionBomVersion(publishCtx, bomapp.PublishProductionBomVersionCommand{VersionID: bomRef.VersionID, Actor: actor}); err != nil {
			return nil, err
		}
		if boolValue(values["set_default"]) {
			output := refs[source.Source]["output"]
			if _, err := e.bom.BindProductionBomOutput(ctx, bomapp.BindProductionBomOutputCommand{OutputType: output.Type, OutputID: output.ID, BomID: bomRef.BOMID, BomVersionID: bomRef.VersionID, Actor: actor}); err != nil {
				return nil, err
			}
		}
	}
	output := refs[source.Source]["output"]
	output.BOMID, output.VersionID, output.Published = bomRef.BOMID, bomRef.VersionID, true
	refs[node.ID]["output"] = output
	refs[node.ID]["published"] = bomRef
	rows, err := queryWithTransaction(ctx).Query(ctx, fmt.Sprintf(`
		SELECT spec.id,spec.spec_key,COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,''),variant.inventory_unit,variant.id
		FROM %s.production_bom_version_variants variant
		JOIN %s.production_bom_specs spec ON spec.id=variant.bom_spec_id
		WHERE variant.version_id=$1 ORDER BY variant.sort_order,variant.id`, e.schema, e.schema), bomRef.VersionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rowIDs := map[string]string{}
	for _, row := range mapRows(run.Inputs[source.Source]["variants"]) {
		rowIDs[stringValue(row["row_id"])] = stringValue(row["row_id"])
	}
	for rows.Next() {
		var id, variantID int64
		var specKey, name, unit string
		if err := rows.Scan(&id, &specKey, &name, &unit, &variantID); err != nil {
			return nil, err
		}
		rowID := rowIDs[specKey]
		if rowID == "" {
			rowID = specKey
		}
		refs[node.ID][rowID] = createdReference{Type: "spec", RowID: rowID, ID: id, Name: name, Unit: unit, OwnerID: output.OwnerID, ProductID: output.ID, BOMID: bomRef.BOMID, VersionID: bomRef.VersionID, SpecID: id, VariantID: variantID, Published: true}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"bom_id": bomRef.BOMID, "version_id": bomRef.VersionID, "default": boolValue(values["set_default"]), "specifications": refsForNode(refs[node.ID])}, nil
}

type resolvedComponent struct {
	item         bomapp.ProductionBomDraftItem
	variantRowID string
}

func resolveBOMOutput(run creatorapp.Run, node creatorapp.Node, values map[string]any, refs map[string]map[string]createdReference) (createdReference, error) {
	rowID := stringValue(values["output_source_row_id"])
	for _, edge := range run.Workflow.Edges {
		if edge.Target == node.ID && edge.TargetHandle == "output" && edge.Kind == creatorapp.EdgeData {
			if ref, ok := refs[edge.Source][rowID]; ok && (ref.Type == "product" || ref.Type == "material") {
				return ref, nil
			}
			if rowID == "output" {
				if ref, ok := refs[edge.Source]["output"]; ok && (ref.Type == "product" || ref.Type == "material") {
					return ref, nil
				}
			}
		}
	}
	return createdReference{}, fmt.Errorf("所选 BOM 产出对象已失效或不是商品/物料")
}

func resolveBOMComponents(run creatorapp.Run, node creatorapp.Node, values map[string]any, refs map[string]map[string]createdReference) ([]resolvedComponent, error) {
	components := make([]resolvedComponent, 0)
	for _, row := range mapRows(values["components"]) {
		rowID := stringValue(row["row_id"])
		sourceNode := stringValue(row["source_node_id"])
		sourceRow := stringValue(row["source_row_id"])
		var ref createdReference
		found := false
		for _, edge := range run.Workflow.Edges {
			if edge.Target == node.ID && edge.TargetHandle == "components" && edge.Source == sourceNode && edge.Kind == creatorapp.EdgeData {
				if value, ok := refs[edge.Source][sourceRow]; ok {
					ref, found = value, true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("BOM 组件 %s 的数据来源已失效", rowID)
		}
		item := bomapp.ProductionBomDraftItem{ConsumeUnit: stringValue(row["unit"]), QtyPerUnit: numberValue(row["quantity"]), MaterialLossRate: percentageFraction(row["loss_rate"])}
		if item.ConsumeUnit == "ratio_pct" {
			item.RatioPct = item.QtyPerUnit
			item.QtyPerUnit = 0
		}
		switch ref.Type {
		case "material":
			item.ComponentType, item.MaterialID = "material", ref.ID
		case "spec":
			item.ComponentType, item.ComponentProductID, item.ComponentBomSpecID = "product", ref.ProductID, ref.SpecID
		case "product":
			return nil, fmt.Errorf("商品组件必须连接已发布规格，不能只连接商品档案")
		default:
			return nil, fmt.Errorf("BOM 组件 %s 的来源类型不支持", rowID)
		}
		components = append(components, resolvedComponent{item: item, variantRowID: stringValue(row["variant_row_id"])})
	}
	return components, nil
}

func materialItems(components []resolvedComponent) []bomapp.ProductionBomDraftItem {
	items := make([]bomapp.ProductionBomDraftItem, 0, len(components))
	for _, component := range components {
		items = append(items, component.item)
	}
	return items
}

func outputUnit(output createdReference, variants []bomapp.ProductionBomDraftVariant) string {
	if output.Type == "material" {
		return defaultString(output.Unit, "unit")
	}
	for _, variant := range variants {
		if variant.IsDefault {
			return variant.InventoryUnit
		}
	}
	if len(variants) > 0 {
		return variants[0].InventoryUnit
	}
	return "unit"
}

func productSpecialAttrs(value any) (string, error) {
	switch current := value.(type) {
	case nil:
		return "{}", nil
	case map[string]any:
		data, err := json.Marshal(current)
		return string(data), err
	case string:
		trimmed := strings.TrimSpace(current)
		if trimmed == "" {
			return "{}", nil
		}
		var decoded map[string]any
		if json.Unmarshal([]byte(trimmed), &decoded) == nil {
			return trimmed, nil
		}
		decoded = map[string]any{}
		for _, line := range strings.Split(trimmed, "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
				decoded[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		data, err := json.Marshal(decoded)
		return string(data), err
	default:
		data, err := json.Marshal(current)
		return string(data), err
	}
}

func materialIndustryFields(value any) []materialsapp.MaterialIndustryFieldValue {
	fields := make([]materialsapp.MaterialIndustryFieldValue, 0)
	switch current := value.(type) {
	case map[string]any:
		for key, raw := range current {
			fields = append(fields, materialsapp.MaterialIndustryFieldValue{FieldKey: key, ValueText: fmt.Sprint(raw)})
		}
	case []any:
		for _, raw := range current {
			row, ok := raw.(map[string]any)
			if ok {
				fields = append(fields, materialsapp.MaterialIndustryFieldValue{FieldKey: stringValue(row["field_key"]), ValueText: stringValue(row["value_text"])})
			}
		}
	}
	return fields
}

func generatedMaterialCode() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return "PC-MAT-" + strings.ToUpper(hex.EncodeToString(data[:])), nil
}

func queryWithTransaction(ctx context.Context) pgx.Tx {
	tx, _ := postgresinfra.TransactionFromContext(ctx)
	return tx
}

func (e BusinessExecutor) readProcessRoute(ctx context.Context, id int64) (name, status string, err error) {
	query := fmt.Sprintf(`SELECT name,status FROM %s.process_routes WHERE id=$1`, e.schema)
	if tx := queryWithTransaction(ctx); tx != nil {
		err = tx.QueryRow(ctx, query, id).Scan(&name, &status)
		return name, status, err
	}
	if e.pool == nil {
		return "", "", fmt.Errorf("product creator process-route lookup requires a database pool or transaction")
	}
	err = e.pool.QueryRow(ctx, query, id).Scan(&name, &status)
	return name, status, err
}

func mapRows(value any) []map[string]any {
	rows := make([]map[string]any, 0)
	switch current := value.(type) {
	case []any:
		for _, value := range current {
			if row, ok := value.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
	case []map[string]any:
		rows = append(rows, current...)
	}
	return rows
}

func numberValue(value any) float64 {
	switch current := value.(type) {
	case float64:
		return current
	case float32:
		return float64(current)
	case int:
		return float64(current)
	case int64:
		return float64(current)
	case json.Number:
		parsed, _ := current.Float64()
		return parsed
	case string:
		var parsed float64
		_, _ = fmt.Sscan(current, &parsed)
		return parsed
	default:
		return 0
	}
}

func positiveNumber(value any) int64 {
	parsed := numberValue(value)
	if parsed <= 0 || parsed != float64(int64(parsed)) {
		return 0
	}
	return int64(parsed)
}

func percentageFraction(value any) float64 {
	return numberValue(value) / 100
}

func boolValue(value any) bool {
	current, _ := value.(bool)
	return current
}

func stringValue(value any) string {
	current, _ := value.(string)
	return strings.TrimSpace(current)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func allReferences(refs map[string]map[string]createdReference) map[string]any {
	result := make(map[string]any, len(refs))
	for nodeID, nodeRefs := range refs {
		result[nodeID] = refsForNode(nodeRefs)
	}
	return result
}

func refsForNode(refs map[string]createdReference) []createdReference {
	result := make([]createdReference, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref)
	}
	return result
}
