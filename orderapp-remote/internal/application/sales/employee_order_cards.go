package sales

import (
	"context"
	"fmt"

	progressdomain "orderapp/internal/domain/orderprogress"
)

type EmployeeOrderCursor struct {
	// An empty date represents a NULL document date, sorted after dated orders.
	Date string
	ID   int64
}

type EmployeeOrderCardsQuery struct {
	Scope      string
	EmployeeID int64
	Q          string
	Limit      int
	After      *EmployeeOrderCursor
}

type EmployeeOrderCard struct {
	ID                   int64                    `json:"id"`
	OrderNo              string                   `json:"order_no"`
	OrderDate            string                   `json:"order_date"`
	Customer             string                   `json:"customer"`
	GrandTotal           string                   `json:"grand_total"`
	PayStatusID          int64                    `json:"pay_status_id"`
	PayStatus            string                   `json:"pay_status"`
	ShipStatusID         int64                    `json:"ship_status_id"`
	ShipStatus           string                   `json:"ship_status"`
	ProcessStatus        string                   `json:"process_status"`
	DeliveryMode         string                   `json:"delivery_mode"`
	DeliveryStatus       string                   `json:"delivery_status"`
	SettlementReason     string                   `json:"settlement_reason"`
	PrepaymentAmount     string                   `json:"prepayment_amount"`
	PaidAmount           string                   `json:"paid_amount"`
	UnpaidAmount         string                   `json:"unpaid_amount"`
	ConfirmationStatus   string                   `json:"confirmation_status"`
	ConfirmationRequired bool                     `json:"confirmation_required"`
	FulfillmentProgress  *progressdomain.Progress `json:"fulfillment_progress,omitempty"`
}

type EmployeeOrderCardsPage struct {
	Rows []EmployeeOrderCard
	Next *EmployeeOrderCursor
}

type EmployeeOrderCardReader interface {
	ListEmployeeOrderCards(context.Context, EmployeeOrderCardsQuery) (EmployeeOrderCardsPage, error)
}

func (s *Service) ListEmployeeOrderCards(ctx context.Context, query EmployeeOrderCardsQuery) (EmployeeOrderCardsPage, error) {
	repo, ok := s.repo.(EmployeeOrderCardReader)
	if !ok {
		return EmployeeOrderCardsPage{}, fmt.Errorf("employee order cards unavailable")
	}
	return repo.ListEmployeeOrderCards(ctx, query)
}
