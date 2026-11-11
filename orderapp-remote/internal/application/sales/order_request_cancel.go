package sales

import (
	"context"
	"fmt"
	"strings"
)

type OrderRequestResolution struct {
	Status  string `json:"status"`
	OrderID int64  `json:"order_id,omitempty"`
}

type OrderRequestCancelCommand struct {
	RequestID, RequestActor, RequestHash, Actor, Reason string
}

type orderRequestCancelRepository interface {
	CancelUncommittedOrderRequest(context.Context, OrderRequestCancelCommand) (OrderRequestResolution, error)
	OrderRequestCancelled(context.Context, string) (bool, error)
}

func (s *Service) OrderRequestCancelled(ctx context.Context, id string) (bool, error) {
	if len(strings.TrimSpace(id)) < 16 || len(id) > 100 {
		return false, fmt.Errorf("请求标识无效")
	}
	repo, ok := s.repo.(orderRequestCancelRepository)
	if !ok {
		return false, fmt.Errorf("订单请求核对不可用")
	}
	return repo.OrderRequestCancelled(ctx, id)
}

func (s *Service) CancelUncommittedOrderRequest(ctx context.Context, cmd OrderRequestCancelCommand) (OrderRequestResolution, error) {
	if len(strings.TrimSpace(cmd.RequestID)) < 16 || len(cmd.RequestID) > 100 || strings.TrimSpace(cmd.RequestActor) == "" || strings.TrimSpace(cmd.RequestHash) == "" || strings.TrimSpace(cmd.Actor) == "" || strings.TrimSpace(cmd.Reason) == "" {
		return OrderRequestResolution{}, fmt.Errorf("原请求及核对原因必填")
	}
	repo, ok := s.repo.(orderRequestCancelRepository)
	if !ok {
		return OrderRequestResolution{}, fmt.Errorf("订单请求核对不可用")
	}
	return repo.CancelUncommittedOrderRequest(ctx, cmd)
}
