package sales

import (
	"context"
	"errors"
)

type employeeOrderDetailRepository interface {
	EmployeeOrderRow(context.Context, OrderListQuery) (*OrderRow, error)
	OrderDetailForm(context.Context, int64) (OrderFormData, error)
}

func (s *Service) EmployeeOrderRow(ctx context.Context, query OrderListQuery) (*OrderRow, error) {
	repo, ok := s.repo.(employeeOrderDetailRepository)
	if !ok || query.OrderID <= 0 {
		return nil, errors.New("employee order read unavailable")
	}
	query.Limit = 1
	return repo.EmployeeOrderRow(ctx, query)
}

func (s *Service) OrderDetailForm(ctx context.Context, id int64) (OrderFormData, error) {
	repo, ok := s.repo.(employeeOrderDetailRepository)
	if !ok || id <= 0 {
		return OrderFormData{}, errors.New("employee order read unavailable")
	}
	return repo.OrderDetailForm(ctx, id)
}
