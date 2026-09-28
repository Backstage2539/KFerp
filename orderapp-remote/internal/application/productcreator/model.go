package productcreator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrConflict = errors.New("product creator revision conflict")
var ErrNotFound = errors.New("product creator object not found")
var ErrNotPublished = errors.New("template has no published version")
var ErrConfigurationExecutorUnavailable = errors.New("product creator configuration executor unavailable")
var ErrRunStepExecutorUnavailable = errors.New("product creator run-step executor unavailable")

type Template struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	Status           string    `json:"status"`
	Revision         int64     `json:"revision"`
	PublishedVersion int64     `json:"published_version"`
	Draft            Workflow  `json:"draft"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type TemplateVersion struct {
	ID          int64     `json:"id"`
	TemplateID  int64     `json:"template_id"`
	Version     int64     `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Workflow    Workflow  `json:"workflow"`
	PublishedAt time.Time `json:"published_at"`
	PublishedBy string    `json:"published_by"`
}

type Run struct {
	ID              int64                     `json:"id"`
	TemplateID      int64                     `json:"template_id"`
	Version         int64                     `json:"template_version"`
	Status          string                    `json:"status"`
	Revision        int64                     `json:"revision"`
	Workflow        Workflow                  `json:"workflow"`
	Inputs          map[string]map[string]any `json:"inputs"`
	Preview         *RunPreview               `json:"preview,omitempty"`
	BusinessResults map[string]any            `json:"business_results,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type RunSummary struct {
	ID              int64     `json:"id"`
	TemplateID      int64     `json:"template_id"`
	TemplateVersion int64     `json:"template_version"`
	TemplateName    string    `json:"template_name"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RunPreview struct {
	Valid  bool              `json:"valid"`
	Steps  []StepPreview     `json:"steps"`
	Issues []ValidationIssue `json:"issues"`
}

type StepPreview struct {
	NodeID   string     `json:"node_id"`
	Name     string     `json:"name"`
	Kind     ModuleKind `json:"kind"`
	Action   string     `json:"action"`
	Status   string     `json:"status"`
	RowCount int        `json:"row_count,omitempty"`
}

type TemplateSave struct {
	ID          int64
	ExpectedRev int64
	Name        string
	Description string
	Workflow    Workflow
	Actor       string
}

type Repository interface {
	ListTemplates(context.Context) ([]Template, error)
	GetTemplate(context.Context, int64) (Template, error)
	SaveTemplate(context.Context, TemplateSave) (Template, error)
	CopyTemplate(context.Context, int64, string, string) (Template, error)
	PublishTemplate(context.Context, int64, int64, string) (TemplateVersion, error)
	ListVersions(context.Context, int64) ([]TemplateVersion, error)
	DisableTemplate(context.Context, int64, int64, string) error
	StartRun(context.Context, int64, string) (Run, error)
	ListTemplateRuns(context.Context, int64, int) ([]RunSummary, error)
	GetRun(context.Context, int64) (Run, error)
	SaveRunInputs(context.Context, int64, int64, map[string]map[string]any, string) (Run, error)
	SaveRunPreview(context.Context, int64, int64, RunPreview, string) (Run, error)
}

// ConfigurationTransactionRepository keeps run state and business writes in
// one transaction. The callback is invoked only after the repository has
// locked and revalidated the run row.
type ConfigurationTransactionRepository interface {
	CommitConfiguration(context.Context, int64, int64, string, string, string, func(context.Context, Run) (map[string]any, error)) (Run, error)
}

type ConfigurationExecutor interface {
	ExecuteConfiguration(context.Context, Run, string) (map[string]any, error)
}

// RunStepTransactionRepository atomically applies one explicitly confirmed
// follow-up action and stores its output and idempotency record.
type RunStepTransactionRepository interface {
	ExecuteRunStep(context.Context, int64, int64, string, string, string, string, string, map[string]any, func(context.Context, Run, Node, string, map[string]any) (map[string]any, error)) (Run, error)
}

type RunStepExecutor interface {
	ExecuteRunStep(context.Context, Run, Node, string, map[string]any, string) (map[string]any, error)
}

type Service struct {
	repo         Repository
	executor     ConfigurationExecutor
	stepExecutor RunStepExecutor
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) UseConfigurationExecutor(executor ConfigurationExecutor) { s.executor = executor }

func (s *Service) UseRunStepExecutor(executor RunStepExecutor) { s.stepExecutor = executor }

func (s *Service) ListTemplates(ctx context.Context) ([]Template, error) {
	return s.repo.ListTemplates(ctx)
}

func (s *Service) GetTemplate(ctx context.Context, id int64) (Template, error) {
	return s.repo.GetTemplate(ctx, id)
}

func (s *Service) SaveTemplate(ctx context.Context, input TemplateSave) (Template, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Template{}, fmt.Errorf("template name required")
	}
	if input.Actor == "" {
		return Template{}, fmt.Errorf("actor required")
	}
	return s.repo.SaveTemplate(ctx, input)
}

func (s *Service) CopyTemplate(ctx context.Context, id int64, name, actor string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, fmt.Errorf("template name required")
	}
	return s.repo.CopyTemplate(ctx, id, name, actor)
}

func (s *Service) ValidateTemplate(ctx context.Context, id int64) ([]ValidationIssue, error) {
	template, err := s.repo.GetTemplate(ctx, id)
	if err != nil {
		return nil, err
	}
	return ValidateWorkflow(template.Draft), nil
}

func (s *Service) PublishTemplate(ctx context.Context, id, revision int64, actor string) (TemplateVersion, error) {
	template, err := s.repo.GetTemplate(ctx, id)
	if err != nil {
		return TemplateVersion{}, err
	}
	if template.Revision != revision {
		return TemplateVersion{}, ErrConflict
	}
	if issues := ValidateWorkflow(template.Draft); len(issues) != 0 {
		return TemplateVersion{}, InvalidWorkflowError{Issues: issues}
	}
	if actor == "" {
		return TemplateVersion{}, fmt.Errorf("actor required")
	}
	return s.repo.PublishTemplate(ctx, id, revision, actor)
}

func (s *Service) ListVersions(ctx context.Context, id int64) ([]TemplateVersion, error) {
	return s.repo.ListVersions(ctx, id)
}

func (s *Service) DisableTemplate(ctx context.Context, id, revision int64, actor string) error {
	if actor == "" {
		return fmt.Errorf("actor required")
	}
	return s.repo.DisableTemplate(ctx, id, revision, actor)
}

func (s *Service) StartRun(ctx context.Context, templateID int64, actor string) (Run, error) {
	if strings.TrimSpace(actor) == "" {
		return Run{}, fmt.Errorf("actor required")
	}
	return s.repo.StartRun(ctx, templateID, actor)
}

func (s *Service) ListTemplateRuns(ctx context.Context, templateID int64, limit int) ([]RunSummary, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.repo.ListTemplateRuns(ctx, templateID, limit)
}

func (s *Service) GetRun(ctx context.Context, id int64) (Run, error) { return s.repo.GetRun(ctx, id) }

func (s *Service) SaveRunInputs(ctx context.Context, id, revision int64, inputs map[string]map[string]any, actor string) (Run, error) {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if run.Status != "draft" {
		return Run{}, fmt.Errorf("run is no longer editable")
	}
	inputs = ResolveWorkflowInputDefaults(run.Workflow, inputs)
	issues := ValidateRunDraft(run.Workflow, inputs)
	if hasRunFatalIssue(issues) {
		return Run{}, InvalidWorkflowError{Issues: issues}
	}
	return s.repo.SaveRunInputs(ctx, id, revision, inputs, actor)
}

func (s *Service) PreviewRun(ctx context.Context, id, revision int64, actor string) (Run, error) {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if run.Revision != revision {
		return Run{}, ErrConflict
	}
	run.Inputs = ResolveWorkflowInputDefaults(run.Workflow, run.Inputs)
	preview := BuildRunPreview(run.Workflow, run.Inputs)
	return s.repo.SaveRunPreview(ctx, id, revision, preview, actor)
}

func (s *Service) CommitConfiguration(ctx context.Context, id, revision int64, idempotencyKey, actor string) (Run, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	actor = strings.TrimSpace(actor)
	if id <= 0 || revision <= 0 || idempotencyKey == "" || actor == "" {
		return Run{}, fmt.Errorf("run, revision, idempotency key, and actor are required")
	}
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	if run.Revision != revision {
		return Run{}, ErrConflict
	}
	if run.Status != "draft" {
		return Run{}, fmt.Errorf("run is no longer editable")
	}
	run.Inputs = ResolveWorkflowInputDefaults(run.Workflow, run.Inputs)
	if run.Preview == nil || !run.Preview.Valid {
		return Run{}, InvalidWorkflowError{Issues: []ValidationIssue{{Code: "preview_required", Message: "请先完成有效的业务预览"}}}
	}
	if issues := ValidateRunInputs(run.Workflow, run.Inputs); hasRunFatalIssue(issues) {
		return Run{}, InvalidWorkflowError{Issues: issues}
	}
	transactional, ok := s.repo.(ConfigurationTransactionRepository)
	if !ok || s.executor == nil {
		return Run{}, ErrConfigurationExecutorUnavailable
	}
	requestBytes, err := json.Marshal(struct {
		Workflow Workflow                  `json:"workflow"`
		Inputs   map[string]map[string]any `json:"inputs"`
	}{Workflow: run.Workflow, Inputs: run.Inputs})
	if err != nil {
		return Run{}, err
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(requestBytes))
	return transactional.CommitConfiguration(ctx, id, revision, idempotencyKey, requestHash, actor, func(txCtx context.Context, locked Run) (map[string]any, error) {
		if locked.Preview == nil || !locked.Preview.Valid {
			return nil, InvalidWorkflowError{Issues: []ValidationIssue{{Code: "preview_required", Message: "请先完成有效的业务预览"}}}
		}
		// Preview resolves template defaults without persisting them. Resolve again
		// from the locked snapshot so fixed template fields cannot be bypassed and
		// the executor sees the same values that were previewed.
		locked.Inputs = ResolveWorkflowInputDefaults(locked.Workflow, locked.Inputs)
		if issues := ValidateRunInputs(locked.Workflow, locked.Inputs); hasRunFatalIssue(issues) {
			return nil, InvalidWorkflowError{Issues: issues}
		}
		return s.executor.ExecuteConfiguration(txCtx, locked, actor)
	})
}

// ExecuteRunStep runs one separately confirmed purchase or pricing action.
// The transaction repository checks idempotency before revision so a retry
// after a lost response returns the already committed result.
func (s *Service) ExecuteRunStep(ctx context.Context, id, revision int64, nodeID, action, idempotencyKey, actor string, stepInputs map[string]any) (Run, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	actor = strings.TrimSpace(actor)
	nodeID = strings.TrimSpace(nodeID)
	action = strings.TrimSpace(action)
	if id <= 0 || revision <= 0 || nodeID == "" || idempotencyKey == "" || len(idempotencyKey) > 200 || actor == "" {
		return Run{}, fmt.Errorf("run, revision, node, idempotency key, and actor are required")
	}
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return Run{}, err
	}
	var node Node
	found := false
	for _, candidate := range run.Workflow.Nodes {
		if candidate.ID == nodeID {
			node, found = candidate, true
			break
		}
	}
	if !found || (node.Kind != ModulePurchase && node.Kind != ModulePricing) {
		return Run{}, fmt.Errorf("step does not support separately confirmed actions")
	}
	if node.Kind == ModulePurchase {
		switch action {
		case "create_purchase_order", "confirm_receipt":
		default:
			return Run{}, fmt.Errorf("unsupported purchase action")
		}
	} else {
		switch action {
		case "preview_pricing", "save_price_draft", "publish_price":
		default:
			return Run{}, fmt.Errorf("unsupported pricing action")
		}
	}
	steps, _ := run.BusinessResults["steps"].(map[string]any)
	stepResult, _ := steps[nodeID].(map[string]any)
	if stringValue(stepResult["status"]) == "skipped" {
		return Run{}, fmt.Errorf("按条件跳过的步骤不能执行")
	}
	inputs := make(map[string]any, len(stepInputs))
	for key, value := range stepInputs {
		inputs[key] = value
	}
	if action == "create_purchase_order" && len(inputs) != 0 {
		return Run{}, fmt.Errorf("purchase order inputs are fixed by the reviewed run draft")
	}
	if action == "confirm_receipt" {
		for key := range inputs {
			if key != "quantity" && key != "unit_price" {
				return Run{}, fmt.Errorf("unsupported receipt field %q", key)
			}
		}
	}
	if node.Kind == ModulePricing && len(inputs) != 0 {
		return Run{}, fmt.Errorf("pricing actions use the reviewed run draft")
	}
	requestBytes, err := json.Marshal(struct {
		RunID       int64          `json:"run_id"`
		NodeID      string         `json:"node_id"`
		Action      string         `json:"action"`
		Version     int64          `json:"template_version"`
		DraftInputs map[string]any `json:"draft_inputs"`
		StepInputs  map[string]any `json:"step_inputs"`
	}{id, nodeID, action, run.Version, run.Inputs[nodeID], inputs})
	if err != nil {
		return Run{}, err
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(requestBytes))
	transactional, ok := s.repo.(RunStepTransactionRepository)
	if !ok || s.stepExecutor == nil {
		return Run{}, ErrRunStepExecutorUnavailable
	}
	return transactional.ExecuteRunStep(ctx, id, revision, nodeID, action, idempotencyKey, requestHash, actor, inputs, func(txCtx context.Context, locked Run, lockedNode Node, lockedAction string, lockedInputs map[string]any) (map[string]any, error) {
		return s.stepExecutor.ExecuteRunStep(txCtx, locked, lockedNode, lockedAction, lockedInputs, actor)
	})
}

type InvalidWorkflowError struct{ Issues []ValidationIssue }

func (e InvalidWorkflowError) Error() string { return "workflow validation failed" }

type ExecutionError struct{ Issues []ValidationIssue }

func (e ExecutionError) Error() string { return "business step execution failed" }

func ValidateRunInputs(workflow Workflow, inputs map[string]map[string]any) []ValidationIssue {
	if workflowVersion(workflow) >= 2 {
		return validateBOMRunInputs(workflow, inputs)
	}
	issues := ValidateWorkflow(workflow)
	nodes := make(map[string]Node, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	for _, node := range workflow.Nodes {
		module, ok := moduleByKind[node.Kind]
		if !ok || !conditionApplies(node, inputs, nodes) {
			continue
		}
		values := inputs[node.ID]
		for _, field := range module.Fields {
			if !field.Required {
				continue
			}
			if node.Kind == ModuleBOM && (values["action"] == "reuse" || values["action"] == "copy") && (field.Key == "variants" || field.Key == "components") {
				continue
			}
			if node.Kind == ModuleProduct && values["action"] == "reuse" && (field.Key == "name" || field.Key == "owner" || field.Key == "industry_fields") {
				continue
			}
			value, exists := values[field.Key]
			if !exists || isEmptyValue(value) {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: field.Key, Code: "required_field", Message: field.Label + "不能为空"})
			}
		}
		issues = append(issues, validateRunRows(node, values, workflow, inputs)...)
		if node.Kind == ModuleBOM && values["action"] != "reuse" {
			outputEdges := make([]Edge, 0)
			for _, edge := range workflow.Edges {
				if edge.Target == node.ID && edge.Kind == EdgeData && edge.TargetHandle == "output" {
					outputEdges = append(outputEdges, edge)
				}
			}
			rowID := stringValue(values["output_source_row_id"])
			validOutput := false
			for _, edge := range outputEdges {
				if containsString(componentSourceRowIDs(workflow, inputs, edge.Source, edge.SourceHandle), rowID) {
					validOutput = true
					break
				}
			}
			if !validOutput {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "output_source_row_id", Code: "missing_output_reference", Message: "请选择本次 BOM 的产出对象"})
			}
		}
		if node.Kind == ModulePublish && boolValue(values["set_default"]) {
			for _, edge := range workflow.Edges {
				if edge.Target != node.ID || edge.TargetHandle != "bom" || edge.Kind != EdgeData {
					continue
				}
				if source, exists := nodes[edge.Source]; exists && source.Kind == ModuleBOM && inputs[source.ID]["action"] == "reuse" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "set_default", Code: "shared_bom_default_change", Message: "引用的已有 BOM 是共享档案；本次不能修改它的默认绑定，请关闭“设为默认 BOM”"})
				}
			}
		}
		if node.Kind == ModulePricing && positiveNumber(values["price_list_id"]) == 0 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "price_list_id", Code: "required_reference", Message: "请选择要复制并生成新版本的已发布价格表"})
		}
		if node.Kind == ModuleProduct && values["action"] != "reuse" && stringValue(values["owner"]) == "customer" && positiveNumber(values["customer_id"]) == 0 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "customer_id", Code: "required_reference", Message: "请选择商品归属客户"})
		}
		if node.Kind == ModuleProduct && values["action"] == "reuse" && positiveNumber(values["product_id"]) == 0 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "product_id", Code: "required_reference", Message: "请选择要引用的商品"})
		}
		if node.Kind == ModuleBOM && (values["action"] == "reuse" || values["action"] == "copy") && positiveNumber(values["bom_id"]) == 0 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "bom_id", Code: "required_reference", Message: "请选择要引用或复制的 BOM"})
		}
	}
	return issues
}

func ResolveWorkflowInputDefaults(workflow Workflow, inputs map[string]map[string]any) map[string]map[string]any {
	raw, err := json.Marshal(inputs)
	if err != nil {
		return inputs
	}
	resolved := map[string]map[string]any{}
	if err := json.Unmarshal(raw, &resolved); err != nil || resolved == nil {
		resolved = map[string]map[string]any{}
	}
	for _, node := range workflow.Nodes {
		values := resolved[node.ID]
		if values == nil {
			values = map[string]any{}
			resolved[node.ID] = values
		}
		defaults := map[string]any{}
		if configured, ok := node.Config["defaults"].(map[string]any); ok {
			for key, value := range configured {
				defaults[key] = value
			}
		}
		for _, key := range []string{"action", "object_action", "product_kind", "owner", "customer_id", "kind", "supply_mode", "unit", "name", "name_pattern", "output_type", "output_qty", "output_unit", "route_id", "material_loss_rate", "rows", "variants", "components"} {
			if value, exists := node.Config[key]; exists {
				defaults[key] = value
			}
		}
		if action, ok := defaults["object_action"]; ok {
			if _, exists := defaults["action"]; !exists {
				defaults["action"] = action
			}
		}
		fixed := map[string]bool{}
		switch keys := node.Config["fixed_fields"].(type) {
		case []any:
			for _, key := range keys {
				fixed[stringValue(key)] = true
			}
		case []string:
			for _, key := range keys {
				fixed[key] = true
			}
		}
		for key, value := range defaults {
			if fixed[key] {
				if key == "components" && node.Kind == ModuleBOM {
					values[key] = applyFixedBOMComponentDefaults(values[key], value)
				} else {
					values[key] = value
				}
			} else if current, exists := values[key]; !exists || isUnsetForDefault(current) {
				values[key] = value
			}
		}
	}
	return resolved
}

func applyFixedBOMComponentDefaults(currentValue, defaultValue any) []map[string]any {
	current := rowValues(currentValue)
	defaults := rowValues(defaultValue)
	if len(current) == 0 {
		current = make([]map[string]any, 0, len(defaults))
		for _, row := range defaults {
			copy := map[string]any{}
			for key, value := range row {
				copy[key] = value
			}
			current = append(current, copy)
		}
		return current
	}
	for _, row := range current {
		var selected map[string]any
		for _, candidate := range defaults {
			if stringValue(candidate["source_node_id"]) != stringValue(row["source_node_id"]) {
				continue
			}
			candidateRow := stringValue(candidate["source_row_id"])
			if candidateRow == "" || candidateRow == stringValue(row["source_row_id"]) {
				selected = candidate
				if candidateRow != "" {
					break
				}
			}
		}
		if selected == nil {
			continue
		}
		for _, key := range []string{"quantity", "unit", "loss_rate", "variant_row_id"} {
			if value, exists := selected[key]; exists {
				row[key] = value
			}
		}
	}
	return current
}

func validateBOMRunInputs(workflow Workflow, inputs map[string]map[string]any) []ValidationIssue {
	issues := ValidateWorkflow(workflow)
	for _, node := range workflow.Nodes {
		values := inputs[node.ID]
		issues = append(issues, validateRowIdentities(node, values)...)
		switch node.Kind {
		case ModuleMaterial:
			if stringValue(node.Config["data_role"]) == "output" {
				action := stringValue(values["action"])
				if action == "" {
					action = stringValue(node.Config["object_action"])
				}
				if action != "reuse" && stringValue(values["name"]) == "" && stringValue(node.Config["name_pattern"]) == "" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "name", Code: "required_field", Message: "产出物料名称不能为空"})
				}
				if action == "reuse" && positiveNumber(values["material_id"]) <= 0 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "material_id", Code: "required_reference", Message: "请选择 BOM 使用的已有物料"})
				}
			} else {
				rows := rowValues(values["rows"])
				if len(rows) == 0 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "rows", Code: "required_row", Message: "请添加至少一个配方物料"})
				}
				for _, row := range rows {
					prefix := "rows." + stringValue(row["row_id"]) + "."
					if stringValue(row["action"]) == "reuse" {
						if positiveNumber(row["material_id"]) <= 0 {
							issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "material_id", Code: "required_reference", Message: "请选择配方物料"})
						}
					} else if stringValue(row["name"]) == "" || stringValue(row["unit"]) == "" {
						issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "name", Code: "required_field", Message: "新物料需要名称和库存单位"})
					}
				}
			}
		case ModuleProduct:
			if stringValue(node.Config["data_role"]) == "output" {
				if values["action"] == "reuse" && positiveNumber(values["product_id"]) <= 0 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "product_id", Code: "required_reference", Message: "请选择已有商品"})
				} else if values["action"] != "reuse" && stringValue(values["name"]) == "" && stringValue(node.Config["name_pattern"]) == "" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "name", Code: "required_field", Message: "商品名称不能为空"})
				}
			} else if positiveNumber(values["product_id"]) <= 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "product_id", Code: "required_reference", Message: "请选择要作为配方的已有商品"})
			} else if positiveNumber(values["bom_spec_id"]) <= 0 && hasProductSpecsInput(workflow, node.ID) {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "bom_spec_id", Code: "required_reference", Message: "请选择商品的具体已发布规格"})
			}
		case ModuleProcess:
			if positiveNumber(values["route_id"]) <= 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "route_id", Code: "required_reference", Message: "请选择有效工艺路线"})
			}
		case ModuleBOM:
			qty := numericValue(values["output_qty"])
			if qty <= 0 {
				qty = numericValue(node.Config["output_qty"])
			}
			if qty <= 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "output_qty", Code: "invalid_quantity", Message: "BOM产出数量必须大于零"})
			}
			if stringValue(node.Config["output_type"]) == "material" {
				unit := stringValue(values["output_unit"])
				if unit == "" {
					unit = stringValue(node.Config["output_unit"])
				}
				if unit == "" {
					for _, edge := range workflow.Edges {
						if edge.Source == node.ID && edge.SourceHandle == "assembly" && edge.TargetHandle == "from_bom" {
							if outputNode, ok := workflowNodeByID(workflow, edge.Target); ok {
								unit = stringValue(outputNode.Config["unit"])
							}
						}
					}
				}
				if unit == "" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "output_unit", Code: "required_reference", Message: "物料产出 BOM 需要选择有效的产出单位"})
				}
			}
			components := rowValues(values["components"])
			if len(components) == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components", Code: "required_row", Message: "请为 BOM 选择配方物料并填写用量"})
			}
			for _, row := range components {
				rowID := stringValue(row["row_id"])
				if numericValue(row["quantity"]) <= 0 || stringValue(row["unit"]) == "" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID + ".quantity", Code: "invalid_component_quantity", Message: "配方用量必须大于零并选择消耗单位"})
				}
				validSource := false
				for _, edge := range workflow.Edges {
					if edge.Target == node.ID && edge.TargetHandle == "components" && edge.Source == stringValue(row["source_node_id"]) {
						validSource = true
						break
					}
				}
				if !validSource || stringValue(row["source_row_id"]) == "" {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID + ".source_node_id", Code: "missing_component_source", Message: "配方行必须对应画布上连接的物料或商品规格"})
				}
			}
			if stringValue(node.Config["output_type"]) == "product" {
				variants := rowValues(values["variants"])
				defaultCount := 0
				if len(variants) == 0 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants", Code: "required_row", Message: "成品 BOM 至少需要一个商品规格"})
				}
				for _, variant := range variants {
					if boolValue(variant["is_default"]) {
						defaultCount++
					}
					if stringValue(variant["name"]) == "" || stringValue(variant["unit"]) == "" {
						issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants." + stringValue(variant["row_id"]), Code: "required_field", Message: "商品规格名称和单位不能为空"})
					}
				}
				if len(variants) > 0 && defaultCount != 1 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants", Code: "default_variant_count", Message: "商品规格必须且只能有一个默认规格"})
				}
			}
		case ModulePurchase:
			for _, key := range []string{"supplier_id", "quantity", "warehouse", "unit_price"} {
				if isEmptyValue(values[key]) || (key == "supplier_id" && positiveNumber(values[key]) <= 0) || (key == "quantity" && numericValue(values[key]) <= 0) || (key == "unit_price" && numericValue(values[key]) < 0) {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: key, Code: "required_field", Message: "采购信息不完整"})
				}
			}
		}
	}
	return issues
}

func workflowNodeByID(workflow Workflow, id string) (Node, bool) {
	for _, node := range workflow.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return Node{}, false
}

func ValidateRunDraft(workflow Workflow, inputs map[string]map[string]any) []ValidationIssue {
	issues := ValidateWorkflow(workflow)
	for _, node := range workflow.Nodes {
		issues = append(issues, validateRowIdentities(node, inputs[node.ID])...)
	}
	return issues
}

func validateRunRows(node Node, values map[string]any, workflow Workflow, inputs map[string]map[string]any) []ValidationIssue {
	issues := validateRowIdentities(node, values)
	if node.Kind == ModuleMaterial {
		for _, row := range rowValues(values["rows"]) {
			rowID := stringValue(row["row_id"])
			prefix := "rows." + rowID + "."
			if stringValue(row["action"]) != "reuse" && strings.TrimSpace(stringValue(row["name"])) == "" {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "name", Code: "required_field", Message: "物料名称不能为空"})
			}
			if stringValue(row["action"]) != "reuse" && strings.TrimSpace(stringValue(row["unit"])) == "" {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "unit", Code: "required_field", Message: "库存单位不能为空"})
			}
			if stringValue(row["action"]) == "reuse" && positiveNumber(row["material_id"]) == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "material_id", Code: "required_reference", Message: "请选择要引用的物料"})
			}
			if stringValue(row["owner_type"]) == "customer" && positiveNumber(row["owner_customer_id"]) == 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: prefix + "owner_customer_id", Code: "required_reference", Message: "请选择物料归属客户"})
			}
		}
	}
	if node.Kind == ModuleBOM {
		variants := rowValues(values["variants"])
		components := rowValues(values["components"])
		if len(variants) == 0 && values["action"] != "reuse" {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants", Code: "required_row", Message: "至少添加一个产出规格"})
		}
		defaults := 0
		variantIDs := map[string]struct{}{}
		for _, variant := range variants {
			rowID := stringValue(variant["row_id"])
			variantIDs[rowID] = struct{}{}
			if boolValue(variant["is_default"]) {
				defaults++
			}
			if strings.TrimSpace(stringValue(variant["name"])) == "" || strings.TrimSpace(stringValue(variant["unit"])) == "" {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants." + rowID, Code: "required_field", Message: "规格名称和单位不能为空"})
			}
		}
		if len(variants) > 0 && defaults != 1 {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "variants", Code: "default_variant_count", Message: "规格必须且只能有一个默认规格"})
		}
		incoming := map[string]bool{}
		for _, edge := range workflow.Edges {
			if edge.Target == node.ID && edge.Kind == EdgeData && edge.TargetHandle == "components" {
				incoming[edge.Source] = true
			}
		}
		if len(components) == 0 && values["action"] != "reuse" {
			issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components", Code: "required_row", Message: "至少添加一个 BOM 组件"})
		}
		for _, component := range components {
			rowID := stringValue(component["row_id"])
			sourceNodeID := stringValue(component["source_node_id"])
			if sourceNodeID == "" || !incoming[sourceNodeID] {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID + ".source_node_id", Code: "missing_component_source", Message: "BOM 组件必须对应一条已连接的数据来源"})
			} else {
				var sourceHandle string
				for _, edge := range workflow.Edges {
					if edge.Target == node.ID && edge.TargetHandle == "components" && edge.Source == sourceNodeID {
						sourceHandle = edge.SourceHandle
						break
					}
				}
				if !containsString(componentSourceRowIDs(workflow, inputs, sourceNodeID, sourceHandle), stringValue(component["source_row_id"])) {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID + ".source_row_id", Code: "missing_component_row", Message: "请选择该数据来源步骤中的具体物料或规格行"})
				}
			}
			if positiveNumber(component["quantity"]) <= 0 || strings.TrimSpace(stringValue(component["unit"])) == "" {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID, Code: "invalid_component_quantity", Message: "组件用量和单位必须填写且用量大于零"})
			}
			variantID := stringValue(component["variant_row_id"])
			if variantID != "" {
				if _, exists := variantIDs[variantID]; !exists {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "components." + rowID + ".variant_row_id", Code: "missing_variant_reference", Message: "组件引用的规格行已不存在"})
				}
			}
		}
	}
	if node.Kind == ModulePricing {
		validSpecs := map[string]bool{}
		for _, edge := range workflow.Edges {
			if edge.Target == node.ID && edge.Kind == EdgeData && edge.TargetHandle == "product" {
				for _, sourceRowID := range componentSourceRowIDs(workflow, inputs, edge.Source, edge.SourceHandle) {
					validSpecs[sourceRowID] = true
				}
			}
		}
		prices := rowValues(values["prices"])
		seenSpecs := map[string]bool{}
		commonMode, commonRuleID := "", float64(0)
		for _, row := range prices {
			rowID := stringValue(row["row_id"])
			specID := stringValue(row["spec_row_id"])
			if specID == "" || !validSpecs[specID] {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".spec_row_id", Code: "missing_price_spec", Message: "定价行必须引用已连接的商品规格"})
				continue
			}
			if seenSpecs[specID] {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".spec_row_id", Code: "duplicate_price_spec", Message: "同一商品规格只能定价一次"})
			}
			seenSpecs[specID] = true
			mode := stringValue(row["pricing_mode"])
			if mode == "" {
				mode = "fixed"
			}
			if mode == "rule" {
				if positiveNumber(row["pricing_rule_id"]) == 0 {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".pricing_rule_id", Code: "required_reference", Message: "请选择定价规则"})
				}
				if commonMode == "" {
					commonMode, commonRuleID = mode, positiveNumber(row["pricing_rule_id"])
				} else if commonMode != mode || commonRuleID != positiveNumber(row["pricing_rule_id"]) {
					issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".pricing_rule_id", Code: "mixed_pricing_rule", Message: "同一商品的所有规格必须使用同一种定价方式和定价规则"})
				}
			} else if mode != "fixed" || positiveNumber(row["price"]) <= 0 {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".price", Code: "invalid_price", Message: "固定售价必须大于零"})
			} else if commonMode == "" {
				commonMode = mode
			} else if commonMode != mode {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices." + rowID + ".pricing_mode", Code: "mixed_pricing_mode", Message: "同一商品的所有规格必须使用同一种定价方式和定价规则"})
			}
		}
		for specID := range validSpecs {
			if !seenSpecs[specID] {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: "prices", Code: "missing_price_spec", Message: "每个已连接商品规格都需要一条定价"})
			}
		}
	}
	return issues
}

func componentSourceRowIDs(workflow Workflow, inputs map[string]map[string]any, sourceNodeID, sourceHandle string) []string {
	nodes := make(map[string]Node, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	node, ok := nodes[sourceNodeID]
	if !ok {
		return nil
	}
	switch node.Kind {
	case ModuleMaterial:
		if sourceHandle != "material" {
			return nil
		}
		ids := make([]string, 0)
		for _, row := range rowValues(inputs[sourceNodeID]["rows"]) {
			ids = append(ids, stringValue(row["row_id"]))
		}
		return ids
	case ModuleProduct:
		if sourceHandle == "product" {
			return []string{"product"}
		}
	case ModuleBOM:
		if sourceHandle == "output" {
			return []string{"output"}
		}
	case ModulePublish:
		if sourceHandle == "output" {
			return []string{"output"}
		}
		if sourceHandle == "specs" {
			for _, edge := range workflow.Edges {
				if edge.Target == sourceNodeID && edge.TargetHandle == "bom" {
					ids := make([]string, 0)
					for _, row := range rowValues(inputs[edge.Source]["variants"]) {
						ids = append(ids, stringValue(row["row_id"]))
					}
					return ids
				}
			}
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateRowIdentities(node Node, values map[string]any) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	for _, key := range []string{"rows", "variants", "components", "prices"} {
		rowIDs := map[string]struct{}{}
		for index, row := range rowValues(values[key]) {
			rowID := strings.TrimSpace(stringValue(row["row_id"]))
			if rowID == "" {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: fmt.Sprintf("%s.%d.row_id", key, index), Code: "missing_row_id", Message: "重复行必须具有稳定标识"})
				continue
			}
			if _, exists := rowIDs[rowID]; exists {
				issues = append(issues, ValidationIssue{NodeID: node.ID, Field: key + "." + rowID, Code: "duplicate_row_id", Message: "重复行标识不能重复"})
			}
			rowIDs[rowID] = struct{}{}
		}
	}
	return issues
}

func rowValues(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		if typed, typedOK := value.([]map[string]any); typedOK {
			return typed
		}
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func boolValue(value any) bool { result, _ := value.(bool); return result }

func positiveNumber(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(number), 64)
		return parsed
	default:
		return 0
	}
}

func conditionApplies(node Node, inputs map[string]map[string]any, nodes map[string]Node) bool {
	condition := node.Condition
	if condition == nil {
		return true
	}
	source, exists := nodes[condition.NodeID]
	if !exists {
		return false
	}
	value := inputs[source.ID][condition.Field]
	empty := isEmptyValue(value)
	switch condition.Operator {
	case "equals":
		return fmt.Sprint(value) == fmt.Sprint(condition.Value)
	case "not_equals":
		return fmt.Sprint(value) != fmt.Sprint(condition.Value)
	case "empty":
		return empty
	case "not_empty":
		return !empty
	default:
		return false
	}
}

func BuildRunPreview(workflow Workflow, inputs map[string]map[string]any) RunPreview {
	issues := ValidateRunInputs(workflow, inputs)
	preview := RunPreview{Valid: !hasRunFatalIssue(issues), Issues: issues, Steps: make([]StepPreview, 0, len(workflow.Nodes))}
	order, err := TopologicalOrder(workflow)
	if err != nil {
		return preview
	}
	nodes := make(map[string]Node, len(workflow.Nodes))
	for _, node := range workflow.Nodes {
		nodes[node.ID] = node
	}
	conditionNodes := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		conditionNodes[node.ID] = node
	}
	statuses := make(map[string]string, len(order))
	for _, id := range order {
		node := nodes[id]
		name := strings.TrimSpace(node.Name)
		if name == "" {
			name = moduleByKind[node.Kind].Name
		}
		values := inputs[id]
		if !conditionApplies(node, inputs, conditionNodes) {
			statuses[id] = "skipped"
			preview.Steps = append(preview.Steps, StepPreview{NodeID: id, Name: name, Kind: node.Kind, Action: "按条件跳过", Status: "skipped"})
			continue
		}
		blockedBySkip := false
		for _, edge := range workflow.Edges {
			if edge.Target == id && edge.Kind == EdgeData && statuses[edge.Source] == "skipped" {
				blockedBySkip = true
			}
		}
		if blockedBySkip {
			issues = append(issues, ValidationIssue{NodeID: id, Code: "skipped_source_required", Message: "数据来源步骤因条件跳过，当前步骤缺少输入"})
		}
		action := describeAction(node.Kind, values)
		rowCount := 0
		rowCount = len(rowValues(values["rows"]))
		status := "ready"
		if node.Condition != nil {
			status = "conditional"
		}
		statuses[id] = status
		preview.Steps = append(preview.Steps, StepPreview{NodeID: id, Name: name, Kind: node.Kind, Action: action, Status: status, RowCount: rowCount})
	}
	preview.Issues = issues
	preview.Valid = !hasRunFatalIssue(issues)
	return preview
}

func describeAction(kind ModuleKind, values map[string]any) string {
	switch kind {
	case ModuleProduct:
		return modeAction(values["action"], "新建商品", "引用已有商品")
	case ModuleMaterial:
		return "按每行选择新建或引用"
	case ModuleBOM:
		switch values["action"] {
		case "reuse":
			return "引用已有 BOM"
		case "copy":
			return "复制为新 BOM"
		default:
			return "新建 BOM"
		}
	case ModulePublish:
		return "发布 BOM 并按配置绑定默认版本"
	case ModulePurchase:
		return "创建采购单；收货需单独确认"
	case ModulePricing:
		return "试算并保存价格草稿；发布需单独确认"
	default:
		return "使用现有工艺路线"
	}
}

func modeAction(value any, create, reuse string) string {
	if mode, ok := value.(string); ok && (mode == "reuse" || mode == "reference") {
		return reuse
	}
	return create
}

func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	case json.RawMessage:
		return len(v) == 0 || string(v) == "null"
	}
	return false
}

func isUnsetForDefault(value any) bool {
	if isEmptyValue(value) {
		return true
	}
	switch current := value.(type) {
	case float64:
		return current == 0
	case float32:
		return current == 0
	case int:
		return current == 0
	case int64:
		return current == 0
	case json.Number:
		parsed, _ := current.Float64()
		return parsed == 0
	}
	return false
}

func numericValue(value any) float64 {
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
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(current), 64)
		return parsed
	default:
		return 0
	}
}

func hasProductSpecsInput(workflow Workflow, nodeID string) bool {
	for _, edge := range workflow.Edges {
		if edge.Source == nodeID && edge.SourceHandle == "specs" && edge.TargetHandle == "components" {
			return true
		}
	}
	return false
}

func hasRunFatalIssue(issues []ValidationIssue) bool {
	return len(issues) > 0
}
