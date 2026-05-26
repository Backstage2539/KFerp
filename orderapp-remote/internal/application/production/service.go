package production

import (
	"context"
	"encoding/json"
	"fmt"
	productiondomain "orderapp/internal/domain/production"
	stockdomain "orderapp/internal/domain/stock"
	"strings"
	"time"
)

type CreateBatchCommand struct {
	OrderIDs             []int64
	Operator             string
	IdempotencyKey       string
	RequestUnitsByItemID map[int64]int64
}

type SummaryItem struct {
	ProductID   int64
	ProductName string
	SpecG       int64
	NeedUnits   int64
	NeedG       int64
	DeductedG   int64
	GapG        int64
}

type CreateBatchResult struct {
	BatchID    string
	OrderCount int
	Summary    []SummaryItem
}

type DeductPreviewItem struct {
	ProductID       int64
	ProductName     string
	SpecG           int64
	NeedUnits       int64
	NeedG           int64
	InvUnits        int64
	InvLooseG       int64
	InvTotalG       int64
	DeductedG       int64
	GapG            int64
	WarningLowStock bool
}

type DeductPreview struct {
	BatchID string
	Summary []DeductPreviewItem
}

type DeductConfirmResult struct {
	BatchID string
	Status  string
	Summary []SummaryItem
}

type ListBatchesCommand struct {
	Limit    int
	Status   string
	Operator string
	From     string
	To       string
}

type BatchListItem struct {
	BatchID      string
	Status       string
	Operator     string
	CreatedAt    string
	OrderCount   int64
	DeductStatus string
	DeductedAt   string
	NeedG        int64
	DeductedG    int64
	GapG         int64

	CreatedBy       string
	CreatedTime     string
	StatusChangedAt string
	StatusText      string
	CreateTime      string
	DeductTime      string
	DeductState     string
}

type BatchDetail struct {
	BatchID      string
	Status       string
	Operator     string
	CreatedAt    string
	Orders       []int64
	Summary      []SummaryItem
	CreatedBy    string
	CreatedTime  string
	StatusSource string
}

type RunningItem struct {
	ID            int64
	BatchID       string
	ProductID     int64
	ProductName   string
	SpecG         int64
	NeedG         int64
	InputG        int64
	BomYieldRate  float64
	PlanUnits     int64
	PlanLooseG    int64
	OrderNos      string
	StartedBy     string
	StartedAt     string
	StartedAtTime time.Time
	Outputs       []RunningOutput
}

type RunningOutput struct {
	ID             int64  `json:"id"`
	SpecG          int64  `json:"spec_g"`
	NeedG          int64  `json:"need_g"`
	OrderNos       string `json:"order_nos"`
	PlanUnits      int64  `json:"plan_units"`
	PlanLooseG     int64  `json:"plan_loose_g"`
	FinishedUnits  int64  `json:"finished_units"`
	FinishedLooseG int64  `json:"finished_loose_g"`
}

type StartCommand struct {
	From       string
	To         string
	CustomerID int64
	Selected   map[string]bool
	InputByKey map[string]int64
	Operator   string
}

type StartNeed struct {
	ProductID           int64
	ProductName         string
	SpecG               int64
	GapG                int64
	OrderNos            string
	OperationTemplateID int64
}

type StartResult struct {
	BatchID string
}

type StartExecutionCommand struct {
	Needs      []StartNeed
	InputByKey map[string]int64
	Operator   string
}

type FinishCommand struct {
	ID               int64
	FinishedUnits    int64
	FinishedLooseG   int64
	HasFinishedInput bool
	Warehouse        string
	Partial          bool
	ConsumedInputG   int64
	Operator         string
	Outputs          []FinishOutputCommand
}

type FinishedOrder struct {
	OrderID int64  `json:"order_id"`
	OrderNo string `json:"order_no"`
}

type FinishResult struct {
	RunningItemID  int64           `json:"running_item_id"`
	Completed      bool            `json:"completed"`
	FinishedOrders []FinishedOrder `json:"finished_orders,omitempty"`
}

type FinishOutputCommand struct {
	SpecG          int64 `json:"spec_g"`
	FinishedUnits  int64 `json:"finished_units"`
	FinishedLooseG int64 `json:"finished_loose_g"`
}

type CancelCommand struct {
	ID       int64
	Operator string
}

type RoastMachine struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CapacityG    int64  `json:"capacity_g"`
	AllowedSpecs string `json:"allowed_specs"`
	MinRoastG    int64  `json:"min_roast_g"`
	Active       bool   `json:"active"`
}

type RoastMachineCommand struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CapacityG    int64  `json:"capacity_g"`
	AllowedSpecs string `json:"allowed_specs"`
	MinRoastG    int64  `json:"min_roast_g"`
	Active       bool   `json:"active"`
}

type UnprodNeedRow struct {
	ProductID                         int64  `json:"product_id"`
	Product                           string `json:"product"`
	OrderNos                          string `json:"order_nos"`
	SpecG                             int64  `json:"spec_g"`
	NeedUnits                         int64  `json:"need_units"`
	NeedG                             int64  `json:"need_g"`
	InvUnits                          int64  `json:"inv_units"`
	InvLooseG                         int64  `json:"inv_loose_g"`
	InvG                              int64  `json:"inv_g"`
	GapG                              int64  `json:"gap_g"`
	ProductionKind                    string `json:"production_kind,omitempty"`
	ProductTypeCategoryID             int64  `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID          int64  `json:"product_subtype_category_id,omitempty"`
	ProductTypeName                   string `json:"product_type_name,omitempty"`
	ProductSubtypeName                string `json:"product_subtype_name,omitempty"`
	OperationTemplateID               int64  `json:"operation_template_id,omitempty"`
	NeedBags                          int64  `json:"need_bags,omitempty"`
	NeedBoxes                         int64  `json:"need_boxes,omitempty"`
	UpstreamProductID                 int64  `json:"upstream_product_id,omitempty"`
	UpstreamRoastDemandG              int64  `json:"upstream_roast_demand_g,omitempty"`
	UpstreamShortageG                 int64  `json:"upstream_shortage_g,omitempty"`
	FinishedProductComponentShortageG int64  `json:"finished_product_component_shortage_g,omitempty"`
}

type MaterialNeed struct {
	Name                   string `json:"name"`
	Qty                    int64  `json:"qty"`
	Unit                   string `json:"unit"`
	ComponentType          string `json:"component_type,omitempty"`
	UpstreamProductID      int64  `json:"upstream_product_id,omitempty"`
	UpstreamShortageG      int64  `json:"upstream_shortage_g,omitempty"`
	WIPG                   int64  `json:"wip_g,omitempty"`
	AvailableG             int64  `json:"available_g,omitempty"`
	RawG                   int64  `json:"raw_g,omitempty"`
	ReservedG              int64  `json:"reserved_g,omitempty"`
	WIPTransferSuggestionG int64  `json:"wip_transfer_suggestion_g,omitempty"`
	ShortageG              int64  `json:"shortage_g,omitempty"`
	PurchaseSuggestionG    int64  `json:"purchase_suggestion_g,omitempty"`
}

type ProducePlanDisplayRow struct {
	UnprodNeedRow
	BomYieldRate float64 `json:"bom_yield_rate"`
	InputG       int64   `json:"input_g"`
}

type PlanSummaryQuery struct {
	From       string
	To         string
	CustomerID int64
	Selected   map[string]bool
	Plan       bool
}

type RoastPlanRow struct {
	Key                 string  `json:"key"`
	ProductID           int64   `json:"product_id"`
	ProductName         string  `json:"product_name"`
	SpecG               int64   `json:"spec_g"`
	Machine             string  `json:"machine"`
	BatchCount          int64   `json:"batch_count"`
	BatchG              int64   `json:"batch_g"`
	FinalInputG         int64   `json:"final_input_g"`
	NeedG               int64   `json:"need_g"`
	OperationTemplateID int64   `json:"operation_template_id,omitempty"`
	YieldRate           float64 `json:"yield_rate"`
	YieldPctStr         string  `json:"yield_pct_str"`
	FinishedKgStr       string  `json:"finished_kg_str"`
}

type RoastPlanMaterialRatio struct {
	Key          string  `json:"key"`
	ProductID    int64   `json:"product_id"`
	SpecG        int64   `json:"spec_g"`
	ProductName  string  `json:"product_name"`
	MaterialName string  `json:"material_name"`
	MaterialUnit string  `json:"material_unit"`
	RatioPct     float64 `json:"ratio_pct"`
}

type RoastSplitRow struct {
	Material    string `json:"material"`
	Machine     string `json:"machine"`
	BatchKg     string `json:"batch_kg"`
	Batches     int64  `json:"batches"`
	TotalKg     string `json:"total_kg"`
	YieldPctStr string `json:"yield_pct_str"`
}

type PlanSummaryData struct {
	From           string                   `json:"from"`
	To             string                   `json:"to"`
	CustomerID     int64                    `json:"customer_id"`
	Rows           []UnprodNeedRow          `json:"rows"`
	PlanRows       []ProducePlanDisplayRow  `json:"plan_rows"`
	Materials      []MaterialNeed           `json:"materials"`
	RoastSplits    []RoastSplitRow          `json:"roast_splits"`
	RoastPlans     []RoastPlanRow           `json:"roast_plans"`
	MaterialRatios []RoastPlanMaterialRatio `json:"material_ratios"`
	Selected       map[string]bool          `json:"selected"`
	PlanReady      bool                     `json:"plan_ready"`
	StockTip       string                   `json:"stock_tip"`
	Error          string                   `json:"error"`
}

type ProductionLogsQuery struct {
	From      string
	To        string
	ProductID int64
	BatchID   string
	Operator  string
	Limit     int
}

type ProductionLogProductOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ProductionLogRow struct {
	ID                    int64   `json:"id"`
	BatchID               string  `json:"batch_id"`
	ProductID             int64   `json:"product_id"`
	ProductName           string  `json:"product_name"`
	SpecG                 int64   `json:"spec_g"`
	OrderNos              string  `json:"order_nos"`
	PlannedNeedG          int64   `json:"planned_need_g"`
	InputG                int64   `json:"input_g"`
	BomYieldRate          float64 `json:"bom_yield_rate"`
	FinishedUnits         int64   `json:"finished_units"`
	FinishedLooseG        int64   `json:"finished_loose_g"`
	FinishedTotalG        int64   `json:"finished_total_g"`
	ActualYieldRate       float64 `json:"actual_yield_rate"`
	StartedBy             string  `json:"started_by"`
	StartedAt             string  `json:"started_at"`
	FinishedBy            string  `json:"finished_by"`
	FinishedAt            string  `json:"finished_at"`
	InventoryUnitsBefore  int64   `json:"inventory_units_before"`
	InventoryLooseGBefore int64   `json:"inventory_loose_g_before"`
	InventoryUnitsAfter   int64   `json:"inventory_units_after"`
	InventoryLooseGAfter  int64   `json:"inventory_loose_g_after"`
	MaterialSummary       string  `json:"material_summary"`
}

type ProductionLogsResult struct {
	Products []ProductionLogProductOption
	Rows     []ProductionLogRow
}

type WorkOrderQuery struct {
	Status string
	Limit  int
}

type WorkOrderRow struct {
	ID                    int64   `json:"id"`
	WorkOrderNo           string  `json:"work_order_no"`
	RunningItemID         int64   `json:"running_item_id"`
	BatchID               string  `json:"batch_id"`
	ProductID             int64   `json:"product_id"`
	ProductName           string  `json:"product_name"`
	SpecG                 int64   `json:"spec_g"`
	PlannedG              int64   `json:"planned_g"`
	Status                string  `json:"status"`
	ActualCost            float64 `json:"actual_cost"`
	CreatedAt             string  `json:"created_at"`
	CompletedAt           string  `json:"completed_at"`
	RoastLevel            string  `json:"roast_level"`
	YieldRate             float64 `json:"yield_rate"`
	ExpectedYieldRate     float64 `json:"expected_yield_rate"`
	ExpectedLossRate      float64 `json:"expected_loss_rate"`
	SuggestedInputG       int64   `json:"suggested_input_g"`
	SuggestedMachine      string  `json:"suggested_machine"`
	SuggestedBatchCount   int64   `json:"suggested_batch_count"`
	SuggestedBatchG       int64   `json:"suggested_batch_g"`
	SuggestedBatchPlan    string  `json:"suggested_batch_plan"`
	PlannedUnits          int64   `json:"planned_units"`
	PlannedLooseG         int64   `json:"planned_loose_g"`
	MaterialSummary       string  `json:"material_summary"`
	OrderNos              string  `json:"order_nos"`
	WIPReservedG          int64   `json:"wip_reserved_g"`
	WIPConsumedG          int64   `json:"wip_consumed_g"`
	WIPRemainingReservedG int64   `json:"remaining_reserved_g"`
	BomVersionID          int64   `json:"bom_version_id"`
	ProcessTemplateID     int64   `json:"process_template_id"`
	ProcessSnapshotJSON   string  `json:"process_snapshot_json"`
	OperationSummaryJSON  string  `json:"operation_summary_json"`
}

type JobCardQuery struct {
	Status string
	Limit  int
}

type JobCardRow struct {
	ID              int64   `json:"id"`
	WorkOrderID     int64   `json:"work_order_id"`
	Operation       string  `json:"operation"`
	Workstation     string  `json:"workstation"`
	Status          string  `json:"status"`
	StartedAt       string  `json:"started_at"`
	CompletedAt     string  `json:"completed_at"`
	Operator        string  `json:"operator"`
	PlannedInputQty float64 `json:"planned_input_qty"`
	ActualInputQty  float64 `json:"actual_input_qty"`
	ActualOutputQty float64 `json:"actual_output_qty"`
	ActualLossQty   float64 `json:"actual_loss_qty"`
	ActualLossRate  float64 `json:"actual_loss_rate"`
	ExceptionReason string  `json:"exception_reason"`
	MetricsJSON     string  `json:"metrics_json"`
}

type JobCardActualsCommand struct {
	ID              int64
	PlannedInputQty float64
	ActualInputQty  float64
	ActualOutputQty float64
	ActualLossQty   float64
	ActualLossRate  float64
	ExceptionReason string
	MetricsJSON     string
	Actor           string
}

type BatchCostQuery struct {
	Limit int
}

type BatchCostRow struct {
	ID            int64   `json:"id"`
	RunningItemID int64   `json:"running_item_id"`
	BatchID       string  `json:"batch_id"`
	ProductName   string  `json:"product_name"`
	MaterialCost  float64 `json:"material_cost"`
	OperationCost float64 `json:"operation_cost"`
	TotalCost     float64 `json:"total_cost"`
	FinishedG     int64   `json:"finished_g"`
	UnitCostPerKG float64 `json:"unit_cost_per_kg"`
	CreatedAt     string  `json:"created_at"`
}

type MaterialPlanQuery struct {
	From       string
	To         string
	CustomerID int64
	Selected   map[string]bool
	InputByKey map[string]int64
}

type MaterialPlanRow struct {
	MaterialID             int64  `json:"material_id"`
	MaterialName           string `json:"material_name"`
	Unit                   string `json:"unit"`
	ComponentType          string `json:"component_type,omitempty"`
	UpstreamProductID      int64  `json:"upstream_product_id,omitempty"`
	UpstreamShortageG      int64  `json:"upstream_shortage_g,omitempty"`
	RequiredG              int64  `json:"required_g"`
	RequiredUnits          int64  `json:"required_units"`
	WIPG                   int64  `json:"wip_g"`
	AvailableG             int64  `json:"available_g"`
	RawG                   int64  `json:"raw_g"`
	ReservedG              int64  `json:"reserved_g"`
	WIPTransferSuggestionG int64  `json:"wip_transfer_suggestion_g"`
	ShortageG              int64  `json:"shortage_g"`
	PurchaseSuggestionG    int64  `json:"purchase_suggestion_g"`
}

type MaterialPlanResult struct {
	Rows []MaterialPlanRow `json:"rows"`
}

type QualityInspectionCommand struct {
	Scope                    string
	ReferenceType            string
	ReferenceNo              string
	ItemName                 string
	Result                   string
	MetricsJSON              string
	FactoryFlavorDescription string
	Moisture                 string
	Density                  string
	Note                     string
	Operator                 string
}

type QualityInspectionQuery struct {
	Scope  string
	Result string
	Limit  int
}

type QualityInspectionRow struct {
	ID            int64  `json:"id"`
	Scope         string `json:"scope"`
	ReferenceType string `json:"reference_type"`
	ReferenceNo   string `json:"reference_no"`
	ItemName      string `json:"item_name"`
	Result        string `json:"result"`
	MetricsJSON   string `json:"metrics_json"`
	Note          string `json:"note"`
	Operator      string `json:"operator"`
	CreatedAt     string `json:"created_at"`
}

type WIPReservationQuery struct {
	Status      string
	WorkOrderNo string
	MaterialID  int64
	Limit       int
}

type WIPReservationRow struct {
	ID                 int64  `json:"id"`
	WorkOrderID        int64  `json:"work_order_id"`
	WorkOrderNo        string `json:"work_order_no"`
	RunningItemID      int64  `json:"running_item_id"`
	ProductName        string `json:"product_name"`
	MaterialID         int64  `json:"material_id"`
	MaterialName       string `json:"material_name"`
	Unit               string `json:"unit"`
	RequiredG          int64  `json:"required_g"`
	RequiredUnits      int64  `json:"required_units"`
	ReservedG          int64  `json:"reserved_g"`
	ReservedUnits      int64  `json:"reserved_units"`
	ConsumedG          int64  `json:"consumed_g"`
	ConsumedUnits      int64  `json:"consumed_units"`
	ReturnedG          int64  `json:"returned_g"`
	ReturnedUnits      int64  `json:"returned_units"`
	RemainingReservedG int64  `json:"remaining_reserved_g"`
	Status             string `json:"status"`
	WIPG               int64  `json:"wip_g"`
	AvailableG         int64  `json:"available_g"`
	UpdatedAt          string `json:"updated_at"`
}

type WIPReservationResult struct {
	Rows            []WIPReservationRow `json:"rows"`
	TotalReservedG  int64               `json:"total_reserved_g"`
	TotalConsumedG  int64               `json:"total_consumed_g"`
	TotalRemainingG int64               `json:"total_remaining_g"`
}

type WIPReservationAdjustCommand struct {
	ReservationID int64
	ReservedG     int64
	ReservedUnits int64
	Operator      string
	Note          string
}

type WIPReservationReleaseCommand struct {
	RunningItemID int64
	WorkOrderNo   string
	Operator      string
	Note          string
}

type WIPReservationReleaseResult struct {
	ReleasedCount int64 `json:"released_count"`
	ReleasedG     int64 `json:"released_g"`
	ReleasedUnits int64 `json:"released_units"`
}

type AcceptanceSmokeRow struct {
	Code       string            `json:"code"`
	Title      string            `json:"title"`
	Status     string            `json:"status"`
	Count      int64             `json:"count"`
	Detail     string            `json:"detail"`
	View       string            `json:"view"`
	ViewParams map[string]string `json:"view_params,omitempty"`
}

type AcceptanceSmokeResult struct {
	Rows []AcceptanceSmokeRow `json:"rows"`
}

type Repository interface {
	CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error)
	ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error)
	Detail(ctx context.Context, batchID string) (BatchDetail, error)
	PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error)
	ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error)
	ListRunning(ctx context.Context) ([]RunningItem, error)
	ListStartNeeds(ctx context.Context, cmd StartCommand) ([]StartNeed, error)
	Start(ctx context.Context, cmd StartExecutionCommand) (StartResult, error)
	Finish(ctx context.Context, cmd FinishCommand) (FinishResult, error)
	Cancel(ctx context.Context, cmd CancelCommand) error
	ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error)
	SaveMachine(ctx context.Context, cmd RoastMachineCommand) error
	PlanSummary(ctx context.Context, query PlanSummaryQuery) (PlanSummaryData, error)
	ListProductionLogs(ctx context.Context, query ProductionLogsQuery) (ProductionLogsResult, error)
	ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error)
	ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error)
	UpdateJobCardActuals(ctx context.Context, cmd JobCardActualsCommand) error
	ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error)
	MaterialPlan(ctx context.Context, query MaterialPlanQuery) (MaterialPlanResult, error)
	CreateQualityInspection(ctx context.Context, cmd QualityInspectionCommand) (QualityInspectionRow, error)
	ListQualityInspections(ctx context.Context, query QualityInspectionQuery) ([]QualityInspectionRow, error)
	ListWIPReservations(ctx context.Context, query WIPReservationQuery) (WIPReservationResult, error)
	AdjustWIPReservation(ctx context.Context, cmd WIPReservationAdjustCommand) (WIPReservationRow, error)
	ReleaseWIPReservations(ctx context.Context, cmd WIPReservationReleaseCommand) (WIPReservationReleaseResult, error)
	AcceptanceSmoke(ctx context.Context) (AcceptanceSmokeResult, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error) {
	return s.repo.CreateBatch(ctx, cmd)
}

func (s *Service) ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error) {
	if cmd.Limit <= 0 {
		cmd.Limit = 20
	}
	if cmd.Limit > 200 {
		cmd.Limit = 200
	}
	cmd.Status = strings.TrimSpace(cmd.Status)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.From = strings.TrimSpace(cmd.From)
	cmd.To = strings.TrimSpace(cmd.To)
	return s.repo.ListBatches(ctx, cmd)
}

func (s *Service) Detail(ctx context.Context, batchID string) (BatchDetail, error) {
	batchID = strings.TrimSpace(batchID)
	if batchID == "" {
		return BatchDetail{}, fmt.Errorf("batch_id required")
	}
	return s.repo.Detail(ctx, batchID)
}

func (s *Service) PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error) {
	return s.repo.PreviewDeduct(ctx, batchID)
}

func (s *Service) ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error) {
	return s.repo.ConfirmDeduct(ctx, batchID, operator)
}

func (s *Service) ListRunning(ctx context.Context) ([]RunningItem, error) {
	return s.repo.ListRunning(ctx)
}

func (s *Service) Finish(ctx context.Context, cmd FinishCommand) (FinishResult, error) {
	cmd.Warehouse = strings.TrimSpace(cmd.Warehouse)
	if cmd.Warehouse == "" {
		cmd.Warehouse = stockdomain.WarehouseFinishedGoods
	}
	if cmd.ConsumedInputG < 0 {
		return FinishResult{}, fmt.Errorf("consumed_input_g must be >= 0")
	}
	return s.repo.Finish(ctx, cmd)
}

func (s *Service) Cancel(ctx context.Context, cmd CancelCommand) error {
	return s.repo.Cancel(ctx, cmd)
}

func (s *Service) ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error) {
	return s.repo.ListMachines(ctx, activeOnly)
}

func (s *Service) SaveMachine(ctx context.Context, cmd RoastMachineCommand) error {
	cmd.Name = strings.TrimSpace(cmd.Name)
	if cmd.Name == "" || cmd.CapacityG <= 0 {
		return fmt.Errorf("name and capacity_g required")
	}
	if cmd.MinRoastG <= 0 {
		cmd.MinRoastG = 1000
	}
	if cmd.MinRoastG > cmd.CapacityG {
		return fmt.Errorf("min_roast_g must be <= capacity_g")
	}
	loadSettings, ok := normalizeMachineLoadSettings(cmd.AllowedSpecs, cmd.MinRoastG, cmd.CapacityG)
	if !ok {
		return fmt.Errorf("allowed_specs must list roast load grams between min_roast_g and capacity_g")
	}
	cmd.AllowedSpecs = loadSettings
	return s.repo.SaveMachine(ctx, cmd)
}

func (s *Service) PlanSummary(ctx context.Context, query PlanSummaryQuery) (PlanSummaryData, error) {
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	if query.Selected == nil {
		query.Selected = map[string]bool{}
	}
	return s.repo.PlanSummary(ctx, query)
}

func (s *Service) ListProductionLogs(ctx context.Context, query ProductionLogsQuery) (ProductionLogsResult, error) {
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.BatchID = strings.TrimSpace(query.BatchID)
	query.Operator = strings.TrimSpace(query.Operator)
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListProductionLogs(ctx, query)
}

func (s *Service) ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error) {
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListWorkOrders(ctx, query)
}

func (s *Service) ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error) {
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListJobCards(ctx, query)
}

func (s *Service) UpdateJobCardActuals(ctx context.Context, cmd JobCardActualsCommand) error {
	if cmd.ID <= 0 {
		return fmt.Errorf("job_card_id required")
	}
	if cmd.ActualInputQty > 0 {
		lossQty, lossRate, err := productiondomain.ActualLossMetrics(cmd.ActualInputQty, cmd.ActualOutputQty)
		if err != nil {
			return err
		}
		cmd.ActualLossQty = lossQty
		cmd.ActualLossRate = lossRate
	}
	cmd.ExceptionReason = strings.TrimSpace(cmd.ExceptionReason)
	cmd.MetricsJSON = strings.TrimSpace(cmd.MetricsJSON)
	if cmd.MetricsJSON == "" {
		cmd.MetricsJSON = "{}"
	}
	return s.repo.UpdateJobCardActuals(ctx, cmd)
}

func (s *Service) ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error) {
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListBatchCosts(ctx, query)
}

func (s *Service) MaterialPlan(ctx context.Context, query MaterialPlanQuery) (MaterialPlanResult, error) {
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	if query.Selected == nil {
		query.Selected = map[string]bool{}
	}
	if query.InputByKey == nil {
		query.InputByKey = map[string]int64{}
	}
	res, err := s.repo.MaterialPlan(ctx, query)
	if err != nil {
		return MaterialPlanResult{}, err
	}
	summary, err := s.repo.PlanSummary(ctx, PlanSummaryQuery{
		From:       query.From,
		To:         query.To,
		CustomerID: query.CustomerID,
		Selected:   query.Selected,
		Plan:       true,
	})
	if err == nil {
		enrichMaterialPlanWithUpstream(&res, summary)
	}
	return res, nil
}

func enrichMaterialPlanWithUpstream(res *MaterialPlanResult, summary PlanSummaryData) {
	if res == nil || len(res.Rows) == 0 {
		return
	}
	upstreamShortageByProduct := map[int64]int64{}
	for _, row := range summary.PlanRows {
		if row.UpstreamProductID > 0 {
			upstreamShortageByProduct[row.UpstreamProductID] += row.UpstreamShortageG
		}
	}
	for _, item := range summary.Materials {
		if item.ComponentType == "finished_product" && item.UpstreamProductID > 0 {
			upstreamShortageByProduct[item.UpstreamProductID] += item.UpstreamShortageG
		}
	}
	for i := range res.Rows {
		if res.Rows[i].ComponentType == "" {
			if shortage, ok := upstreamShortageByProduct[res.Rows[i].MaterialID]; ok {
				res.Rows[i].ComponentType = "finished_product"
				res.Rows[i].UpstreamProductID = res.Rows[i].MaterialID
				res.Rows[i].UpstreamShortageG = shortage
			}
		}
		if res.Rows[i].ComponentType == "finished_product" && res.Rows[i].UpstreamProductID == 0 {
			res.Rows[i].UpstreamProductID = res.Rows[i].MaterialID
		}
	}
}

func (s *Service) CreateQualityInspection(ctx context.Context, cmd QualityInspectionCommand) (QualityInspectionRow, error) {
	cmd.Scope = normalizeQualityInspectionScope(cmd.Scope)
	cmd.ReferenceType = normalizeQualityInspectionScope(cmd.ReferenceType)
	cmd.ReferenceNo = strings.TrimSpace(cmd.ReferenceNo)
	cmd.ItemName = strings.TrimSpace(cmd.ItemName)
	cmd.Result = normalizeQualityInspectionResult(cmd.Result)
	cmd.MetricsJSON = strings.TrimSpace(cmd.MetricsJSON)
	cmd.FactoryFlavorDescription = strings.TrimSpace(cmd.FactoryFlavorDescription)
	cmd.Moisture = strings.TrimSpace(cmd.Moisture)
	cmd.Density = strings.TrimSpace(cmd.Density)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Scope == "" || cmd.ReferenceNo == "" || cmd.Result == "" {
		return QualityInspectionRow{}, fmt.Errorf("scope, reference_no and result required")
	}
	if cmd.ReferenceType == "" {
		cmd.ReferenceType = cmd.Scope
	}
	if !validQualityInspectionResult(cmd.Result) {
		return QualityInspectionRow{}, fmt.Errorf("invalid quality inspection result")
	}
	metricsJSON, err := mergeQualityInspectionMetrics(cmd)
	if err != nil {
		return QualityInspectionRow{}, err
	}
	cmd.MetricsJSON = metricsJSON
	return s.repo.CreateQualityInspection(ctx, cmd)
}

func (s *Service) ListQualityInspections(ctx context.Context, query QualityInspectionQuery) ([]QualityInspectionRow, error) {
	query.Scope = normalizeQualityInspectionScope(query.Scope)
	query.Result = normalizeQualityInspectionResult(query.Result)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 200
	}
	return s.repo.ListQualityInspections(ctx, query)
}

func (s *Service) ListWIPReservations(ctx context.Context, query WIPReservationQuery) (WIPReservationResult, error) {
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	query.WorkOrderNo = strings.TrimSpace(query.WorkOrderNo)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 200
	}
	return s.repo.ListWIPReservations(ctx, query)
}

func (s *Service) AdjustWIPReservation(ctx context.Context, cmd WIPReservationAdjustCommand) (WIPReservationRow, error) {
	if cmd.ReservationID <= 0 {
		return WIPReservationRow{}, fmt.Errorf("reservation_id required")
	}
	if cmd.ReservedG < 0 || cmd.ReservedUnits < 0 {
		return WIPReservationRow{}, fmt.Errorf("reserved quantity must be >= 0")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.AdjustWIPReservation(ctx, cmd)
}

func (s *Service) ReleaseWIPReservations(ctx context.Context, cmd WIPReservationReleaseCommand) (WIPReservationReleaseResult, error) {
	cmd.WorkOrderNo = strings.TrimSpace(cmd.WorkOrderNo)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.Note = strings.TrimSpace(cmd.Note)
	if cmd.RunningItemID <= 0 && cmd.WorkOrderNo == "" {
		return WIPReservationReleaseResult{}, fmt.Errorf("running_item_id or work_order_no required")
	}
	return s.repo.ReleaseWIPReservations(ctx, cmd)
}

func (s *Service) AcceptanceSmoke(ctx context.Context) (AcceptanceSmokeResult, error) {
	return s.repo.AcceptanceSmoke(ctx)
}

func normalizeQualityInspectionScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "原料", "raw", "raw_material", "material":
		return "raw_material"
	case "生产工单", "工单", "workorder", "work_order":
		return "work_order"
	case "成品批次", "成品", "finished", "finished_batch":
		return "finished_batch"
	default:
		return strings.ToLower(strings.TrimSpace(scope))
	}
}

func normalizeQualityInspectionResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "通过", "合格", "pass", "passed", "ok":
		return "pass"
	case "待定", "待处理", "待确认", "pending", "hold", "held":
		return "hold"
	case "不通过", "不合格", "失败", "reject", "rejected", "fail", "failed":
		return "reject"
	default:
		return strings.ToLower(strings.TrimSpace(result))
	}
}

func validQualityInspectionResult(result string) bool {
	switch result {
	case "pass", "hold", "reject":
		return true
	default:
		return false
	}
}

func mergeQualityInspectionMetrics(cmd QualityInspectionCommand) (string, error) {
	raw := strings.TrimSpace(cmd.MetricsJSON)
	if raw == "" {
		raw = "{}"
	}
	metrics := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &metrics); err != nil {
		return "", fmt.Errorf("metrics_json must be valid json")
	}
	if metrics == nil {
		metrics = map[string]any{}
	}
	if cmd.FactoryFlavorDescription != "" {
		metrics["factory_flavor_description"] = cmd.FactoryFlavorDescription
	}
	if cmd.Moisture != "" {
		metrics["moisture"] = cmd.Moisture
	}
	if cmd.Density != "" {
		metrics["density"] = cmd.Density
	}
	b, err := json.Marshal(metrics)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
