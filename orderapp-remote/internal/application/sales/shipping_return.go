package sales

import (
	"context"
	"fmt"
	d "orderapp/internal/domain/shippingreturn"
)

type ShippingReturnRepository interface {
	RematchShippingReturn(context.Context, string, int64, []int64) (d.Batch, error)
	ShippingReturnTemplates(context.Context) ([]d.Template, error)
	SaveShippingReturnTemplates(context.Context, string, []d.Template) error
	ImportShippingReturn(context.Context, string, string, string, d.Template, []d.Row) (d.Batch, error)
	ShippingReturnCandidates(context.Context, int64) ([]d.Row, error)
	SelectShippingReturn(context.Context, string, int64, int64, string) (FillShipmentTrackingResult, error)
}

func (s *Service) ShippingReturns() (ShippingReturnRepository, error) {
	r, ok := s.repo.(ShippingReturnRepository)
	if !ok {
		return nil, fmt.Errorf("寄件回传服务不可用")
	}
	return r, nil
}
