package production

import (
	"context"
	"encoding/json"
	"fmt"
)

type OperationRecord struct {
	Legacy           bool                    `json:"legacy"`
	CorrectionCards  []ReceiptCorrectionCard `json:"correction_cards"`
	ID               int64                   `json:"id"`
	JobCardID        int64                   `json:"job_card_id"`
	WorkOrderID      int64                   `json:"work_order_id"`
	WorkOrderNo      string                  `json:"work_order_no"`
	ProductName      string                  `json:"product_name"`
	PackageSpecLabel string                  `json:"package_spec_label"`
	Operation        string                  `json:"operation"`
	Unit             string                  `json:"unit"`
	OriginalQty      float64                 `json:"original_qty"`
	Quantity         float64                 `json:"quantity"`
	Warehouse        string                  `json:"warehouse"`
	WarehouseName    string                  `json:"warehouse_name"`
	Final            bool                    `json:"final"`
	Version          string                  `json:"version"`
	CreatedAt        string                  `json:"created_at"`
	Operator         string                  `json:"operator"`
	EstimatedCost    bool                    `json:"estimated_cost"`
	Details          json.RawMessage         `json:"details"`
}
type operationRecordsRepository interface {
	ListOperationRecords(context.Context, int64) ([]OperationRecord, error)
	ListOperationBalances(context.Context) ([]OperationBatchBalance, error)
}

func (s *Service) ListOperationRecords(ctx context.Context, workID int64) ([]OperationRecord, error) {
	if r, ok := s.repo.(operationRecordsRepository); ok {
		return r.ListOperationRecords(ctx, workID)
	}
	return nil, fmt.Errorf("生产记录不可用")
}
func (s *Service) ListOperationBalances(ctx context.Context) ([]OperationBatchBalance, error) {
	if r, ok := s.repo.(operationRecordsRepository); ok {
		return r.ListOperationBalances(ctx)
	}
	return nil, fmt.Errorf("工序库存不可用")
}
