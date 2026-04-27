package costing

import (
	"context"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Parameters(context.Context) (domain.Parameters, error)
	Settings(context.Context) ([]appcosting.ParameterSetting, error)
	UpdateSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error)
	Calculate(context.Context, appcosting.CalculateRequest) (*appcosting.CalculateResponse, error)
	BeanList(context.Context) (*appcosting.CalculateResponse, error)
	ListBeanListPublications(context.Context, string) ([]appcosting.BeanListPublication, error)
	PublishBeanList(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error)
	WithdrawBeanList(context.Context, appcosting.WithdrawBeanListCommand) error
	CreateRun(context.Context, string) (*appcosting.Run, error)
	PublishRun(context.Context, string, int64) error
}

type Dependencies struct {
	Costing Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerCostingAPI(e, deps.Costing)
}
