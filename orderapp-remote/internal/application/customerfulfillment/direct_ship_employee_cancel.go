package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
)

type CancelEmployeeDirectShipOrderCommand struct {
	CustomerID int64
	OrderID    int64
	Actor      string
}
type employeeDirectShipOrderRequestRepository interface {
	FindDirectShipRequestForOrder(context.Context, int64, int64) (int64, error)
}

// Resolve the order inside the selected customer, then use the same cancellation
// transaction as the customer portal so reservations and fees reverse together.
func (s *Service) CancelEmployeeDirectShipOrder(ctx context.Context, c CancelEmployeeDirectShipOrderCommand) (MiniDirectShipRequest, error) {
	if c.CustomerID <= 0 || c.OrderID <= 0 || strings.TrimSpace(c.Actor) == "" {
		return MiniDirectShipRequest{}, fmt.Errorf("请选择有效客户、订单和操作人")
	}
	r, ok := s.repo.(employeeDirectShipOrderRequestRepository)
	if !ok {
		return MiniDirectShipRequest{}, ErrMiniDirectShipUnavailable
	}
	id, err := r.FindDirectShipRequestForOrder(ctx, c.CustomerID, c.OrderID)
	if err != nil {
		return MiniDirectShipRequest{}, err
	}
	return s.CancelMiniDirectShipRequest(ctx, c.CustomerID, id, c.Actor)
}
