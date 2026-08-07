package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrCustomerBillNotFound = errors.New("customer bill not found")

type CustomerBillSummary struct {
	ID             int64  `json:"id"`
	SettlementNo   string `json:"settlement_no"`
	Status         string `json:"status"`
	TotalAmount    string `json:"total_amount"`
	Currency       string `json:"currency"`
	ConfirmedAt    string `json:"confirmed_at,omitempty"`
	PaidAt         string `json:"paid_at,omitempty"`
	WorkOrderCount int    `json:"work_order_count"`
	Summary        string `json:"summary,omitempty"`
}

type CustomerBillWorkOrder struct {
	WorkOrderID int64  `json:"work_order_id"`
	WorkOrderNo string `json:"work_order_no"`
	ProductName string `json:"product_name"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type CustomerBillLine struct {
	WorkOrderID  int64  `json:"work_order_id"`
	FeeType      string `json:"fee_type"`
	FeeName      string `json:"fee_name"`
	Basis        string `json:"basis"`
	BaseQuantity string `json:"base_quantity"`
	UnitPrice    string `json:"unit_price"`
	Amount       string `json:"amount"`
}

type CustomerBillDetail struct {
	CustomerBillSummary
	WorkOrders []CustomerBillWorkOrder `json:"work_orders"`
	Lines      []CustomerBillLine      `json:"lines"`
}

type customerProcessingBillRepository interface {
	ListCustomerProcessingBills(context.Context, int64, int) ([]CustomerBillSummary, error)
	GetCustomerProcessingBill(context.Context, int64, int64) (CustomerBillDetail, error)
}

func (s *Service) ListCustomerBills(ctx context.Context, token string) ([]CustomerBillSummary, error) {
	current, repo, err := s.customerProcessingBillContext(ctx, token)
	if err != nil {
		return nil, err
	}
	return repo.ListCustomerProcessingBills(ctx, current.CurrentCustomerID, 100)
}

func (s *Service) GetCustomerBill(ctx context.Context, token string, billID int64) (CustomerBillDetail, error) {
	if billID <= 0 {
		return CustomerBillDetail{}, fmt.Errorf("bill required")
	}
	current, repo, err := s.customerProcessingBillContext(ctx, token)
	if err != nil {
		return CustomerBillDetail{}, err
	}
	return repo.GetCustomerProcessingBill(ctx, current.CurrentCustomerID, billID)
}

func (s *Service) customerProcessingBillContext(ctx context.Context, token string) (CurrentContext, customerProcessingBillRepository, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, nil, fmt.Errorf("mini token required")
	}
	current, err := s.Me(ctx, token)
	if err != nil {
		return CurrentContext{}, nil, err
	}
	if current.CurrentCustomerID <= 0 {
		return CurrentContext{}, nil, ErrCustomerBindingNotFound
	}
	if !current.HasCapability(CapabilitySettlement) {
		return CurrentContext{}, nil, ErrCapabilityNotEnabled
	}
	repo, ok := s.repo.(customerProcessingBillRepository)
	if !ok || repo == nil {
		return CurrentContext{}, nil, fmt.Errorf("customer processing bill repository unavailable")
	}
	return current, repo, nil
}
