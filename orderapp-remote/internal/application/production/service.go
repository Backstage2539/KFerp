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

type CreateProductionPlanCommand struct {
	From       string
	To         string
	CustomerID int64
	SourceType string
	Selected   map[string]bool
	InputByKey map[string]int64
	Operator   string
}

type ProductionPlanQuery struct {
	Status    string
	TimeField string
	From      string
	To        string
	Limit     int
}

type ProductionPlanRow struct {
	ID          int64  `json:"id"`
	PlanNo      string `json:"plan_no"`
	SourceType  string `json:"source_type"`
	Status      string `json:"status"`
	ItemCount   int64  `json:"item_count"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	SubmittedBy string `json:"submitted_by"`
	SubmittedAt string `json:"submitted_at"`
	CompletedAt string `json:"completed_at"`
}

type ProductionPlanItem struct {
	ID                           int64  `json:"id"`
	PlanID                       int64  `json:"plan_id"`
	ProductID                    int64  `json:"product_id"`
	ProductName                  string `json:"product_name"`
	SpecG                        int64  `json:"spec_g"`
	PlannedG                     int64  `json:"planned_g"`
	PlannedOutputG               int64  `json:"planned_output_g"`
	GapG                         int64  `json:"gap_g"`
	OrderNos                     string `json:"order_nos"`
	BomVersionID                 int64  `json:"bom_version_id"`
	OperationTemplateID          int64  `json:"operation_template_id"`
	ProcessRouteID               int64  `json:"process_route_id"`
	MaterialSnapshot             string `json:"material_snapshot"`
	ProcessSnapshotJSON          string `json:"process_snapshot_json"`
	ProductionConfigSnapshotJSON string `json:"production_config_snapshot_json"`
	CustomerProductSnapshotJSON  string `json:"customer_product_snapshot_json"`
}

type ProductionPlanDetail struct {
	ID                int64                            `json:"id"`
	PlanNo            string                           `json:"plan_no"`
	SourceType        string                           `json:"source_type"`
	Status            string                           `json:"status"`
	CreatedBy         string                           `json:"created_by"`
	CreatedAt         string                           `json:"created_at"`
	SubmittedBy       string                           `json:"submitted_by"`
	SubmittedAt       string                           `json:"submitted_at"`
	CompletedAt       string                           `json:"completed_at"`
	Items             []ProductionPlanItem             `json:"items"`
	MaterialSummary   []MaterialNeed                   `json:"material_summary"`
	RelatedWorkOrders []ProductionPlanRelatedWorkOrder `json:"related_work_orders"`
	JobCardCount      int64                            `json:"job_card_count"`
}

type ProductionPlanRelatedWorkOrder struct {
	ID                   int64  `json:"id"`
	WorkOrderNo          string `json:"work_order_no"`
	ProductionPlanID     int64  `json:"production_plan_id"`
	ProductionPlanItemID int64  `json:"production_plan_item_id"`
	ProductName          string `json:"product_name"`
	SpecG                int64  `json:"spec_g"`
	PlannedG             int64  `json:"planned_g"`
	PlannedOutputG       int64  `json:"planned_output_g"`
	Status               string `json:"status"`
	CreatedAt            string `json:"created_at"`
	CompletedAt          string `json:"completed_at"`
	JobCardCount         int64  `json:"job_card_count"`
}

type SubmitProductionPlanCommand struct {
	ID       int64
	Operator string
}

type SubmitProductionPlansCommand struct {
	IDs      []int64 `json:"ids"`
	Operator string  `json:"operator"`
}

type ProductionPlanSubmitResult struct {
	Plan       ProductionPlanDetail `json:"plan"`
	WorkOrders []WorkOrderRow       `json:"work_orders"`
	JobCards   []JobCardRow         `json:"job_cards"`
}

type ProductionPlanSubmitFailure struct {
	ID    int64  `json:"id"`
	Error string `json:"error"`
}

type ProductionPlanSubmitBatchResult struct {
	Success        []ProductionPlanSubmitResult  `json:"success"`
	Failed         []ProductionPlanSubmitFailure `json:"failed"`
	WorkOrderCount int                           `json:"work_order_count"`
	JobCardCount   int                           `json:"job_card_count"`
}

type WorkOrderStartCommand struct {
	ID       int64
	Operator string
}

type WorkOrderStartResult struct {
	BatchID       string       `json:"batch_id"`
	RunningItemID int64        `json:"running_item_id"`
	WorkOrder     WorkOrderRow `json:"work_order"`
}

type WorkOrderCompleteCommand struct {
	ID             int64
	FinishedUnits  int64
	FinishedLooseG int64
	ConsumedInputG int64
	Warehouse      string
	Operator       string
	Note           string
}

type WorkOrderCompleteResult struct {
	WorkOrder    WorkOrderRow    `json:"work_order"`
	StockEntries []StockEntryRow `json:"stock_entries"`
	Cost         BatchCostRow    `json:"cost"`
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
	ProductionPlanID      int64   `json:"production_plan_id"`
	ProductionPlanItemID  int64   `json:"production_plan_item_id"`
	BatchID               string  `json:"batch_id"`
	ProductID             int64   `json:"product_id"`
	ProductName           string  `json:"product_name"`
	SpecG                 int64   `json:"spec_g"`
	PlannedG              int64   `json:"planned_g"`
	PlannedOutputG        int64   `json:"planned_output_g"`
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
	OperationTemplateID   int64   `json:"operation_template_id"`
	ProcessTemplateID     int64   `json:"process_template_id"`
	ProcessTemplateName   string  `json:"process_template_name"`
	ProcessSnapshotJSON   string  `json:"process_snapshot_json"`
	OperationSummaryJSON  string  `json:"operation_summary_json"`
	PlannedStartAt        string  `json:"planned_start_at"`
	PlannedEndAt          string  `json:"planned_end_at"`
	ShiftCode             string  `json:"shift_code"`
	AssignedTo            string  `json:"assigned_to"`
	Priority              int     `json:"priority"`
	SchedulingNote        string  `json:"scheduling_note"`
	WorkCenter            string  `json:"work_center"`
}

type JobCardQuery struct {
	Status string
	Limit  int
}

type JobCardRow struct {
	ID                      int64   `json:"id"`
	WorkOrderID             int64   `json:"work_order_id"`
	SequenceNo              int     `json:"sequence_no"`
	OperationID             int64   `json:"operation_id"`
	WorkstationID           int64   `json:"workstation_id"`
	Operation               string  `json:"operation"`
	Workstation             string  `json:"workstation"`
	WorkstationCapacityID   int64   `json:"workstation_capacity_id"`
	WorkstationCapacityName string  `json:"workstation_capacity_name"`
	BatchSizeQty            float64 `json:"batch_size_qty"`
	BatchSizeUnit           string  `json:"batch_size_unit"`
	PlannedBatchCount       int     `json:"planned_batch_count"`
	PlannedMinutes          int     `json:"planned_minutes"`
	HourlyRate              float64 `json:"hourly_rate"`
	PlannedOperationCost    float64 `json:"planned_operation_cost"`
	ActualMinutes           int     `json:"actual_minutes"`
	ActualOperationCost     float64 `json:"actual_operation_cost"`
	Status                  string  `json:"status"`
	StartedAt               string  `json:"started_at"`
	PausedAt                string  `json:"paused_at"`
	ResumedAt               string  `json:"resumed_at"`
	CompletedAt             string  `json:"completed_at"`
	Operator                string  `json:"operator"`
	PlannedInputQty         float64 `json:"planned_input_qty"`
	ActualInputQty          float64 `json:"actual_input_qty"`
	ActualOutputQty         float64 `json:"actual_output_qty"`
	ActualLossQty           float64 `json:"actual_loss_qty"`
	ActualLossRate          float64 `json:"actual_loss_rate"`
	RecordsLoss             bool    `json:"records_loss"`
	LossReason              string  `json:"loss_reason"`
	ExceptionReason         string  `json:"exception_reason"`
	MetricsJSON             string  `json:"metrics_json"`
	ParameterSchemaJSON     string  `json:"parameter_schema_json"`
	PlannedStartAt          string  `json:"planned_start_at"`
	PlannedEndAt            string  `json:"planned_end_at"`
	ShiftCode               string  `json:"shift_code"`
	AssignedTo              string  `json:"assigned_to"`
	Priority                int     `json:"priority"`
	SchedulingNote          string  `json:"scheduling_note"`
	WorkCenter              string  `json:"work_center"`
}

type ScheduleBoardQuery struct {
	From       string
	To         string
	WorkCenter string
	Status     string
	Limit      int
}

type ScheduleConflict struct {
	Severity        string `json:"severity"`
	WorkCenter      string `json:"work_center,omitempty"`
	WorkDate        string `json:"work_date,omitempty"`
	ShiftCode       string `json:"shift_code,omitempty"`
	LoadMinutes     int    `json:"load_minutes,omitempty"`
	CapacityMinutes int    `json:"capacity_minutes,omitempty"`
	Message         string `json:"message"`
}

type CapacityCalendarCommand struct {
	ID               int64
	WorkCenter       string
	WorkDate         string
	ShiftCode        string
	AvailableMinutes int
	DowntimeMinutes  int
	Note             string
	Operator         string
}

type CapacityCalendarRow struct {
	ID               int64  `json:"id"`
	WorkCenter       string `json:"work_center"`
	WorkDate         string `json:"work_date"`
	ShiftCode        string `json:"shift_code"`
	AvailableMinutes int    `json:"available_minutes"`
	DowntimeMinutes  int    `json:"downtime_minutes"`
	Note             string `json:"note"`
	UpdatedAt        string `json:"updated_at"`
}

type ScheduleAssignmentCommand struct {
	WorkOrderID    int64
	JobCardID      int64
	WorkCenter     string
	PlannedStartAt string
	PlannedEndAt   string
	ShiftCode      string
	AssignedTo     string
	Priority       int
	Note           string
	Operator       string
}

type ScheduleAssignmentResult struct {
	WorkOrder WorkOrderRow       `json:"work_order"`
	JobCard   JobCardRow         `json:"job_card,omitempty"`
	Conflicts []ScheduleConflict `json:"conflicts"`
}

type ScheduleBoardResult struct {
	WorkOrders []WorkOrderRow        `json:"work_orders"`
	JobCards   []JobCardRow          `json:"job_cards"`
	Capacity   []CapacityCalendarRow `json:"capacity"`
	Conflicts  []ScheduleConflict    `json:"conflicts"`
}

type MRPSuggestionQuery struct {
	From       string
	To         string
	Status     string
	WorkCenter string
	MaterialID int64
	Limit      int
}

type MRPSuggestionRow struct {
	MaterialID             int64  `json:"material_id"`
	MaterialName           string `json:"material_name"`
	Unit                   string `json:"unit"`
	RequiredG              int64  `json:"required_g"`
	RequiredUnits          int64  `json:"required_units"`
	WIPG                   int64  `json:"wip_g"`
	RawG                   int64  `json:"raw_g"`
	ReservedG              int64  `json:"reserved_g"`
	ConsumedG              int64  `json:"consumed_g"`
	ReturnedG              int64  `json:"returned_g"`
	AvailableG             int64  `json:"available_g"`
	WIPTransferSuggestionG int64  `json:"wip_transfer_suggestion_g"`
	ShortageG              int64  `json:"shortage_g"`
	PurchaseSuggestionG    int64  `json:"purchase_suggestion_g"`
	WorkOrderCount         int64  `json:"work_order_count"`
	SourceWorkOrders       string `json:"source_work_orders"`
	EarliestPlannedAt      string `json:"earliest_planned_at"`
	SuggestionType         string `json:"suggestion_type"`
}

type MRPSuggestionResult struct {
	Rows                []MRPSuggestionRow `json:"rows"`
	PurchaseSuggestionG int64              `json:"purchase_suggestion_g"`
	TransferSuggestionG int64              `json:"transfer_suggestion_g"`
}

type ProductionTraceAnalyticsQuery struct {
	WorkOrderID int64
	BatchID     string
	Limit       int
}

type ProductionTraceLinkRow struct {
	WorkOrderID   int64  `json:"work_order_id"`
	WorkOrderNo   string `json:"work_order_no"`
	RunningItemID int64  `json:"running_item_id"`
	BatchID       string `json:"batch_id"`
	JobCardID     int64  `json:"job_card_id"`
	Operation     string `json:"operation"`
	JobCardStatus string `json:"job_card_status"`
	StockEntryID  int64  `json:"stock_entry_id"`
	EntryNo       string `json:"entry_no"`
	EntryType     string `json:"entry_type"`
	MaterialID    int64  `json:"material_id"`
	MaterialName  string `json:"material_name"`
	BatchCode     string `json:"batch_code"`
	QtyG          int64  `json:"qty_g"`
	CreatedAt     string `json:"created_at"`
}

type ProductionCostVarianceRow struct {
	WorkOrderID          int64   `json:"work_order_id"`
	WorkOrderNo          string  `json:"work_order_no"`
	BatchID              string  `json:"batch_id"`
	ProductName          string  `json:"product_name"`
	PlannedCost          float64 `json:"planned_cost"`
	ActualCost           float64 `json:"actual_cost"`
	PlannedOperationCost float64 `json:"planned_operation_cost"`
	ActualOperationCost  float64 `json:"actual_operation_cost"`
	Variance             float64 `json:"variance"`
	VarianceRate         float64 `json:"variance_rate"`
}

type ProductionAbnormalLossRow struct {
	JobCardID       int64   `json:"job_card_id"`
	WorkOrderID     int64   `json:"work_order_id"`
	WorkOrderNo     string  `json:"work_order_no"`
	Operation       string  `json:"operation"`
	ActualInputQty  float64 `json:"actual_input_qty"`
	ActualOutputQty float64 `json:"actual_output_qty"`
	ActualLossQty   float64 `json:"actual_loss_qty"`
	ActualLossRate  float64 `json:"actual_loss_rate"`
	LossReason      string  `json:"loss_reason"`
	ExceptionReason string  `json:"exception_reason"`
	Severity        string  `json:"severity"`
}

type ProductionTraceAnalyticsResult struct {
	TraceLinks        []ProductionTraceLinkRow    `json:"trace_links"`
	CostVariance      []ProductionCostVarianceRow `json:"cost_variance"`
	AbnormalLosses    []ProductionAbnormalLossRow `json:"abnormal_losses"`
	TotalVariance     float64                     `json:"total_variance"`
	AbnormalLossCount int                         `json:"abnormal_loss_count"`
}

type JobCardActualsCommand struct {
	ID              int64
	PlannedInputQty float64
	ActualInputQty  float64
	ActualOutputQty float64
	ActualLossQty   float64
	ActualLossRate  float64
	ActualMinutes   int
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

type StockEntryCommand struct {
	EntryType     string                  `json:"entry_type"`
	WorkOrderID   int64                   `json:"work_order_id"`
	JobCardID     int64                   `json:"job_card_id"`
	RunningItemID int64                   `json:"running_item_id"`
	SourceType    string                  `json:"source_type"`
	SourceID      int64                   `json:"source_id"`
	Operator      string                  `json:"operator"`
	Note          string                  `json:"note"`
	Items         []StockEntryItemCommand `json:"items"`
}

type StockEntryItemCommand struct {
	MaterialID    int64   `json:"material_id"`
	ProductID     int64   `json:"product_id"`
	ItemType      string  `json:"item_type"`
	ItemName      string  `json:"item_name"`
	SpecG         int64   `json:"spec_g"`
	FromWarehouse string  `json:"from_warehouse"`
	ToWarehouse   string  `json:"to_warehouse"`
	QtyG          int64   `json:"qty_g"`
	QtyUnits      int64   `json:"qty_units"`
	BatchCode     string  `json:"batch_code"`
	UnitCost      float64 `json:"unit_cost"`
}

type StockEntryQuery struct {
	EntryType   string
	Status      string
	WorkOrderID int64
	JobCardID   int64
	Limit       int
}

type StockEntryRow struct {
	ID            int64   `json:"id"`
	EntryNo       string  `json:"entry_no"`
	EntryType     string  `json:"entry_type"`
	Status        string  `json:"status"`
	WorkOrderID   int64   `json:"work_order_id"`
	JobCardID     int64   `json:"job_card_id"`
	RunningItemID int64   `json:"running_item_id"`
	SourceType    string  `json:"source_type"`
	SourceID      int64   `json:"source_id"`
	ItemCount     int64   `json:"item_count"`
	TotalQtyG     int64   `json:"total_qty_g"`
	TotalCost     float64 `json:"total_cost"`
	Operator      string  `json:"operator"`
	Note          string  `json:"note"`
	CreatedAt     string  `json:"created_at"`
}

type StockEntryItemRow struct {
	ID            int64   `json:"id"`
	StockEntryID  int64   `json:"stock_entry_id"`
	MaterialID    int64   `json:"material_id"`
	ProductID     int64   `json:"product_id"`
	ItemType      string  `json:"item_type"`
	ItemName      string  `json:"item_name"`
	SpecG         int64   `json:"spec_g"`
	FromWarehouse string  `json:"from_warehouse"`
	ToWarehouse   string  `json:"to_warehouse"`
	QtyG          int64   `json:"qty_g"`
	QtyUnits      int64   `json:"qty_units"`
	BatchCode     string  `json:"batch_code"`
	UnitCost      float64 `json:"unit_cost"`
	TotalCost     float64 `json:"total_cost"`
}

type StockEntryDetail struct {
	ID            int64               `json:"id"`
	EntryNo       string              `json:"entry_no"`
	EntryType     string              `json:"entry_type"`
	Status        string              `json:"status"`
	WorkOrderID   int64               `json:"work_order_id"`
	JobCardID     int64               `json:"job_card_id"`
	RunningItemID int64               `json:"running_item_id"`
	SourceType    string              `json:"source_type"`
	SourceID      int64               `json:"source_id"`
	Operator      string              `json:"operator"`
	Note          string              `json:"note"`
	CreatedAt     string              `json:"created_at"`
	Items         []StockEntryItemRow `json:"items"`
}

type JobCardActionCommand struct {
	ID              int64
	Action          string
	Operator        string
	ActualInputQty  float64
	ActualOutputQty float64
	ActualLossQty   float64
	ActualLossRate  float64
	ActualMinutes   int
	LossReason      string
	ExceptionReason string
	MetricsJSON     string
}

type JobCardActionResult struct {
	JobCard   JobCardRow   `json:"job_card"`
	WorkOrder WorkOrderRow `json:"work_order"`
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
	CreateProductionPlan(ctx context.Context, cmd CreateProductionPlanCommand) (ProductionPlanDetail, error)
	ListProductionPlans(ctx context.Context, query ProductionPlanQuery) ([]ProductionPlanRow, error)
	GetProductionPlan(ctx context.Context, id int64) (ProductionPlanDetail, error)
	SubmitProductionPlan(ctx context.Context, cmd SubmitProductionPlanCommand) (ProductionPlanSubmitResult, error)
	StartWorkOrder(ctx context.Context, cmd WorkOrderStartCommand) (WorkOrderStartResult, error)
	CompleteWorkOrder(ctx context.Context, cmd WorkOrderCompleteCommand) (WorkOrderCompleteResult, error)
	SaveScheduleAssignment(ctx context.Context, cmd ScheduleAssignmentCommand) (ScheduleAssignmentResult, error)
	SaveCapacityCalendar(ctx context.Context, cmd CapacityCalendarCommand) (CapacityCalendarRow, error)
	ScheduleBoard(ctx context.Context, query ScheduleBoardQuery) (ScheduleBoardResult, error)
	MRPSuggestions(ctx context.Context, query MRPSuggestionQuery) (MRPSuggestionResult, error)
	ProductionTraceAnalytics(ctx context.Context, query ProductionTraceAnalyticsQuery) (ProductionTraceAnalyticsResult, error)
	CreateStockEntry(ctx context.Context, cmd StockEntryCommand) (StockEntryDetail, error)
	ListStockEntries(ctx context.Context, query StockEntryQuery) ([]StockEntryRow, error)
	GetStockEntry(ctx context.Context, id int64) (StockEntryDetail, error)
	TransitionJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error)
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

func (s *Service) CreateProductionPlan(ctx context.Context, cmd CreateProductionPlanCommand) (ProductionPlanDetail, error) {
	cmd.From = strings.TrimSpace(cmd.From)
	cmd.To = strings.TrimSpace(cmd.To)
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	if cmd.SourceType == "" {
		cmd.SourceType = "erp_order"
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Selected == nil || len(cmd.Selected) == 0 {
		return ProductionPlanDetail{}, fmt.Errorf("selected production items required")
	}
	if cmd.InputByKey == nil {
		cmd.InputByKey = map[string]int64{}
	}
	hasSelected := false
	for _, selected := range cmd.Selected {
		if !selected {
			continue
		}
		hasSelected = true
	}
	if !hasSelected {
		return ProductionPlanDetail{}, fmt.Errorf("selected production items required")
	}
	return s.repo.CreateProductionPlan(ctx, cmd)
}

func (s *Service) ListProductionPlans(ctx context.Context, query ProductionPlanQuery) ([]ProductionPlanRow, error) {
	query.Status = strings.TrimSpace(query.Status)
	query.TimeField = normalizeProductionPlanTimeField(query.TimeField)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 500 {
		query.Limit = 500
	}
	return s.repo.ListProductionPlans(ctx, query)
}

func normalizeProductionPlanTimeField(value string) string {
	switch strings.TrimSpace(value) {
	case "submitted_at":
		return "submitted_at"
	case "completed_at":
		return "completed_at"
	default:
		return "created_at"
	}
}

func (s *Service) GetProductionPlan(ctx context.Context, id int64) (ProductionPlanDetail, error) {
	if id <= 0 {
		return ProductionPlanDetail{}, fmt.Errorf("production_plan_id required")
	}
	return s.repo.GetProductionPlan(ctx, id)
}

func (s *Service) SubmitProductionPlan(ctx context.Context, cmd SubmitProductionPlanCommand) (ProductionPlanSubmitResult, error) {
	if cmd.ID <= 0 {
		return ProductionPlanSubmitResult{}, fmt.Errorf("production_plan_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	return s.repo.SubmitProductionPlan(ctx, cmd)
}

func (s *Service) SubmitProductionPlans(ctx context.Context, cmd SubmitProductionPlansCommand) (ProductionPlanSubmitBatchResult, error) {
	if len(cmd.IDs) == 0 {
		return ProductionPlanSubmitBatchResult{}, fmt.Errorf("production_plan_ids required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	result := ProductionPlanSubmitBatchResult{
		Success: make([]ProductionPlanSubmitResult, 0),
		Failed:  make([]ProductionPlanSubmitFailure, 0),
	}
	seen := map[int64]bool{}
	for _, id := range cmd.IDs {
		if id <= 0 {
			result.Failed = append(result.Failed, ProductionPlanSubmitFailure{ID: id, Error: "production_plan_id required"})
			continue
		}
		if seen[id] {
			result.Failed = append(result.Failed, ProductionPlanSubmitFailure{ID: id, Error: "duplicate production_plan_id"})
			continue
		}
		seen[id] = true
		submitted, err := s.SubmitProductionPlan(ctx, SubmitProductionPlanCommand{ID: id, Operator: cmd.Operator})
		if err != nil {
			result.Failed = append(result.Failed, ProductionPlanSubmitFailure{ID: id, Error: err.Error()})
			continue
		}
		result.WorkOrderCount += len(submitted.WorkOrders)
		result.JobCardCount += len(submitted.JobCards)
		result.Success = append(result.Success, submitted)
	}
	return result, nil
}

func (s *Service) StartWorkOrder(ctx context.Context, cmd WorkOrderStartCommand) (WorkOrderStartResult, error) {
	if cmd.ID <= 0 {
		return WorkOrderStartResult{}, fmt.Errorf("work_order_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	return s.repo.StartWorkOrder(ctx, cmd)
}

func (s *Service) CompleteWorkOrder(ctx context.Context, cmd WorkOrderCompleteCommand) (WorkOrderCompleteResult, error) {
	if cmd.ID <= 0 {
		return WorkOrderCompleteResult{}, fmt.Errorf("work_order_id required")
	}
	if cmd.FinishedUnits <= 0 && cmd.FinishedLooseG <= 0 {
		return WorkOrderCompleteResult{}, fmt.Errorf("finished output required")
	}
	if cmd.ConsumedInputG < 0 {
		return WorkOrderCompleteResult{}, fmt.Errorf("consumed_input_g must be >= 0")
	}
	cmd.Warehouse = strings.TrimSpace(cmd.Warehouse)
	if cmd.Warehouse == "" {
		cmd.Warehouse = stockdomain.WarehouseFinishedGoods
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return WorkOrderCompleteResult{}, fmt.Errorf("operator required")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.CompleteWorkOrder(ctx, cmd)
}

func (s *Service) SaveScheduleAssignment(ctx context.Context, cmd ScheduleAssignmentCommand) (ScheduleAssignmentResult, error) {
	if cmd.WorkOrderID <= 0 {
		return ScheduleAssignmentResult{}, fmt.Errorf("work_order_id required")
	}
	cmd.WorkCenter = strings.TrimSpace(cmd.WorkCenter)
	cmd.ShiftCode = strings.TrimSpace(cmd.ShiftCode)
	cmd.AssignedTo = strings.TrimSpace(cmd.AssignedTo)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return ScheduleAssignmentResult{}, fmt.Errorf("operator required")
	}
	if cmd.Priority < 0 {
		return ScheduleAssignmentResult{}, fmt.Errorf("priority must be >= 0")
	}
	var err error
	cmd.PlannedStartAt, err = normalizeScheduleTimestamp(cmd.PlannedStartAt)
	if err != nil {
		return ScheduleAssignmentResult{}, err
	}
	cmd.PlannedEndAt, err = normalizeScheduleTimestamp(cmd.PlannedEndAt)
	if err != nil {
		return ScheduleAssignmentResult{}, err
	}
	if cmd.PlannedStartAt != "" && cmd.PlannedEndAt != "" && cmd.PlannedStartAt > cmd.PlannedEndAt {
		return ScheduleAssignmentResult{}, fmt.Errorf("planned_end_at must be after planned_start_at")
	}
	return s.repo.SaveScheduleAssignment(ctx, cmd)
}

func (s *Service) SaveCapacityCalendar(ctx context.Context, cmd CapacityCalendarCommand) (CapacityCalendarRow, error) {
	cmd.WorkCenter = strings.TrimSpace(cmd.WorkCenter)
	cmd.WorkDate = strings.TrimSpace(cmd.WorkDate)
	cmd.ShiftCode = strings.TrimSpace(cmd.ShiftCode)
	cmd.Note = strings.TrimSpace(cmd.Note)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.WorkCenter == "" {
		return CapacityCalendarRow{}, fmt.Errorf("work_center required")
	}
	if cmd.WorkDate == "" {
		return CapacityCalendarRow{}, fmt.Errorf("work_date required")
	}
	if _, err := time.Parse("2006-01-02", cmd.WorkDate); err != nil {
		return CapacityCalendarRow{}, fmt.Errorf("invalid work_date")
	}
	if cmd.ShiftCode == "" {
		cmd.ShiftCode = "默认"
	}
	if cmd.AvailableMinutes < 0 || cmd.DowntimeMinutes < 0 {
		return CapacityCalendarRow{}, fmt.Errorf("capacity minutes must be >= 0")
	}
	if cmd.Operator == "" {
		return CapacityCalendarRow{}, fmt.Errorf("operator required")
	}
	return s.repo.SaveCapacityCalendar(ctx, cmd)
}

func (s *Service) ScheduleBoard(ctx context.Context, query ScheduleBoardQuery) (ScheduleBoardResult, error) {
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.WorkCenter = strings.TrimSpace(query.WorkCenter)
	query.Status = strings.TrimSpace(query.Status)
	if query.From == "" {
		query.From = time.Now().Format("2006-01-02")
	}
	if query.To == "" {
		query.To = query.From
	}
	if _, err := time.Parse("2006-01-02", query.From); err != nil {
		return ScheduleBoardResult{}, fmt.Errorf("invalid from")
	}
	if _, err := time.Parse("2006-01-02", query.To); err != nil {
		return ScheduleBoardResult{}, fmt.Errorf("invalid to")
	}
	if query.From > query.To {
		return ScheduleBoardResult{}, fmt.Errorf("to must be after from")
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ScheduleBoard(ctx, query)
}

func (s *Service) MRPSuggestions(ctx context.Context, query MRPSuggestionQuery) (MRPSuggestionResult, error) {
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Status = strings.TrimSpace(query.Status)
	query.WorkCenter = strings.TrimSpace(query.WorkCenter)
	if query.From == "" {
		query.From = time.Now().Format("2006-01-02")
	}
	if query.To == "" {
		query.To = query.From
	}
	if _, err := time.Parse("2006-01-02", query.From); err != nil {
		return MRPSuggestionResult{}, fmt.Errorf("invalid from")
	}
	if _, err := time.Parse("2006-01-02", query.To); err != nil {
		return MRPSuggestionResult{}, fmt.Errorf("invalid to")
	}
	if query.From > query.To {
		return MRPSuggestionResult{}, fmt.Errorf("to must be after from")
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 50
	}
	return s.repo.MRPSuggestions(ctx, query)
}

func (s *Service) ProductionTraceAnalytics(ctx context.Context, query ProductionTraceAnalyticsQuery) (ProductionTraceAnalyticsResult, error) {
	query.BatchID = strings.TrimSpace(query.BatchID)
	if query.WorkOrderID < 0 {
		return ProductionTraceAnalyticsResult{}, fmt.Errorf("work_order_id must be >= 0")
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 50
	}
	return s.repo.ProductionTraceAnalytics(ctx, query)
}

func normalizeScheduleTimestamp(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02 15:04"), nil
		}
	}
	return "", fmt.Errorf("invalid schedule time")
}

func (s *Service) CreateStockEntry(ctx context.Context, cmd StockEntryCommand) (StockEntryDetail, error) {
	cmd.EntryType = normalizeStockEntryType(cmd.EntryType)
	if cmd.EntryType == "" {
		return StockEntryDetail{}, fmt.Errorf("invalid stock entry type")
	}
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return StockEntryDetail{}, fmt.Errorf("operator required")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	if len(cmd.Items) == 0 {
		return StockEntryDetail{}, fmt.Errorf("stock entry items required")
	}
	for i := range cmd.Items {
		item, err := normalizeStockEntryItem(cmd.EntryType, cmd.Items[i])
		if err != nil {
			return StockEntryDetail{}, err
		}
		cmd.Items[i] = item
	}
	return s.repo.CreateStockEntry(ctx, cmd)
}

func (s *Service) ListStockEntries(ctx context.Context, query StockEntryQuery) ([]StockEntryRow, error) {
	query.EntryType = normalizeStockEntryType(query.EntryType)
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 200
	}
	return s.repo.ListStockEntries(ctx, query)
}

func (s *Service) GetStockEntry(ctx context.Context, id int64) (StockEntryDetail, error) {
	if id <= 0 {
		return StockEntryDetail{}, fmt.Errorf("stock_entry_id required")
	}
	return s.repo.GetStockEntry(ctx, id)
}

func (s *Service) StartJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	cmd.Action = "start"
	return s.transitionJobCard(ctx, cmd)
}

func (s *Service) PauseJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	cmd.Action = "pause"
	return s.transitionJobCard(ctx, cmd)
}

func (s *Service) ResumeJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	cmd.Action = "resume"
	return s.transitionJobCard(ctx, cmd)
}

func (s *Service) CompleteJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	cmd.Action = "complete"
	return s.transitionJobCard(ctx, cmd)
}

func (s *Service) transitionJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	if cmd.ID <= 0 {
		return JobCardActionResult{}, fmt.Errorf("job_card_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return JobCardActionResult{}, fmt.Errorf("operator required")
	}
	cmd.Action = normalizeJobCardAction(cmd.Action)
	if cmd.Action == "" {
		return JobCardActionResult{}, fmt.Errorf("invalid job card action")
	}
	if cmd.ActualInputQty < 0 || cmd.ActualOutputQty < 0 || cmd.ActualLossQty < 0 {
		return JobCardActionResult{}, fmt.Errorf("quantity must be >= 0")
	}
	if cmd.ActualMinutes < 0 {
		return JobCardActionResult{}, fmt.Errorf("actual_minutes must be >= 0")
	}
	if cmd.ActualInputQty > 0 {
		lossQty, lossRate, err := productiondomain.ActualLossMetrics(cmd.ActualInputQty, cmd.ActualOutputQty)
		if err != nil {
			return JobCardActionResult{}, err
		}
		cmd.ActualLossQty = lossQty
		cmd.ActualLossRate = lossRate
	}
	cmd.LossReason = strings.TrimSpace(cmd.LossReason)
	cmd.ExceptionReason = strings.TrimSpace(cmd.ExceptionReason)
	if cmd.ExceptionReason == "" {
		cmd.ExceptionReason = cmd.LossReason
	}
	cmd.MetricsJSON = strings.TrimSpace(cmd.MetricsJSON)
	if cmd.MetricsJSON == "" {
		cmd.MetricsJSON = "{}"
	}
	return s.repo.TransitionJobCard(ctx, cmd)
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
	if cmd.PlannedInputQty < 0 || cmd.ActualInputQty < 0 || cmd.ActualOutputQty < 0 || cmd.ActualLossQty < 0 {
		return fmt.Errorf("quantity must be >= 0")
	}
	if cmd.ActualMinutes < 0 {
		return fmt.Errorf("actual_minutes must be >= 0")
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

func normalizeStockEntryType(entryType string) string {
	switch strings.ToLower(strings.TrimSpace(entryType)) {
	case "material_issue_to_wip", "issue_to_wip", "领料到wip", "领料到 wip":
		return "material_issue_to_wip"
	case "wip_return", "return_from_wip", "wip退料":
		return "wip_return"
	case "material_consume", "work_order_consume", "工单消耗":
		return "material_consume"
	case "finished_receipt", "finish_receipt", "完工入库":
		return "finished_receipt"
	case "scrap_loss", "scrap", "报废", "损耗":
		return "scrap_loss"
	default:
		return ""
	}
}

func normalizeStockEntryItem(entryType string, item StockEntryItemCommand) (StockEntryItemCommand, error) {
	item.ItemType = strings.TrimSpace(item.ItemType)
	if item.ItemType == "" {
		if item.ProductID > 0 {
			item.ItemType = "finished_product"
		} else {
			item.ItemType = "material"
		}
	}
	item.ItemName = strings.TrimSpace(item.ItemName)
	item.FromWarehouse = strings.TrimSpace(item.FromWarehouse)
	item.ToWarehouse = strings.TrimSpace(item.ToWarehouse)
	item.BatchCode = strings.TrimSpace(item.BatchCode)
	if item.MaterialID <= 0 && item.ProductID <= 0 {
		return item, fmt.Errorf("stock entry item material_id or product_id required")
	}
	if item.QtyG <= 0 && item.QtyUnits <= 0 {
		return item, fmt.Errorf("stock entry item quantity required")
	}
	if item.UnitCost < 0 {
		return item, fmt.Errorf("unit_cost must be >= 0")
	}
	switch entryType {
	case "material_issue_to_wip":
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseRawMaterials
		}
		if item.ToWarehouse == "" {
			item.ToWarehouse = stockdomain.WarehouseWIP
		}
	case "wip_return":
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseWIP
		}
		if item.ToWarehouse == "" {
			item.ToWarehouse = stockdomain.WarehouseRawMaterials
		}
	case "material_consume", "scrap_loss":
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseWIP
		}
	case "finished_receipt":
		if item.ToWarehouse == "" {
			item.ToWarehouse = stockdomain.WarehouseFinishedGoods
		}
	}
	return item, nil
}

func normalizeJobCardAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start", "pause", "resume", "complete":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return ""
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
