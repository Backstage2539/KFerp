package costing

import (
	"context"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Parameters(context.Context) (domain.Parameters, error)
	Calculate(context.Context, appcosting.CalculateRequest) (*appcosting.CalculateResponse, error)
	BeanList(context.Context) (*appcosting.CalculateResponse, error)
	CreateRun(context.Context, string) (*appcosting.Run, error)
	PublishRun(context.Context, string, int64) error
}

type Dependencies struct {
	Costing Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerCostingAPI(e, deps.Costing)
}
