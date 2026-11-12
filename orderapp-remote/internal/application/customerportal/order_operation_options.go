package customerportal

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
)

type MallOperationOptionsRepository interface {
	LoadMallOperationOptions(context.Context, int64, int64) ([]salesapp.OrderOperationOption, error)
}

func (s *Service) GetERPMallOperationOptions(ctx context.Context, customerID, mallProductID int64) ([]salesapp.OrderOperationOption, error) {
	if customerID <= 0 {
		return nil, ErrCustomerBindingNotFound
	}
	repo, ok := s.repo.(MallOperationOptionsRepository)
	if !ok {
		return nil, fmt.Errorf("商城工序候选暂不可用")
	}
	return repo.LoadMallOperationOptions(ctx, customerID, mallProductID)
}
