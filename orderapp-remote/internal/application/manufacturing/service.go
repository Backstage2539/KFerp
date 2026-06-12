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
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	DefaultMinutes int    `json:"default_minutes"`
	Note           string `json:"note"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ManufacturingWorkstation struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	DefaultMinutes int     `json:"default_minutes"`
	HourlyRate     float64 `json:"hourly_rate"`
	Note           string  `json:"note"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type ManufacturingWorkstationCapacity struct {
	ID                 int64   `json:"id"`
	WorkstationID      int64   `json:"workstation_id"`
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	Status             string  `json:"status"`
	BatchSizeQty       float64 `json:"batch_size_qty"`
	BatchSizeUnit      string  `json:"batch_size_unit"`
	StandardMinutes    int     `json:"standard_minutes"`
	HourlyRate         float64 `json:"hourly_rate"`
	ProductionCapacity int     `json:"production_capacity"`
	SortOrder          int     `json:"sort_order"`
	Note               string  `json:"note"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
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
	ID                      int64   `json:"id"`
	RouteID                 int64   `json:"route_id"`
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
	PlannedBatchCount       int     `json:"planned_batch_count"`
	PlannedMinutes          int     `json:"planned_minutes"`
	PlannedOperationCost    float64 `json:"planned_operation_cost"`
	RecordsLoss             bool    `json:"records_loss"`
	QualityChecklistJSON    string  `json:"quality_checklist_json"`
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
	ID             int64
	Code           string
	Name           string
	Status         string
	DefaultMinutes int
	Note           string
	Actor          string
}

type SaveManufacturingWorkstationCommand struct {
	ID             int64
	Code           string
	Name           string
	Status         string
	DefaultMinutes int
	HourlyRate     float64
	Note           string
	Actor          string
}

type SaveWorkstationCapacityCommand struct {
	ID                 int64
	WorkstationID      int64
	Code               string
	Name               string
	Status             string
	BatchSizeQty       float64
	BatchSizeUnit      string
	StandardMinutes    int
	HourlyRate         float64
	ProductionCapacity int
	SortOrder          int
	Note               string
	Actor              string
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

type IndustryCalculatorPreviewCommand struct {
	IndustryKey string             `json:"industry_key"`
	Inputs      map[string]float64 `json:"inputs"`
	Config      map[string]float64 `json:"config"`
}

type IndustryCalculatorPreviewLine struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type IndustryCalculatorPreviewResult struct {
	IndustryKey    string                          `json:"industry_key"`
	DemandOutputG  int64                           `json:"demand_output_g"`
	PlannedInputG  int64                           `json:"planned_input_g"`
	ExpectedLossG  int64                           `json:"expected_loss_g"`
	LossRate       float64                         `json:"loss_rate"`
	YieldRate      float64                         `json:"yield_rate"`
	MaterialCost   float64                         `json:"material_cost"`
	OperationCost  float64                         `json:"operation_cost"`
	TotalCost      float64                         `json:"total_cost"`
	CalculatorMode string                          `json:"calculator_mode"`
	Lines          []IndustryCalculatorPreviewLine `json:"lines"`
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
	if cmd.HourlyRate < 0 {
		return ManufacturingWorkstation{}, fmt.Errorf("hourly_rate must be >= 0")
	}
	if cmd.Status != "active" && cmd.Status != "inactive" {
		return ManufacturingWorkstation{}, fmt.Errorf("invalid status")
	}
	if cmd.Code == "" {
		cmd.Code = codeFromName(cmd.Name)
	}
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
	if cmd.HourlyRate < 0 {
		return ManufacturingWorkstationCapacity{}, fmt.Errorf("hourly_rate must be >= 0")
	}
	if cmd.ProductionCapacity <= 0 {
		cmd.ProductionCapacity = 1
	}
	if cmd.BatchSizeUnit == "" {
		cmd.BatchSizeUnit = "unit"
	}
	if cmd.Code == "" {
		cmd.Code = codeFromName(cmd.Name)
	}
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

func (s *Service) PreviewIndustryCalculator(ctx context.Context, cmd IndustryCalculatorPreviewCommand) (IndustryCalculatorPreviewResult, error) {
	industryKey := strings.TrimSpace(cmd.IndustryKey)
	if industryKey == "" {
		industryKey = "general"
	}
	demandOutputG := int64(math.Round(firstCalculatorValue(cmd.Inputs, cmd.Config, "demand_output_g", "output_g", "qty_g")))
	if demandOutputG <= 0 {
		return IndustryCalculatorPreviewResult{}, fmt.Errorf("demand_output_g required")
	}
	lossRate, hasLossRate := calculatorValue(cmd.Config, "loss_rate")
	if !hasLossRate {
		lossRate, hasLossRate = calculatorValue(cmd.Inputs, "loss_rate")
	}
	if !hasLossRate {
		lossRate = defaultIndustryLossRate(industryKey)
	}
	if lossRate < 0 || lossRate >= 1 {
		return IndustryCalculatorPreviewResult{}, fmt.Errorf("loss_rate must be between 0 and 1")
	}
	yieldRate := 1 - lossRate
	plannedInputG := int64(math.Ceil(float64(demandOutputG) / yieldRate))
	expectedLossG := plannedInputG - demandOutputG
	unitCostPerKG := firstCalculatorValue(cmd.Config, cmd.Inputs, "material_unit_cost_per_kg", "unit_cost_per_kg")
	operationMinutes := firstCalculatorValue(cmd.Config, cmd.Inputs, "operation_minutes", "default_minutes")
	hourlyRate := firstCalculatorValue(cmd.Config, cmd.Inputs, "hourly_rate", "operation_hourly_rate")
	if unitCostPerKG < 0 || operationMinutes < 0 || hourlyRate < 0 {
		return IndustryCalculatorPreviewResult{}, fmt.Errorf("calculator cost inputs must be >= 0")
	}
	materialCost := roundMoney(float64(plannedInputG) / 1000 * unitCostPerKG)
	operationCost := roundMoney(operationMinutes / 60 * hourlyRate)
	totalCost := roundMoney(materialCost + operationCost)
	return IndustryCalculatorPreviewResult{
		IndustryKey:    industryKey,
		DemandOutputG:  demandOutputG,
		PlannedInputG:  plannedInputG,
		ExpectedLossG:  expectedLossG,
		LossRate:       lossRate,
		YieldRate:      yieldRate,
		MaterialCost:   materialCost,
		OperationCost:  operationCost,
		TotalCost:      totalCost,
		CalculatorMode: "generic_manufacturing",
		Lines: []IndustryCalculatorPreviewLine{
			{Key: "demand_output_g", Label: "需求产出", Value: float64(demandOutputG), Unit: "g"},
			{Key: "planned_input_g", Label: "计划投入", Value: float64(plannedInputG), Unit: "g"},
			{Key: "expected_loss_g", Label: "预计损耗", Value: float64(expectedLossG), Unit: "g"},
			{Key: "loss_rate", Label: "损耗率", Value: lossRate, Unit: "ratio"},
			{Key: "total_cost", Label: "预计成本", Value: totalCost, Unit: "currency"},
		},
	}, nil
}

func (s *Service) ListProcessTemplates(ctx context.Context, query ProcessTemplateQuery) ([]ProcessTemplate, error) {
	query.Status = strings.TrimSpace(query.Status)
	return s.repo.ListProcessTemplates(ctx, query)
}

func defaultIndustryLossRate(industryKey string) float64 {
	switch strings.TrimSpace(industryKey) {
	case "coffee":
		return 0.18
	case "packaging_box":
		return 0.03
	case "garment", "apparel":
		return 0.08
	default:
		return 0.05
	}
}

func firstCalculatorValue(primary, fallback map[string]float64, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := calculatorValue(primary, key); ok {
			return value
		}
	}
	for _, key := range keys {
		if value, ok := calculatorValue(fallback, key); ok {
			return value
		}
	}
	return 0
}

func calculatorValue(values map[string]float64, key string) (float64, bool) {
	if values == nil {
		return 0, false
	}
	value, ok := values[key]
	return value, ok
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
	op.WorkstationID = 0
	op.WorkstationCapacityID = 0
	op.Workstation = ""
	op.WorkstationCapacityName = ""
	op.DefaultEquipment = ""
	op.DefaultMinutes = 0
	op.BatchSizeQty = 0
	op.BatchSizeUnit = ""
	op.StandardMinutes = 0
	op.HourlyRate = 0
	op.PlannedBatchCount = 0
	op.PlannedMinutes = 0
	op.PlannedOperationCost = 0
	op.QualityChecklistJSON = qualityChecklistJSON
	return op, nil
}

func (s *Service) applyWorkstationCapacitySnapshot(ctx context.Context, op ProcessRouteOperation) (ProcessRouteOperation, error) {
	if op.WorkstationCapacityID <= 0 {
		return op, nil
	}
	if op.WorkstationID <= 0 {
		return op, fmt.Errorf("workstation capacity requires workstation_id")
	}
	rows, err := s.repo.ListManufacturingWorkstationCapacities(ctx, WorkstationCapacityQuery{WorkstationID: op.WorkstationID, Status: "active"})
	if err != nil {
		return op, err
	}
	for _, row := range rows {
		if row.ID != op.WorkstationCapacityID {
			continue
		}
		op.WorkstationCapacityName = row.Name
		op.BatchSizeQty = row.BatchSizeQty
		op.BatchSizeUnit = row.BatchSizeUnit
		op.StandardMinutes = row.StandardMinutes
		op.HourlyRate = row.HourlyRate
		if op.DefaultMinutes == 0 && row.StandardMinutes > 0 {
			op.DefaultMinutes = row.StandardMinutes
		}
		if op.PlannedMinutes == 0 && op.PlannedBatchCount > 0 && op.StandardMinutes > 0 {
			op.PlannedMinutes = op.PlannedBatchCount * op.StandardMinutes
		}
		if op.PlannedOperationCost == 0 && op.PlannedMinutes > 0 && op.HourlyRate > 0 {
			op.PlannedOperationCost = roundMoney(float64(op.PlannedMinutes) / 60 * op.HourlyRate)
		}
		return op, nil
	}
	return op, fmt.Errorf("workstation capacity does not belong to workstation")
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
