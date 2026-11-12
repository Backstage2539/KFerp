package production

import (
	"context"
	"fmt"
	"strings"
)

type PlanRemainderGroup struct {
	PlanID         int64  `json:"plan_id"`
	ProductID      int64  `json:"product_id"`
	ProductName    string `json:"product_name"`
	TotalTasks     int    `json:"total_tasks"`
	CompletedTasks int    `json:"completed_tasks"`
	CanConfirm     bool   `json:"can_confirm"`
}
type PlanRemainderTask struct {
	JobCardID   int64  `json:"job_card_id"`
	WorkOrderNo string `json:"work_order_no"`
	Status      string `json:"status"`
}
type PlanRemainderSource struct {
	SourceID        string `json:"source_id"`
	SourceType      string `json:"source_type"`
	BatchCode       string `json:"batch_code"`
	Warehouse       string `json:"warehouse"`
	OwnerCustomerID int64  `json:"owner_customer_id"`
	AvailableG      int64  `json:"available_g"`
}
type PlanRemainderDiscrepancy struct {
	ID              int64  `json:"id"`
	SourceID        string `json:"source_id"`
	BatchCode       string `json:"batch_code"`
	Warehouse       string `json:"warehouse"`
	OwnerCustomerID int64  `json:"owner_customer_id"`
	QuantityG       int64  `json:"quantity_g"`
}
type PlanRemainderDetail struct {
	PlanRemainderGroup
	BlockedReason   string                     `json:"blocked_reason,omitempty"`
	RecheckRequired bool                       `json:"recheck_required,omitempty"`
	ReweighRequired bool                       `json:"reweigh_required,omitempty"`
	DiscrepancyG    int64                      `json:"discrepancy_g"`
	Discrepancies   []PlanRemainderDiscrepancy `json:"discrepancies"`
	Version         string                     `json:"version"`
	Tasks           []PlanRemainderTask        `json:"tasks"`
	Sources         []PlanRemainderSource      `json:"sources"`
}
type PlanRemainderSelection struct {
	SourceID  string `json:"source_id"`
	QuantityG int64  `json:"quantity_g"`
}
type PlanRemainderCommand struct {
	PlanID           int64                    `json:"-"`
	ProductID        int64                    `json:"product_id"`
	ExpectedVersion  string                   `json:"expected_version"`
	RequestID        string                   `json:"request_id"`
	TargetWarehouse  string                   `json:"target_warehouse"`
	Sources          []PlanRemainderSelection `json:"sources"`
	ReweighConfirmed bool                     `json:"reweigh_confirmed,omitempty"`
	Operator         string                   `json:"-"`
}
type PlanRemainderRecheckCommand struct {
	PlanID          int64  `json:"-"`
	ProductID       int64  `json:"product_id"`
	ExpectedVersion string `json:"expected_version"`
	RequestID       string `json:"request_id"`
	Operator        string `json:"-"`
}
type PlanRemainderResolveCommand struct {
	PlanID          int64   `json:"-"`
	ProductID       int64   `json:"product_id"`
	ExpectedVersion string  `json:"expected_version"`
	RequestID       string  `json:"request_id"`
	DiscrepancyIDs  []int64 `json:"discrepancy_ids"`
	Reason          string  `json:"reason"`
	Operator        string  `json:"-"`
}
type PlanRemainderResult struct {
	ID            int64   `json:"id"`
	OutstandingG  int64   `json:"outstanding_g"`
	DiscrepancyG  int64   `json:"discrepancy_g"`
	StockEntryIDs []int64 `json:"stock_entry_ids"`
}
type planRemainderRepository interface {
	ListPlanRemainderGroups(context.Context) ([]PlanRemainderGroup, error)
	GetPlanRemainder(context.Context, int64, int64) (PlanRemainderDetail, error)
	ConfirmPlanRemainder(context.Context, PlanRemainderCommand) (PlanRemainderResult, error)
	RecheckPlanRemainder(context.Context, PlanRemainderRecheckCommand) (PlanRemainderDetail, error)
	ResolvePlanRemainderDiscrepancies(context.Context, PlanRemainderResolveCommand) (PlanRemainderDetail, error)
}

func (s *Service) ListPlanRemainderGroups(ctx context.Context) ([]PlanRemainderGroup, error) {
	r, ok := s.repo.(planRemainderRepository)
	if !ok {
		return nil, fmt.Errorf("包装余料暂不可用")
	}
	return r.ListPlanRemainderGroups(ctx)
}
func (s *Service) GetPlanRemainder(ctx context.Context, plan, product int64) (PlanRemainderDetail, error) {
	r, ok := s.repo.(planRemainderRepository)
	if !ok || plan <= 0 || product <= 0 {
		return PlanRemainderDetail{}, fmt.Errorf("请选择生产计划及产品")
	}
	return r.GetPlanRemainder(ctx, plan, product)
}
func (s *Service) ConfirmPlanRemainder(ctx context.Context, c PlanRemainderCommand) (PlanRemainderResult, error) {
	r, ok := s.repo.(planRemainderRepository)
	if !ok || c.PlanID <= 0 || c.ProductID <= 0 || strings.TrimSpace(c.ExpectedVersion) == "" || strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.TargetWarehouse) == "" {
		return PlanRemainderResult{}, fmt.Errorf("请核对生产计划及余料信息")
	}
	return r.ConfirmPlanRemainder(ctx, c)
}
func (s *Service) RecheckPlanRemainder(ctx context.Context, c PlanRemainderRecheckCommand) (PlanRemainderDetail, error) {
	r, ok := s.repo.(planRemainderRepository)
	if !ok || c.PlanID <= 0 || c.ProductID <= 0 || strings.TrimSpace(c.ExpectedVersion) == "" || strings.TrimSpace(c.RequestID) == "" {
		return PlanRemainderDetail{}, fmt.Errorf("请重新核对余料")
	}
	return r.RecheckPlanRemainder(ctx, c)
}
func (s *Service) ResolvePlanRemainderDiscrepancies(ctx context.Context, c PlanRemainderResolveCommand) (PlanRemainderDetail, error) {
	r, ok := s.repo.(planRemainderRepository)
	if !ok || c.PlanID <= 0 || c.ProductID <= 0 || strings.TrimSpace(c.ExpectedVersion) == "" || strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.Reason) == "" || len(c.DiscrepancyIDs) == 0 {
		return PlanRemainderDetail{}, fmt.Errorf("请选择待核对差额并填写核对说明")
	}
	return r.ResolvePlanRemainderDiscrepancies(ctx, c)
}
func ValidatePlanRemainderSelection(c PlanRemainderCommand, sources []PlanRemainderSource) error {
	if c.PlanID <= 0 || c.ProductID <= 0 || strings.TrimSpace(c.ExpectedVersion) == "" || strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.TargetWarehouse) == "" {
		return fmt.Errorf("请核对生产计划及余料信息")
	}
	limits := map[string]int64{}
	for _, s := range sources {
		limits[s.SourceID] = s.AvailableG
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		limit, ok := limits[s.SourceID]
		if !ok || seen[s.SourceID] || s.QuantityG < 0 || s.QuantityG > limit {
			return fmt.Errorf("来源批次或数量已变化，请重新核对")
		}
		seen[s.SourceID] = true
	}
	if len(seen) != len(limits) {
		return fmt.Errorf("请逐项核对所有来源批次")
	}
	return nil
}
