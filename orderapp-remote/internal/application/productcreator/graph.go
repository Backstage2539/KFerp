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
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
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
	Kind        ModuleKind `json:"kind"`
	Name        string     `json:"name"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	Inputs      []Port     `json:"inputs"`
	Outputs     []Port     `json:"outputs"`
	Fields      []Field    `json:"fields"`
	Actions     []string   `json:"actions"`
}

type ValidationIssue struct {
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
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
	out := make([]Module, len(modules))
	copy(out, modules)
	return out
}

func ValidateWorkflow(workflow Workflow) []ValidationIssue {
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
