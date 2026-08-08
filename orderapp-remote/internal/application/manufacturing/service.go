package manufacturing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type IndustryFieldDefinition struct {
	ID          int64  `json:"id"`
	TemplateID  int64  `json:"template_id"`
	FieldKey    string `json:"field_key"`
	Label       string `json:"label"`
	FieldType   string `json:"field_type"`
	Unit        string `json:"unit"`
	Required    bool   `json:"required"`
	OptionsJSON string `json:"options_json"`
	SortOrder   int    `json:"sort_order"`
}

type IndustryFieldTemplate struct {
	ID          int64                     `json:"id"`
	Name        string                    `json:"name"`
	IndustryKey string                    `json:"industry_key"`
	Description string                    `json:"description"`
	Status      string                    `json:"status"`
	Fields      []IndustryFieldDefinition `json:"fields"`
	CreatedAt   string                    `json:"created_at"`
	UpdatedAt   string                    `json:"updated_at"`
}

type ManufacturingOperation struct {
	ID                    int64   `json:"id"`
	Code                  string  `json:"code"`
	Name                  string  `json:"name"`
	Status                string  `json:"status"`
	DefaultMinutes        int     `json:"default_minutes"`
	StandardOperationCost float64 `json:"standard_operation_cost"`
	Note                  string  `json:"note"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

type ManufacturingWorkstation struct {
	ID                     int64                    `json:"id"`
	Code                   string                   `json:"code"`
	Name                   string                   `json:"name"`
	Status                 string                   `json:"status"`
	DefaultMinutes         int                      `json:"default_minutes"`
	MachineHourlyCost      float64                  `json:"machine_hourly_cost"`
	LaborHourlyCost        float64                  `json:"labor_hourly_cost"`
	OverheadHourlyCost     float64                  `json:"overhead_hourly_cost"`
	HourlyRate             float64                  `json:"hourly_rate"`
	ApplicableOperationIDs []int64                  `json:"applicable_operation_ids"`
	ApplicableOperations   []ManufacturingOperation `json:"applicable_operations,omitempty"`
	Note                   string                   `json:"note"`
	CreatedAt              string                   `json:"created_at"`
	UpdatedAt              string                   `json:"updated_at"`
}

type ManufacturingWorkstationCapacity struct {
	ID                     int64                    `json:"id"`
	WorkstationID          int64                    `json:"workstation_id"`
	Workstation            string                   `json:"workstation"`
	Code                   string                   `json:"code"`
	Name                   string                   `json:"name"`
	Status                 string                   `json:"status"`
	BatchSizeQty           float64                  `json:"batch_size_qty"`
	BatchSizeUnit          string                   `json:"batch_size_unit"`
	StandardMinutes        int                      `json:"standard_minutes"`
	HourlyRate             float64                  `json:"hourly_rate"`
	CostMethod             string                   `json:"cost_method"`
	PieceRate              float64                  `json:"piece_rate"`
	ProductionCapacity     int                      `json:"production_capacity"`
	SortOrder              int                      `json:"sort_order"`
	Note                   string                   `json:"note"`
	ApplicableOperationIDs []int64                  `json:"applicable_operation_ids"`
	ApplicableOperations   []ManufacturingOperation `json:"applicable_operations,omitempty"`
	CreatedAt              string                   `json:"created_at"`
	UpdatedAt              string                   `json:"updated_at"`
}

type ProcessTemplateOperation struct {
	ID                      int64   `json:"id"`
	TemplateID              int64   `json:"template_id"`
	Seq                     int     `json:"seq"`
	OperationID             int64   `json:"operation_id"`
	WorkstationID           int64   `json:"workstation_id"`
	WorkstationCapacityID   int64   `json:"workstation_capacity_id"`
	Operation               string  `json:"operation"`
	Workstation             string  `json:"workstation"`
	WorkstationCapacityName string  `json:"workstation_capacity_name"`
	DefaultEquipment        string  `json:"default_equipment"`
	DefaultMinutes          int     `json:"default_minutes"`
	BatchSizeQty            float64 `json:"batch_size_qty"`
	BatchSizeUnit           string  `json:"batch_size_unit"`
	StandardMinutes         int     `json:"standard_minutes"`
	HourlyRate              float64 `json:"hourly_rate"`
	CostMethod              string  `json:"cost_method"`
	PieceRate               float64 `json:"piece_rate"`
	PlannedBatchCount       int     `json:"planned_batch_count"`
	PlannedMinutes          int     `json:"planned_minutes"`
	PlannedOperationCost    float64 `json:"planned_operation_cost"`
	RecordsLoss             bool    `json:"records_loss"`
	ParameterSchemaJSON     string  `json:"parameter_schema_json"`
	QualityChecklistJSON    string  `json:"quality_checklist_json"`
}

type ProcessTemplate struct {
	ID                   int64                      `json:"id"`
	Name                 string                     `json:"name"`
	ProductID            int64                      `json:"product_id"`
	ProductName          string                     `json:"product_name"`
	BomVersionID         int64                      `json:"bom_version_id"`
	BomVersionNo         string                     `json:"bom_version_no"`
	IndustryTemplateID   int64                      `json:"industry_template_id"`
	IndustryTemplateName string                     `json:"industry_template_name"`
	Status               string                     `json:"status"`
	DefaultEquipment     string                     `json:"default_equipment"`
	DefaultMinutes       int                        `json:"default_minutes"`
	KeyParamsJSON        string                     `json:"key_params_json"`
	Note                 string                     `json:"note"`
	Operations           []ProcessTemplateOperation `json:"operations"`
	CreatedAt            string                     `json:"created_at"`
	UpdatedAt            string                     `json:"updated_at"`
}

type ProcessRouteOperation struct {
	ID                       int64   `json:"id"`
	RouteID                  int64   `json:"route_id"`
	Seq                      int     `json:"seq"`
	OperationID              int64   `json:"operation_id"`
	WorkstationID            int64   `json:"workstation_id"`
	WorkstationCapacityID    int64   `json:"workstation_capacity_id"`
	StandardCostCapacityID   int64   `json:"standard_cost_capacity_id"`
	Operation                string  `json:"operation"`
	Workstation              string  `json:"workstation"`
	WorkstationCapacityName  string  `json:"workstation_capacity_name"`
	StandardCostCapacityName string  `json:"standard_cost_capacity_name,omitempty"`
	StandardCostWorkstation  string  `json:"standard_cost_workstation,omitempty"`
	StandardCostSummary      string  `json:"standard_cost_summary,omitempty"`
	DefaultEquipment         string  `json:"default_equipment"`
	DefaultMinutes           int     `json:"default_minutes"`
	BatchSizeQty             float64 `json:"batch_size_qty"`
	BatchSizeUnit            string  `json:"batch_size_unit"`
	StandardMinutes          int     `json:"standard_minutes"`
	HourlyRate               float64 `json:"hourly_rate"`
	CostMethod               string  `json:"cost_method"`
	PieceRate                float64 `json:"piece_rate"`
	PlannedBatchCount        int     `json:"planned_batch_count"`
	PlannedMinutes           int     `json:"planned_minutes"`
	PlannedOperationCost     float64 `json:"planned_operation_cost"`
	RecordsLoss              bool    `json:"records_loss"`
	QualityChecklistJSON     string  `json:"quality_checklist_json"`
}

type ProcessRoute struct {
	ID               int64                   `json:"id"`
	Name             string                  `json:"name"`
	Status           string                  `json:"status"`
	DefaultEquipment string                  `json:"default_equipment"`
	DefaultMinutes   int                     `json:"default_minutes"`
	Note             string                  `json:"note"`
	Operations       []ProcessRouteOperation `json:"operations"`
	CreatedAt        string                  `json:"created_at"`
	UpdatedAt        string                  `json:"updated_at"`
}

type ProcessTemplateQuery struct {
	ProductID int64
	Status    string
}

type ProcessRouteQuery struct {
	Status string
}

type WorkstationCapacityQuery struct {
	WorkstationID int64
	Status        string
}

type SaveIndustryTemplateCommand struct {
	ID          int64
	Name        string
	IndustryKey string
	Description string
	Status      string
	Fields      []IndustryFieldDefinition
	Actor       string
}

type SaveManufacturingOperationCommand struct {
	ID                    int64
	Code                  string
	Name                  string
	Status                string
	DefaultMinutes        int
	StandardOperationCost float64
	Note                  string
	Actor                 string
}

type SaveManufacturingWorkstationCommand struct {
	ID                     int64
	Code                   string
	Name                   string
	Status                 string
	DefaultMinutes         int
	MachineHourlyCost      float64
	LaborHourlyCost        float64
	OverheadHourlyCost     float64
	HourlyRate             float64
	ApplicableOperationIDs []int64
	Note                   string
	Actor                  string
}

type SaveWorkstationCapacityCommand struct {
	ID                     int64
	WorkstationID          int64
	Code                   string
	Name                   string
	Status                 string
	BatchSizeQty           float64
	BatchSizeUnit          string
	StandardMinutes        int
	HourlyRate             float64
	CostMethod             string
	PieceRate              float64
	ProductionCapacity     int
	SortOrder              int
	Note                   string
	ApplicableOperationIDs []int64
	Actor                  string
}

type SaveProcessTemplateCommand struct {
	ID                 int64
	Name               string
	ProductID          int64
	BomVersionID       int64
	IndustryTemplateID int64
	Status             string
	DefaultEquipment   string
	DefaultMinutes     int
	KeyParamsJSON      string
	Note               string
	Operations         []ProcessTemplateOperation
	Actor              string
}

type SaveProcessRouteCommand struct {
	ID               int64
	Name             string
	Status           string
	DefaultEquipment string
	DefaultMinutes   int
	Note             string
	Operations       []ProcessRouteOperation
	Actor            string
}

type TemplateStatusCommand struct {
	ID    int64
	Actor string
}

type Repository interface {
	ListManufacturingOperations(ctx context.Context) ([]ManufacturingOperation, error)
	SaveManufacturingOperation(ctx context.Context, cmd SaveManufacturingOperationCommand) (ManufacturingOperation, error)
	DeactivateManufacturingOperation(ctx context.Context, cmd TemplateStatusCommand) error
	ListManufacturingWorkstations(ctx context.Context) ([]ManufacturingWorkstation, error)
	SaveManufacturingWorkstation(ctx context.Context, cmd SaveManufacturingWorkstationCommand) (ManufacturingWorkstation, error)
	DeactivateManufacturingWorkstation(ctx context.Context, cmd TemplateStatusCommand) error
	ListManufacturingWorkstationCapacities(ctx context.Context, query WorkstationCapacityQuery) ([]ManufacturingWorkstationCapacity, error)
	SaveManufacturingWorkstationCapacity(ctx context.Context, cmd SaveWorkstationCapacityCommand) (ManufacturingWorkstationCapacity, error)
	DeactivateManufacturingWorkstationCapacity(ctx context.Context, cmd TemplateStatusCommand) error
	ListIndustryTemplates(ctx context.Context) ([]IndustryFieldTemplate, error)
	SaveIndustryTemplate(ctx context.Context, cmd SaveIndustryTemplateCommand) (IndustryFieldTemplate, error)
	DeactivateIndustryTemplate(ctx context.Context, cmd TemplateStatusCommand) error
	ListProcessTemplates(ctx context.Context, query ProcessTemplateQuery) ([]ProcessTemplate, error)
	SaveProcessTemplate(ctx context.Context, cmd SaveProcessTemplateCommand) (ProcessTemplate, error)
	PublishProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error
	DeactivateProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error
	ListProcessRoutes(ctx context.Context, query ProcessRouteQuery) ([]ProcessRoute, error)
	SaveProcessRoute(ctx context.Context, cmd SaveProcessRouteCommand) (ProcessRoute, error)
	PublishProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error
	DeactivateProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListManufacturingOperations(ctx context.Context) ([]ManufacturingOperation, error) {
	return s.repo.ListManufacturingOperations(ctx)
}

func (s *Service) SaveManufacturingOperation(ctx context.Context, cmd SaveManufacturingOperationCommand) (ManufacturingOperation, error) {
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = normalizeStatus(cmd.Status, "active")
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.Name == "" {
		return ManufacturingOperation{}, fmt.Errorf("name required")
	}
	if cmd.DefaultMinutes < 0 {
		return ManufacturingOperation{}, fmt.Errorf("default_minutes must be >= 0")
	}
	if cmd.StandardOperationCost < 0 {
		return ManufacturingOperation{}, fmt.Errorf("standard_operation_cost must be >= 0")
	}
	if cmd.Status != "active" && cmd.Status != "inactive" {
		return ManufacturingOperation{}, fmt.Errorf("invalid status")
	}
	if cmd.Code == "" {
		cmd.Code = codeFromName(cmd.Name)
	}
	return s.repo.SaveManufacturingOperation(ctx, cmd)
}

func (s *Service) DeactivateManufacturingOperation(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("operation id required")
	}
	return s.repo.DeactivateManufacturingOperation(ctx, cmd)
}

func (s *Service) ListManufacturingWorkstations(ctx context.Context) ([]ManufacturingWorkstation, error) {
	return s.repo.ListManufacturingWorkstations(ctx)
}

func (s *Service) SaveManufacturingWorkstation(ctx context.Context, cmd SaveManufacturingWorkstationCommand) (ManufacturingWorkstation, error) {
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = normalizeStatus(cmd.Status, "active")
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.Name == "" {
		return ManufacturingWorkstation{}, fmt.Errorf("name required")
	}
	if cmd.DefaultMinutes < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("default_minutes must be >= 0")
	}
	if cmd.MachineHourlyCost < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("machine_hourly_cost must be >= 0")
	}
	if cmd.LaborHourlyCost < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("labor_hourly_cost must be >= 0")
	}
	if cmd.OverheadHourlyCost < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("overhead_hourly_cost must be >= 0")
	}
	if cmd.HourlyRate < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("hourly_rate must be >= 0")
	}
	if cmd.Status != "active" && cmd.Status != "inactive" {
		return ManufacturingWorkstation{}, fmt.Errorf("invalid status")
	}
	if cmd.Code == "" {
		cmd.Code = codeFromName(cmd.Name)
	}
	componentTotal := cmd.MachineHourlyCost + cmd.LaborHourlyCost + cmd.OverheadHourlyCost
	if componentTotal > 0 || cmd.HourlyRate == 0 {
		cmd.HourlyRate = roundMoney(componentTotal)
	}
	cmd.ApplicableOperationIDs = normalizePositiveInt64IDs(cmd.ApplicableOperationIDs)
	return s.repo.SaveManufacturingWorkstation(ctx, cmd)
}

func (s *Service) DeactivateManufacturingWorkstation(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("workstation id required")
	}
	return s.repo.DeactivateManufacturingWorkstation(ctx, cmd)
}

func (s *Service) ListManufacturingWorkstationCapacities(ctx context.Context, query WorkstationCapacityQuery) ([]ManufacturingWorkstationCapacity, error) {
	query.Status = strings.TrimSpace(query.Status)
	return s.repo.ListManufacturingWorkstationCapacities(ctx, query)
}

func (s *Service) SaveManufacturingWorkstationCapacity(ctx context.Context, cmd SaveWorkstationCapacityCommand) (ManufacturingWorkstationCapacity, error) {
	cmd.Code = strings.TrimSpace(cmd.Code)
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = normalizeStatus(cmd.Status, "active")
	cmd.BatchSizeUnit = strings.TrimSpace(cmd.BatchSizeUnit)
	cmd.CostMethod = normalizeCapacityCostMethod(cmd.CostMethod)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.WorkstationID <= 0 {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("workstation_id required")
	}
	if cmd.Name == "" {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("name required")
	}
	if cmd.Status != "active" && cmd.Status != "inactive" {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("invalid status")
	}
	if cmd.BatchSizeQty < 0 {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("batch_size_qty must be >= 0")
	}
	if cmd.StandardMinutes < 0 {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("standard_minutes must be >= 0")
	}
	if cmd.PieceRate < 0 {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("计件成本不能小于 0")
	}
	cmd.HourlyRate = 0
	if cmd.ProductionCapacity <= 0 {
		cmd.ProductionCapacity = 1
	}
	if cmd.BatchSizeUnit == "" {
		cmd.BatchSizeUnit = "unit"
	}
	if cmd.CostMethod == "piece" {
		if !isCountCapacityUnit(cmd.BatchSizeUnit) {
			return ManufacturingWorkstationCapacity{}, fmt.Errorf("按件成本的标准产能单位必须使用“件”；每件表示当前商品的一个销售规格，不能使用 %s", cmd.BatchSizeUnit)
		}
		// Accept API aliases for compatibility, but persist one unambiguous
		// business unit. A piece always means one concrete SKU sales spec.
		cmd.BatchSizeUnit = "件"
		if cmd.PieceRate <= 0 {
			return ManufacturingWorkstationCapacity{}, fmt.Errorf("按件成本必须大于 0")
		}
	} else {
		cmd.PieceRate = 0
	}
	if cmd.Code == "" {
		cmd.Code = codeFromName(cmd.Name)
	}
	cmd.ApplicableOperationIDs = nil
	return s.repo.SaveManufacturingWorkstationCapacity(ctx, cmd)
}

func (s *Service) DeactivateManufacturingWorkstationCapacity(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("workstation capacity id required")
	}
	return s.repo.DeactivateManufacturingWorkstationCapacity(ctx, cmd)
}

func (s *Service) ListIndustryTemplates(ctx context.Context) ([]IndustryFieldTemplate, error) {
	return s.repo.ListIndustryTemplates(ctx)
}

func (s *Service) SaveIndustryTemplate(ctx context.Context, cmd SaveIndustryTemplateCommand) (IndustryFieldTemplate, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.IndustryKey = strings.TrimSpace(cmd.IndustryKey)
	cmd.Description = strings.TrimSpace(cmd.Description)
	cmd.Status = normalizeStatus(cmd.Status, "active")
	if cmd.Name == "" {
		return IndustryFieldTemplate{}, fmt.Errorf("name required")
	}
	if cmd.IndustryKey == "" {
		cmd.IndustryKey = "general"
	}
	if cmd.Status != "active" && cmd.Status != "inactive" {
		return IndustryFieldTemplate{}, fmt.Errorf("invalid status")
	}
	for i := range cmd.Fields {
		field, err := normalizeIndustryField(cmd.Fields[i], i+1)
		if err != nil {
			return IndustryFieldTemplate{}, err
		}
		cmd.Fields[i] = field
	}
	return s.repo.SaveIndustryTemplate(ctx, cmd)
}

func (s *Service) DeactivateIndustryTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("template id required")
	}
	return s.repo.DeactivateIndustryTemplate(ctx, cmd)
}

func (s *Service) ListProcessTemplates(ctx context.Context, query ProcessTemplateQuery) ([]ProcessTemplate, error) {
	query.Status = strings.TrimSpace(query.Status)
	return s.repo.ListProcessTemplates(ctx, query)
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func (s *Service) SaveProcessTemplate(ctx context.Context, cmd SaveProcessTemplateCommand) (ProcessTemplate, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = normalizeStatus(cmd.Status, "draft")
	cmd.DefaultEquipment = strings.TrimSpace(cmd.DefaultEquipment)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.Name == "" {
		return ProcessTemplate{}, fmt.Errorf("name required")
	}
	if cmd.ProductID <= 0 {
		return ProcessTemplate{}, fmt.Errorf("product_id required")
	}
	if cmd.DefaultMinutes < 0 {
		return ProcessTemplate{}, fmt.Errorf("default_minutes must be >= 0")
	}
	if cmd.Status != "draft" && cmd.Status != "active" && cmd.Status != "inactive" {
		return ProcessTemplate{}, fmt.Errorf("invalid status")
	}
	keyParamsJSON, err := normalizeJSONObject(cmd.KeyParamsJSON)
	if err != nil {
		return ProcessTemplate{}, fmt.Errorf("key_params_json must be a JSON object")
	}
	cmd.KeyParamsJSON = keyParamsJSON
	if len(cmd.Operations) == 0 {
		return ProcessTemplate{}, fmt.Errorf("operations required")
	}
	for i := range cmd.Operations {
		op, err := normalizeProcessOperation(cmd.Operations[i], i+1)
		if err != nil {
			return ProcessTemplate{}, err
		}
		cmd.Operations[i] = op
	}
	return s.repo.SaveProcessTemplate(ctx, cmd)
}

func (s *Service) PublishProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("template id required")
	}
	return s.repo.PublishProcessTemplate(ctx, cmd)
}

func (s *Service) DeactivateProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("template id required")
	}
	return s.repo.DeactivateProcessTemplate(ctx, cmd)
}

func (s *Service) ListProcessRoutes(ctx context.Context, query ProcessRouteQuery) ([]ProcessRoute, error) {
	query.Status = strings.TrimSpace(query.Status)
	return s.repo.ListProcessRoutes(ctx, query)
}

func (s *Service) SaveProcessRoute(ctx context.Context, cmd SaveProcessRouteCommand) (ProcessRoute, error) {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.Status = normalizeStatus(cmd.Status, "draft")
	cmd.DefaultEquipment = strings.TrimSpace(cmd.DefaultEquipment)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.Name == "" {
		return ProcessRoute{}, fmt.Errorf("name required")
	}
	if cmd.DefaultMinutes < 0 {
		return ProcessRoute{}, fmt.Errorf("default_minutes must be >= 0")
	}
	if cmd.Status != "draft" && cmd.Status != "active" && cmd.Status != "inactive" {
		return ProcessRoute{}, fmt.Errorf("invalid status")
	}
	if len(cmd.Operations) == 0 {
		return ProcessRoute{}, fmt.Errorf("operations required")
	}
	for i := range cmd.Operations {
		op, err := normalizeProcessRouteOperation(cmd.Operations[i], i+1)
		if err != nil {
			return ProcessRoute{}, err
		}
		op, err = s.applyStandardCostCapacitySnapshot(ctx, op)
		if err != nil {
			return ProcessRoute{}, err
		}
		if cmd.Status == "active" && op.StandardCostCapacityID <= 0 {
			return ProcessRoute{}, fmt.Errorf("standard_cost_capacity_id required for active route operation %s", op.Operation)
		}
		cmd.Operations[i] = op
	}
	return s.repo.SaveProcessRoute(ctx, cmd)
}

func (s *Service) PublishProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("route id required")
	}
	return s.repo.PublishProcessRoute(ctx, cmd)
}

func (s *Service) DeactivateProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("route id required")
	}
	return s.repo.DeactivateProcessRoute(ctx, cmd)
}

func normalizeStatus(status, fallback string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return fallback
	}
	return status
}

func normalizeCapacityCostMethod(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "piece") {
		return "piece"
	}
	return "time"
}

func isCountCapacityUnit(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "件", "unit", "units", "pc", "pcs", "piece", "pieces":
		return true
	default:
		return false
	}
}

func codeFromName(name string) string {
	code := strings.ToLower(strings.TrimSpace(name))
	code = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r >= '\u4e00' && r <= '\u9fa5':
			return r
		default:
			return '_'
		}
	}, code)
	code = strings.Trim(code, "_")
	code = strings.Join(strings.FieldsFunc(code, func(r rune) bool { return r == '_' }), "_")
	if code == "" {
		return "operation"
	}
	return code
}

func normalizeIndustryField(field IndustryFieldDefinition, fallbackOrder int) (IndustryFieldDefinition, error) {
	field.FieldKey = strings.TrimSpace(field.FieldKey)
	field.Label = strings.TrimSpace(field.Label)
	field.FieldType = strings.ToLower(strings.TrimSpace(field.FieldType))
	field.Unit = strings.TrimSpace(field.Unit)
	if field.SortOrder <= 0 {
		field.SortOrder = fallbackOrder
	}
	if field.FieldKey == "" {
		return field, fmt.Errorf("field_key required")
	}
	if field.Label == "" {
		field.Label = field.FieldKey
	}
	if field.FieldType == "" {
		field.FieldType = "text"
	}
	switch field.FieldType {
	case "text", "textarea", "number", "ratio", "select", "checkbox", "date":
	default:
		return field, fmt.Errorf("invalid field_type")
	}
	optionsJSON, err := normalizeJSONArray(field.OptionsJSON)
	if err != nil {
		return field, fmt.Errorf("options_json must be a JSON array")
	}
	field.OptionsJSON = optionsJSON
	return field, nil
}

func normalizeProcessOperation(op ProcessTemplateOperation, fallbackSeq int) (ProcessTemplateOperation, error) {
	op.Operation = strings.TrimSpace(op.Operation)
	op.Workstation = strings.TrimSpace(op.Workstation)
	op.WorkstationCapacityName = strings.TrimSpace(op.WorkstationCapacityName)
	op.BatchSizeUnit = strings.TrimSpace(op.BatchSizeUnit)
	op.DefaultEquipment = strings.TrimSpace(op.DefaultEquipment)
	if op.Seq <= 0 {
		op.Seq = fallbackSeq
	}
	if op.DefaultMinutes < 0 {
		return op, fmt.Errorf("operation default_minutes must be >= 0")
	}
	if op.Operation == "" {
		return op, fmt.Errorf("operation required")
	}
	if err := validateOperationCostSnapshot(op.BatchSizeQty, op.StandardMinutes, op.HourlyRate, op.PlannedBatchCount, op.PlannedMinutes, op.PlannedOperationCost); err != nil {
		return op, err
	}
	if op.StandardMinutes == 0 && op.DefaultMinutes > 0 {
		op.StandardMinutes = op.DefaultMinutes
	}
	if op.DefaultMinutes == 0 && op.StandardMinutes > 0 {
		op.DefaultMinutes = op.StandardMinutes
	}
	if op.PlannedMinutes == 0 && op.PlannedBatchCount > 0 && op.StandardMinutes > 0 {
		op.PlannedMinutes = op.PlannedBatchCount * op.StandardMinutes
	}
	if op.PlannedOperationCost == 0 && op.PlannedMinutes > 0 && op.HourlyRate > 0 {
		op.PlannedOperationCost = roundMoney(float64(op.PlannedMinutes) / 60 * op.HourlyRate)
	}
	parameterSchemaJSON, err := normalizeJSONObject(op.ParameterSchemaJSON)
	if err != nil {
		return op, fmt.Errorf("parameter_schema_json must be a JSON object")
	}
	qualityChecklistJSON, err := normalizeJSONArray(op.QualityChecklistJSON)
	if err != nil {
		return op, fmt.Errorf("quality_checklist_json must be a JSON array")
	}
	op.ParameterSchemaJSON = parameterSchemaJSON
	op.QualityChecklistJSON = qualityChecklistJSON
	return op, nil
}

func normalizeProcessRouteOperation(op ProcessRouteOperation, fallbackSeq int) (ProcessRouteOperation, error) {
	op.Operation = strings.TrimSpace(op.Operation)
	if op.Seq <= 0 {
		op.Seq = fallbackSeq
	}
	if op.Operation == "" {
		return op, fmt.Errorf("operation required")
	}
	qualityChecklistJSON, err := normalizeJSONArray(op.QualityChecklistJSON)
	if err != nil {
		return op, fmt.Errorf("quality_checklist_json must be a JSON array")
	}
	op.DefaultEquipment = ""
	op.DefaultMinutes = 0
	op.WorkstationID = 0
	op.Workstation = ""
	op.WorkstationCapacityID = 0
	op.WorkstationCapacityName = ""
	if op.StandardCostCapacityID < 0 {
		op.StandardCostCapacityID = 0
	}
	op.StandardCostCapacityName = ""
	op.StandardCostWorkstation = ""
	op.StandardCostSummary = ""
	op.BatchSizeQty = 0
	op.BatchSizeUnit = ""
	op.StandardMinutes = 0
	op.HourlyRate = 0
	op.CostMethod = "time"
	op.PieceRate = 0
	op.PlannedBatchCount = 0
	op.PlannedMinutes = 0
	op.PlannedOperationCost = 0
	op.QualityChecklistJSON = qualityChecklistJSON
	return op, nil
}

func (s *Service) applyStandardCostCapacitySnapshot(ctx context.Context, op ProcessRouteOperation) (ProcessRouteOperation, error) {
	if op.StandardCostCapacityID <= 0 {
		return op, nil
	}
	if op.OperationID <= 0 {
		return op, fmt.Errorf("standard_cost_capacity_id requires operation_id")
	}
	rows, err := s.repo.ListManufacturingWorkstationCapacities(ctx, WorkstationCapacityQuery{Status: "active"})
	if err != nil {
		return op, err
	}
	for _, row := range rows {
		if row.ID != op.StandardCostCapacityID {
			continue
		}
		if !containsInt64(row.ApplicableOperationIDs, op.OperationID) {
			return op, fmt.Errorf("standard_cost_capacity_id %d is not applicable to operation %d", row.ID, op.OperationID)
		}
		op.StandardCostCapacityName = row.Name
		op.StandardCostWorkstation = row.Workstation
		op.BatchSizeQty = row.BatchSizeQty
		op.BatchSizeUnit = row.BatchSizeUnit
		op.StandardMinutes = row.StandardMinutes
		op.HourlyRate = row.HourlyRate
		op.CostMethod = normalizeCapacityCostMethod(row.CostMethod)
		op.PieceRate = row.PieceRate
		op.PlannedBatchCount = 1
		op.PlannedMinutes = row.StandardMinutes
		if op.CostMethod == "piece" && row.PieceRate > 0 {
			op.PlannedOperationCost = row.PieceRate
			op.StandardCostSummary = fmt.Sprintf("%s：计件成本 %.4f元/销售规格件", row.Name, row.PieceRate)
		} else if row.BatchSizeQty > 0 && row.StandardMinutes > 0 && row.HourlyRate > 0 {
			op.PlannedOperationCost = roundMoney((float64(row.StandardMinutes) / 60 * row.HourlyRate) / row.BatchSizeQty)
			op.StandardCostSummary = fmt.Sprintf("%s：%.4f × %d / 60 / %.4f%s = %.4f/%s", row.Name, row.HourlyRate, row.StandardMinutes, row.BatchSizeQty, row.BatchSizeUnit, op.PlannedOperationCost, row.BatchSizeUnit)
		}
		return op, nil
	}
	return op, fmt.Errorf("standard_cost_capacity_id %d not found", op.StandardCostCapacityID)
}

func (s *Service) applyWorkstationCapacitySnapshot(ctx context.Context, op ProcessRouteOperation) (ProcessRouteOperation, error) {
	if op.WorkstationCapacityID <= 0 {
		return op, nil
	}
	query := WorkstationCapacityQuery{Status: "active"}
	if op.WorkstationID > 0 {
		query.WorkstationID = op.WorkstationID
	}
	rows, err := s.repo.ListManufacturingWorkstationCapacities(ctx, query)
	if err != nil {
		return op, err
	}
	for _, row := range rows {
		if row.ID != op.WorkstationCapacityID {
			continue
		}
		op.WorkstationID = row.WorkstationID
		op.Workstation = row.Workstation
		op.WorkstationCapacityName = row.Name
		op.BatchSizeQty = row.BatchSizeQty
		op.BatchSizeUnit = row.BatchSizeUnit
		op.StandardMinutes = row.StandardMinutes
		op.HourlyRate = row.HourlyRate
		op.CostMethod = normalizeCapacityCostMethod(row.CostMethod)
		op.PieceRate = row.PieceRate
		op.DefaultMinutes = row.StandardMinutes
		op.PlannedBatchCount = 1
		op.PlannedMinutes = row.StandardMinutes
		if op.CostMethod == "piece" && op.PieceRate > 0 {
			op.PlannedOperationCost = op.PieceRate
		} else if op.BatchSizeQty > 0 && op.StandardMinutes > 0 && op.HourlyRate > 0 {
			op.PlannedOperationCost = roundMoney((float64(op.StandardMinutes) / 60 * op.HourlyRate) / op.BatchSizeQty)
		} else {
			op.PlannedOperationCost = 0
		}
		return op, nil
	}
	return op, fmt.Errorf("workstation capacity does not belong to workstation")
}

func containsInt64(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func validateOperationCostSnapshot(batchSizeQty float64, standardMinutes int, hourlyRate float64, plannedBatchCount int, plannedMinutes int, plannedOperationCost float64) error {
	if batchSizeQty < 0 {
		return fmt.Errorf("batch_size_qty must be >= 0")
	}
	if standardMinutes < 0 {
		return fmt.Errorf("standard_minutes must be >= 0")
	}
	if hourlyRate < 0 {
		return fmt.Errorf("hourly_rate must be >= 0")
	}
	if plannedBatchCount < 0 {
		return fmt.Errorf("planned_batch_count must be >= 0")
	}
	if plannedMinutes < 0 {
		return fmt.Errorf("planned_minutes must be >= 0")
	}
	if plannedOperationCost < 0 {
		return fmt.Errorf("planned_operation_cost must be >= 0")
	}
	return nil
}

func normalizePositiveInt64IDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func normalizeJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", err
	}
	if obj == nil {
		obj = map[string]any{}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func normalizeJSONArray(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "[]"
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return "", err
	}
	if arr == nil {
		arr = []any{}
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
