package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	productiondomain "orderapp/internal/domain/production"
	stockdomain "orderapp/internal/domain/stock"
	"sort"
	"strconv"
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
	ProductID                int64
	ParentProductID          int64
	ProductName              string
	SpecLabel                string
	SalesUnit                string
	SpecG                    int64
	GapG                     int64
	SalesSpecCount           float64
	InventoryQtyPerSalesUnit float64
	InventoryUnit            string
	PlannedInventoryQty      float64
	SalesSpecSnapshotJSON    string
	OrderNos                 string
	OperationTemplateID      int64
	CustomerID               int64
	TargetWarehouse          string
	ProcessingRequestItemID  int64
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
	WorkOrderID      int64
	StockDocumentID  int64
	FinishedUnits    int64
	FinishedLooseG   int64
	HasFinishedInput bool
	Warehouse        string
	Partial          bool
	ConsumedInputG   int64
	Operator         string
	Note             string
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
	StockEntryID   int64           `json:"-"`
}

type FinishOutputCommand struct {
	SpecG          int64 `json:"spec_g"`
	FinishedUnits  int64 `json:"finished_units"`
	FinishedLooseG int64 `json:"finished_loose_g"`
}

type CancelCommand struct {
	ID       int64
	Operator string
	Note     string
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
	ProductID                         int64   `json:"product_id"`
	ParentProductID                   int64   `json:"parent_product_id"`
	Product                           string  `json:"product"`
	OrderNos                          string  `json:"order_nos"`
	SpecLabel                         string  `json:"spec_label"`
	SalesUnit                         string  `json:"sales_unit"`
	SpecG                             int64   `json:"spec_g"`
	NeedUnits                         int64   `json:"need_units"`
	NeedG                             int64   `json:"need_g"`
	InvUnits                          int64   `json:"inv_units"`
	InvLooseG                         int64   `json:"inv_loose_g"`
	InvG                              int64   `json:"inv_g"`
	GapG                              int64   `json:"gap_g"`
	SalesSpecCount                    float64 `json:"sales_spec_count"`
	InventoryQtyPerSalesUnit          float64 `json:"inventory_qty_per_sales_unit"`
	InventoryUnit                     string  `json:"inventory_unit"`
	NeedInventoryQty                  float64 `json:"need_inventory_qty"`
	AvailableInventoryQty             float64 `json:"available_inventory_qty"`
	GapInventoryQty                   float64 `json:"gap_inventory_qty"`
	GapSalesSpecCount                 float64 `json:"gap_sales_spec_count"`
	SalesSpecSnapshotJSON             string  `json:"sales_spec_snapshot_json"`
	ProductionKind                    string  `json:"production_kind,omitempty"`
	ProductTypeCategoryID             int64   `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID          int64   `json:"product_subtype_category_id,omitempty"`
	ProductTypeName                   string  `json:"product_type_name,omitempty"`
	ProductSubtypeName                string  `json:"product_subtype_name,omitempty"`
	OperationTemplateID               int64   `json:"operation_template_id,omitempty"`
	NeedBags                          int64   `json:"need_bags,omitempty"`
	NeedBoxes                         int64   `json:"need_boxes,omitempty"`
	UpstreamProductID                 int64   `json:"upstream_product_id,omitempty"`
	UpstreamRoastDemandG              int64   `json:"upstream_roast_demand_g,omitempty"`
	UpstreamShortageG                 int64   `json:"upstream_shortage_g,omitempty"`
	FinishedProductComponentShortageG int64   `json:"finished_product_component_shortage_g,omitempty"`
	DemandStatus                      string  `json:"demand_status,omitempty"`
	DemandStatusLabel                 string  `json:"demand_status_label,omitempty"`
	DemandSelectable                  bool    `json:"demand_selectable"`
	BlockingReason                    string  `json:"blocking_reason,omitempty"`
	ProductionPlanID                  int64   `json:"production_plan_id,omitempty"`
	ProductionPlanNo                  string  `json:"production_plan_no,omitempty"`
	WorkOrderID                       int64   `json:"work_order_id,omitempty"`
	WorkOrderNo                       string  `json:"work_order_no,omitempty"`
}

type MaterialNeed struct {
	Name                   string  `json:"name"`
	Qty                    int64   `json:"-"`
	ExactQty               float64 `json:"-"`
	Unit                   string  `json:"unit"`
	ComponentType          string  `json:"component_type,omitempty"`
	UpstreamProductID      int64   `json:"upstream_product_id,omitempty"`
	UpstreamShortageG      int64   `json:"upstream_shortage_g,omitempty"`
	WIPG                   int64   `json:"wip_g,omitempty"`
	AvailableG             int64   `json:"available_g,omitempty"`
	RawG                   int64   `json:"raw_g,omitempty"`
	ReservedG              int64   `json:"reserved_g,omitempty"`
	WIPTransferSuggestionG int64   `json:"wip_transfer_suggestion_g,omitempty"`
	ShortageG              int64   `json:"shortage_g,omitempty"`
	PurchaseSuggestionG    int64   `json:"purchase_suggestion_g,omitempty"`
}

func (m MaterialNeed) MarshalJSON() ([]byte, error) {
	type alias MaterialNeed
	qty := m.ExactQty
	if qty <= 0 {
		qty = float64(m.Qty)
	}
	return json.Marshal(struct {
		alias
		Qty float64 `json:"qty"`
	}{
		alias: alias(m),
		Qty:   qty,
	})
}

func (m *MaterialNeed) UnmarshalJSON(data []byte) error {
	type alias MaterialNeed
	var payload struct {
		alias
		Qty float64 `json:"qty"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*m = MaterialNeed(payload.alias)
	m.ExactQty = payload.Qty
	m.Qty = int64(math.Ceil(payload.Qty))
	return nil
}

type ProducePlanDisplayRow struct {
	UnprodNeedRow
	BomYieldRate        float64 `json:"bom_yield_rate"`
	BomMaterialLossRate float64 `json:"bom_material_loss_rate"`
	BomSummaryError     string  `json:"bom_summary_error,omitempty"`
	InputG              int64   `json:"input_g"`
}

type PlanSummaryQuery struct {
	From         string
	To           string
	CustomerID   int64
	Selected     map[string]bool
	Plan         bool
	DemandStatus string
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
	Key              string  `json:"key"`
	ProductID        int64   `json:"product_id"`
	SpecG            int64   `json:"spec_g"`
	ProductName      string  `json:"product_name"`
	MaterialName     string  `json:"material_name"`
	MaterialUnit     string  `json:"material_unit"`
	RatioPct         float64 `json:"ratio_pct"`
	MaterialLossRate float64 `json:"material_loss_rate,omitempty"`
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
	CancelledAt string `json:"cancelled_at"`
}

type ProductionPlanItem struct {
	ID                           int64   `json:"id"`
	PlanID                       int64   `json:"plan_id"`
	OutputType                   string  `json:"output_type"`
	OutputProductID              int64   `json:"output_product_id"`
	OutputMaterialID             int64   `json:"output_material_id"`
	OutputName                   string  `json:"output_name"`
	OutputQty                    float64 `json:"output_qty"`
	OutputUnit                   string  `json:"output_unit"`
	ProductID                    int64   `json:"product_id"`
	ParentProductID              int64   `json:"parent_product_id"`
	BomSourceProductID           int64   `json:"bom_source_product_id"`
	BomSource                    string  `json:"bom_source"`
	BomInherited                 bool    `json:"bom_inherited"`
	ProductName                  string  `json:"product_name"`
	SpecG                        int64   `json:"spec_g"`
	SalesSpecCount               float64 `json:"sales_spec_count"`
	InventoryQtyPerSalesUnit     float64 `json:"inventory_qty_per_sales_unit"`
	InventoryUnit                string  `json:"inventory_unit"`
	PlannedInventoryQty          float64 `json:"planned_inventory_qty"`
	SalesSpecSnapshotJSON        string  `json:"sales_spec_snapshot_json"`
	PlannedG                     int64   `json:"planned_g"`
	PlannedOutputG               int64   `json:"planned_output_g"`
	GapG                         int64   `json:"gap_g"`
	OrderNos                     string  `json:"order_nos"`
	BomVersionID                 int64   `json:"bom_version_id"`
	OperationTemplateID          int64   `json:"operation_template_id"`
	ProcessRouteID               int64   `json:"process_route_id"`
	MaterialSnapshot             string  `json:"material_snapshot"`
	ProcessSnapshotJSON          string  `json:"process_snapshot_json"`
	ProductionConfigSnapshotJSON string  `json:"production_config_snapshot_json"`
	CustomerProductSnapshotJSON  string  `json:"customer_product_snapshot_json"`
	CustomerID                   int64   `json:"customer_id,omitempty"`
	TargetWarehouse              string  `json:"target_warehouse,omitempty"`
	ProcessingRequestItemID      int64   `json:"processing_request_item_id,omitempty"`
}

type ProductionPlanOperationSplit struct {
	ID                      int64   `json:"id"`
	ProductionPlanID        int64   `json:"production_plan_id"`
	ProductionPlanItemID    int64   `json:"production_plan_item_id"`
	OperationSeq            int     `json:"operation_seq"`
	OperationID             int64   `json:"operation_id"`
	Operation               string  `json:"operation"`
	WorkstationID           int64   `json:"workstation_id"`
	Workstation             string  `json:"workstation"`
	WorkstationCapacityID   int64   `json:"workstation_capacity_id"`
	WorkstationCapacityName string  `json:"workstation_capacity_name"`
	BatchSizeQty            float64 `json:"batch_size_qty"`
	BatchSizeUnit           string  `json:"batch_size_unit"`
	StandardMinutes         int     `json:"standard_minutes"`
	HourlyRate              float64 `json:"hourly_rate"`
	CostMethod              string  `json:"cost_method"`
	PieceRate               float64 `json:"piece_rate"`
	PlannedBatchCount       int     `json:"planned_batch_count"`
	PlannedQty              float64 `json:"planned_qty"`
	PlannedQtyG             int64   `json:"planned_qty_g"`
	PlannedMinutes          int     `json:"planned_minutes"`
	PlannedOperationCost    float64 `json:"planned_operation_cost"`
	Note                    string  `json:"note"`
}

type SaveProductionPlanOperationSplitsCommand struct {
	ID       int64
	Items    []ProductionPlanOperationSplit
	Operator string
}

type PreviewProductionPlanOperationSplitsCommand struct {
	ID    int64
	Items []ProductionPlanOperationSplit
}

type ProductionPlanOperationSplitPreview struct {
	CoverageSummary   ProductionPlanOperationSplitCoverageSummary   `json:"coverage_summary"`
	OperationCoverage []ProductionPlanOperationSplitCoverageRow     `json:"operation_coverage"`
	MaterialSummary   []ProductionPlanOperationSplitMaterialPreview `json:"material_summary"`
	Warnings          []string                                      `json:"warnings"`
}

type ProductionPlanOperationSplitCoverageSummary struct {
	RequiredG int64  `json:"required_g"`
	ArrangedG int64  `json:"arranged_g"`
	DiffG     int64  `json:"diff_g"`
	Status    string `json:"status"`
}

type ProductionPlanOperationSplitCoverageRow struct {
	ProductionPlanItemID int64  `json:"production_plan_item_id"`
	ProductName          string `json:"product_name"`
	OperationSeq         int    `json:"operation_seq"`
	OperationID          int64  `json:"operation_id"`
	Operation            string `json:"operation"`
	RequiredG            int64  `json:"required_g"`
	ArrangedG            int64  `json:"arranged_g"`
	DiffG                int64  `json:"diff_g"`
	Status               string `json:"status"`
}

type ProductionPlanOperationSplitMaterialPreview struct {
	Name        string  `json:"name"`
	Unit        string  `json:"unit"`
	RequiredQty float64 `json:"required_qty"`
	ArrangedQty float64 `json:"arranged_qty"`
	DiffQty     float64 `json:"diff_qty"`
	Status      string  `json:"status"`
}

type SaveWorkOrderOperationSplitsCommand struct {
	ID       int64
	Items    []ProductionPlanOperationSplit
	Operator string
}

type WorkOrderOperationSplitsResult struct {
	WorkOrder WorkOrderRow `json:"work_order"`
	JobCards  []JobCardRow `json:"job_cards"`
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
	CancelledAt       string                           `json:"cancelled_at"`
	Items             []ProductionPlanItem             `json:"items"`
	OperationSplits   []ProductionPlanOperationSplit   `json:"operation_splits"`
	MaterialSummary   []MaterialNeed                   `json:"material_summary"`
	SupplyGaps        []ProductionPlanSupplyGap        `json:"supply_gaps"`
	ManufacturingPlan ProductionManufacturingPlan      `json:"manufacturing_plan"`
	RelatedWorkOrders []ProductionPlanRelatedWorkOrder `json:"related_work_orders"`
	JobCardCount      int64                            `json:"job_card_count"`
}

type ProductionManufacturingPlan struct {
	Nodes    []ProductionManufacturingPlanNode `json:"nodes"`
	Edges    []ProductionManufacturingPlanEdge `json:"edges"`
	Blocking bool                              `json:"blocking"`
}

type ProductionManufacturingPlanNode struct {
	Key               string  `json:"key"`
	PlanItemID        int64   `json:"plan_item_id"`
	ParentPlanItemID  int64   `json:"parent_plan_item_id"`
	OutputType        string  `json:"output_type"`
	OutputProductID   int64   `json:"output_product_id"`
	OutputMaterialID  int64   `json:"output_material_id"`
	OutputName        string  `json:"output_name"`
	OutputUnit        string  `json:"output_unit"`
	RequiredQty       float64 `json:"required_qty"`
	StockCoveredQty   float64 `json:"stock_covered_qty"`
	ShortageQty       float64 `json:"shortage_qty"`
	RequiredG         int64   `json:"required_g"`
	RequiredUnits     int64   `json:"required_units"`
	StockCoveredG     int64   `json:"stock_covered_g"`
	StockCoveredUnits int64   `json:"stock_covered_units"`
	ShortageG         int64   `json:"shortage_g"`
	ShortageUnits     int64   `json:"shortage_units"`
	Action            string  `json:"action"`
	Blocking          bool    `json:"blocking"`
	Depth             int     `json:"depth"`
	BOMVersionID      int64   `json:"bom_version_id"`
	TargetWarehouse   string  `json:"target_warehouse"`
}

type ProductionManufacturingPlanEdge struct {
	ConsumerKey        string  `json:"consumer_key"`
	SupplierKey        string  `json:"supplier_key"`
	ConsumerPlanItemID int64   `json:"consumer_plan_item_id"`
	SupplierPlanItemID int64   `json:"supplier_plan_item_id"`
	RequiredQty        float64 `json:"required_qty"`
	RequiredG          int64   `json:"required_g"`
	RequiredUnits      int64   `json:"required_units"`
}

type ProductionPlanSupplyGap struct {
	ID                   int64  `json:"id"`
	ProductionPlanID     int64  `json:"production_plan_id"`
	ProductionPlanItemID int64  `json:"production_plan_item_id"`
	ItemType             string `json:"item_type"`
	ItemID               int64  `json:"item_id"`
	ItemName             string `json:"item_name"`
	RequiredG            int64  `json:"required_g"`
	RequiredUnits        int64  `json:"required_units"`
	Reason               string `json:"reason"`
	Status               string `json:"status"`
}

type UpdateProductionPlanItemTargetWarehouseCommand struct {
	ProductionPlanID     int64
	ProductionPlanItemID int64
	TargetWarehouse      string
	Operator             string
}

type ProductionPlanRelatedWorkOrder struct {
	ID                   int64   `json:"id"`
	WorkOrderNo          string  `json:"work_order_no"`
	RunningItemID        int64   `json:"running_item_id"`
	ProductionPlanID     int64   `json:"production_plan_id"`
	ProductionPlanItemID int64   `json:"production_plan_item_id"`
	OutputType           string  `json:"output_type"`
	OutputProductID      int64   `json:"output_product_id"`
	OutputMaterialID     int64   `json:"output_material_id"`
	OutputName           string  `json:"output_name"`
	OutputQty            float64 `json:"output_qty"`
	OutputUnit           string  `json:"output_unit"`
	TargetWarehouse      string  `json:"target_warehouse"`
	ProductName          string  `json:"product_name"`
	SpecG                int64   `json:"spec_g"`
	PlannedG             int64   `json:"planned_g"`
	PlannedOutputG       int64   `json:"planned_output_g"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
	CompletedAt          string  `json:"completed_at"`
	JobCardCount         int64   `json:"job_card_count"`
}

type SubmitProductionPlanCommand struct {
	ID       int64
	Operator string
}

type CancelProductionPlanCommand struct {
	ID       int64
	Operator string
	Note     string
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
	ID               int64
	StockDocumentID  int64
	FinishedUnits    int64
	FinishedLooseG   int64
	FinishedQtyG     int64
	FinishedQtyUnits int64
	ConsumedInputG   int64
	Warehouse        string
	Operator         string
	Note             string
}

type WorkOrderCompleteResult struct {
	WorkOrder    WorkOrderRow    `json:"work_order"`
	StockEntries []StockEntryRow `json:"stock_entries"`
	Cost         BatchCostRow    `json:"cost"`
}

type WorkOrderCancelCommand struct {
	ID       int64
	Operator string
	Note     string
}

type WorkOrderIssueMaterialsCommand struct {
	ID       int64
	Operator string
	Note     string
	Items    []StockEntryItemCommand
}

type ProductionLogsQuery struct {
	From          string
	To            string
	ProductID     int64
	BatchID       string
	Operator      string
	RunningItemID int64
	Limit         int
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
	FinishedBatchCode     string  `json:"finished_batch_code"`
}

type ProductionLogsResult struct {
	Products []ProductionLogProductOption
	Rows     []ProductionLogRow
}

type WorkOrderQuery struct {
	ID     int64
	Status string
	Limit  int
}

type WorkOrderDependencyRow struct {
	WorkOrderID          int64   `json:"work_order_id"`
	DependsOnWorkOrderID int64   `json:"depends_on_work_order_id"`
	DependsOnWorkOrderNo string  `json:"depends_on_work_order_no"`
	OutputType           string  `json:"output_type"`
	OutputProductID      int64   `json:"output_product_id"`
	OutputMaterialID     int64   `json:"output_material_id"`
	OutputName           string  `json:"output_name"`
	OutputQty            float64 `json:"output_qty"`
	OutputUnit           string  `json:"output_unit"`
	MaterialID           int64   `json:"material_id"`
	RequiredG            int64   `json:"required_g"`
	RequiredUnits        int64   `json:"required_units"`
	Status               string  `json:"status"`
	Completed            bool    `json:"completed"`
}

type WorkOrderRow struct {
	ID                        int64                    `json:"id"`
	WorkOrderNo               string                   `json:"work_order_no"`
	RunningItemID             int64                    `json:"running_item_id"`
	ProductionPlanID          int64                    `json:"production_plan_id"`
	ProductionPlanItemID      int64                    `json:"production_plan_item_id"`
	OutputType                string                   `json:"output_type"`
	OutputProductID           int64                    `json:"output_product_id"`
	OutputMaterialID          int64                    `json:"output_material_id"`
	OutputName                string                   `json:"output_name"`
	OutputQty                 float64                  `json:"output_qty"`
	OutputUnit                string                   `json:"output_unit"`
	HasUnfinishedDependencies bool                     `json:"has_unfinished_dependencies"`
	DependencyBlockingReason  string                   `json:"dependency_blocking_reason,omitempty"`
	UpstreamWorkOrderIDs      []int64                  `json:"upstream_work_order_ids,omitempty"`
	UpstreamBlocked           bool                     `json:"upstream_blocked"`
	UpstreamDependencies      []WorkOrderDependencyRow `json:"upstream_dependencies"`
	BatchID                   string                   `json:"batch_id"`
	ProductID                 int64                    `json:"product_id"`
	ParentProductID           int64                    `json:"parent_product_id"`
	BomSourceProductID        int64                    `json:"bom_source_product_id"`
	BomSource                 string                   `json:"bom_source"`
	BomInherited              bool                     `json:"bom_inherited"`
	ProductName               string                   `json:"product_name"`
	SpecG                     int64                    `json:"spec_g"`
	SalesSpecCount            float64                  `json:"sales_spec_count"`
	InventoryQtyPerSalesUnit  float64                  `json:"inventory_qty_per_sales_unit"`
	InventoryUnit             string                   `json:"inventory_unit"`
	PlannedInventoryQty       float64                  `json:"planned_inventory_qty"`
	SalesSpecSnapshotJSON     string                   `json:"sales_spec_snapshot_json"`
	PlannedG                  int64                    `json:"planned_g"`
	PlannedOutputG            int64                    `json:"planned_output_g"`
	Status                    string                   `json:"status"`
	ActualCost                float64                  `json:"actual_cost"`
	CreatedAt                 string                   `json:"created_at"`
	CompletedAt               string                   `json:"completed_at"`
	RoastLevel                string                   `json:"roast_level"`
	YieldRate                 float64                  `json:"yield_rate"`
	ExpectedYieldRate         float64                  `json:"expected_yield_rate"`
	ExpectedLossRate          float64                  `json:"expected_loss_rate"`
	SuggestedInputG           int64                    `json:"suggested_input_g"`
	SuggestedMachine          string                   `json:"suggested_machine"`
	SuggestedBatchCount       int64                    `json:"suggested_batch_count"`
	SuggestedBatchG           int64                    `json:"suggested_batch_g"`
	SuggestedBatchPlan        string                   `json:"suggested_batch_plan"`
	PlannedUnits              int64                    `json:"planned_units"`
	PlannedLooseG             int64                    `json:"planned_loose_g"`
	MaterialSummary           string                   `json:"material_summary"`
	OrderNos                  string                   `json:"order_nos"`
	WIPReservedG              int64                    `json:"wip_reserved_g"`
	WIPConsumedG              int64                    `json:"wip_consumed_g"`
	WIPRemainingReservedG     int64                    `json:"remaining_reserved_g"`
	BomVersionID              int64                    `json:"bom_version_id"`
	OperationTemplateID       int64                    `json:"operation_template_id"`
	ProcessTemplateID         int64                    `json:"process_template_id"`
	ProcessTemplateName       string                   `json:"process_template_name"`
	ProcessSnapshotJSON       string                   `json:"process_snapshot_json"`
	OperationSummaryJSON      string                   `json:"operation_summary_json"`
	PlannedStartAt            string                   `json:"planned_start_at"`
	PlannedEndAt              string                   `json:"planned_end_at"`
	ShiftCode                 string                   `json:"shift_code"`
	AssignedTo                string                   `json:"assigned_to"`
	Priority                  int                      `json:"priority"`
	SchedulingNote            string                   `json:"scheduling_note"`
	WorkCenter                string                   `json:"work_center"`
	CustomerID                int64                    `json:"customer_id,omitempty"`
	TargetWarehouse           string                   `json:"target_warehouse,omitempty"`
	ProcessingRequestItemID   int64                    `json:"processing_request_item_id,omitempty"`
}

type JobCardQuery struct {
	WorkOrderID int64
	Status      string
	Limit       int
}

type JobCardRow struct {
	ID                           int64   `json:"id"`
	WorkOrderID                  int64   `json:"work_order_id"`
	WorkOrderNo                  string  `json:"work_order_no"`
	ProductID                    int64   `json:"product_id"`
	ProductName                  string  `json:"product_name"`
	SpecG                        int64   `json:"spec_g"`
	OrderNos                     string  `json:"order_nos"`
	PlannedG                     int64   `json:"planned_g"`
	PlannedOutputG               int64   `json:"planned_output_g"`
	BomVersionID                 int64   `json:"bom_version_id"`
	MaterialSnapshot             string  `json:"material_snapshot"`
	ProcessSnapshotJSON          string  `json:"process_snapshot_json"`
	ProductionConfigSnapshotJSON string  `json:"production_config_snapshot_json"`
	CustomerProductSnapshotJSON  string  `json:"customer_product_snapshot_json"`
	SequenceNo                   int     `json:"sequence_no"`
	OperationID                  int64   `json:"operation_id"`
	WorkstationID                int64   `json:"workstation_id"`
	Operation                    string  `json:"operation"`
	Workstation                  string  `json:"workstation"`
	WorkstationCapacityID        int64   `json:"workstation_capacity_id"`
	WorkstationCapacityName      string  `json:"workstation_capacity_name"`
	BatchSizeQty                 float64 `json:"batch_size_qty"`
	BatchSizeUnit                string  `json:"batch_size_unit"`
	PlannedBatchCount            int     `json:"planned_batch_count"`
	PlannedMinutes               int     `json:"planned_minutes"`
	HourlyRate                   float64 `json:"hourly_rate"`
	CostMethod                   string  `json:"cost_method"`
	PieceRate                    float64 `json:"piece_rate"`
	PlannedOperationCost         float64 `json:"planned_operation_cost"`
	ActualMinutes                int     `json:"actual_minutes"`
	ActualOperationCost          float64 `json:"actual_operation_cost"`
	Status                       string  `json:"status"`
	StartedAt                    string  `json:"started_at"`
	PausedAt                     string  `json:"paused_at"`
	ResumedAt                    string  `json:"resumed_at"`
	CompletedAt                  string  `json:"completed_at"`
	Operator                     string  `json:"operator"`
	PlannedInputQty              float64 `json:"planned_input_qty"`
	ActualInputQty               float64 `json:"actual_input_qty"`
	ActualOutputQty              float64 `json:"actual_output_qty"`
	ActualLossQty                float64 `json:"actual_loss_qty"`
	ActualLossRate               float64 `json:"actual_loss_rate"`
	RecordsLoss                  bool    `json:"records_loss"`
	LossReason                   string  `json:"loss_reason"`
	ExceptionReason              string  `json:"exception_reason"`
	ProcessRequirement           string  `json:"process_requirement"`
	MetricsJSON                  string  `json:"metrics_json"`
	ParameterSchemaJSON          string  `json:"parameter_schema_json"`
	PlannedStartAt               string  `json:"planned_start_at"`
	PlannedEndAt                 string  `json:"planned_end_at"`
	ShiftCode                    string  `json:"shift_code"`
	AssignedTo                   string  `json:"assigned_to"`
	Priority                     int     `json:"priority"`
	SchedulingNote               string  `json:"scheduling_note"`
	WorkCenter                   string  `json:"work_center"`
}

type ProductionWorkstationOverviewQuery struct {
	Limit int
}

type ProductionWorkstationOverview struct {
	Date            string                        `json:"date"`
	TotalTasks      int                           `json:"total_tasks"`
	TodaySummary    ProductionTodaySummary        `json:"today_summary"`
	NavBadges       map[string]ProductionNavBadge `json:"nav_badges"`
	StatusSummary   []ProductionSummaryCount      `json:"status_summary"`
	BlockedSummary  []ProductionSummaryCount      `json:"blocked_summary"`
	PrioritySummary []ProductionSummaryCount      `json:"priority_summary"`
	WorkstationLoad []ProductionWorkstationLoad   `json:"workstation_load"`
	Tasks           []ProductionTask              `json:"tasks"`
}

type ProductionTodaySummary struct {
	PlannedTasks   int `json:"planned_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	RunningTasks   int `json:"running_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	BlockedTasks   int `json:"blocked_tasks"`
}

type ProductionNavBadge struct {
	Pending int `json:"pending"`
	Blocked int `json:"blocked"`
	Running int `json:"running"`
}

type ProductionSummaryCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type ProductionWorkstationLoad struct {
	Workstation      string `json:"workstation"`
	PendingTasks     int    `json:"pending_tasks"`
	RunningTasks     int    `json:"running_tasks"`
	BlockedTasks     int    `json:"blocked_tasks"`
	TotalTasks       int    `json:"total_tasks"`
	LoadMinutes      int    `json:"load_minutes"`
	QueueCount       int    `json:"queue_count"`
	BlockedCount     int    `json:"blocked_count"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	LoadStatus       string `json:"load_status"`
	CurrentTask      string `json:"current_task"`
	NextTask         string `json:"next_task"`
	BlockingReason   string `json:"blocking_reason"`
}

type ProductionTask struct {
	JobCardID                int64                        `json:"job_card_id"`
	WorkOrderID              int64                        `json:"work_order_id"`
	RunningItemID            int64                        `json:"running_item_id"`
	WorkOrderNo              string                       `json:"work_order_no"`
	ProductName              string                       `json:"product_name"`
	SpecG                    int64                        `json:"spec_g"`
	InventoryQtyPerSalesUnit float64                      `json:"inventory_qty_per_sales_unit"`
	InventoryUnit            string                       `json:"inventory_unit"`
	PlannedG                 int64                        `json:"planned_g"`
	PlannedUnits             int64                        `json:"planned_units"`
	PlannedLooseG            int64                        `json:"planned_loose_g"`
	PlannedOutputG           int64                        `json:"planned_output_g"`
	Operation                string                       `json:"operation"`
	ProcessRequirement       string                       `json:"process_requirement"`
	Workstation              string                       `json:"workstation"`
	WorkCenter               string                       `json:"work_center"`
	Status                   string                       `json:"status"`
	StatusLabel              string                       `json:"status_label"`
	Readiness                string                       `json:"readiness"`
	ReadinessLabel           string                       `json:"readiness_label"`
	BlockingReason           string                       `json:"blocking_reason"`
	NextHandler              string                       `json:"next_handler"`
	AssignedTo               string                       `json:"assigned_to"`
	Operator                 string                       `json:"operator"`
	PlannedStartAt           string                       `json:"planned_start_at"`
	PlannedEndAt             string                       `json:"planned_end_at"`
	OrderNos                 string                       `json:"order_nos"`
	Priority                 int                          `json:"priority"`
	PlannedMinutes           int                          `json:"planned_minutes"`
	PlannedBatchCount        int                          `json:"planned_batch_count"`
	PlannedInputQty          float64                      `json:"planned_input_qty"`
	PlannedInputInventoryQty float64                      `json:"planned_input_inventory_qty"`
	ActualMinutes            int                          `json:"actual_minutes"`
	ActualInputQty           float64                      `json:"actual_input_qty"`
	ActualOutputQty          float64                      `json:"actual_output_qty"`
	ActualLossQty            float64                      `json:"actual_loss_qty"`
	ActualLossRate           float64                      `json:"actual_loss_rate"`
	RecordsLoss              bool                         `json:"records_loss"`
	LossReason               string                       `json:"loss_reason"`
	ExceptionReason          string                       `json:"exception_reason"`
	IsBlocked                bool                         `json:"is_blocked"`
	SchedulingNote           string                       `json:"scheduling_note"`
	AvailableActions         []string                     `json:"available_actions"`
	ReadinessDetail          ProductionExecutionReadiness `json:"readiness_detail"`
	CanStart                 bool                         `json:"can_start"`
	CanComplete              bool                         `json:"can_complete"`
	BlockingReasons          []ProductionBlockingReason   `json:"blocking_reasons"`
	SuggestedAction          string                       `json:"suggested_action"`
	Severity                 string                       `json:"severity"`
	RelatedLinks             []ProductionRelatedLink      `json:"related_links"`
}

type ProductionRelatedLink struct {
	Key    string         `json:"key"`
	Label  string         `json:"label"`
	View   string         `json:"view"`
	Params map[string]any `json:"params,omitempty"`
}

type ProductionBlockingReason struct {
	Code         string                  `json:"code"`
	Label        string                  `json:"label"`
	Severity     string                  `json:"severity"`
	NextHandler  string                  `json:"next_handler"`
	RelatedLinks []ProductionRelatedLink `json:"related_links,omitempty"`
}

type ProductionExecutionReadiness struct {
	CanStart        bool                       `json:"can_start"`
	CanComplete     bool                       `json:"can_complete"`
	BlockingReasons []ProductionBlockingReason `json:"blocking_reasons"`
	NextHandler     string                     `json:"next_handler"`
	SuggestedAction string                     `json:"suggested_action"`
	Severity        string                     `json:"severity"`
	RelatedLinks    []ProductionRelatedLink    `json:"related_links"`
}

type WorkOrderExecutionHeader struct {
	WorkOrderID      int64  `json:"work_order_id"`
	WorkOrderNo      string `json:"work_order_no"`
	ProductID        int64  `json:"product_id"`
	ProductName      string `json:"product_name"`
	SpecG            int64  `json:"spec_g"`
	OrderNos         string `json:"order_nos"`
	PlannedG         int64  `json:"planned_g"`
	PlannedOutputG   int64  `json:"planned_output_g"`
	PlannedUnits     int64  `json:"planned_units"`
	PlannedLooseG    int64  `json:"planned_loose_g"`
	Status           string `json:"status"`
	BatchID          string `json:"batch_id"`
	BomVersionID     int64  `json:"bom_version_id"`
	ProductionPlanID int64  `json:"production_plan_id"`
	RunningItemID    int64  `json:"running_item_id"`
	Priority         int    `json:"priority"`
	AssignedTo       string `json:"assigned_to"`
	WorkCenter       string `json:"work_center"`
	CreatedAt        string `json:"created_at"`
}

type ProductionOperationProgress struct {
	JobCardID      int64   `json:"job_card_id"`
	SequenceNo     int     `json:"sequence_no"`
	Operation      string  `json:"operation"`
	Workstation    string  `json:"workstation"`
	Status         string  `json:"status"`
	StatusLabel    string  `json:"status_label"`
	AssignedTo     string  `json:"assigned_to"`
	Operator       string  `json:"operator"`
	PlannedMinutes int     `json:"planned_minutes"`
	ActualMinutes  int     `json:"actual_minutes"`
	PlannedCost    float64 `json:"planned_cost"`
	ActualCost     float64 `json:"actual_cost"`
	StartedAt      string  `json:"started_at"`
	CompletedAt    string  `json:"completed_at"`
	BlockingReason string  `json:"blocking_reason"`
}

type ProductionWorkstationAssignment struct {
	WorkCenter      string `json:"work_center"`
	AssignedTo      string `json:"assigned_to"`
	Priority        int    `json:"priority"`
	UnassignedCount int    `json:"unassigned_count"`
}

type ProductionWIPStatus struct {
	RequiredG      int64               `json:"required_g"`
	ReservedG      int64               `json:"reserved_g"`
	ConsumedG      int64               `json:"consumed_g"`
	RemainingG     int64               `json:"remaining_g"`
	AvailableG     int64               `json:"available_g"`
	ShortageG      int64               `json:"shortage_g"`
	RequiredUnits  int64               `json:"required_units"`
	AvailableUnits int64               `json:"available_units"`
	ShortageUnits  int64               `json:"shortage_units"`
	DataComplete   bool                `json:"data_complete"`
	Status         string              `json:"status"`
	BlockingReason string              `json:"blocking_reason"`
	Materials      []WIPReservationRow `json:"materials"`
}

type ProductionQualityStatus struct {
	Status      string `json:"status"`
	Result      string `json:"result"`
	ReferenceNo string `json:"reference_no"`
	Note        string `json:"note"`
	CheckedAt   string `json:"checked_at"`
}

type ProductionTraceTimelineEntry struct {
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	At      string         `json:"at"`
	RefType string         `json:"ref_type"`
	RefID   int64          `json:"ref_id"`
	View    string         `json:"view"`
	Params  map[string]any `json:"params,omitempty"`
}

type ProductionContextAction struct {
	Key        string         `json:"key"`
	Label      string         `json:"label"`
	ActionType string         `json:"action_type,omitempty"`
	Endpoint   string         `json:"endpoint,omitempty"`
	View       string         `json:"view,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	Disabled   bool           `json:"disabled,omitempty"`
	Reason     string         `json:"reason,omitempty"`
}

type WorkOrderExecutionHub struct {
	Header                WorkOrderExecutionHeader        `json:"header"`
	Readiness             ProductionExecutionReadiness    `json:"readiness"`
	BomSummary            string                          `json:"bom_summary"`
	RouteSummary          string                          `json:"route_summary"`
	OperationProgress     []ProductionOperationProgress   `json:"operation_progress"`
	WorkstationAssignment ProductionWorkstationAssignment `json:"workstation_assignment"`
	WIPStatus             ProductionWIPStatus             `json:"wip_status"`
	QualityStatus         ProductionQualityStatus         `json:"quality_status"`
	StockEntries          []StockEntryRow                 `json:"stock_entries"`
	FinishedReceipts      []StockEntryRow                 `json:"finished_receipts"`
	CostSummary           BatchCostRow                    `json:"cost_summary"`
	TraceTimeline         []ProductionTraceTimelineEntry  `json:"trace_timeline"`
	ContextActions        []ProductionContextAction       `json:"context_actions"`
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
	RunningItemID int64
	Limit         int
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
	ID            int64                   `json:"id,omitempty"`
	EntryNo       string                  `json:"entry_no,omitempty"`
	Status        string                  `json:"status,omitempty"`
	EntryType     string                  `json:"entry_type"`
	Purpose       string                  `json:"purpose"`
	IsReturn      bool                    `json:"is_return"`
	WorkOrderID   int64                   `json:"work_order_id"`
	WorkOrderNo   string                  `json:"work_order_no,omitempty"`
	JobCardID     int64                   `json:"job_card_id"`
	RunningItemID int64                   `json:"running_item_id"`
	SourceType    string                  `json:"source_type"`
	SourceID      int64                   `json:"source_id"`
	ReturnSource  string                  `json:"return_source"`
	Operator      string                  `json:"operator"`
	Note          string                  `json:"note"`
	Items         []StockEntryItemCommand `json:"items"`
}

type StockDocumentPreviewCommand struct {
	ID              int64
	Action          string
	StockDocumentID int64
	MaterialID      int64
	JobCardID       int64
	RunningItemID   int64
	ReturnSource    string
	Operator        string
}

type StockDocumentPreview struct {
	Action    string            `json:"action"`
	WorkOrder WorkOrderRow      `json:"work_order"`
	Document  StockEntryCommand `json:"document"`
	Warnings  []string          `json:"warnings,omitempty"`
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
	InventoryUnit string  `json:"inventory_unit,omitempty"`
	QuantityBasis string  `json:"quantity_basis,omitempty"`
	RequiredQty   float64 `json:"required_qty,omitempty"`
	RemainingQty  float64 `json:"remaining_qty"`
	RememberedQty float64 `json:"remembered_qty,omitempty"`
	DefaultQty    float64 `json:"default_qty,omitempty"`
	BatchCode     string  `json:"batch_code"`
	UnitCost      float64 `json:"unit_cost"`
}

type StockEntryQuery struct {
	EntryType   string
	Purpose     string
	Status      string
	WorkOrderID int64
	JobCardID   int64
	Limit       int
}

type StockEntryRow struct {
	ID            int64   `json:"id"`
	EntryNo       string  `json:"entry_no"`
	EntryType     string  `json:"entry_type"`
	Purpose       string  `json:"purpose"`
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
	Purpose       string              `json:"purpose"`
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

type WorkOrderLedgerQuery struct {
	WorkOrderID   int64
	RunningItemID int64
	Limit         int
}

type WorkOrderLedgerEntryRow struct {
	ID              int64  `json:"id"`
	StockEntryID    int64  `json:"stock_entry_id"`
	EntryNo         string `json:"entry_no"`
	EntryType       string `json:"entry_type"`
	Purpose         string `json:"purpose"`
	ItemType        string `json:"item_type"`
	ItemID          int64  `json:"item_id"`
	ItemName        string `json:"item_name"`
	SpecG           int64  `json:"spec_g"`
	Warehouse       string `json:"warehouse"`
	QtyChangeG      int64  `json:"qty_change_g"`
	QtyAfterG       int64  `json:"qty_after_g"`
	QtyChangeUnits  int64  `json:"qty_change_units"`
	QtyAfterUnits   int64  `json:"qty_after_units"`
	SourceDocType   string `json:"source_doc_type"`
	SourceDocID     int64  `json:"source_doc_id"`
	SourceBatchCode string `json:"source_batch_code"`
	Operator        string `json:"operator"`
	CreatedAt       string `json:"created_at"`
}

type WorkOrderDetail struct {
	WorkOrder      WorkOrderRow              `json:"work_order"`
	Materials      []WIPReservationRow       `json:"materials"`
	JobCards       []JobCardRow              `json:"job_cards"`
	StockDocuments []StockEntryRow           `json:"stock_documents"`
	StockEntries   []StockEntryRow           `json:"stock_entries"`
	LedgerEntries  []WorkOrderLedgerEntryRow `json:"ledger_entries"`
	ProductionLogs ProductionLogsResult      `json:"production_logs"`
	CostSummary    BatchCostRow              `json:"cost_summary"`
	ExecutionHub   WorkOrderExecutionHub     `json:"execution_hub"`
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
	From                  string
	To                    string
	CustomerID            int64
	Selected              map[string]bool
	IncludedDemandKeys    map[string]bool
	InputByKey            map[string]int64
	InputByDemandKey      map[string]int64
	BomVersionByDemandKey map[string]int64
	BomLossByDemandKey    map[string]float64
	SkipDemandKeys        map[string]bool
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
	ID                 int64   `json:"id"`
	WorkOrderID        int64   `json:"work_order_id"`
	WorkOrderNo        string  `json:"work_order_no"`
	RunningItemID      int64   `json:"running_item_id"`
	ProductName        string  `json:"product_name"`
	MaterialID         int64   `json:"material_id"`
	MaterialName       string  `json:"material_name"`
	Unit               string  `json:"unit"`
	RequiredG          int64   `json:"required_g"`
	RequiredUnits      int64   `json:"required_units"`
	ReservedG          int64   `json:"reserved_g"`
	ReservedUnits      int64   `json:"reserved_units"`
	ConsumedG          int64   `json:"consumed_g"`
	ConsumedUnits      int64   `json:"consumed_units"`
	ReturnedG          int64   `json:"returned_g"`
	ReturnedUnits      int64   `json:"returned_units"`
	RemainingReservedG int64   `json:"remaining_reserved_g"`
	Status             string  `json:"status"`
	WIPG               int64   `json:"wip_g"`
	AvailableG         int64   `json:"available_g"`
	WIPUnits           int64   `json:"wip_units"`
	AvailableUnits     int64   `json:"available_units"`
	ShortageG          int64   `json:"shortage_g"`
	ShortageUnits      int64   `json:"shortage_units"`
	InventoryUnit      string  `json:"inventory_unit"`
	QuantityBasis      string  `json:"quantity_basis"`
	RequiredQty        float64 `json:"required_qty"`
	AvailableQty       float64 `json:"available_qty"`
	ShortageQty        float64 `json:"shortage_qty"`
	RememberedQty      float64 `json:"remembered_qty"`
	UpdatedAt          string  `json:"updated_at"`
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
	SaveProductionPlanOperationSplits(ctx context.Context, cmd SaveProductionPlanOperationSplitsCommand) ([]ProductionPlanOperationSplit, error)
	PreviewProductionPlanOperationSplits(ctx context.Context, cmd PreviewProductionPlanOperationSplitsCommand) (ProductionPlanOperationSplitPreview, error)
	SaveWorkOrderOperationSplits(ctx context.Context, cmd SaveWorkOrderOperationSplitsCommand) (WorkOrderOperationSplitsResult, error)
	SubmitProductionPlan(ctx context.Context, cmd SubmitProductionPlanCommand) (ProductionPlanSubmitResult, error)
	CancelProductionPlan(ctx context.Context, cmd CancelProductionPlanCommand) (ProductionPlanDetail, error)
	StartWorkOrder(ctx context.Context, cmd WorkOrderStartCommand) (WorkOrderStartResult, error)
	CompleteWorkOrder(ctx context.Context, cmd WorkOrderCompleteCommand) (WorkOrderCompleteResult, error)
	CancelWorkOrder(ctx context.Context, cmd WorkOrderCancelCommand) (WorkOrderRow, error)
	SaveScheduleAssignment(ctx context.Context, cmd ScheduleAssignmentCommand) (ScheduleAssignmentResult, error)
	SaveCapacityCalendar(ctx context.Context, cmd CapacityCalendarCommand) (CapacityCalendarRow, error)
	ScheduleBoard(ctx context.Context, query ScheduleBoardQuery) (ScheduleBoardResult, error)
	MRPSuggestions(ctx context.Context, query MRPSuggestionQuery) (MRPSuggestionResult, error)
	ProductionTraceAnalytics(ctx context.Context, query ProductionTraceAnalyticsQuery) (ProductionTraceAnalyticsResult, error)
	CreateStockEntry(ctx context.Context, cmd StockEntryCommand) (StockEntryDetail, error)
	ListStockEntries(ctx context.Context, query StockEntryQuery) ([]StockEntryRow, error)
	GetStockEntry(ctx context.Context, id int64) (StockEntryDetail, error)
	ListWorkOrderLedgerEntries(ctx context.Context, query WorkOrderLedgerQuery) ([]WorkOrderLedgerEntryRow, error)
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

// workOrderWIPCoverageRepository is intentionally optional so older adapters
// and focused application fakes remain compatible while the PostgreSQL
// implementation supplies the authoritative frozen-snapshot projection.
type workOrderWIPCoverageRepository interface {
	GetWorkOrderWIPCoverage(ctx context.Context, workOrderID int64) (ProductionWIPStatus, error)
}

type workOrderStockDraftRepository interface {
	GetWorkOrderStockDocumentDraft(ctx context.Context, workOrderID int64, action string, stockDocumentID int64) (*StockEntryCommand, error)
}

// productionPlanTargetWarehouseRepository is optional so focused adapters and
// existing application fakes remain source-compatible. PostgreSQL implements
// the draft-only, audited warehouse freeze used by the production-plan API.
type productionPlanTargetWarehouseRepository interface {
	UpdateProductionPlanItemTargetWarehouse(ctx context.Context, cmd UpdateProductionPlanItemTargetWarehouseCommand) (ProductionPlanItem, error)
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

func (s *Service) UpdateProductionPlanItemTargetWarehouse(ctx context.Context, cmd UpdateProductionPlanItemTargetWarehouseCommand) (ProductionPlanItem, error) {
	if cmd.ProductionPlanID <= 0 {
		return ProductionPlanItem{}, fmt.Errorf("production_plan_id required")
	}
	if cmd.ProductionPlanItemID <= 0 {
		return ProductionPlanItem{}, fmt.Errorf("production_plan_item_id required")
	}
	cmd.TargetWarehouse = strings.TrimSpace(cmd.TargetWarehouse)
	if cmd.TargetWarehouse == "" {
		return ProductionPlanItem{}, fmt.Errorf("target_warehouse required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	repo, ok := s.repo.(productionPlanTargetWarehouseRepository)
	if !ok {
		return ProductionPlanItem{}, fmt.Errorf("production plan target warehouse update not supported")
	}
	return repo.UpdateProductionPlanItemTargetWarehouse(ctx, cmd)
}

func (s *Service) SaveProductionPlanOperationSplits(ctx context.Context, cmd SaveProductionPlanOperationSplitsCommand) ([]ProductionPlanOperationSplit, error) {
	if cmd.ID <= 0 {
		return nil, fmt.Errorf("production_plan_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	for i := range cmd.Items {
		item := &cmd.Items[i]
		item.ProductionPlanID = cmd.ID
		item.Operation = strings.TrimSpace(item.Operation)
		item.Workstation = strings.TrimSpace(item.Workstation)
		item.WorkstationCapacityName = strings.TrimSpace(item.WorkstationCapacityName)
		item.BatchSizeUnit = strings.TrimSpace(item.BatchSizeUnit)
		item.Note = strings.TrimSpace(item.Note)
		if item.ProductionPlanItemID <= 0 {
			return nil, fmt.Errorf("production_plan_item_id required")
		}
		if item.WorkstationCapacityID <= 0 {
			return nil, fmt.Errorf("workstation_capacity_id required")
		}
		if item.PlannedQty <= 0 {
			return nil, fmt.Errorf("planned_qty required")
		}
		if item.OperationSeq < 0 {
			return nil, fmt.Errorf("operation_seq must be >= 0")
		}
		if item.BatchSizeQty < 0 {
			return nil, fmt.Errorf("batch_size_qty must be >= 0")
		}
		if item.StandardMinutes < 0 {
			return nil, fmt.Errorf("standard_minutes must be >= 0")
		}
		if item.HourlyRate < 0 {
			return nil, fmt.Errorf("hourly_rate must be >= 0")
		}
	}
	return s.repo.SaveProductionPlanOperationSplits(ctx, cmd)
}

func (s *Service) PreviewProductionPlanOperationSplits(ctx context.Context, cmd PreviewProductionPlanOperationSplitsCommand) (ProductionPlanOperationSplitPreview, error) {
	if cmd.ID <= 0 {
		return ProductionPlanOperationSplitPreview{}, fmt.Errorf("production_plan_id required")
	}
	for i := range cmd.Items {
		item := &cmd.Items[i]
		item.ProductionPlanID = cmd.ID
		item.Operation = strings.TrimSpace(item.Operation)
		item.Workstation = strings.TrimSpace(item.Workstation)
		item.WorkstationCapacityName = strings.TrimSpace(item.WorkstationCapacityName)
		item.BatchSizeUnit = strings.TrimSpace(item.BatchSizeUnit)
		item.Note = strings.TrimSpace(item.Note)
		if item.ProductionPlanItemID <= 0 {
			return ProductionPlanOperationSplitPreview{}, fmt.Errorf("production_plan_item_id required")
		}
		if item.WorkstationCapacityID <= 0 {
			return ProductionPlanOperationSplitPreview{}, fmt.Errorf("workstation_capacity_id required")
		}
		if item.PlannedQty <= 0 {
			return ProductionPlanOperationSplitPreview{}, fmt.Errorf("planned_qty required")
		}
		if item.OperationSeq < 0 {
			return ProductionPlanOperationSplitPreview{}, fmt.Errorf("operation_seq must be >= 0")
		}
	}
	return s.repo.PreviewProductionPlanOperationSplits(ctx, cmd)
}

func (s *Service) SaveWorkOrderOperationSplits(ctx context.Context, cmd SaveWorkOrderOperationSplitsCommand) (WorkOrderOperationSplitsResult, error) {
	if cmd.ID <= 0 {
		return WorkOrderOperationSplitsResult{}, fmt.Errorf("work_order_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	for i := range cmd.Items {
		item := &cmd.Items[i]
		item.ProductionPlanID = 0
		item.ProductionPlanItemID = 0
		item.Operation = strings.TrimSpace(item.Operation)
		item.Workstation = strings.TrimSpace(item.Workstation)
		item.WorkstationCapacityName = strings.TrimSpace(item.WorkstationCapacityName)
		item.BatchSizeUnit = strings.TrimSpace(item.BatchSizeUnit)
		item.Note = strings.TrimSpace(item.Note)
		if item.WorkstationCapacityID <= 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("workstation_capacity_id required")
		}
		if item.PlannedQty <= 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("planned_qty required")
		}
		if item.OperationSeq < 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("operation_seq must be >= 0")
		}
		if item.BatchSizeQty < 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("batch_size_qty must be >= 0")
		}
		if item.StandardMinutes < 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("standard_minutes must be >= 0")
		}
		if item.HourlyRate < 0 {
			return WorkOrderOperationSplitsResult{}, fmt.Errorf("hourly_rate must be >= 0")
		}
	}
	return s.repo.SaveWorkOrderOperationSplits(ctx, cmd)
}

func (s *Service) SubmitProductionPlan(ctx context.Context, cmd SubmitProductionPlanCommand) (ProductionPlanSubmitResult, error) {
	if cmd.ID <= 0 {
		return ProductionPlanSubmitResult{}, fmt.Errorf("production_plan_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	return s.repo.SubmitProductionPlan(ctx, cmd)
}

func (s *Service) CancelProductionPlan(ctx context.Context, cmd CancelProductionPlanCommand) (ProductionPlanDetail, error) {
	if cmd.ID <= 0 {
		return ProductionPlanDetail{}, fmt.Errorf("production_plan_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.CancelProductionPlan(ctx, cmd)
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
	if cmd.FinishedUnits <= 0 && cmd.FinishedLooseG <= 0 && cmd.FinishedQtyG <= 0 && cmd.FinishedQtyUnits <= 0 {
		return WorkOrderCompleteResult{}, fmt.Errorf("finished output required")
	}
	if cmd.ConsumedInputG < 0 {
		return WorkOrderCompleteResult{}, fmt.Errorf("consumed_input_g must be >= 0")
	}
	cmd.Warehouse = strings.TrimSpace(cmd.Warehouse)
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return WorkOrderCompleteResult{}, fmt.Errorf("operator required")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.CompleteWorkOrder(ctx, cmd)
}

func (s *Service) CancelWorkOrder(ctx context.Context, cmd WorkOrderCancelCommand) (WorkOrderRow, error) {
	if cmd.ID <= 0 {
		return WorkOrderRow{}, fmt.Errorf("work_order_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return WorkOrderRow{}, fmt.Errorf("operator required")
	}
	cmd.Note = strings.TrimSpace(cmd.Note)
	return s.repo.CancelWorkOrder(ctx, cmd)
}

func (s *Service) IssueWorkOrderMaterials(ctx context.Context, cmd WorkOrderIssueMaterialsCommand) (StockEntryDetail, error) {
	if cmd.ID <= 0 {
		return StockEntryDetail{}, fmt.Errorf("work_order_id required")
	}
	cmd.Operator = strings.TrimSpace(cmd.Operator)
	if cmd.Operator == "" {
		return StockEntryDetail{}, fmt.Errorf("operator required")
	}
	return s.CreateStockEntry(ctx, StockEntryCommand{
		Purpose:     "material_transfer_for_manufacture",
		WorkOrderID: cmd.ID,
		SourceType:  "work_order",
		SourceID:    cmd.ID,
		Operator:    cmd.Operator,
		Note:        cmd.Note,
		Items:       cmd.Items,
	})
}

func (s *Service) PreviewWorkOrderStockDocument(ctx context.Context, cmd StockDocumentPreviewCommand) (StockDocumentPreview, error) {
	if cmd.ID <= 0 {
		return StockDocumentPreview{}, fmt.Errorf("work_order_id required")
	}
	cmd.Action = strings.TrimSpace(cmd.Action)
	switch cmd.Action {
	case "issue", "supplement", "return", "consume", "finish":
	default:
		return StockDocumentPreview{}, fmt.Errorf("invalid stock document action")
	}
	detail, err := s.GetWorkOrderDetail(ctx, cmd.ID)
	if err != nil {
		return StockDocumentPreview{}, err
	}
	if detail.WorkOrder.Status == "cancelled" || detail.WorkOrder.Status == "completed" {
		return StockDocumentPreview{}, fmt.Errorf("work order is not open")
	}
	if cmd.JobCardID > 0 {
		belongs := false
		for _, jobCard := range detail.JobCards {
			if jobCard.ID == cmd.JobCardID && jobCard.WorkOrderID == detail.WorkOrder.ID {
				belongs = true
				break
			}
		}
		if !belongs {
			return StockDocumentPreview{}, fmt.Errorf("job card does not belong to work order")
		}
	}
	if draftRepo, ok := s.repo.(workOrderStockDraftRepository); ok {
		draft, err := draftRepo.GetWorkOrderStockDocumentDraft(ctx, detail.WorkOrder.ID, cmd.Action, cmd.StockDocumentID)
		if err != nil {
			return StockDocumentPreview{}, err
		}
		if draft == nil && cmd.StockDocumentID > 0 {
			return StockDocumentPreview{}, fmt.Errorf("指定库存草稿不存在，或不属于当前工单和库存动作")
		}
		if draft != nil {
			draftCopy := *draft
			draftCopy.Items = append([]StockEntryItemCommand(nil), draft.Items...)
			draft = &draftCopy
			draft.WorkOrderID = detail.WorkOrder.ID
			draft.WorkOrderNo = detail.WorkOrder.WorkOrderNo
			draft.RunningItemID = detail.WorkOrder.RunningItemID
			warnings := refreshWorkOrderStockDocumentDraft(draft, detail.Materials, cmd.Action)
			return StockDocumentPreview{
				Action: cmd.Action, WorkOrder: detail.WorkOrder, Document: *draft, Warnings: warnings,
			}, nil
		}
	} else if cmd.StockDocumentID > 0 {
		return StockDocumentPreview{}, fmt.Errorf("当前库存仓储不支持按指定草稿打开")
	}
	document := StockEntryCommand{
		WorkOrderID:   detail.WorkOrder.ID,
		WorkOrderNo:   detail.WorkOrder.WorkOrderNo,
		JobCardID:     cmd.JobCardID,
		RunningItemID: detail.WorkOrder.RunningItemID,
		SourceType:    "work_order",
		SourceID:      detail.WorkOrder.ID,
		ReturnSource:  strings.TrimSpace(cmd.ReturnSource),
		Operator:      strings.TrimSpace(cmd.Operator),
	}
	if document.ReturnSource == "" {
		document.ReturnSource = "work_order"
	}
	switch cmd.Action {
	case "issue", "supplement":
		document.Purpose = "material_transfer_for_manufacture"
		document.Note = map[bool]string{true: "工单补料", false: "工单生产领料"}[cmd.Action == "supplement"]
	case "return":
		document.Purpose = "material_transfer_for_manufacture"
		document.IsReturn = true
		document.Note = "退回未用原料"
	case "consume":
		document.Purpose = "material_consumption_for_manufacture"
		document.Note = "记录生产消耗"
	case "finish":
		document.Purpose = "manufacture"
		document.Note = "完工入库"
	}
	if cmd.Action == "finish" {
		workOrder := detail.WorkOrder
		outputType := strings.ToLower(strings.TrimSpace(workOrder.OutputType))
		if outputType == "material" {
			qtyG, qtyUnits := canonicalManufacturingOutputQuantity(workOrder.OutputQty, workOrder.OutputUnit)
			warehouse := strings.TrimSpace(workOrder.TargetWarehouse)
			if warehouse == "" {
				warehouse = stockdomain.WarehouseWIP
			}
			quantityBasis := "count"
			if qtyG > 0 {
				quantityBasis = "weight"
			}
			document.Items = []StockEntryItemCommand{{
				MaterialID:    workOrder.OutputMaterialID,
				ItemType:      "material",
				ItemName:      firstNonEmptyString(workOrder.OutputName, workOrder.ProductName),
				InventoryUnit: strings.TrimSpace(workOrder.OutputUnit),
				QuantityBasis: quantityBasis,
				DefaultQty:    workOrder.OutputQty,
				ToWarehouse:   warehouse,
				QtyG:          qtyG,
				QtyUnits:      qtyUnits,
			}}
		} else {
			qtyG := workOrder.PlannedLooseG
			qtyUnits := workOrder.PlannedUnits
			if qtyUnits <= 0 && qtyG <= 0 {
				qtyG = workOrder.PlannedOutputG
			}
			warehouse := strings.TrimSpace(workOrder.TargetWarehouse)
			if warehouse == "" {
				warehouse = stockdomain.WarehouseFinishedGoods
			}
			productID := workOrder.OutputProductID
			if productID <= 0 {
				productID = workOrder.ProductID
			}
			document.Items = []StockEntryItemCommand{{
				ProductID:   productID,
				ItemType:    "finished_product",
				ItemName:    firstNonEmptyString(workOrder.OutputName, workOrder.ProductName),
				SpecG:       workOrder.SpecG,
				ToWarehouse: warehouse,
				QtyG:        qtyG,
				QtyUnits:    qtyUnits,
			}}
		}
	} else {
		wipByMaterial := workOrderWIPBalances(detail.LedgerEntries)
		consumedByMaterial := workOrderMaterialConsumptionBalances(detail.LedgerEntries)
		for _, material := range detail.Materials {
			if cmd.MaterialID > 0 && material.MaterialID != cmd.MaterialID {
				continue
			}
			qtyG := material.RequiredG
			qtyUnits := material.RequiredUnits
			defaultQty := material.ShortageQty
			remainingQty := material.ShortageQty
			wip := wipByMaterial[material.MaterialID]
			if cmd.Action == "issue" || cmd.Action == "supplement" {
				if material.QuantityBasis != "" {
					qtyG = material.ShortageG
					qtyUnits = material.ShortageUnits
					if material.RememberedQty > 0 {
						defaultQty = material.RememberedQty
						if material.QuantityBasis == "weight" {
							qtyG = inventoryQuantityToGrams(defaultQty, material.InventoryUnit)
						} else {
							qtyUnits = int64(math.Ceil(defaultQty))
						}
					}
				} else {
					issuedG := wip.QtyG + material.ConsumedG + material.ReturnedG
					issuedUnits := wip.QtyUnits + material.ConsumedUnits + material.ReturnedUnits
					qtyG = nonnegativeInt64(material.RequiredG - issuedG)
					qtyUnits = nonnegativeInt64(material.RequiredUnits - issuedUnits)
				}
			}
			if cmd.Action == "return" || cmd.Action == "consume" {
				qtyG = nonnegativeInt64(wip.QtyG)
				qtyUnits = nonnegativeInt64(wip.QtyUnits)
			}
			if cmd.Action == "consume" {
				consumed := consumedByMaterial[material.MaterialID]
				consumedG := greaterInt64(nonnegativeInt64(material.ConsumedG), nonnegativeInt64(consumed.QtyG))
				consumedUnits := greaterInt64(nonnegativeInt64(material.ConsumedUnits), nonnegativeInt64(consumed.QtyUnits))
				remainingG := nonnegativeInt64(material.RequiredG - consumedG)
				remainingUnits := nonnegativeInt64(material.RequiredUnits - consumedUnits)
				qtyG = lesserInt64(qtyG, remainingG)
				qtyUnits = lesserInt64(qtyUnits, remainingUnits)
				if material.QuantityBasis == "weight" {
					remainingQty = inventoryQuantityFromGrams(remainingG, material.InventoryUnit)
					defaultQty = inventoryQuantityFromGrams(qtyG, material.InventoryUnit)
				} else {
					remainingQty = float64(remainingUnits)
					defaultQty = float64(qtyUnits)
				}
			}
			if qtyG <= 0 && qtyUnits <= 0 && cmd.Action != "issue" && cmd.Action != "supplement" {
				continue
			}
			fromWarehouse, toWarehouse := stockdomain.WarehouseRawMaterials, stockdomain.WarehouseWIP
			if cmd.Action == "return" {
				fromWarehouse, toWarehouse = stockdomain.WarehouseWIP, stockdomain.WarehouseRawMaterials
			}
			if cmd.Action == "consume" {
				fromWarehouse, toWarehouse = stockdomain.WarehouseWIP, ""
			}
			document.Items = append(document.Items, StockEntryItemCommand{
				MaterialID: material.MaterialID, ItemType: "material", ItemName: material.MaterialName,
				FromWarehouse: fromWarehouse, ToWarehouse: toWarehouse, QtyG: qtyG, QtyUnits: qtyUnits,
				InventoryUnit: material.InventoryUnit, QuantityBasis: material.QuantityBasis,
				RequiredQty: material.RequiredQty, RemainingQty: remainingQty,
				RememberedQty: material.RememberedQty, DefaultQty: defaultQty,
			})
		}
	}
	if len(document.Items) == 0 {
		return StockDocumentPreview{}, fmt.Errorf("no stock document items available")
	}
	return StockDocumentPreview{Action: cmd.Action, WorkOrder: detail.WorkOrder, Document: document}, nil
}

func refreshWorkOrderStockDocumentDraft(draft *StockEntryCommand, materials []WIPReservationRow, action string) []string {
	if draft == nil || (action != "issue" && action != "supplement") {
		return nil
	}
	coverageByMaterial := make(map[int64]WIPReservationRow, len(materials))
	for _, material := range materials {
		if material.MaterialID > 0 {
			coverageByMaterial[material.MaterialID] = material
		}
	}
	warnings := make([]string, 0)
	refreshedItems := make([]StockEntryItemCommand, 0, len(draft.Items))
	for index := range draft.Items {
		item := draft.Items[index]
		material, ok := coverageByMaterial[item.MaterialID]
		if !ok {
			refreshedItems = append(refreshedItems, item)
			continue
		}
		item.InventoryUnit = material.InventoryUnit
		item.QuantityBasis = material.QuantityBasis
		item.RequiredQty = material.RequiredQty
		item.RemainingQty = material.ShortageQty
		item.RememberedQty = material.RememberedQty
		currentQty := float64(item.QtyUnits)
		if material.QuantityBasis == "weight" {
			currentQty = inventoryQuantityFromGrams(item.QtyG, material.InventoryUnit)
		}
		item.DefaultQty = currentQty
		if material.ShortageQty <= 0 {
			if currentQty > 0 {
				warnings = append(warnings, fmt.Sprintf(
					"%s：当前工单 WIP 已满足，草稿%s仍保留；提交后作为可用 WIP 库存保留",
					material.MaterialName,
					productionInventoryQuantityLabel(currentQty, material.InventoryUnit),
				))
			}
			refreshedItems = append(refreshedItems, item)
			continue
		}
		if currentQty <= material.ShortageQty {
			refreshedItems = append(refreshedItems, item)
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s：当前建议领用%s，草稿保留%s；超出部分提交后作为可用 WIP 库存保留",
			material.MaterialName,
			productionInventoryQuantityLabel(material.ShortageQty, material.InventoryUnit),
			productionInventoryQuantityLabel(currentQty, material.InventoryUnit),
		))
		refreshedItems = append(refreshedItems, item)
	}
	draft.Items = refreshedItems
	return warnings
}

func productionInventoryQuantityLabel(quantity float64, unit string) string {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		unit = "件"
	}
	return strconv.FormatFloat(quantity, 'f', -1, 64) + unit
}

type workOrderWIPBalance struct {
	QtyG     int64
	QtyUnits int64
}

func nonnegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func lesserInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func greaterInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func inventoryQuantityToGrams(value float64, unit string) int64 {
	factor := 1.0
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		factor = 1000
	case "lb", "磅":
		factor = 453.59237
	}
	return int64(math.Ceil(value * factor))
}

func canonicalManufacturingOutputQuantity(value float64, unit string) (int64, int64) {
	if value <= 0 {
		return 0, 0
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克", "kg", "千克", "公斤", "lb", "磅":
		return inventoryQuantityToGrams(value, unit), 0
	default:
		return 0, int64(math.Ceil(value))
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func inventoryQuantityFromGrams(value int64, unit string) float64 {
	factor := 1.0
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		factor = 1000
	case "lb", "磅":
		factor = 453.59237
	}
	return float64(value) / factor
}

func workOrderWIPBalances(entries []WorkOrderLedgerEntryRow) map[int64]workOrderWIPBalance {
	out := make(map[int64]workOrderWIPBalance)
	for _, entry := range entries {
		if entry.ItemType != "material" || entry.ItemID <= 0 || entry.Warehouse != stockdomain.WarehouseWIP {
			continue
		}
		current := out[entry.ItemID]
		current.QtyG += entry.QtyChangeG
		current.QtyUnits += entry.QtyChangeUnits
		out[entry.ItemID] = current
	}
	return out
}

func workOrderMaterialConsumptionBalances(entries []WorkOrderLedgerEntryRow) map[int64]workOrderWIPBalance {
	out := make(map[int64]workOrderWIPBalance)
	for _, entry := range entries {
		if entry.ItemType != "material" ||
			entry.ItemID <= 0 ||
			entry.Warehouse != stockdomain.WarehouseWIP ||
			strings.TrimSpace(strings.ToLower(entry.Purpose)) != "material_consumption_for_manufacture" {
			continue
		}
		current := out[entry.ItemID]
		current.QtyG -= entry.QtyChangeG
		current.QtyUnits -= entry.QtyChangeUnits
		out[entry.ItemID] = current
	}
	return out
}

func (s *Service) GetWorkOrderDetail(ctx context.Context, id int64) (WorkOrderDetail, error) {
	if id <= 0 {
		return WorkOrderDetail{}, fmt.Errorf("work_order_id required")
	}
	workOrders, err := s.ListWorkOrders(ctx, WorkOrderQuery{ID: id, Limit: 1})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	if len(workOrders) == 0 {
		return WorkOrderDetail{}, fmt.Errorf("work order not found")
	}
	wo := workOrders[0]
	reservations, err := s.ListWIPReservations(ctx, WIPReservationQuery{WorkOrderNo: wo.WorkOrderNo, Limit: 200})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	wipStatus := buildProductionWIPStatus(reservations.Rows)
	if coverageRepo, ok := s.repo.(workOrderWIPCoverageRepository); ok {
		wipStatus, err = coverageRepo.GetWorkOrderWIPCoverage(ctx, wo.ID)
		if err != nil {
			return WorkOrderDetail{}, err
		}
	}
	materials := wipStatus.Materials
	jobCards, err := s.ListJobCards(ctx, JobCardQuery{WorkOrderID: wo.ID, Limit: 200})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	stockEntries, err := s.ListStockEntries(ctx, StockEntryQuery{WorkOrderID: wo.ID, Limit: 200})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	ledgerEntries, err := s.ListWorkOrderLedgerEntries(ctx, WorkOrderLedgerQuery{WorkOrderID: wo.ID, RunningItemID: wo.RunningItemID, Limit: 200})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	logs, err := s.ListProductionLogs(ctx, ProductionLogsQuery{RunningItemID: wo.RunningItemID, Limit: 50})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	costs, err := s.ListBatchCosts(ctx, BatchCostQuery{RunningItemID: wo.RunningItemID, Limit: 1})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	cost := BatchCostRow{RunningItemID: wo.RunningItemID, BatchID: wo.BatchID, ProductName: wo.ProductName, TotalCost: wo.ActualCost}
	if len(costs) > 0 {
		cost = costs[0]
	}
	qualityRows, err := s.ListQualityInspections(ctx, QualityInspectionQuery{Scope: "work_order", Limit: 200})
	if err != nil {
		return WorkOrderDetail{}, err
	}
	return WorkOrderDetail{
		WorkOrder:      wo,
		Materials:      materials,
		JobCards:       jobCards,
		StockDocuments: stockEntries,
		StockEntries:   stockEntries,
		LedgerEntries:  ledgerEntries,
		ProductionLogs: logs,
		CostSummary:    cost,
		ExecutionHub:   buildWorkOrderExecutionHub(wo, wipStatus, jobCards, stockEntries, ledgerEntries, logs, cost, qualityRows),
	}, nil
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
	cmd.Purpose = normalizeStockEntryPurpose(cmd.Purpose)
	if cmd.Purpose != "" {
		cmd.EntryType = normalizeStockEntryType(cmd.Purpose)
	} else {
		cmd.EntryType = normalizeStockEntryType(cmd.EntryType)
	}
	if cmd.EntryType == "" {
		return StockEntryDetail{}, fmt.Errorf("invalid stock entry type")
	}
	if cmd.Purpose == "" {
		cmd.Purpose = stockEntryPurposeForType(cmd.EntryType)
	}
	cmd.SourceType = strings.TrimSpace(cmd.SourceType)
	if cmd.SourceType == "" && cmd.WorkOrderID > 0 {
		cmd.SourceType = "work_order"
		cmd.SourceID = cmd.WorkOrderID
	}
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
	detail, err := s.repo.CreateStockEntry(ctx, cmd)
	if err != nil {
		return StockEntryDetail{}, err
	}
	return hydrateStockEntryDetailPurpose(detail), nil
}

func (s *Service) ListStockEntries(ctx context.Context, query StockEntryQuery) ([]StockEntryRow, error) {
	query.Purpose = normalizeStockEntryPurpose(query.Purpose)
	if query.Purpose != "" {
		query.EntryType = normalizeStockEntryType(query.Purpose)
	} else {
		query.EntryType = normalizeStockEntryType(query.EntryType)
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 200
	}
	rows, err := s.repo.ListStockEntries(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i] = hydrateStockEntryRowPurpose(rows[i])
	}
	return rows, nil
}

func (s *Service) GetStockEntry(ctx context.Context, id int64) (StockEntryDetail, error) {
	if id <= 0 {
		return StockEntryDetail{}, fmt.Errorf("stock_entry_id required")
	}
	detail, err := s.repo.GetStockEntry(ctx, id)
	if err != nil {
		return StockEntryDetail{}, err
	}
	return hydrateStockEntryDetailPurpose(detail), nil
}

func (s *Service) ListWorkOrderLedgerEntries(ctx context.Context, query WorkOrderLedgerQuery) ([]WorkOrderLedgerEntryRow, error) {
	if query.WorkOrderID <= 0 && query.RunningItemID <= 0 {
		return nil, fmt.Errorf("work_order_id or running_item_id required")
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	rows, err := s.repo.ListWorkOrderLedgerEntries(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].EntryType = normalizeStockEntryType(rows[i].EntryType)
		if rows[i].Purpose == "" {
			rows[i].Purpose = stockEntryPurposeForType(rows[i].EntryType)
		}
	}
	return rows, nil
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
	if query.RunningItemID < 0 {
		return ProductionLogsResult{}, fmt.Errorf("running_item_id must be >= 0")
	}
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListProductionLogs(ctx, query)
}

func (s *Service) ProductionWorkstationOverview(ctx context.Context, query ProductionWorkstationOverviewQuery) (ProductionWorkstationOverview, error) {
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	workOrders, err := s.ListWorkOrders(ctx, WorkOrderQuery{Limit: limit})
	if err != nil {
		return ProductionWorkstationOverview{}, err
	}
	jobCards, err := s.ListJobCards(ctx, JobCardQuery{Limit: limit})
	if err != nil {
		return ProductionWorkstationOverview{}, err
	}

	workOrderByID := make(map[int64]WorkOrderRow, len(workOrders))
	for _, row := range workOrders {
		workOrderByID[row.ID] = row
	}

	activeWorkOrders := map[int64]bool{}
	tasks := make([]ProductionTask, 0, len(jobCards)+len(workOrders))
	for _, card := range jobCards {
		if !isActiveProductionTaskStatus(card.Status) {
			continue
		}
		activeWorkOrders[card.WorkOrderID] = true
		tasks = append(tasks, productionTaskFromJobCard(card, workOrderByID[card.WorkOrderID]))
	}
	for _, workOrder := range workOrders {
		if activeWorkOrders[workOrder.ID] || !isActiveProductionTaskStatus(workOrder.Status) {
			continue
		}
		tasks = append(tasks, productionTaskFromWorkOrder(workOrder))
	}
	if coverageRepo, ok := s.repo.(workOrderWIPCoverageRepository); ok {
		coverageByWorkOrder := make(map[int64]ProductionWIPStatus)
		for i := range tasks {
			task := &tasks[i]
			coverage, found := coverageByWorkOrder[task.WorkOrderID]
			if !found {
				coverage, err = coverageRepo.GetWorkOrderWIPCoverage(ctx, task.WorkOrderID)
				if err != nil {
					return ProductionWorkstationOverview{}, err
				}
				coverageByWorkOrder[task.WorkOrderID] = coverage
			}
			if coverage.DataComplete && coverage.ShortageG <= 0 && coverage.ShortageUnits <= 0 {
				continue
			}
			task.IsBlocked = true
			task.StatusLabel = "异常"
			task.Readiness = "blocked"
			task.ReadinessLabel = "WIP库存不足"
			task.NextHandler = "仓库/物料"
			task.BlockingReason = firstNonEmpty(coverage.BlockingReason, "WIP资料待完善")
			applyProductionTaskReadinessDetail(task)
		}
	}

	sortProductionTasks(tasks)
	load := buildProductionWorkstationLoad(tasks)
	statusSummary, blockedSummary, prioritySummary := buildProductionTaskSummaries(tasks)
	todaySummary := buildProductionTodaySummary(tasks, workOrders, jobCards)
	return ProductionWorkstationOverview{
		Date:            time.Now().Format("2006-01-02"),
		TotalTasks:      len(tasks),
		TodaySummary:    todaySummary,
		NavBadges:       buildProductionNavBadges(todaySummary),
		StatusSummary:   statusSummary,
		BlockedSummary:  blockedSummary,
		PrioritySummary: prioritySummary,
		WorkstationLoad: load,
		Tasks:           tasks,
	}, nil
}

func (s *Service) ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error) {
	if query.ID < 0 {
		return nil, fmt.Errorf("work_order_id must be >= 0")
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	return s.repo.ListWorkOrders(ctx, query)
}

func (s *Service) ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error) {
	if query.WorkOrderID < 0 {
		return nil, fmt.Errorf("work_order_id must be >= 0")
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Limit <= 0 || query.Limit > 500 {
		query.Limit = 200
	}
	rows, err := s.repo.ListJobCards(ctx, query)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if strings.TrimSpace(rows[i].ProcessRequirement) == "" {
			rows[i].ProcessRequirement = jobCardProcessRequirement(rows[i])
		}
	}
	return rows, nil
}

func jobCardProcessRequirement(card JobCardRow) string {
	const fallback = "按冻结工艺路线执行"
	var snapshot struct {
		Operations []map[string]any `json:"operations"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(card.ProcessSnapshotJSON)), &snapshot); err != nil {
		return fallback
	}
	var matched map[string]any
	for _, operation := range snapshot.Operations {
		seq := int64(numberValue(operation["seq"]))
		if seq == 0 {
			seq = int64(numberValue(operation["sequence_no"]))
		}
		if card.SequenceNo > 0 && seq == int64(card.SequenceNo) {
			matched = operation
			break
		}
	}
	if matched == nil && card.OperationID > 0 {
		for _, operation := range snapshot.Operations {
			if int64(numberValue(operation["operation_id"])) == card.OperationID {
				matched = operation
				break
			}
		}
	}
	if matched == nil && strings.TrimSpace(card.Operation) != "" {
		for _, operation := range snapshot.Operations {
			name := firstNonEmpty(stringValue(operation["operation"]), stringValue(operation["operation_name"]), stringValue(operation["name"]))
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(card.Operation)) {
				matched = operation
				break
			}
		}
	}
	if matched == nil {
		return fallback
	}
	for _, key := range []string{"process_requirement", "requirement", "instruction", "instructions", "note"} {
		if requirement := strings.TrimSpace(stringValue(matched[key])); requirement != "" {
			return requirement
		}
	}
	items := stringListValue(matched["quality_checklist_json"])
	if len(items) == 0 {
		items = stringListValue(matched["quality_checklist"])
	}
	if len(items) > 0 {
		return "质检项：" + strings.Join(items, "、")
	}
	return fallback
}

func numberValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		number, _ := typed.Float64()
		return number
	case string:
		number, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number
	default:
		return 0
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func stringListValue(value any) []string {
	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(text), &values); err != nil {
			return []string{text}
		}
	default:
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(stringValue(value)); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func productionTaskFromJobCard(card JobCardRow, workOrder WorkOrderRow) ProductionTask {
	assignedTo := firstNonEmpty(card.AssignedTo, workOrder.AssignedTo)
	workCenter := firstNonEmpty(card.WorkCenter, card.Workstation, workOrder.WorkCenter)
	workstation := firstNonEmpty(card.Workstation, card.WorkCenter, workOrder.WorkCenter)
	status := normalizeProductionTaskStatus(card.Status)
	blockingReason := productionBlockingReason(status, card.ExceptionReason, workCenter, assignedTo)
	task := ProductionTask{
		JobCardID:                card.ID,
		WorkOrderID:              firstNonZeroInt64(card.WorkOrderID, workOrder.ID),
		RunningItemID:            workOrder.RunningItemID,
		WorkOrderNo:              firstNonEmpty(card.WorkOrderNo, workOrder.WorkOrderNo),
		ProductName:              firstNonEmpty(card.ProductName, workOrder.ProductName),
		SpecG:                    firstNonZeroInt64(card.SpecG, workOrder.SpecG),
		InventoryQtyPerSalesUnit: workOrder.InventoryQtyPerSalesUnit,
		InventoryUnit:            productionTaskInventoryUnit(workOrder),
		PlannedG:                 firstNonZeroInt64(card.PlannedG, workOrder.PlannedG),
		PlannedUnits:             workOrder.PlannedUnits,
		PlannedLooseG:            workOrder.PlannedLooseG,
		PlannedOutputG:           firstNonZeroInt64(card.PlannedOutputG, workOrder.PlannedOutputG),
		Operation:                strings.TrimSpace(card.Operation),
		ProcessRequirement:       firstNonEmpty(card.ProcessRequirement, "按冻结工艺路线执行"),
		Workstation:              workstation,
		WorkCenter:               workCenter,
		Status:                   status,
		StatusLabel:              productionTaskStatusLabel(status, blockingReason),
		BlockingReason:           blockingReason,
		AssignedTo:               assignedTo,
		Operator:                 strings.TrimSpace(card.Operator),
		PlannedStartAt:           firstNonEmpty(card.PlannedStartAt, workOrder.PlannedStartAt),
		PlannedEndAt:             firstNonEmpty(card.PlannedEndAt, workOrder.PlannedEndAt),
		OrderNos:                 firstNonEmpty(card.OrderNos, workOrder.OrderNos),
		Priority:                 firstNonZeroInt(card.Priority, workOrder.Priority),
		PlannedMinutes:           card.PlannedMinutes,
		PlannedBatchCount:        card.PlannedBatchCount,
		PlannedInputQty:          card.PlannedInputQty,
		PlannedInputInventoryQty: plannedInputInventoryQuantity(card.PlannedInputQty, workOrder),
		ActualMinutes:            card.ActualMinutes,
		ActualInputQty:           card.ActualInputQty,
		ActualOutputQty:          card.ActualOutputQty,
		ActualLossQty:            card.ActualLossQty,
		ActualLossRate:           card.ActualLossRate,
		RecordsLoss:              card.RecordsLoss,
		LossReason:               strings.TrimSpace(card.LossReason),
		ExceptionReason:          strings.TrimSpace(card.ExceptionReason),
		IsBlocked:                blockingReason != "",
		SchedulingNote:           firstNonEmpty(card.SchedulingNote, workOrder.SchedulingNote),
	}
	task.NextHandler = productionNextHandler(task)
	task.Readiness, task.ReadinessLabel = productionTaskReadiness(task)
	task.AvailableActions = productionAvailableActions(task)
	applyProductionTaskReadinessDetail(&task)
	return task
}

func productionTaskFromWorkOrder(workOrder WorkOrderRow) ProductionTask {
	workCenter := strings.TrimSpace(workOrder.WorkCenter)
	status := normalizeProductionTaskStatus(workOrder.Status)
	blockingReason := productionBlockingReason(status, "", workCenter, workOrder.AssignedTo)
	task := ProductionTask{
		WorkOrderID:              workOrder.ID,
		RunningItemID:            workOrder.RunningItemID,
		WorkOrderNo:              strings.TrimSpace(workOrder.WorkOrderNo),
		ProductName:              strings.TrimSpace(workOrder.ProductName),
		SpecG:                    workOrder.SpecG,
		InventoryQtyPerSalesUnit: workOrder.InventoryQtyPerSalesUnit,
		InventoryUnit:            productionTaskInventoryUnit(workOrder),
		PlannedG:                 workOrder.PlannedG,
		PlannedUnits:             workOrder.PlannedUnits,
		PlannedLooseG:            workOrder.PlannedLooseG,
		PlannedOutputG:           workOrder.PlannedOutputG,
		Operation:                "工单准备",
		Workstation:              workCenter,
		WorkCenter:               workCenter,
		Status:                   status,
		StatusLabel:              productionTaskStatusLabel(status, blockingReason),
		BlockingReason:           blockingReason,
		AssignedTo:               strings.TrimSpace(workOrder.AssignedTo),
		PlannedStartAt:           strings.TrimSpace(workOrder.PlannedStartAt),
		PlannedEndAt:             strings.TrimSpace(workOrder.PlannedEndAt),
		OrderNos:                 strings.TrimSpace(workOrder.OrderNos),
		Priority:                 workOrder.Priority,
		PlannedInputInventoryQty: plannedInputInventoryQuantity(float64(workOrder.PlannedG), workOrder),
		IsBlocked:                blockingReason != "",
		SchedulingNote:           strings.TrimSpace(workOrder.SchedulingNote),
	}
	task.NextHandler = productionNextHandler(task)
	task.Readiness, task.ReadinessLabel = productionTaskReadiness(task)
	task.AvailableActions = productionAvailableActions(task)
	applyProductionTaskReadinessDetail(&task)
	return task
}

func plannedInputInventoryQuantity(rawPlannedQty float64, workOrder WorkOrderRow) float64 {
	if rawPlannedQty <= 0 {
		return 0
	}
	if workOrder.PlannedG <= 0 ||
		workOrder.PlannedInventoryQty <= 0 ||
		workOrder.InventoryQtyPerSalesUnit <= 0 ||
		strings.TrimSpace(workOrder.InventoryUnit) == "" {
		return rawPlannedQty
	}
	cardShare := rawPlannedQty / float64(workOrder.PlannedG)
	return math.Round(workOrder.PlannedInventoryQty*cardShare*1_000_000_000) / 1_000_000_000
}

func productionTaskInventoryUnit(workOrder WorkOrderRow) string {
	if unit := strings.TrimSpace(workOrder.InventoryUnit); unit != "" {
		return unit
	}
	if workOrder.PlannedG > 0 || workOrder.SpecG > 0 {
		return "g"
	}
	if workOrder.PlannedUnits > 0 {
		return "件"
	}
	return ""
}

func buildProductionTodaySummary(tasks []ProductionTask, workOrders []WorkOrderRow, jobCards []JobCardRow) ProductionTodaySummary {
	summary := ProductionTodaySummary{PlannedTasks: len(tasks)}
	for _, task := range tasks {
		switch task.StatusLabel {
		case "执行中":
			summary.RunningTasks++
		case "待处理":
			summary.PendingTasks++
		case "异常":
			// Blocked/paused tasks are counted below through IsBlocked so the
			// summary remains aligned with the read model's blocking reason.
		}
		if task.IsBlocked {
			summary.BlockedTasks++
		}
	}
	completedCards := 0
	for _, card := range jobCards {
		if normalizeProductionTaskStatus(card.Status) == "completed" {
			completedCards++
		}
	}
	completedWorkOrders := 0
	for _, workOrder := range workOrders {
		if normalizeProductionTaskStatus(workOrder.Status) == "completed" {
			completedWorkOrders++
		}
	}
	if completedCards > 0 {
		summary.CompletedTasks = completedCards
	} else {
		summary.CompletedTasks = completedWorkOrders
	}
	summary.PlannedTasks += summary.CompletedTasks
	return summary
}

func buildProductionNavBadges(summary ProductionTodaySummary) map[string]ProductionNavBadge {
	overviewBadge := ProductionNavBadge{
		Pending: summary.PendingTasks,
		Blocked: summary.BlockedTasks,
		Running: summary.RunningTasks,
	}
	return map[string]ProductionNavBadge{
		"productionOverview": overviewBadge,
		"workstationView":    overviewBadge,
		"produceRunning":     {Running: summary.RunningTasks},
	}
}

func buildProductionTaskSummaries(tasks []ProductionTask) ([]ProductionSummaryCount, []ProductionSummaryCount, []ProductionSummaryCount) {
	statusCounts := map[string]int{}
	statusLabels := map[string]string{}
	blockedCounts := map[string]int{}
	priorityCounts := map[string]int{}
	for _, task := range tasks {
		statusKey := task.StatusLabel
		if statusKey == "" {
			statusKey = "待处理"
		}
		statusCounts[statusKey]++
		statusLabels[statusKey] = statusKey
		if task.BlockingReason != "" {
			blockedCounts[task.BlockingReason]++
		}
		priorityLabel := fmt.Sprintf("P%d", task.Priority)
		priorityCounts[priorityLabel]++
	}
	return summaryCounts(statusCounts, statusLabels), summaryCounts(blockedCounts, nil), summaryCounts(priorityCounts, nil)
}

func buildProductionWorkstationLoad(tasks []ProductionTask) []ProductionWorkstationLoad {
	byWorkstation := map[string]*ProductionWorkstationLoad{}
	order := make([]string, 0)
	for _, task := range tasks {
		key := productionTaskWorkstationKey(task)
		row := byWorkstation[key]
		if row == nil {
			row = &ProductionWorkstationLoad{Workstation: key}
			byWorkstation[key] = row
			order = append(order, key)
		}
		row.TotalTasks++
		row.LoadMinutes += task.PlannedMinutes
		if task.IsBlocked {
			row.BlockedTasks++
			if row.BlockingReason == "" {
				row.BlockingReason = task.BlockingReason
			}
		}
		switch task.StatusLabel {
		case "执行中":
			row.RunningTasks++
			if row.CurrentTask == "" {
				row.CurrentTask = productionTaskTitle(task)
			}
		case "待处理":
			row.PendingTasks++
			if row.NextTask == "" {
				row.NextTask = productionTaskTitle(task)
			}
		case "异常":
			if row.CurrentTask == "" {
				row.CurrentTask = productionTaskTitle(task)
			}
		}
		if row.CurrentTask == "" {
			row.CurrentTask = productionTaskTitle(task)
		}
	}
	rows := make([]ProductionWorkstationLoad, 0, len(order))
	for _, key := range order {
		row := *byWorkstation[key]
		row.QueueCount = row.TotalTasks
		row.BlockedCount = row.BlockedTasks
		row.EstimatedMinutes = row.LoadMinutes
		row.LoadStatus = productionWorkstationLoadStatus(row)
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].BlockedTasks != rows[j].BlockedTasks {
			return rows[i].BlockedTasks > rows[j].BlockedTasks
		}
		if rows[i].RunningTasks != rows[j].RunningTasks {
			return rows[i].RunningTasks > rows[j].RunningTasks
		}
		if rows[i].PendingTasks != rows[j].PendingTasks {
			return rows[i].PendingTasks > rows[j].PendingTasks
		}
		return rows[i].Workstation < rows[j].Workstation
	})
	return rows
}

func productionWorkstationLoadStatus(row ProductionWorkstationLoad) string {
	if row.BlockedTasks > 0 {
		return "blocked"
	}
	if row.TotalTasks <= 0 {
		return "idle"
	}
	if row.LoadMinutes >= 480 || row.TotalTasks >= 6 {
		return "full"
	}
	return "normal"
}

func buildWorkOrderExecutionHub(wo WorkOrderRow, wipStatus ProductionWIPStatus, jobCards []JobCardRow, stockEntries []StockEntryRow, ledgerEntries []WorkOrderLedgerEntryRow, logs ProductionLogsResult, cost BatchCostRow, qualityRows []QualityInspectionRow) WorkOrderExecutionHub {
	filteredQualityRows := qualityRowsForWorkOrder(wo, qualityRows)
	readiness := buildWorkOrderExecutionReadiness(wo, wipStatus, jobCards, filteredQualityRows)
	return WorkOrderExecutionHub{
		Header:                buildWorkOrderExecutionHeader(wo),
		Readiness:             readiness,
		BomSummary:            workOrderBomSummary(wo),
		RouteSummary:          workOrderRouteSummary(wo, jobCards),
		OperationProgress:     buildWorkOrderOperationProgress(jobCards),
		WorkstationAssignment: buildWorkOrderAssignment(wo, jobCards),
		WIPStatus:             wipStatus,
		QualityStatus:         buildProductionQualityStatus(wo, filteredQualityRows),
		StockEntries:          stockEntries,
		FinishedReceipts:      filterFinishedReceiptEntries(stockEntries),
		CostSummary:           cost,
		TraceTimeline:         buildProductionTraceTimeline(wo, jobCards, stockEntries, ledgerEntries, filteredQualityRows, logs, cost),
		ContextActions:        buildWorkOrderContextActions(wo, jobCards, readiness),
	}
}

func buildWorkOrderExecutionHeader(wo WorkOrderRow) WorkOrderExecutionHeader {
	return WorkOrderExecutionHeader{
		WorkOrderID:      wo.ID,
		WorkOrderNo:      wo.WorkOrderNo,
		ProductID:        wo.ProductID,
		ProductName:      wo.ProductName,
		SpecG:            wo.SpecG,
		OrderNos:         wo.OrderNos,
		PlannedG:         wo.PlannedG,
		PlannedOutputG:   wo.PlannedOutputG,
		PlannedUnits:     wo.PlannedUnits,
		PlannedLooseG:    wo.PlannedLooseG,
		Status:           wo.Status,
		BatchID:          wo.BatchID,
		BomVersionID:     wo.BomVersionID,
		ProductionPlanID: wo.ProductionPlanID,
		RunningItemID:    wo.RunningItemID,
		Priority:         wo.Priority,
		AssignedTo:       wo.AssignedTo,
		WorkCenter:       wo.WorkCenter,
		CreatedAt:        wo.CreatedAt,
	}
}

func buildWorkOrderExecutionReadiness(wo WorkOrderRow, wipStatus ProductionWIPStatus, jobCards []JobCardRow, qualityRows []QualityInspectionRow) ProductionExecutionReadiness {
	reasons := make([]ProductionBlockingReason, 0)
	status := normalizeProductionTaskStatus(wo.Status)
	if !wipStatus.DataComplete {
		reasons = append(reasons, productionBlockingReasonRow(
			"wip_data_incomplete",
			firstNonEmpty(wipStatus.BlockingReason, "WIP资料待完善"),
			"blocked",
			"生产配置",
			[]ProductionRelatedLink{productionRelatedLink("workOrder", "打开工单", "workOrders", workOrderContextParams(wo, 0, nil))},
		))
	} else if shortage := wipStatus.ShortageG; shortage > 0 || wipStatus.ShortageUnits > 0 {
		label := fmt.Sprintf("WIP 不足 %dg", shortage)
		if shortage <= 0 {
			label = fmt.Sprintf("WIP 不足 %d件", wipStatus.ShortageUnits)
		}
		reasons = append(reasons, productionBlockingReasonRow(
			"wip_shortage",
			label,
			"blocked",
			"仓库/物料",
			[]ProductionRelatedLink{productionRelatedLink("wip", "生产领料", "stockOperations", workOrderContextParams(wo, firstJobCardID(jobCards), map[string]any{"tab": "stockEntries", "action": "issue", "return_source": "work_order"}))},
		))
	}
	quality := buildProductionQualityStatus(wo, qualityRows)
	if quality.Status == "blocked" {
		reasons = append(reasons, productionBlockingReasonRow(
			"quality_freeze",
			firstNonEmpty(quality.Note, "质检冻结/待复核"),
			"blocked",
			"质检",
			[]ProductionRelatedLink{productionRelatedLink("quality", "打开质检", "qualityInspections", workOrderContextParams(wo, firstJobCardID(jobCards), map[string]any{"reference_no": wo.WorkOrderNo}))},
		))
	}
	if hasPriorOperationBlock(jobCards) {
		severity := "blocked"
		if status == "released" {
			severity = "warning"
		}
		reasons = append(reasons, productionBlockingReasonRow(
			"prior_operation_incomplete",
			"前序工序未完成",
			severity,
			"现场主管",
			[]ProductionRelatedLink{productionRelatedLink("jobCard", "打开工序卡", "jobCards", workOrderContextParams(wo, firstIncompleteJobCardID(jobCards), nil))},
		))
	}
	if (status == "running" || status == "partially_completed") && !allJobCardsFinished(jobCards) {
		reasons = append(reasons, productionBlockingReasonRow(
			"job_cards_incomplete",
			"工序尚未全部完成",
			"blocked",
			"工位操作员",
			[]ProductionRelatedLink{productionRelatedLink("workstation", "进入工位", "workstationView", workOrderContextParams(wo, firstIncompleteJobCardID(jobCards), nil))},
		))
	}
	if unassigned := unassignedWorkstationCount(wo, jobCards); unassigned > 0 {
		reasons = append(reasons, productionBlockingReasonRow(
			"workstation_unassigned",
			"未分配工位",
			"blocked",
			"调度",
			[]ProductionRelatedLink{productionRelatedLink("assignWorkstation", "分配工位", "productionOverview", workOrderContextParams(wo, firstJobCardID(jobCards), map[string]any{"focus": "assignment"}))},
		))
	}
	if code, label := workOrderStatusBlock(wo.Status); code != "" {
		reasons = append(reasons, productionBlockingReasonRow(
			code,
			label,
			"info",
			"生产负责人",
			[]ProductionRelatedLink{productionRelatedLink("workOrder", "打开工单", "workOrders", workOrderContextParams(wo, 0, nil))},
		))
	}
	if isScheduleRisk(wo.PlannedEndAt) {
		reasons = append(reasons, productionBlockingReasonRow(
			"schedule_risk",
			"计划时间已超时",
			"warning",
			"调度",
			[]ProductionRelatedLink{productionRelatedLink("schedule", "调整优先级", "productionOverview", workOrderContextParams(wo, firstJobCardID(jobCards), map[string]any{"focus": "schedule"}))},
		))
	}

	canStart := len(blockingReasonsWithSeverity(reasons, "blocked")) == 0 && status == "released"
	canComplete := len(blockingReasonsWithSeverity(reasons, "blocked")) == 0 && (status == "running" || status == "partially_completed")
	return buildExecutionReadiness(canStart, canComplete, reasons, workOrderSuggestedAction(status, reasons, canStart, canComplete), "生产负责人")
}

func buildExecutionReadiness(canStart, canComplete bool, reasons []ProductionBlockingReason, suggestedAction, fallbackHandler string) ProductionExecutionReadiness {
	links := make([]ProductionRelatedLink, 0)
	for _, reason := range reasons {
		links = append(links, reason.RelatedLinks...)
	}
	severity := "info"
	nextHandler := fallbackHandler
	if len(reasons) > 0 {
		severity = "warning"
		nextHandler = firstNonEmpty(reasons[0].NextHandler, fallbackHandler)
		for _, reason := range reasons {
			if reason.Severity == "blocked" {
				severity = "blocked"
				nextHandler = firstNonEmpty(reason.NextHandler, nextHandler)
				break
			}
		}
	} else if canStart || canComplete {
		severity = "ready"
	}
	return ProductionExecutionReadiness{
		CanStart:        canStart,
		CanComplete:     canComplete,
		BlockingReasons: reasons,
		NextHandler:     firstNonEmpty(nextHandler, fallbackHandler),
		SuggestedAction: suggestedAction,
		Severity:        severity,
		RelatedLinks:    links,
	}
}

func productionBlockingReasonRow(code, label, severity, nextHandler string, links []ProductionRelatedLink) ProductionBlockingReason {
	return ProductionBlockingReason{Code: code, Label: label, Severity: severity, NextHandler: nextHandler, RelatedLinks: links}
}

func buildProductionWIPStatus(rows []WIPReservationRow) ProductionWIPStatus {
	status := ProductionWIPStatus{Materials: rows, Status: "ok", DataComplete: len(rows) > 0}
	for _, row := range rows {
		status.RequiredG += row.RequiredG
		status.RequiredUnits += row.RequiredUnits
		status.ReservedG += row.ReservedG
		status.ConsumedG += row.ConsumedG
		status.RemainingG += row.RemainingReservedG
		status.AvailableG += row.AvailableG
		status.AvailableUnits += row.AvailableUnits
		if row.QuantityBasis != "" {
			status.ShortageG += row.ShortageG
			status.ShortageUnits += row.ShortageUnits
		} else if row.RequiredG > row.ReservedG+row.ConsumedG {
			status.ShortageG += row.RequiredG - row.ReservedG - row.ConsumedG
		} else if row.RequiredUnits > row.ReservedUnits+row.ConsumedUnits {
			status.ShortageUnits += row.RequiredUnits - row.ReservedUnits - row.ConsumedUnits
		}
	}
	if len(rows) == 0 {
		status.Status = "blocked"
		status.BlockingReason = "WIP资料待完善"
	} else if status.ShortageG > 0 || status.ShortageUnits > 0 {
		status.Status = "blocked"
		if status.ShortageG > 0 {
			status.BlockingReason = fmt.Sprintf("WIP 不足 %dg", status.ShortageG)
		} else {
			status.BlockingReason = fmt.Sprintf("WIP 不足 %d件", status.ShortageUnits)
		}
	}
	return status
}

func productionWIPShortageG(rows []WIPReservationRow) int64 {
	return buildProductionWIPStatus(rows).ShortageG
}

func buildProductionQualityStatus(wo WorkOrderRow, rows []QualityInspectionRow) ProductionQualityStatus {
	status := ProductionQualityStatus{Status: "ok", ReferenceNo: wo.WorkOrderNo}
	if len(rows) == 0 {
		return status
	}
	row := rows[len(rows)-1]
	status.ReferenceNo = firstNonEmpty(row.ReferenceNo, wo.WorkOrderNo)
	status.Result = strings.TrimSpace(row.Result)
	status.Note = strings.TrimSpace(row.Note)
	status.CheckedAt = strings.TrimSpace(row.CreatedAt)
	if qualityResultBlocks(status.Result) {
		status.Status = "blocked"
	} else if status.Result != "" {
		status.Status = status.Result
	}
	return status
}

func qualityResultBlocks(result string) bool {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "hold", "reject", "failed", "fail", "blocked", "待处理", "不合格", "冻结":
		return true
	default:
		return false
	}
}

func qualityRowsForWorkOrder(wo WorkOrderRow, rows []QualityInspectionRow) []QualityInspectionRow {
	out := make([]QualityInspectionRow, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.Scope) != "" && strings.TrimSpace(row.Scope) != "work_order" {
			continue
		}
		ref := strings.TrimSpace(row.ReferenceNo)
		if ref == "" || ref == wo.WorkOrderNo || ref == fmt.Sprintf("%d", wo.ID) {
			out = append(out, row)
		}
	}
	return out
}

func buildWorkOrderOperationProgress(cards []JobCardRow) []ProductionOperationProgress {
	rows := make([]ProductionOperationProgress, 0, len(cards))
	sorted := append([]JobCardRow(nil), cards...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SequenceNo != sorted[j].SequenceNo {
			return sorted[i].SequenceNo < sorted[j].SequenceNo
		}
		return sorted[i].ID < sorted[j].ID
	})
	for _, card := range sorted {
		status := normalizeProductionTaskStatus(card.Status)
		blocking := productionBlockingReason(status, card.ExceptionReason, firstNonEmpty(card.WorkCenter, card.Workstation), card.AssignedTo)
		rows = append(rows, ProductionOperationProgress{
			JobCardID:      card.ID,
			SequenceNo:     card.SequenceNo,
			Operation:      card.Operation,
			Workstation:    firstNonEmpty(card.Workstation, card.WorkCenter),
			Status:         status,
			StatusLabel:    productionTaskStatusLabel(status, blocking),
			AssignedTo:     card.AssignedTo,
			Operator:       card.Operator,
			PlannedMinutes: card.PlannedMinutes,
			ActualMinutes:  card.ActualMinutes,
			PlannedCost:    card.PlannedOperationCost,
			ActualCost:     card.ActualOperationCost,
			StartedAt:      card.StartedAt,
			CompletedAt:    card.CompletedAt,
			BlockingReason: blocking,
		})
	}
	return rows
}

func buildWorkOrderAssignment(wo WorkOrderRow, cards []JobCardRow) ProductionWorkstationAssignment {
	return ProductionWorkstationAssignment{
		WorkCenter:      wo.WorkCenter,
		AssignedTo:      wo.AssignedTo,
		Priority:        wo.Priority,
		UnassignedCount: unassignedWorkstationCount(wo, cards),
	}
}

func unassignedWorkstationCount(wo WorkOrderRow, cards []JobCardRow) int {
	count := 0
	if len(cards) == 0 && strings.TrimSpace(wo.WorkCenter) == "" {
		return 1
	}
	for _, card := range cards {
		status := normalizeProductionTaskStatus(card.Status)
		if status == "completed" || status == "cancelled" {
			continue
		}
		if strings.TrimSpace(firstNonEmpty(card.WorkCenter, card.Workstation)) == "" {
			count++
		}
	}
	return count
}

func hasPriorOperationBlock(cards []JobCardRow) bool {
	for _, card := range cards {
		if strings.Contains(card.ExceptionReason, "前序") {
			return true
		}
	}
	return false
}

func allJobCardsFinished(cards []JobCardRow) bool {
	if len(cards) == 0 {
		return false
	}
	for _, card := range cards {
		status := normalizeProductionTaskStatus(card.Status)
		if status != "completed" && status != "cancelled" {
			return false
		}
	}
	return true
}

func firstIncompleteJobCardID(cards []JobCardRow) int64 {
	for _, card := range cards {
		status := normalizeProductionTaskStatus(card.Status)
		if status != "completed" && status != "cancelled" {
			return card.ID
		}
	}
	return firstJobCardID(cards)
}

func firstJobCardID(cards []JobCardRow) int64 {
	for _, card := range cards {
		if card.ID > 0 {
			return card.ID
		}
	}
	return 0
}

func firstShortageMaterialID(rows []WIPReservationRow) int64 {
	for _, row := range rows {
		if row.RequiredG > row.ReservedG+row.ConsumedG && row.MaterialID > 0 {
			return row.MaterialID
		}
	}
	return 0
}

func workOrderStatusBlock(status string) (string, string) {
	switch normalizeProductionTaskStatus(status) {
	case "completed":
		return "complete_cancelled", "工单已完成"
	case "cancelled":
		return "complete_cancelled", "工单已取消"
	case "released", "running", "partially_completed":
		return "", ""
	default:
		return "work_order_status_disallowed", "工单状态不允许执行"
	}
}

func workOrderSuggestedAction(status string, reasons []ProductionBlockingReason, canStart, canComplete bool) string {
	if len(reasons) > 0 {
		switch reasons[0].Code {
		case "wip_shortage":
			return "open_wip_issue"
		case "quality_freeze":
			return "open_quality"
		case "workstation_unassigned":
			return "assign_workstation"
		case "prior_operation_incomplete":
			return "open_job_card"
		case "schedule_risk":
			return "adjust_priority"
		default:
			return "open_work_order"
		}
	}
	if canStart {
		return "start_production"
	}
	if canComplete {
		return "finished_receipt"
	}
	if status == "running" {
		return "open_job_card"
	}
	return "open_work_order"
}

func workOrderContextParams(wo WorkOrderRow, jobCardID int64, extra map[string]any) map[string]any {
	params := map[string]any{"work_order_id": wo.ID}
	if jobCardID > 0 {
		params["job_card_id"] = jobCardID
	}
	if wo.RunningItemID > 0 {
		params["running_item_id"] = wo.RunningItemID
	}
	for key, value := range extra {
		if isEmptyContextValue(value) {
			continue
		}
		params[key] = value
	}
	return params
}

func isEmptyContextValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case int:
		return v == 0
	case int64:
		return v == 0
	case float64:
		return v == 0
	default:
		return false
	}
}

func productionRelatedLink(key, label, view string, params map[string]any) ProductionRelatedLink {
	return ProductionRelatedLink{Key: key, Label: label, View: view, Params: params}
}

func blockingReasonsWithSeverity(rows []ProductionBlockingReason, severity string) []ProductionBlockingReason {
	out := make([]ProductionBlockingReason, 0)
	for _, row := range rows {
		if row.Severity == severity {
			out = append(out, row)
		}
	}
	return out
}

func filterFinishedReceiptEntries(rows []StockEntryRow) []StockEntryRow {
	out := make([]StockEntryRow, 0)
	for _, row := range rows {
		entryType := strings.TrimSpace(strings.ToLower(row.EntryType))
		purpose := strings.TrimSpace(strings.ToLower(row.Purpose))
		if strings.Contains(entryType, "finished") || purpose == "manufacture" {
			out = append(out, row)
		}
	}
	return out
}

func buildProductionTraceTimeline(wo WorkOrderRow, cards []JobCardRow, stockEntries []StockEntryRow, ledgerEntries []WorkOrderLedgerEntryRow, qualityRows []QualityInspectionRow, logs ProductionLogsResult, cost BatchCostRow) []ProductionTraceTimelineEntry {
	rows := make([]ProductionTraceTimelineEntry, 0)
	if wo.ProductionPlanID > 0 {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "operation", Title: "生产计划提交", Summary: fmt.Sprintf("计划 #%d 生成工单", wo.ProductionPlanID), At: wo.CreatedAt, RefType: "production_plan", RefID: wo.ProductionPlanID, View: "producePlan", Params: map[string]any{"production_plan_id": wo.ProductionPlanID}})
	}
	rows = append(rows, ProductionTraceTimelineEntry{Type: "operation", Title: "工单创建", Summary: wo.WorkOrderNo, At: wo.CreatedAt, RefType: "work_order", RefID: wo.ID, View: "workOrders", Params: workOrderContextParams(wo, 0, nil)})
	for _, card := range cards {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "operation", Title: "工序卡 " + firstNonEmpty(card.Operation, fmt.Sprintf("#%d", card.ID)), Summary: productionTaskStatusLabel(normalizeProductionTaskStatus(card.Status), card.ExceptionReason), At: firstNonEmpty(card.CompletedAt, card.StartedAt, card.PausedAt), RefType: "job_card", RefID: card.ID, View: "jobCards", Params: workOrderContextParams(wo, card.ID, nil)})
	}
	for _, entry := range stockEntries {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "inventory", Title: "Stock Entry " + firstNonEmpty(entry.EntryNo, fmt.Sprintf("#%d", entry.ID)), Summary: firstNonEmpty(entry.Purpose, entry.EntryType, entry.Status), At: entry.CreatedAt, RefType: "stock_entry", RefID: entry.ID, View: "stockOperations", Params: workOrderContextParams(wo, entry.JobCardID, map[string]any{"tab": "stockEntries"})})
	}
	for _, entry := range ledgerEntries {
		if entry.StockEntryID > 0 {
			continue
		}
		rows = append(rows, ProductionTraceTimelineEntry{Type: "inventory", Title: "库存流水", Summary: firstNonEmpty(entry.ItemName, entry.EntryNo), At: entry.CreatedAt, RefType: "stock_ledger", RefID: entry.ID, View: "stockOperations", Params: workOrderContextParams(wo, 0, map[string]any{"tab": "stockEntries"})})
	}
	for _, row := range qualityRows {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "quality", Title: "质检 " + firstNonEmpty(row.Result, "-"), Summary: firstNonEmpty(row.Note, row.ItemName, row.ReferenceNo), At: row.CreatedAt, RefType: "quality_inspection", RefID: row.ID, View: "qualityInspections", Params: workOrderContextParams(wo, 0, map[string]any{"reference_no": firstNonEmpty(row.ReferenceNo, wo.WorkOrderNo)})})
	}
	for _, row := range logs.Rows {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "log", Title: "生产日志", Summary: firstNonEmpty(row.FinishedBatchCode, row.BatchID, row.ProductName), At: row.FinishedAt, RefType: "production_log", RefID: row.ID, View: "produceLogs", Params: workOrderContextParams(wo, 0, nil)})
	}
	if cost.RunningItemID > 0 || cost.TotalCost > 0 {
		rows = append(rows, ProductionTraceTimelineEntry{Type: "cost", Title: "成本归集", Summary: fmt.Sprintf("%.2f", cost.TotalCost), At: cost.CreatedAt, RefType: "batch_cost", RefID: cost.ID, View: "productionCosts", Params: workOrderContextParams(wo, 0, map[string]any{"batch_id": firstNonEmpty(cost.BatchID, wo.BatchID)})})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i].At, rows[j].At
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		return left < right
	})
	return rows
}

func buildWorkOrderContextActions(wo WorkOrderRow, cards []JobCardRow, readiness ProductionExecutionReadiness) []ProductionContextAction {
	jobCardID := firstJobCardID(cards)
	actions := []ProductionContextAction{
		{Key: "startProduction", Label: "开始生产", ActionType: "command", Endpoint: fmt.Sprintf("/api/produce/work-orders/%d/start", wo.ID), Params: workOrderContextParams(wo, 0, nil), Disabled: !readiness.CanStart, Reason: disabledReason(!readiness.CanStart, readiness)},
		{Key: "productionIssue", Label: "生产领料", ActionType: "navigate", View: "stockOperations", Params: workOrderContextParams(wo, jobCardID, map[string]any{"tab": "stockEntries", "action": "issue", "return_source": "work_order"})},
		{Key: "productionSupplement", Label: "补料", ActionType: "navigate", View: "stockOperations", Params: workOrderContextParams(wo, jobCardID, map[string]any{"tab": "stockEntries", "action": "supplement", "return_source": "work_order"})},
		{Key: "productionReturn", Label: "退回未用原料", ActionType: "navigate", View: "stockOperations", Params: workOrderContextParams(wo, jobCardID, map[string]any{"tab": "stockEntries", "action": "return", "return_source": "work_order"})},
		{Key: "productionConsume", Label: "记录生产消耗", ActionType: "navigate", View: "stockOperations", Params: workOrderContextParams(wo, jobCardID, map[string]any{"tab": "stockEntries", "action": "consume", "return_source": "work_order"})},
		{Key: "finishedReceipt", Label: "完工入库", ActionType: "navigate", View: "stockOperations", Params: workOrderContextParams(wo, 0, map[string]any{"tab": "stockEntries", "action": "finish", "return_source": "work_order"}), Disabled: !readiness.CanComplete, Reason: disabledReason(!readiness.CanComplete, readiness)},
		{Key: "openJobCard", Label: "打开工序卡", ActionType: "navigate", View: "jobCards", Params: workOrderContextParams(wo, jobCardID, nil)},
		{Key: "openQuality", Label: "打开质检", ActionType: "navigate", View: "qualityInspections", Params: workOrderContextParams(wo, jobCardID, map[string]any{"reference_no": wo.WorkOrderNo})},
		{Key: "openCost", Label: "成本", ActionType: "navigate", View: "productionCosts", Params: workOrderContextParams(wo, 0, nil)},
		{Key: "openLogs", Label: "日志", ActionType: "navigate", View: "produceLogs", Params: workOrderContextParams(wo, 0, nil)},
	}
	return actions
}

func disabledReason(disabled bool, readiness ProductionExecutionReadiness) string {
	if !disabled {
		return ""
	}
	if len(readiness.BlockingReasons) > 0 {
		return readiness.BlockingReasons[0].Label
	}
	return "当前状态不可执行"
}

func workOrderBomSummary(wo WorkOrderRow) string {
	if wo.BomVersionID > 0 {
		return fmt.Sprintf("BOM版本 #%d", wo.BomVersionID)
	}
	return "默认 BOM"
}

func workOrderRouteSummary(wo WorkOrderRow, cards []JobCardRow) string {
	if name := routeNameFromProcessSnapshot(wo.ProcessSnapshotJSON); name != "" {
		return name
	}
	parts := make([]string, 0, len(cards))
	for _, card := range cards {
		if strings.TrimSpace(card.Operation) != "" {
			parts = append(parts, strings.TrimSpace(card.Operation))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " -> ")
	}
	return "默认工艺路线"
}

func routeNameFromProcessSnapshot(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return ""
	}
	for _, key := range []string{"route_name", "name", "process_name"} {
		if value, ok := data[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isScheduleRisk(plannedEndAt string) bool {
	text := strings.TrimSpace(plannedEndAt)
	if text == "" {
		return false
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, text, time.Local); err == nil {
			return t.Before(time.Now())
		}
	}
	return false
}

func summaryCounts(counts map[string]int, labels map[string]string) []ProductionSummaryCount {
	rows := make([]ProductionSummaryCount, 0, len(counts))
	for key, count := range counts {
		label := key
		if labels != nil && labels[key] != "" {
			label = labels[key]
		}
		rows = append(rows, ProductionSummaryCount{Key: key, Label: label, Count: count})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Label < rows[j].Label
	})
	return rows
}

func sortProductionTasks(tasks []ProductionTask) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		if tasks[i].PlannedStartAt != tasks[j].PlannedStartAt {
			if tasks[i].PlannedStartAt == "" {
				return false
			}
			if tasks[j].PlannedStartAt == "" {
				return true
			}
			return tasks[i].PlannedStartAt < tasks[j].PlannedStartAt
		}
		if tasks[i].WorkOrderNo != tasks[j].WorkOrderNo {
			return tasks[i].WorkOrderNo < tasks[j].WorkOrderNo
		}
		return tasks[i].JobCardID < tasks[j].JobCardID
	})
}

func isActiveProductionTaskStatus(status string) bool {
	switch normalizeProductionTaskStatus(status) {
	case "pending", "ready", "released", "running", "paused":
		return true
	default:
		return false
	}
}

func normalizeProductionTaskStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func productionTaskStatusLabel(status, blockingReason string) string {
	if blockingReason != "" {
		return "异常"
	}
	switch status {
	case "running":
		return "执行中"
	case "pending", "ready", "released":
		return "待处理"
	case "paused":
		return "异常"
	case "completed":
		return "已完成"
	case "cancelled":
		return "已取消"
	default:
		return "待处理"
	}
}

func productionBlockingReason(status, exceptionReason, workCenter, assignedTo string) string {
	normalizedStatus := normalizeProductionTaskStatus(status)
	if normalizedStatus != "running" {
		if reason := strings.TrimSpace(exceptionReason); reason != "" {
			return reason
		}
	}
	if normalizedStatus == "paused" {
		return "已暂停"
	}
	if strings.TrimSpace(workCenter) == "" {
		return "未分配工位"
	}
	if (normalizedStatus == "pending" || normalizedStatus == "ready" || normalizedStatus == "released") && strings.TrimSpace(assignedTo) == "" {
		return "未分配处理人"
	}
	return ""
}

func productionNextHandler(task ProductionTask) string {
	if task.AssignedTo != "" {
		return task.AssignedTo
	}
	if task.Operator != "" {
		return task.Operator
	}
	if task.BlockingReason != "" {
		if strings.Contains(task.BlockingReason, "补料") || strings.Contains(task.BlockingReason, "物料") || strings.Contains(task.BlockingReason, "领料") || strings.Contains(task.BlockingReason, "库存") {
			return "仓库/物料"
		}
		if task.BlockingReason == "未分配工位" || task.BlockingReason == "未分配处理人" {
			return "调度"
		}
		return "现场主管"
	}
	if task.StatusLabel == "待处理" {
		return "工位操作员"
	}
	return "生产负责人"
}

func productionTaskReadiness(task ProductionTask) (string, string) {
	if task.BlockingReason != "" || task.IsBlocked {
		return "blocked", "不能做"
	}
	switch task.Status {
	case "running":
		return "running", "执行中"
	case "pending", "ready", "released":
		return "ready", "可开始"
	case "completed":
		return "completed", "已完成"
	case "cancelled":
		return "cancelled", "已取消"
	default:
		return "pending", "待处理"
	}
}

func productionAvailableActions(task ProductionTask) []string {
	if task.JobCardID <= 0 {
		return nil
	}
	switch task.Status {
	case "pending", "ready":
		return []string{"start"}
	case "running":
		return []string{"pause", "complete", "report_exception", "material_call"}
	case "paused":
		return []string{"resume", "complete"}
	default:
		return nil
	}
}

func applyProductionTaskReadinessDetail(task *ProductionTask) {
	if task == nil {
		return
	}
	reasons := make([]ProductionBlockingReason, 0)
	if strings.TrimSpace(task.BlockingReason) != "" {
		code := productionTaskBlockingCode(task.BlockingReason)
		link := productionTaskBlockingLink(*task, code)
		reasons = append(reasons, productionBlockingReasonRow(code, task.BlockingReason, "blocked", firstNonEmpty(task.NextHandler, "现场主管"), []ProductionRelatedLink{link}))
	}
	if task.Status == "completed" || task.Status == "cancelled" {
		reasons = append(reasons, productionBlockingReasonRow("complete_cancelled", task.StatusLabel, "info", "生产负责人", []ProductionRelatedLink{productionRelatedLink("workOrder", "打开工单", "workOrders", productionTaskContextParams(*task, nil))}))
	}
	canStart := len(blockingReasonsWithSeverity(reasons, "blocked")) == 0 && task.JobCardID > 0 && (task.Status == "pending" || task.Status == "ready" || task.Status == "released")
	canComplete := len(blockingReasonsWithSeverity(reasons, "blocked")) == 0 && task.JobCardID > 0 && task.Status == "running"
	detail := buildExecutionReadiness(canStart, canComplete, reasons, productionTaskSuggestedAction(*task, reasons, canStart, canComplete), firstNonEmpty(task.NextHandler, "生产负责人"))
	if detail.Severity == "ready" && task.Status == "running" {
		detail.Severity = "info"
	}
	if len(detail.RelatedLinks) == 0 {
		detail.RelatedLinks = append(detail.RelatedLinks,
			productionRelatedLink("workOrder", "打开工单", "workOrders", productionTaskContextParams(*task, nil)),
			productionRelatedLink("jobCard", "打开工序卡", "jobCards", productionTaskContextParams(*task, nil)),
		)
	}
	task.ReadinessDetail = detail
	task.CanStart = detail.CanStart
	task.CanComplete = detail.CanComplete
	task.BlockingReasons = detail.BlockingReasons
	task.SuggestedAction = detail.SuggestedAction
	task.Severity = detail.Severity
	task.RelatedLinks = detail.RelatedLinks
}

func productionTaskBlockingCode(reason string) string {
	text := strings.TrimSpace(reason)
	switch {
	case strings.Contains(text, "WIP") || strings.Contains(text, "补料") || strings.Contains(text, "物料") || strings.Contains(text, "领料") || strings.Contains(text, "库存") || strings.Contains(text, "生豆"):
		return "wip_shortage"
	case strings.Contains(text, "质检") || strings.Contains(text, "冻结"):
		return "quality_freeze"
	case strings.Contains(text, "工位") || strings.Contains(text, "处理人"):
		return "workstation_unassigned"
	case strings.Contains(text, "前序"):
		return "prior_operation_incomplete"
	case strings.Contains(text, "超时") || strings.Contains(text, "延期"):
		return "schedule_risk"
	default:
		return "task_blocked"
	}
}

func productionTaskBlockingLink(task ProductionTask, code string) ProductionRelatedLink {
	switch code {
	case "wip_shortage":
		return productionRelatedLink("wip", "处理库存作业", "stockOperations", productionTaskContextParams(task, map[string]any{"tab": "wip"}))
	case "quality_freeze":
		return productionRelatedLink("quality", "打开质检", "qualityInspections", productionTaskContextParams(task, map[string]any{"reference_no": task.WorkOrderNo}))
	case "workstation_unassigned", "schedule_risk":
		return productionRelatedLink("assignment", "分配工位/调整优先级", "productionOverview", productionTaskContextParams(task, map[string]any{"focus": "assignment"}))
	case "prior_operation_incomplete":
		return productionRelatedLink("jobCard", "打开工序卡", "jobCards", productionTaskContextParams(task, nil))
	default:
		return productionRelatedLink("workOrder", "打开工单", "workOrders", productionTaskContextParams(task, nil))
	}
}

func productionTaskSuggestedAction(task ProductionTask, reasons []ProductionBlockingReason, canStart, canComplete bool) string {
	if len(reasons) > 0 {
		switch reasons[0].Code {
		case "wip_shortage":
			return "open_wip_issue"
		case "quality_freeze":
			return "open_quality"
		case "workstation_unassigned":
			return "assign_workstation"
		case "prior_operation_incomplete":
			return "open_job_card"
		case "schedule_risk":
			return "adjust_priority"
		default:
			return "open_work_order"
		}
	}
	if canStart {
		return "start_job_card"
	}
	if canComplete {
		return "complete_job_card"
	}
	if task.Status == "running" {
		return "complete_job_card"
	}
	return "open_work_order"
}

func productionTaskContextParams(task ProductionTask, extra map[string]any) map[string]any {
	params := map[string]any{}
	if task.WorkOrderID > 0 {
		params["work_order_id"] = task.WorkOrderID
	}
	if task.JobCardID > 0 {
		params["job_card_id"] = task.JobCardID
	}
	if task.RunningItemID > 0 {
		params["running_item_id"] = task.RunningItemID
	}
	for key, value := range extra {
		if isEmptyContextValue(value) {
			continue
		}
		params[key] = value
	}
	return params
}

func productionTaskWorkstationKey(task ProductionTask) string {
	if task.WorkCenter != "" {
		return task.WorkCenter
	}
	if task.Workstation != "" {
		return task.Workstation
	}
	return "未分配工位"
}

func productionTaskTitle(task ProductionTask) string {
	operation := strings.TrimSpace(task.Operation)
	product := strings.TrimSpace(task.ProductName)
	if operation != "" && product != "" {
		return operation + " / " + product
	}
	if product != "" {
		return product
	}
	if operation != "" {
		return operation
	}
	return task.WorkOrderNo
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
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
	if query.RunningItemID < 0 {
		return nil, fmt.Errorf("running_item_id must be >= 0")
	}
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
	case "material_issue_to_wip", "issue_to_wip", "material_transfer_for_manufacture", "领料到wip", "领料到 wip", "生产领料":
		return "material_issue_to_wip"
	case "wip_return", "return_from_wip", "material_return_from_manufacture", "wip退料", "生产退料":
		return "wip_return"
	case "material_consume", "work_order_consume", "material_consumption_for_manufacture", "工单消耗", "生产消耗":
		return "material_consume"
	case "finished_receipt", "finish_receipt", "manufacture", "完工入库":
		return "finished_receipt"
	case "scrap_loss", "scrap", "stock_adjustment", "报废", "损耗", "库存调整":
		return "scrap_loss"
	case "finished_transfer", "成品转仓":
		return "finished_transfer"
	default:
		return ""
	}
}

func normalizeStockEntryPurpose(purpose string) string {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case "material_transfer_for_manufacture", "material_issue_to_wip", "issue_to_wip", "领料到wip", "领料到 wip", "生产领料":
		return "material_transfer_for_manufacture"
	case "material_return_from_manufacture", "wip_return", "return_from_wip", "wip退料", "生产退料":
		return "material_return_from_manufacture"
	case "material_consumption_for_manufacture", "material_consume", "work_order_consume", "工单消耗", "生产消耗":
		return "material_consumption_for_manufacture"
	case "manufacture", "finished_receipt", "finish_receipt", "完工入库":
		return "manufacture"
	case "stock_adjustment", "scrap_loss", "scrap", "报废", "损耗", "库存调整":
		return "stock_adjustment"
	case "finished_transfer", "成品转仓":
		return "finished_transfer"
	default:
		return ""
	}
}

func stockEntryPurposeForType(entryType string) string {
	switch normalizeStockEntryType(entryType) {
	case "material_issue_to_wip":
		return "material_transfer_for_manufacture"
	case "wip_return":
		return "material_return_from_manufacture"
	case "material_consume":
		return "material_consumption_for_manufacture"
	case "finished_receipt":
		return "manufacture"
	case "scrap_loss":
		return "stock_adjustment"
	case "finished_transfer":
		return "finished_transfer"
	default:
		return ""
	}
}

func StockEntryPurposeForType(entryType string) string {
	return stockEntryPurposeForType(entryType)
}

func hydrateStockEntryRowPurpose(row StockEntryRow) StockEntryRow {
	row.EntryType = normalizeStockEntryType(row.EntryType)
	if row.Purpose == "" {
		row.Purpose = stockEntryPurposeForType(row.EntryType)
	}
	return row
}

func hydrateStockEntryDetailPurpose(detail StockEntryDetail) StockEntryDetail {
	detail.EntryType = normalizeStockEntryType(detail.EntryType)
	if detail.Purpose == "" {
		detail.Purpose = stockEntryPurposeForType(detail.EntryType)
	}
	return detail
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
	case "finished_transfer":
		if item.FromWarehouse == "" {
			item.FromWarehouse = stockdomain.WarehouseFinishedGoods
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
