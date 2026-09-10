package sales

import (
	"context"
	"encoding/json"
	"fmt"
)

type OrderConfirmation struct {
	ProcessStatus         string          `json:"process_status"`
	ShipStatus            string          `json:"ship_status"`
	OrderID               int64           `json:"order_id"`
	OrderNo               string          `json:"order_no"`
	CustomerID            int64           `json:"customer_id"`
	Required              bool            `json:"confirmation_required"`
	Status                string          `json:"confirmation_status"`
	Revision              int64           `json:"confirmation_revision"`
	AcceptedRevision      int64           `json:"accepted_revision"`
	ResponsibleEmployeeID int64           `json:"responsible_employee_id"`
	Reason                string          `json:"confirmation_reason"`
	CanConfirm            bool            `json:"can_confirm"`
	CanEdit               bool            `json:"can_edit"`
	EditBlockReason       string          `json:"edit_block_reason"`
	History               json.RawMessage `json:"confirmation_history"`
}
type ReviewOrderCommand struct {
	OrderID    int64
	Revision   int64  `json:"revision"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
	EmployeeID int64
	Admin      bool
	Actor      string
}
type orderConfirmationRepository interface {
	OrderConfirmation(context.Context, int64) (OrderConfirmation, error)
	ReviewOrder(context.Context, ReviewOrderCommand) (OrderConfirmation, error)
}

func (s *Service) OrderConfirmation(ctx context.Context, id int64) (OrderConfirmation, error) {
	repo, ok := s.repo.(orderConfirmationRepository)
	if !ok {
		return OrderConfirmation{}, fmt.Errorf("订单确认服务不可用")
	}
	return repo.OrderConfirmation(ctx, id)
}
func (s *Service) ReviewOrder(ctx context.Context, cmd ReviewOrderCommand) (OrderConfirmation, error) {
	repo, ok := s.repo.(orderConfirmationRepository)
	if !ok {
		return OrderConfirmation{}, fmt.Errorf("订单确认服务不可用")
	}
	return repo.ReviewOrder(ctx, cmd)
}
