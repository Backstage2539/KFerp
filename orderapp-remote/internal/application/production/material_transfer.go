package production

import (
	"context"
	"fmt"
	stockapp "orderapp/internal/application/stock"
	"strings"
)

type PlanMaterialTransferBatch struct {
	BatchCode string `json:"batch_code"`
	QtyG      int64  `json:"qty_g"`
	QtyUnits  int64  `json:"qty_units"`
}
type PlanMaterialTransferItem struct {
	SourceID        int64                       `json:"source_id"`
	Warehouse       string                      `json:"warehouse"`
	OwnerCustomerID int64                       `json:"owner_customer_id"`
	Batches         []PlanMaterialTransferBatch `json:"batches"`
}
type PlanMaterialTransferCommand struct {
	ProductionPlanID int64                      `json:"-"`
	DraftToken       string                     `json:"draft_token"`
	IdempotencyKey   string                     `json:"idempotency_key"`
	Items            []PlanMaterialTransferItem `json:"items"`
	Operator         string                     `json:"-"`
}
type PlanMaterialTransferPending struct {
	ID       int64  `json:"id"`
	EntryNo  string `json:"entry_no"`
	Status   string `json:"status"`
	SourceID int64  `json:"source_id"`
	QtyG     int64  `json:"qty_g"`
	QtyUnits int64  `json:"qty_units"`
}
type PlanMaterialTransferOptions struct {
	Source           ProductionPlanComponentSource         `json:"source"`
	Warehouses       []ProductionPlanComponentSourceOption `json:"warehouses"`
	Options          []stockapp.TransferOptionRow          `json:"options"`
	PendingDocuments []PlanMaterialTransferPending         `json:"pending_documents"`
}
type PlanMaterialTransferGroup struct {
	Warehouse       string                     `json:"warehouse"`
	OwnerCustomerID int64                      `json:"owner_customer_id"`
	Items           []PlanMaterialTransferItem `json:"items"`
}
type PlanMaterialTransferUnavailable struct {
	SourceID int64  `json:"source_id"`
	Reason   string `json:"reason"`
}
type PlanMaterialTransferPreview struct {
	Groups           []PlanMaterialTransferGroup       `json:"groups"`
	Unavailable      []PlanMaterialTransferUnavailable `json:"unavailable"`
	PendingDocuments []PlanMaterialTransferPending     `json:"pending_documents"`
	DocumentCount    int                               `json:"document_count"`
}
type PlanMaterialTransferPreviewRepository interface {
	PreviewPlanMaterialTransfers(context.Context, PlanMaterialTransferCommand) (PlanMaterialTransferPreview, error)
}

func (s *Service) PreviewPlanMaterialTransfers(ctx context.Context, c PlanMaterialTransferCommand) (PlanMaterialTransferPreview, error) {
	if c.ProductionPlanID <= 0 || strings.TrimSpace(c.DraftToken) == "" {
		return PlanMaterialTransferPreview{}, fmt.Errorf("请先保存计划并重新核对")
	}
	if len(c.Items) > 0 {
		copy := c
		copy.IdempotencyKey = "preview"
		if err := validatePlanMaterialTransferCommand(copy); err != nil {
			return PlanMaterialTransferPreview{}, err
		}
	}
	r, ok := s.repo.(PlanMaterialTransferPreviewRepository)
	if !ok {
		return PlanMaterialTransferPreview{}, fmt.Errorf("计划调拨预览暂不可用")
	}
	return r.PreviewPlanMaterialTransfers(ctx, c)
}

type SemiFinishedTransferCommand struct {
	PlanID         int64  `json:"-"`
	DraftToken     string `json:"draft_token"`
	IdempotencyKey string `json:"idempotency_key"`
	Operator       string `json:"-"`
}

type SemiFinishedTransferRepository interface {
	TransferSemiFinishedToWIP(context.Context, SemiFinishedTransferCommand) (PlanMaterialTransferResult, error)
}

func (s *Service) TransferSemiFinishedToWIP(ctx context.Context, cmd SemiFinishedTransferCommand) (PlanMaterialTransferResult, error) {
	if cmd.PlanID <= 0 || strings.TrimSpace(cmd.DraftToken) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return PlanMaterialTransferResult{}, fmt.Errorf("请先保存计划并重新核对")
	}
	r, ok := s.repo.(SemiFinishedTransferRepository)
	if !ok {
		return PlanMaterialTransferResult{}, fmt.Errorf("半成品转仓暂不可用")
	}
	return r.TransferSemiFinishedToWIP(ctx, cmd)
}

type PlanMaterialTransferResult struct {
	Plan             *ProductionPlanDetail             `json:"plan,omitempty"`
	Documents        []stockapp.StockDocumentDetail    `json:"documents,omitempty"`
	Groups           []PlanMaterialTransferGroup       `json:"groups,omitempty"`
	Unavailable      []PlanMaterialTransferUnavailable `json:"unavailable,omitempty"`
	PendingDocuments []PlanMaterialTransferPending     `json:"pending_documents,omitempty"`
	DocumentCount    int                               `json:"document_count,omitempty"`
}
type PlanMaterialTransferOptionsRepository interface {
	PlanMaterialTransferOptions(context.Context, int64, int64, string) (PlanMaterialTransferOptions, error)
}
type PlanMaterialTransferCreateRepository interface {
	CreatePlanMaterialTransfers(context.Context, PlanMaterialTransferCommand) (PlanMaterialTransferResult, error)
}

func validatePlanMaterialTransferCommand(c PlanMaterialTransferCommand) error {
	if c.ProductionPlanID <= 0 || strings.TrimSpace(c.DraftToken) == "" || strings.TrimSpace(c.IdempotencyKey) == "" || len(c.Items) == 0 {
		return fmt.Errorf("请先保存计划并确认调拨物料")
	}
	for _, i := range c.Items {
		if i.SourceID <= 0 || strings.TrimSpace(i.Warehouse) == "" || i.Warehouse == "wip" || len(i.Batches) == 0 {
			return fmt.Errorf("请选择来源仓和批次")
		}
		for _, b := range i.Batches {
			if strings.TrimSpace(b.BatchCode) == "" || b.QtyG < 0 || b.QtyUnits < 0 || (b.QtyG == 0 && b.QtyUnits == 0) {
				return fmt.Errorf("请填写有效的批次数量")
			}
		}
	}
	return nil
}
func (s *Service) PlanMaterialTransferOptions(ctx context.Context, planID, sourceID int64, warehouse string) (PlanMaterialTransferOptions, error) {
	r, ok := s.repo.(PlanMaterialTransferOptionsRepository)
	if !ok {
		return PlanMaterialTransferOptions{}, fmt.Errorf("计划调拨暂不可用")
	}
	return r.PlanMaterialTransferOptions(ctx, planID, sourceID, warehouse)
}
func (s *Service) CreatePlanMaterialTransfers(ctx context.Context, c PlanMaterialTransferCommand) (PlanMaterialTransferResult, error) {
	if err := validatePlanMaterialTransferCommand(c); err != nil {
		return PlanMaterialTransferResult{}, err
	}
	r, ok := s.repo.(PlanMaterialTransferCreateRepository)
	if !ok {
		return PlanMaterialTransferResult{}, fmt.Errorf("计划调拨暂不可用")
	}
	return r.CreatePlanMaterialTransfers(ctx, c)
}
