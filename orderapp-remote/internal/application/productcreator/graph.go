package productcreator

import (
	"fmt"
	"sort"
	"strings"
)

type ModuleKind string

const (
	ModuleProduct  ModuleKind = "product"
	ModuleMaterial ModuleKind = "material"
	ModuleBOM      ModuleKind = "bom"
	ModuleProcess  ModuleKind = "process"
	ModulePublish  ModuleKind = "publish"
	ModulePurchase ModuleKind = "purchase"
	ModulePricing  ModuleKind = "pricing"
)

type EdgeKind string

const (
	EdgeData         EdgeKind = "data"
	EdgePrerequisite EdgeKind = "prerequisite"
)

type Workflow struct {
	Version   int                `json:"version,omitempty"`
	Variables []WorkflowVariable `json:"variables,omitempty"`
	Nodes     []Node             `json:"nodes"`
	Edges     []Edge             `json:"edges"`
}

type WorkflowVariable struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DefaultValue string `json:"default_value,omitempty"`
}

type Node struct {
	ID        string         `json:"id"`
	Kind      ModuleKind     `json:"kind"`
	Name      string         `json:"name,omitempty"`
	X         float64        `json:"x,omitempty"`
	Y         float64        `json:"y,omitempty"`
	Config    map[string]any `json:"config,omitempty"`
	Condition *Condition     `json:"condition,omitempty"`
}

type Condition struct {
	NodeID   string `json:"node_id"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type Edge struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"`
	SourceHandle string   `json:"source_handle,omitempty"`
	Target       string   `json:"target"`
	TargetHandle string   `json:"target_handle,omitempty"`
	Kind         EdgeKind `json:"kind"`
	Label        string   `json:"label,omitempty"`
}

type Port struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Types    []string `json:"types"`
	Required bool     `json:"required,omitempty"`
	Multiple bool     `json:"multiple,omitempty"`
}

type Field struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	SourceMode  string `json:"source_mode"`
	Description string `json:"description,omitempty"`
}

type Module struct {
	Kind            ModuleKind `json:"kind"`
	Name            string     `json:"name"`
	Category        string     `json:"category"`
	WorkflowVersion int        `json:"workflow_version,omitempty"`
	PaletteVisible  bool       `json:"palette_visible"`
	Description     string     `json:"description"`
	Inputs          []Port     `json:"inputs"`
	Outputs         []Port     `json:"outputs"`
	Fields          []Field    `json:"fields"`
	Actions         []string   `json:"actions"`
}

type ValidationIssue struct {
	NodeID     string `json:"node_id,omitempty"`
	EdgeID     string `json:"edge_id,omitempty"`
	VariableID string `json:"variable_id,omitempty"`
	Field      string `json:"field,omitempty"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

var modules = []Module{
	{Kind: ModuleProduct, Name: "商品档案", Category: "基础档案", Description: "新建商品或引用已有商品", Inputs: nil, Outputs: []Port{{ID: "product", Label: "商品档案", Types: []string{"product.ref"}}}, Fields: []Field{{Key: "name", Label: "商品名称", Type: "text", Required: true, SourceMode: "input"}, {Key: "action", Label: "处理方式", Type: "choice", Required: true, SourceMode: "input"}, {Key: "product_kind", Label: "商品类型", Type: "choice", SourceMode: "input"}, {Key: "owner", Label: "商品归属", Type: "owner", Required: true, SourceMode: "input"}, {Key: "customer_id", Label: "归属客户", Type: "reference", SourceMode: "input"}, {Key: "industry_fields", Label: "行业字段", Type: "record", SourceMode: "input"}}, Actions: []string{"新建商品", "引用已有"}},
	{Kind: ModuleMaterial, Name: "物料档案", Category: "基础档案", Description: "新建、复用原料或包材，支持重复行", Inputs: nil, Outputs: []Port{{ID: "material", Label: "物料档案", Types: []string{"material.ref"}}}, Fields: []Field{{Key: "rows", Label: "物料行", Type: "repeater", Required: true, SourceMode: "input"}}, Actions: []string{"新建物料", "引用已有"}},
	{Kind: ModuleBOM, Name: "BOM 与规格", Category: "生产配置", Description: "为商品或自制物料建立多规格 BOM", Inputs: []Port{{ID: "output", Label: "产出对象", Types: []string{"product.ref", "material.ref"}, Required: true}, {ID: "components", Label: "主料 / 包材", Types: []string{"material.ref", "item.specs"}, Multiple: true}, {ID: "route", Label: "工艺路线", Types: []string{"process.route"}}}, Outputs: []Port{{ID: "bom", Label: "BOM 草稿", Types: []string{"bom.draft"}}, {ID: "specs", Label: "未发布规格", Types: []string{"item.specs.draft"}}, {ID: "output", Label: "产出对象", Types: []string{"product.ref", "material.ref"}}}, Fields: []Field{{Key: "action", Label: "处理方式", Type: "choice", Required: true, SourceMode: "input"}, {Key: "output_source_row_id", Label: "本次产出对象", Type: "reference", Required: true, SourceMode: "reference"}, {Key: "bom_id", Label: "已有 BOM", Type: "reference", SourceMode: "input"}, {Key: "variants", Label: "规格", Type: "repeater", Required: true, SourceMode: "input"}, {Key: "components", Label: "BOM 用量", Type: "repeater", Required: true, SourceMode: "reference"}}, Actions: []string{"新建 BOM", "复制为新 BOM", "引用已有"}},
	{Kind: ModuleProcess, Name: "工艺配置", Category: "生产配置", Description: "选择现有有效工艺路线", Inputs: nil, Outputs: []Port{{ID: "route", Label: "工艺路线", Types: []string{"process.route"}}}, Fields: []Field{{Key: "route_id", Label: "工艺路线", Type: "reference", Required: true, SourceMode: "input"}}, Actions: []string{"选择现有路线"}},
	{Kind: ModulePublish, Name: "发布与默认绑定", Category: "生产配置", Description: "发布新建 BOM 并绑定到本次产出对象", Inputs: []Port{{ID: "bom", Label: "待发布 BOM", Types: []string{"bom.draft"}, Required: true}}, Outputs: []Port{{ID: "published", Label: "已发布 BOM", Types: []string{"bom.published"}}, {ID: "specs", Label: "有效规格", Types: []string{"item.specs"}}, {ID: "output", Label: "已绑定产出对象", Types: []string{"product.ref", "material.ref"}}}, Fields: []Field{{Key: "set_default", Label: "设为默认 BOM", Type: "boolean", Required: true, SourceMode: "input"}}, Actions: []string{"发布 BOM", "设置默认"}},
	{Kind: ModulePurchase, Name: "物料购入", Category: "后续业务", Description: "创建采购单；到货后另行确认收货", Inputs: []Port{{ID: "material", Label: "采购物料", Types: []string{"material.ref"}, Required: true}}, Outputs: []Port{{ID: "purchase", Label: "采购单", Types: []string{"purchase.order"}}, {ID: "receipt", Label: "入库批次", Types: []string{"stock.receipt"}}}, Fields: []Field{{Key: "material_source_row_id", Label: "采购物料", Type: "reference", Required: true, SourceMode: "reference"}, {Key: "supplier_id", Label: "供应商", Type: "reference", Required: true, SourceMode: "input"}, {Key: "quantity", Label: "预计采购量", Type: "quantity", Required: true, SourceMode: "input"}, {Key: "warehouse", Label: "目标仓库", Type: "reference", Required: true, SourceMode: "input"}, {Key: "unit_price", Label: "采购单价", Type: "money", Required: true, SourceMode: "input"}}, Actions: []string{"创建采购单", "确认收货"}},
	{Kind: ModulePricing, Name: "价格配置", Category: "后续业务", Description: "按商品规格试算售价，另存价格表草稿并明确发布", Inputs: []Port{{ID: "product", Label: "已发布商品规格", Types: []string{"item.specs"}, Required: true}, {ID: "receipt", Label: "采购收货成本", Types: []string{"stock.receipt"}}}, Outputs: []Port{{ID: "draft", Label: "价格草稿", Types: []string{"price.draft"}}, {ID: "published", Label: "价格版本", Types: []string{"price.published"}}}, Fields: []Field{{Key: "price_list_id", Label: "复制到价格表", Type: "reference", Required: true, SourceMode: "input"}, {Key: "prices", Label: "规格定价", Type: "repeater", Required: true, SourceMode: "input"}}, Actions: []string{"试算", "保存草稿", "发布价格"}},
}

var moduleByKind = func() map[ModuleKind]Module {
	byKind := make(map[ModuleKind]Module, len(modules))
	for _, module := range modules {
		byKind[module.Kind] = module
	}
	return byKind
}()

func ModuleCatalog() []Module {
	out := make([]Module, 0, len(modules)+10)
	for _, module := range modules {
		module.PaletteVisible = false
		module.WorkflowVersion = 1
		out = append(out, module)
	}
	out = append(out, bomCentricModules()...)
	out = append(out, variableWorkflowModules()...)
	return out
}

func workflowVersion(workflow Workflow) int {
	if workflow.Version >= 2 {
		return workflow.Version
	}
	return 1 // Missing versions are the original workflow format.
}

func bomCentricModules() []Module {
	return []Module{
		{Kind: ModuleMaterial, Name: "物料", Category: "数据类型", WorkflowVersion: 2, PaletteVisible: true, Description: "选择配方物料，或接收 BOM 生成的半成品", Inputs: []Port{{ID: "from_bom", Label: "BOM产出", Types: []string{"bom.output"}, Required: true}}, Outputs: []Port{{ID: "material", Label: "物料对象", Types: []string{"material.ref"}}}, Fields: []Field{{Key: "data_role", Label: "物料用途", Type: "choice", Required: true, SourceMode: "template"}, {Key: "rows", Label: "配方物料", Type: "repeater", SourceMode: "input"}, {Key: "name", Label: "产出物料名称", Type: "text", SourceMode: "input"}, {Key: "unit", Label: "库存单位", Type: "unit", SourceMode: "template"}, {Key: "kind", Label: "物料类型", Type: "choice", SourceMode: "template"}, {Key: "supply_mode", Label: "外购或自制", Type: "choice", SourceMode: "template"}}, Actions: []string{"选择物料", "自动生成物料"}},
		{Kind: ModuleProduct, Name: "商品", Category: "数据类型", WorkflowVersion: 2, PaletteVisible: true, Description: "创建成品或引用已有商品", Inputs: []Port{{ID: "from_bom", Label: "BOM产出", Types: []string{"bom.output"}, Required: true}}, Outputs: []Port{{ID: "product", Label: "商品对象", Types: []string{"product.ref"}}, {ID: "specs", Label: "商品规格", Types: []string{"item.specs"}}}, Fields: []Field{{Key: "data_role", Label: "商品用途", Type: "choice", Required: true, SourceMode: "template"}, {Key: "name", Label: "商品名称", Type: "text", Required: true, SourceMode: "input"}, {Key: "action", Label: "处理方式", Type: "choice", Required: true, SourceMode: "template"}, {Key: "product_kind", Label: "商品类型", Type: "choice", SourceMode: "template"}, {Key: "owner", Label: "商品归属", Type: "owner", Required: true, SourceMode: "input"}, {Key: "customer_id", Label: "归属客户", Type: "reference", SourceMode: "input"}, {Key: "industry_fields", Label: "行业字段", Type: "record", SourceMode: "input"}}, Actions: []string{"自动生成商品", "引用已有商品"}},
		{Kind: ModuleProcess, Name: "工艺", Category: "数据类型", WorkflowVersion: 2, PaletteVisible: true, Description: "选择可供 BOM 使用的有效工艺路线", Outputs: []Port{{ID: "route", Label: "工艺路线", Types: []string{"process.route"}}}, Fields: []Field{{Key: "route_id", Label: "工艺路线", Type: "reference", Required: true, SourceMode: "template"}}, Actions: []string{"选择有效工艺"}},
		{Kind: ModuleBOM, Name: "BOM组装", Category: "动作", WorkflowVersion: 2, PaletteVisible: true, Description: "按模板默认值组装配方、规格、工艺和损耗，并发布绑定", Inputs: []Port{{ID: "components", Label: "配方物料或商品规格", Types: []string{"material.ref", "item.specs"}, Required: true, Multiple: true}, {ID: "route", Label: "工艺路线", Types: []string{"process.route"}}}, Outputs: []Port{{ID: "assembly", Label: "BOM产出", Types: []string{"bom.output"}}}, Fields: []Field{{Key: "name", Label: "BOM名称", Type: "text", SourceMode: "template"}, {Key: "output_type", Label: "产出类型", Type: "choice", Required: true, SourceMode: "template"}, {Key: "output_qty", Label: "产出数量", Type: "quantity", Required: true, SourceMode: "template"}, {Key: "output_unit", Label: "产出单位", Type: "unit", Required: true, SourceMode: "template"}, {Key: "route_id", Label: "默认工艺路线", Type: "reference", SourceMode: "template"}, {Key: "material_loss_rate", Label: "物料损耗率", Type: "percentage", SourceMode: "template"}, {Key: "variants", Label: "商品规格", Type: "repeater", SourceMode: "template"}, {Key: "components", Label: "配方用量", Type: "repeater", Required: true, SourceMode: "reference"}}, Actions: []string{"BOM组装并发布"}},
		{Kind: ModulePurchase, Name: "物料购入", Category: "动作", WorkflowVersion: 2, PaletteVisible: true, Description: "创建采购单；到货后由用户确认收货", Inputs: []Port{{ID: "material", Label: "采购物料", Types: []string{"material.ref"}, Required: true, Multiple: true}}, Outputs: []Port{{ID: "purchase", Label: "采购单", Types: []string{"purchase.order"}}}, Fields: []Field{{Key: "supplier_id", Label: "供应商", Type: "reference", Required: true, SourceMode: "input"}, {Key: "quantity", Label: "预计采购量", Type: "quantity", Required: true, SourceMode: "input"}, {Key: "warehouse", Label: "目标仓库", Type: "reference", Required: true, SourceMode: "input"}, {Key: "unit_price", Label: "采购单价", Type: "money", Required: true, SourceMode: "input"}}, Actions: []string{"创建采购单", "确认收货"}},
	}
}

func variableWorkflowModules() []Module {
	modules := bomCentricModules()
	for index := range modules {
		module := &modules[index]
		module.WorkflowVersion = 3
		for fieldIndex := 0; fieldIndex < len(module.Fields); {
			key := module.Fields[fieldIndex].Key
			if (module.Kind == ModuleMaterial && key == "kind") || (module.Kind == ModuleProduct && key == "product_kind") {
				module.Fields = append(module.Fields[:fieldIndex], module.Fields[fieldIndex+1:]...)
				continue
			}
			fieldIndex++
		}
	}
	return modules
}

func moduleForNode(node Node, version int) Module {
	if version < 2 {
		return moduleByKind[node.Kind]
	}
	definitions := bomCentricModules()
	if version >= 3 {
		definitions = variableWorkflowModules()
	}
	for _, module := range definitions {
		if module.Kind != node.Kind {
			continue
		}
		if node.Kind == ModuleMaterial || node.Kind == ModuleProduct {
			if stringValue(node.Config["data_role"]) == "output" {
				module.Inputs = []Port{{ID: "from_bom", Label: "BOM产出", Types: []string{"bom.output"}, Required: true}}
			} else {
				module.Inputs = nil
			}
		}
		return module
	}
	return Module{}
}

func ValidateWorkflow(workflow Workflow) []ValidationIssue {
	if workflowVersion(workflow) >= 2 {
		issues := validateBOMWorkflow(workflow)
		if workflow.Version >= 3 {
			issues = append(issues, ValidateWorkflowVariableDefinitions(workflow)...)
		}
		return issues
	}
	issues := make([]ValidationIssue, 0)
	if len(workflow.Nodes) == 0 {
		issues = append(issues, ValidationIssue{Code: "empty_workflow", Message: "模板至少需要一个业务步骤"})
	}
	nodes := make(map[string]Node, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			issues = append(issues, ValidationIssue{Code: "missing_node_id", Message: "每个节点必须有稳定的节点标识"})
			continue
		}
		if _, exists := nodes[id]; exists {
			issues = append(issues, ValidationIssue{NodeID: id, Code: "duplicate_node_id", Message: "节点标识重复"})
			continue
		}
		nodes[id] = node
		if _, ok := moduleByKind[node.Kind]; !ok {
			issues = append(issues, ValidationIssue{NodeID: id, Code: "unknown_module", Message: "模板包含未注册的业务模块"})
		}
		if condition := node.Condition; condition != nil {
			if strings.TrimSpace(condition.NodeID) == "" || strings.TrimSpace(condition.Field) == "" {
				issues = append(issues, ValidationIssue{NodeID: id, Field: "condition", Code: "invalid_condition", Message: "执行条件必须引用一个步骤字段"})
			}
			if condition.Operator != "equals" && condition.Operator != "not_equals" && condition.Operator != "empty" && condition.Operator != "not_empty" {
				issues = append(issues, ValidationIssue{NodeID: id, Field: "condition.operator", Code: "invalid_condition_operator", Message: "条件运算符不受支持"})
			}
		}
	}

	edgeIDs := make(map[string]struct{}, len(workflow.Edges))
	connectedInputs := make(map[string]int, len(workflow.Edges))
	for _, edge := range workflow.Edges {
		if id := strings.TrimSpace(edge.ID); id != "" {
			if _, exists := edgeIDs[id]; exists {
				issues = append(issues, ValidationIssue{EdgeID: id, Code: "duplicate_edge_id", Message: "连线标识重复"})
			}
			edgeIDs[id] = struct{}{}
		}
		source, sourceOK := nodes[edge.Source]
		target, targetOK := nodes[edge.Target]
		if !sourceOK || !targetOK {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "missing_node", Message: "连线引用的节点不存在"})
			continue
		}
		if edge.Source == edge.Target {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "cycle", Message: "流程不能连接到自身"})
			continue
		}
		if edge.Kind == EdgePrerequisite {
			continue
		}
		if edge.Kind != EdgeData {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "unknown_edge_kind", Message: "连线类型不受支持"})
			continue
		}
		outPort, okOut := findPort(moduleByKind[source.Kind].Outputs, edge.SourceHandle)
		inPort, okIn := findPort(moduleByKind[target.Kind].Inputs, edge.TargetHandle)
		if !okOut || !okIn {
			issues = append(issues, ValidationIssue{NodeID: target.ID, EdgeID: edge.ID, Code: "unknown_port", Message: "连线端口不存在"})
			continue
		}
		if !portsCompatible(outPort, inPort) {
			issues = append(issues, ValidationIssue{NodeID: target.ID, EdgeID: edge.ID, Code: "incompatible_data_type", Message: fmt.Sprintf("%s 不能作为 %s 的数据来源", outPort.Label, inPort.Label)})
			continue
		}
		connectedInputs[target.ID+"\x00"+inPort.ID]++
	}
	for _, node := range workflow.Nodes {
		module, ok := moduleByKind[node.Kind]
		if !ok {
			continue
		}
		for _, port := range module.Inputs {
			if port.Required && connectedInputs[node.ID+"\x00"+port.ID] == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: port.ID, Code: "missing_data_source", Message: port.Label + "需要连接一个数据来源"})
			}
		}
	}

	for _, node := range workflow.Nodes {
		if node.Condition == nil {
			continue
		}
		if _, ok := nodes[node.Condition.NodeID]; !ok {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "condition.node_id", Code: "missing_condition_source", Message: "执行条件引用的步骤不存在"})
		} else if node.Condition.NodeID == node.ID {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "condition.node_id", Code: "cycle", Message: "步骤不能依赖自身的执行条件"})
		} else if source, ok := nodes[node.Condition.NodeID]; ok {
			validField := false
			if sourceModule, ok := moduleByKind[source.Kind]; ok {
				for _, field := range sourceModule.Fields {
					if field.Key == node.Condition.Field {
						validField = true
						break
					}
				}
			}
			if !validField {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "condition.field", Code: "missing_condition_field", Message: "执行条件字段不属于所选前置步骤"})
			}
		}
	}
	if _, err := topologicalOrder(workflow); err != nil && !hasValidationCode(issues, "cycle") {
		issues = append(issues, ValidationIssue{Code: "cycle", Message: "流程依赖中存在循环"})
	}
	return issues
}

func ValidateWorkflowVariableDefinitions(workflow Workflow) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	variables := make(map[string]WorkflowVariable, len(workflow.Variables))
	names := make(map[string]string, len(workflow.Variables))
	for _, variable := range workflow.Variables {
		id := strings.TrimSpace(variable.ID)
		name := strings.TrimSpace(variable.Name)
		if id == "" || name == "" {
			issues = append(issues, ValidationIssue{VariableID: id, Code: "invalid_workflow_variable", Message: "变量需要稳定标识和名称"})
			continue
		}
		if _, exists := variables[id]; exists {
			issues = append(issues, ValidationIssue{VariableID: id, Code: "duplicate_variable_id", Message: "变量标识重复"})
			continue
		}
		key := strings.ToLower(name)
		if existingID, exists := names[key]; exists && existingID != id {
			issues = append(issues, ValidationIssue{VariableID: id, Code: "duplicate_variable_name", Message: "变量名称重复，请复用已有变量"})
			continue
		}
		variables[id] = variable
		names[key] = id
	}
	for _, node := range workflow.Nodes {
		parts := rowValues(node.Config["name_parts"])
		if len(parts) > 0 && ((node.Kind != ModuleMaterial && node.Kind != ModuleProduct) || stringValue(node.Config["data_role"]) != "output") {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "name_parts", Code: "invalid_name_target", Message: "命名变量只能用于新建物料或商品名称"})
		}
		for _, part := range parts {
			switch stringValue(part["type"]) {
			case "text":
			case "variable":
				id := strings.TrimSpace(stringValue(part["variable_id"]))
				if _, exists := variables[id]; !exists {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "name_parts", VariableID: id, Code: "unknown_name_variable", Message: "命名引用的变量不存在，请重新选择"})
				}
			default:
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "name_parts", Code: "invalid_name_part", Message: "名称只能由文字和变量组成"})
			}
		}
	}
	return issues
}

func validateBOMWorkflow(workflow Workflow) []ValidationIssue {
	version := workflowVersion(workflow)
	issues := make([]ValidationIssue, 0)
	if len(workflow.Nodes) == 0 {
		return []ValidationIssue{{Code: "empty_workflow", Message: "模板至少需要一个业务步骤"}}
	}
	nodes := make(map[string]Node, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			issues = append(issues, ValidationIssue{Code: "missing_node_id", Message: "每个节点必须有稳定的节点标识"})
			continue
		}
		if _, exists := nodes[id]; exists {
			issues = append(issues, ValidationIssue{NodeID: id, Code: "duplicate_node_id", Message: "节点标识重复"})
			continue
		}
		nodes[id] = node
		if node.Condition != nil {
			issues = append(issues, ValidationIssue{NodeID: id, Field: "condition", Code: "conditions_disabled", Message: "新版模板暂不支持执行条件"})
		}
		if moduleForNode(node, version).Kind == "" {
			issues = append(issues, ValidationIssue{NodeID: id, Code: "module_not_available", Message: "该模块不属于新版模板的数据类型或动作"})
		}
		if (node.Kind == ModuleMaterial || node.Kind == ModuleProduct) && stringValue(node.Config["data_role"]) != "input" && stringValue(node.Config["data_role"]) != "output" {
			issues = append(issues, ValidationIssue{NodeID: id, Field: "data_role", Code: "invalid_data_role", Message: "请选择配方输入或 BOM 产出对象"})
		}
		if node.Kind == ModuleBOM {
			if output := stringValue(node.Config["output_type"]); output != "material" && output != "product" {
				issues = append(issues, ValidationIssue{NodeID: id, Field: "output_type", Code: "invalid_output_type", Message: "请选择物料或商品作为 BOM 产出类型"})
			}
		}
		if node.Kind == ModuleProcess && positiveNumber(node.Config["route_id"]) == 0 {
			issues = append(issues, ValidationIssue{NodeID: id, Field: "route_id", Code: "route_required", Message: "工艺节点需要在模板中选择一条有效路线"})
		}
	}

	connectedInputs := make(map[string]int)
	outputTargets := make(map[string][]Node)
	edgeIDs := make(map[string]struct{}, len(workflow.Edges))
	recipeSources := make(map[string]map[string]struct{})
	for _, edge := range workflow.Edges {
		if version >= 3 {
			id := strings.TrimSpace(edge.ID)
			if id == "" {
				issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "missing_edge_id", Message: "V3 连线需要稳定标识，请重新连接后保存"})
			} else if _, exists := edgeIDs[id]; exists {
				issues = append(issues, ValidationIssue{EdgeID: id, Code: "duplicate_edge_id", Message: "连线标识重复"})
			} else {
				edgeIDs[id] = struct{}{}
			}
		}
		source, sourceOK := nodes[edge.Source]
		target, targetOK := nodes[edge.Target]
		if !sourceOK || !targetOK {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "missing_node", Message: "连线引用的节点不存在"})
			continue
		}
		if edge.Source == edge.Target {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "cycle", Message: "流程不能连接到自身"})
			continue
		}
		if edge.Kind != EdgeData {
			issues = append(issues, ValidationIssue{EdgeID: edge.ID, Code: "edge_kind_disabled", Message: "新版模板只使用传递业务数据的连线"})
			continue
		}
		outPort, okOut := findPort(moduleForNode(source, version).Outputs, edge.SourceHandle)
		inPort, okIn := findPort(moduleForNode(target, version).Inputs, edge.TargetHandle)
		if !okOut || !okIn {
			issues = append(issues, ValidationIssue{NodeID: target.ID, EdgeID: edge.ID, Code: "unknown_port", Message: "连线端口不存在"})
			continue
		}
		if !portsCompatible(outPort, inPort) {
			issues = append(issues, ValidationIssue{NodeID: target.ID, EdgeID: edge.ID, Code: "incompatible_data_type", Message: fmt.Sprintf("%s 不能作为 %s 的数据来源", outPort.Label, inPort.Label)})
			continue
		}
		connectedInputs[target.ID+"\x00"+inPort.ID]++
		if version >= 3 && target.Kind == ModuleBOM && inPort.ID == "components" {
			if recipeSources[target.ID] == nil {
				recipeSources[target.ID] = map[string]struct{}{}
			}
			if _, exists := recipeSources[target.ID][source.ID]; exists {
				issues = append(issues, ValidationIssue{NodeID: target.ID, EdgeID: edge.ID, Field: "components", Code: "duplicate_recipe_source", Message: "同一配方来源不能重复连接；需要多个配方行时在填写表格中增加行"})
			} else {
				recipeSources[target.ID][source.ID] = struct{}{}
			}
		}
		if source.Kind == ModuleBOM && edge.SourceHandle == "assembly" && (target.Kind == ModuleMaterial || target.Kind == ModuleProduct) {
			outputTargets[source.ID] = append(outputTargets[source.ID], target)
		}
	}

	for _, node := range workflow.Nodes {
		module := moduleForNode(node, version)
		for _, port := range module.Inputs {
			if port.Required && connectedInputs[node.ID+"\x00"+port.ID] == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: port.ID, Code: "missing_data_source", Message: port.Label + "需要连接一个数据来源"})
			}
		}
		if node.Kind == ModuleBOM {
			if connectedInputs[node.ID+"\x00components"] == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components", Code: "missing_data_source", Message: "BOM组装至少需要连接一个配方物料或商品规格"})
			}
			if len(outputTargets[node.ID]) != 1 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "assembly", Code: "bom_output_required", Message: "BOM组装需要连接一个产出物料或商品"})
			} else if stringValue(node.Config["output_type"]) != string(outputTargets[node.ID][0].Kind) {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "output_type", Code: "output_type_mismatch", Message: "BOM产出类型必须与连接的物料或商品一致"})
			}
			if connectedInputs[node.ID+"\x00route"] == 0 && positiveNumber(node.Config["route_id"]) == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "route", Code: "route_required", Message: "请连接工艺路线或在模板中选择默认路线"})
			}
			if connectedInputs[node.ID+"\x00route"] > 1 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "route", Code: "multiple_routes", Message: "一个 BOM 只能连接一条工艺路线"})
			}
		}
		if (node.Kind == ModuleMaterial || node.Kind == ModuleProduct) && stringValue(node.Config["data_role"]) == "output" && connectedInputs[node.ID+"\x00from_bom"] != 1 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "from_bom", Code: "output_bom_required", Message: "产出对象需要且只能连接一个 BOM组装"})
		}
	}
	if _, err := topologicalOrder(workflow); err != nil && !hasValidationCode(issues, "cycle") {
		issues = append(issues, ValidationIssue{Code: "cycle", Message: "流程依赖中存在循环"})
	}
	return issues
}

func TopologicalOrder(workflow Workflow) ([]string, error) {
	if issues := ValidateWorkflow(workflow); len(issues) > 0 {
		for _, issue := range issues {
			if issue.Code == "cycle" {
				return nil, fmt.Errorf("cycle in workflow dependencies")
			}
		}
		return nil, fmt.Errorf("invalid workflow: %s", issues[0].Code)
	}
	return topologicalOrder(workflow)
}

func topologicalOrder(workflow Workflow) ([]string, error) {
	nodes := make(map[string]struct{}, len(workflow.Nodes))
	incoming := make(map[string]int, len(workflow.Nodes))
	outgoing := make(map[string][]string, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		if node.ID == "" {
			continue
		}
		nodes[node.ID] = struct{}{}
		incoming[node.ID] = 0
	}
	seen := make(map[string]struct{})
	addDependency := func(from, to string) {
		if _, okFrom := nodes[from]; !okFrom {
			return
		}
		if _, okTo := nodes[to]; !okTo {
			return
		}
		key := from + "\x00" + to
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		outgoing[from] = append(outgoing[from], to)
		incoming[to]++
	}
	for _, edge := range workflow.Edges {
		addDependency(edge.Source, edge.Target)
	}
	for _, node := range workflow.Nodes {
		if node.Condition != nil {
			addDependency(node.Condition.NodeID, node.ID)
		}
	}
	ready := make([]string, 0)
	for id := range nodes {
		if incoming[id] == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)
	order := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		order = append(order, id)
		for _, target := range outgoing[id] {
			incoming[target]--
			if incoming[target] == 0 {
				ready = append(ready, target)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(nodes) {
		return nil, fmt.Errorf("cycle in workflow dependencies")
	}
	return order, nil
}

func findPort(ports []Port, id string) (Port, bool) {
	for _, port := range ports {
		if port.ID == id {
			return port, true
		}
	}
	return Port{}, false
}

func portsCompatible(output, input Port) bool {
	for _, sourceType := range output.Types {
		for _, targetType := range input.Types {
			if sourceType == targetType || targetType == "*" {
				return true
			}
		}
	}
	return false
}

func hasValidationCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
