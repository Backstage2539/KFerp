package production

import (
	"context"
	"fmt"
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
	ProductID   int64
	ProductName string
	SpecG       int64
	GapG        int64
	OrderNos    string
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
	Operator         string
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
	ProductID int64  `json:"product_id"`
	Product   string `json:"product"`
	OrderNos  string `json:"order_nos"`
	SpecG     int64  `json:"spec_g"`
	NeedUnits int64  `json:"need_units"`
	NeedG     int64  `json:"need_g"`
	InvUnits  int64  `json:"inv_units"`
	InvLooseG int64  `json:"inv_loose_g"`
	InvG      int64  `json:"inv_g"`
	GapG      int64  `json:"gap_g"`
}

type MaterialNeed struct {
	Name string `json:"name"`
	Qty  int64  `json:"qty"`
	Unit string `json:"unit"`
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
	Key           string  `json:"key"`
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	SpecG         int64   `json:"spec_g"`
	Machine       string  `json:"machine"`
	BatchCount    int64   `json:"batch_count"`
	BatchG        int64   `json:"batch_g"`
	FinalInputG   int64   `json:"final_input_g"`
	NeedG         int64   `json:"need_g"`
	YieldRate     float64 `json:"yield_rate"`
	YieldPctStr   string  `json:"yield_pct_str"`
	FinishedKgStr string  `json:"finished_kg_str"`
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
	ID            int64   `json:"id"`
	WorkOrderNo   string  `json:"work_order_no"`
	RunningItemID int64   `json:"running_item_id"`
	BatchID       string  `json:"batch_id"`
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	SpecG         int64   `json:"spec_g"`
	PlannedG      int64   `json:"planned_g"`
	Status        string  `json:"status"`
	ActualCost    float64 `json:"actual_cost"`
	CreatedAt     string  `json:"created_at"`
	CompletedAt   string  `json:"completed_at"`
}

type JobCardQuery struct {
	Status string
	Limit  int
}

type JobCardRow struct {
	ID          int64  `json:"id"`
	WorkOrderID int64  `json:"work_order_id"`
	Operation   string `json:"operation"`
	Workstation string `json:"workstation"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	Operator    string `json:"operator"`
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

type Repository interface {
	CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error)
	ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error)
	Detail(ctx context.Context, batchID string) (BatchDetail, error)
	PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error)
	ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error)
	ListRunning(ctx context.Context) ([]RunningItem, error)
	ListStartNeeds(ctx context.Context, cmd StartCommand) ([]StartNeed, error)
	Start(ctx context.Context, cmd StartExecutionCommand) (StartResult, error)
	Finish(ctx context.Context, cmd FinishCommand) error
	Cancel(ctx context.Context, cmd CancelCommand) error
	ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error)
	SaveMachine(ctx context.Context, cmd RoastMachineCommand) error
	PlanSummary(ctx context.Context, query PlanSummaryQuery) (PlanSummaryData, error)
	ListProductionLogs(ctx context.Context, query ProductionLogsQuery) (ProductionLogsResult, error)
	ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error)
	ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error)
	ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error)
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

func (s *Service) Finish(ctx context.Context, cmd FinishCommand) error {
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
		return fmt.Errorf("invalid allowed_specs")
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

func (s *Service) ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error) {
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListBatchCosts(ctx, query)
}
