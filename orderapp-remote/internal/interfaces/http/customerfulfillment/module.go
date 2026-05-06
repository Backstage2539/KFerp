package customerfulfillment

import (
	"context"

	app "orderapp/internal/application/customerfulfillment"

	"github.com/labstack/echo/v4"
)

type Service interface {
	ParseImport(context.Context, app.ParseImportCommand) (app.ImportBatch, error)
	ApplyImport(context.Context, app.ApplyImportCommand) (app.ApplyResult, error)
	CreateSettlement(context.Context, app.CreateSettlementCommand) (app.SettlementResult, error)
	Overview(context.Context, app.OverviewQuery) (app.Overview, error)
	ListImports(context.Context, app.ListImportsQuery) ([]app.ImportBatch, error)
}

type Dependencies struct {
	CustomerFulfillment Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	api := api{svc: deps.CustomerFulfillment}
	e.GET("/api/customer-fulfillment/:customer_id/overview", api.overview)
	e.POST("/api/customer-fulfillment/:customer_id/imports/parse", api.parseImport)
	e.POST("/api/customer-fulfillment/imports/:batch_id/apply", api.applyImport)
	e.GET("/api/customer-fulfillment/:customer_id/imports", api.listImports)
	e.POST("/api/customer-fulfillment/:customer_id/settlements", api.createSettlement)
}
