package costing

import (
	"context"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Parameters(context.Context) (domain.Parameters, error)
	Settings(context.Context) ([]appcosting.ParameterSetting, error)
	UpdateSetting(context.Context, appcosting.UpdateParameterCommand) (appcosting.ParameterSetting, error)
	Calculate(context.Context, appcosting.CalculateRequest) (*appcosting.CalculateResponse, error)
	ExplainPrice(context.Context, appcosting.PriceExplanationCommand) (*domain.PriceExplanation, error)
	PricingRuleTrial(context.Context, appcosting.PricingRuleTrialCommand) (*appcosting.PricingRuleTrialResult, error)
	PricingRuleTrialBatch(context.Context, []appcosting.PricingRuleTrialCommand) ([]appcosting.PricingRuleTrialBatchRow, error)
	BeanList(context.Context, appcosting.BeanListQuery) (*appcosting.CalculateResponse, error)
	ListBeanListPublications(context.Context, appcosting.BeanListPublicationQuery) ([]appcosting.BeanListPublication, error)
	PublishedBeanList(context.Context, appcosting.BeanListPublicationQuery) (*appcosting.BeanListPublication, error)
	GenerateBeanListPublicationPDF(context.Context, appcosting.BeanListPublicationPDFCommand, func(appcosting.BeanListPublication) ([]byte, error)) (appcosting.BeanListPublicationPDFFile, error)
	LoadBeanListPublicationPDF(context.Context, appcosting.BeanListPublicationPDFCommand) (appcosting.BeanListPublicationPDFFile, error)
	PublishBeanList(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error)
	SaveBeanListDraft(context.Context, appcosting.PublishBeanListCommand) (*appcosting.BeanListPublication, error)
	WithdrawBeanList(context.Context, appcosting.WithdrawBeanListCommand) error
	ArchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error
	UnarchiveBeanListPublications(context.Context, appcosting.ArchiveBeanListPublicationsCommand) error
	CreateRun(context.Context, string) (*appcosting.Run, error)
	PublishRun(context.Context, string, int64) error
}

// MaterialCostTrialService is optional for compatibility with lightweight
// costing fakes used by existing route tests and integrations.
type MaterialCostTrialService interface {
	MaterialCostTrialOptions(context.Context, int64) (appcosting.MaterialCostTrialOptions, error)
	MaterialCostTrial(context.Context, appcosting.MaterialCostTrialCommand) (appcosting.MaterialCostTrialResult, error)
}

type Dependencies struct {
	Costing Service
	Authz   support.AuthzService
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerCostingAPI(e, deps.Costing, deps.Authz)
}
