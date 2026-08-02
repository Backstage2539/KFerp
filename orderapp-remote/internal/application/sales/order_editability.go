package sales

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type OrderEditState struct {
	IsVoid             bool
	ShipStatus         string
	ProcessStatus      string
	HasProductionPlan  bool
	HasProduceBatch    bool
	HasWorkOrder       bool
	HasShipment        bool
	HasStockAllocation bool
	HasStockDeduction  bool
}

type OrderEditability struct {
	CanEdit     bool   `json:"can_edit"`
	BlockReason string `json:"edit_block_reason"`
}

func EvaluateOrderEditability(state OrderEditState) OrderEditability {
	if state.IsVoid {
		return OrderEditability{BlockReason: "订单已作废，不能再编辑"}
	}
	if strings.TrimSpace(state.ShipStatus) == "已发货" {
		return OrderEditability{BlockReason: "订单已发货，不能再编辑"}
	}
	if state.HasShipment {
		return OrderEditability{BlockReason: "订单已进入发货流程，不能再编辑"}
	}
	if state.HasStockDeduction {
		return OrderEditability{BlockReason: "订单已扣减库存，不能再编辑"}
	}
	if state.HasStockAllocation {
		return OrderEditability{BlockReason: "订单已占用库存，不能再编辑"}
	}
	if state.HasWorkOrder {
		return OrderEditability{BlockReason: "订单已进入生产流程，不能再编辑"}
	}
	if state.HasProduceBatch {
		return OrderEditability{BlockReason: "订单已进入旧版生产批次，不能再编辑"}
	}
	if state.HasProductionPlan {
		return OrderEditability{BlockReason: "订单已进入生产计划，不能再编辑"}
	}
	status := strings.TrimSpace(state.ProcessStatus)
	if status != "" && status != "待处理" && status != "待生产" {
		return OrderEditability{BlockReason: "订单已进入生产流程，不能再编辑"}
	}
	return OrderEditability{CanEdit: true}
}

type OrderEditConflictError struct {
	message string
}

func (e *OrderEditConflictError) Error() string {
	return e.message
}

func NewOrderEditConflictError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "订单当前状态不允许编辑"
	}
	return &OrderEditConflictError{message: message}
}

func OrderEditConflictMessage(err error) (string, bool) {
	var conflict *OrderEditConflictError
	if !errors.As(err, &conflict) || conflict == nil || strings.TrimSpace(conflict.message) == "" {
		return "", false
	}
	return conflict.message, true
}

type OrderEditabilityRepository interface {
	OrderEditability(context.Context, int64) (OrderEditability, error)
}

func (s *Service) OrderEditability(ctx context.Context, orderID int64) (OrderEditability, error) {
	if orderID <= 0 {
		return OrderEditability{}, fmt.Errorf("invalid order id")
	}
	repo, ok := s.repo.(OrderEditabilityRepository)
	if !ok {
		return OrderEditability{}, fmt.Errorf("order editability unavailable")
	}
	return repo.OrderEditability(ctx, orderID)
}
