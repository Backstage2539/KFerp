package customerfulfillment

import (
	"context"

	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"

	"github.com/labstack/echo/v4"
)

type Service interface {
	ParseImport(context.Context, app.ParseImportCommand) (app.ImportBatch, error)
	ApplyImport(context.Context, app.ApplyImportCommand) (app.ApplyResult, error)
	CustomerPortalOverview(context.Context, int64) (app.CustomerPortalOverview, error)
	SubmitCustomerProcessingWorkOrder(context.Context, app.SubmitCustomerProcessingWorkOrderCommand) (app.ProcessingOrderSummary, error)
	SubmitCustomerDirectShipOrder(context.Context, app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error)
	AdjustCustodyInventory(context.Context, app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error)
	UpsertCustomerERPBinding(context.Context, app.UpsertCustomerERPBindingCommand) (app.CustomerERPBinding, error)
	ListCustomerERPBindings(context.Context, int64) ([]app.CustomerERPBinding, error)
	ImportPreview(context.Context, app.ImportPreviewQuery) (app.ImportPreview, error)
	ListImportRows(context.Context, app.ListImportRowsQuery) ([]app.ImportRow, error)
	CreateSettlement(context.Context, app.CreateSettlementCommand) (app.SettlementResult, error)
	Overview(context.Context, app.OverviewQuery) (app.Overview, error)
	ListImports(context.Context, app.ListImportsQuery) ([]app.ImportBatch, error)
}

type CustomerDirectory interface {
	List(context.Context, customerapp.ListQuery) (customerapp.ListResult, error)
}

type Dependencies struct {
	CustomerFulfillment Service
	Customers           CustomerDirectory
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	api := api{svc: deps.CustomerFulfillment, customers: deps.Customers}
	e.GET("/api/customer-fulfillment/customers", api.listCustomers)
	e.GET("/api/customer-fulfillment/:customer_id/overview", api.overview)
	e.POST("/api/customer-fulfillment/:customer_id/imports/parse", api.parseImport)
	e.GET("/api/customer-fulfillment/imports/:batch_id/rows", api.listImportRows)
	e.GET("/api/customer-fulfillment/imports/:batch_id/preview", api.importPreview)
	e.POST("/api/customer-fulfillment/imports/:batch_id/apply", api.applyImport)
	e.GET("/api/customer-fulfillment/:customer_id/imports", api.listImports)
	e.POST("/api/customer-fulfillment/:customer_id/settlements", api.createSettlement)
	e.POST("/api/customer-fulfillment/:customer_id/work-orders", api.submitInternalProcessingWorkOrder)
	e.POST("/api/customer-fulfillment/:customer_id/direct-ship-orders", api.submitInternalDirectShipOrder)
	e.POST("/api/customer-fulfillment/:customer_id/custody-adjustments", api.adjustCustodyInventory)
	e.GET("/api/customer-fulfillment/:customer_id/erp-bindings", api.listERPBindings)
	e.POST("/api/customer-fulfillment/:customer_id/erp-bindings", api.upsertERPBinding)
	e.GET("/api/customer-processing/portal/overview", api.customerPortalOverview)
	e.POST("/api/customer-processing/portal/work-orders", api.submitCustomerProcessingWorkOrder)
	e.POST("/api/customer-processing/portal/direct-ship-orders", api.submitCustomerDirectShipOrder)
}
