package production

import (
	"context"
	"fmt"
)

type SemiFinishedSelection struct {
	Key       string  `json:"key"`
	BatchID   int64   `json:"batch_id"`
	Warehouse string  `json:"warehouse"`
	Qty       float64 `json:"qty"`
}
type SemiFinishedStockCommand struct {
	PlanID       int64                   `json:"-"`
	DraftToken   string                  `json:"draft_token"`
	CheckOnly    bool                    `json:"check_only"`
	PreviewToken string                  `json:"preview_token"`
	Selections   []SemiFinishedSelection `json:"selections"`
	Operator     string                  `json:"-"`
}
type SemiFinishedBatch struct {
	BatchID           int64   `json:"batch_id"`
	BatchCode         string  `json:"batch_code"`
	Warehouse         string  `json:"warehouse"`
	WarehouseName     string  `json:"warehouse_name"`
	OwnerName         string  `json:"owner_name"`
	QualityStatus     string  `json:"quality_status"`
	OnhandQty         float64 `json:"onhand_qty"`
	ReservedQty       float64 `json:"reserved_qty"`
	AvailableQty      float64 `json:"available_qty"`
	SelectedQty       float64 `json:"selected_qty"`
	ReceivedAt        string  `json:"received_at"`
	SourceWorkOrderNo string  `json:"source_work_order_no"`
	SourceEntryNo     string  `json:"source_entry_no"`
}
type SemiFinishedStockItem struct {
	Key                string              `json:"key"`
	Name               string              `json:"name"`
	Unit               string              `json:"unit"`
	RequiredQty        float64             `json:"required_qty"`
	StockCoveredQty    float64             `json:"stock_covered_qty"`
	InflightCoveredQty float64             `json:"inflight_covered_qty"`
	ManufactureQty     float64             `json:"manufacture_qty"`
	Legacy             bool                `json:"legacy"`
	Batches            []SemiFinishedBatch `json:"batches"`
}
type SemiFinishedStockChange struct {
	Name           string                       `json:"name"`
	BeforeQty      float64                      `json:"before_qty"`
	AfterQty       float64                      `json:"after_qty"`
	Unit           string                       `json:"unit"`
	Operations     []SemiFinishedCapacityChange `json:"operations"`
	BeforeInputQty *float64                     `json:"before_input_qty,omitempty"`
	AfterInputQty  *float64                     `json:"after_input_qty,omitempty"`
	InputUnit      string                       `json:"input_unit,omitempty"`
}
type SemiFinishedCapacityChange struct {
	Name          string   `json:"name"`
	Workstation   string   `json:"workstation"`
	Batches       string   `json:"batches"`
	BeforeMinutes *int     `json:"before_minutes"`
	AfterMinutes  *int     `json:"after_minutes"`
	BeforeCost    *float64 `json:"before_cost"`
	AfterCost     *float64 `json:"after_cost"`
}
type SemiFinishedStockResult struct {
	DraftToken   string                    `json:"draft_token"`
	PreviewToken string                    `json:"preview_token,omitempty"`
	Items        []SemiFinishedStockItem   `json:"items"`
	Changes      []SemiFinishedStockChange `json:"changes"`
	Plan         *ProductionPlanDetail     `json:"plan,omitempty"`
}
type semiFinishedStockRepository interface {
	GetSemiFinishedStock(context.Context, int64) (SemiFinishedStockResult, error)
	UpdateSemiFinishedStock(context.Context, SemiFinishedStockCommand) (SemiFinishedStockResult, error)
}

func (s *Service) GetSemiFinishedStock(ctx context.Context, id int64) (SemiFinishedStockResult, error) {
	r, ok := s.repo.(semiFinishedStockRepository)
	if !ok || id <= 0 {
		return SemiFinishedStockResult{}, fmt.Errorf("半成品选择暂不可用")
	}
	return r.GetSemiFinishedStock(ctx, id)
}
func (s *Service) UpdateSemiFinishedStock(ctx context.Context, c SemiFinishedStockCommand) (SemiFinishedStockResult, error) {
	r, ok := s.repo.(semiFinishedStockRepository)
	if !ok || c.PlanID <= 0 || c.DraftToken == "" {
		return SemiFinishedStockResult{}, fmt.Errorf("请刷新计划后重试")
	}
	return r.UpdateSemiFinishedStock(ctx, c)
}
