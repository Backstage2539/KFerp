package customerfulfillment

import (
	"context"

	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type Service interface {
	CustomerPortalOverview(context.Context, int64) (app.CustomerPortalOverview, error)
	CustomerPortalOptions(context.Context, int64) (app.CustomerFulfillmentOptions, error)
	InternalCustomerPortalOverview(context.Context, int64) (app.CustomerPortalOverview, error)
	InternalCustomerPortalOptions(context.Context, int64) (app.CustomerFulfillmentOptions, error)
	SubmitCustomerProcessingWorkOrder(context.Context, app.SubmitCustomerProcessingWorkOrderCommand) (app.ProcessingOrderSummary, error)
	SubmitCustomerDirectShipOrder(context.Context, app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error)
	AdjustCustodyInventory(context.Context, app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error)
	UpsertCustomerERPBinding(context.Context, app.UpsertCustomerERPBindingCommand) (app.CustomerERPBinding, error)
	ListCustomerERPBindings(context.Context, int64) ([]app.CustomerERPBinding, error)
	CustomerERPWorkbenchAvailable(context.Context, int64) (bool, error)
	CreateExternalUser(context.Context, app.CreateExternalUserCommand) (app.CustomerExternalUser, error)
	ListExternalUsers(context.Context, int64) ([]app.CustomerExternalUser, error)
	ResetExternalUserPassword(context.Context, app.ResetExternalUserPasswordCommand) (app.CustomerExternalUser, error)
	SetExternalUserLoginEnabled(context.Context, app.SetExternalUserLoginEnabledCommand) (app.CustomerExternalUser, error)
	CustomerFulfillmentOptions(context.Context, int64) (app.CustomerFulfillmentOptions, error)
	CreateSettlement(context.Context, app.CreateSettlementCommand) (app.SettlementResult, error)
	Overview(context.Context, app.OverviewQuery) (app.Overview, error)
	BillingSummary(context.Context, int64) (app.BillingTotals, error)
	ListUnbilledCharges(context.Context, app.BillingQuery) (app.BillingPage, error)
	ListIssuedBills(context.Context, app.BillingQuery) (app.BillingBillsPage, error)
	GetIssuedBill(context.Context, int64, int64) (app.BillingBill, error)
	PreviewBilling(context.Context, app.BillingPreviewCommand) (app.BillingPreview, error)
	IssueBilling(context.Context, app.IssueBillingCommand) (app.BillingBill, error)
	GetBillingPDFSettings(context.Context, int64, int64) (app.BillingPDFSettings, error)
	SaveBillingPDFSettings(context.Context, app.BillingPDFSettings, string) (app.BillingPDFSettings, error)
}

type SalesSaver interface {
	SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error)
}

type CustomerDirectory interface {
	List(context.Context, customerapp.ListQuery) (customerapp.ListResult, error)
}

type MessagePublisher interface {
	Publish(context.Context, messagecenterapp.PublishCommand) (int64, error)
}

type SharedProcessingService interface {
	CreateProcessingRequestForCustomer(context.Context, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error)
	PreviewProcessingRequestForCustomer(context.Context, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequestPreview, error)
	ListProcessingRequestsForCustomer(context.Context, int64, int) ([]customerportalapp.ProcessingRequest, error)
	ListProcessingCatalogTargetsForCustomer(context.Context, int64, []int64) ([]customerportalapp.ProcessingCatalogTarget, error)
}

type Dependencies struct {
	AssetDir            string
	CustomerFulfillment Service
	Customers           CustomerDirectory
	MessageCenter       MessagePublisher
	Sales               SalesSaver
	SharedProcessing    SharedProcessingService
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerFulfillmentSettingsRoutes(e, deps.CustomerFulfillment)
	registerEmployeeDirectShipAPI(e, deps.CustomerFulfillment)
	registerEmployeeDirectShipCancelAPI(e, deps.CustomerFulfillment)
	registerPortalDirectShipAPI(e, deps)
	api := api{svc: deps.CustomerFulfillment, customers: deps.Customers, messages: deps.MessageCenter, sales: deps.Sales, sharedProcessing: deps.SharedProcessing, assetDir: deps.AssetDir}
	e.GET("/api/customer-fulfillment/customers", api.listCustomers)
	e.GET("/api/customer-fulfillment/:customer_id/overview", api.overview)
	e.GET("/api/customer-fulfillment/:customer_id/options", api.options)
	e.POST("/api/customer-fulfillment/:customer_id/settlements", api.createSettlement)
	e.GET("/api/customer-fulfillment/:customer_id/billing/summary", api.billingSummary)
	e.GET("/api/customer-fulfillment/:customer_id/billing/unbilled", api.unbilledCharges)
	e.GET("/api/customer-fulfillment/:customer_id/billing/bills", api.issuedBills)
	e.GET("/api/customer-fulfillment/:customer_id/billing/bills/:bill_id", api.issuedBill)
	e.POST("/api/customer-fulfillment/:customer_id/billing/bills/:bill_id/payment", api.changeBillingPayment)
	e.GET("/api/customer-fulfillment/:customer_id/billing/bills/:bill_id/pdf-settings", api.billingPDFSettings)
	e.PUT("/api/customer-fulfillment/:customer_id/billing/bills/:bill_id/pdf-settings", api.saveBillingPDFSettings)
	e.POST("/api/customer-fulfillment/:customer_id/settlements/preview", api.previewBilling)
	e.POST("/api/customer-fulfillment/:customer_id/settlements/preview.xlsx", api.previewBillingExcel)
	e.POST("/api/customer-fulfillment/:customer_id/settlements/issue", api.issueBilling)
	e.POST("/api/customer-fulfillment/:customer_id/work-orders", api.submitInternalProcessingWorkOrder)
	e.POST("/api/customer-fulfillment/:customer_id/direct-ship-orders", api.submitInternalDirectShipOrder)
	e.POST("/api/customer-fulfillment/:customer_id/custody-adjustments", api.adjustCustodyInventory)
	e.GET("/api/customer-fulfillment/:customer_id/erp-bindings", api.listERPBindings)
	e.POST("/api/customer-fulfillment/:customer_id/erp-bindings", api.upsertERPBinding)
	e.GET("/api/customer-fulfillment/:customer_id/external-users", api.listExternalUsers)
	e.POST("/api/customer-fulfillment/:customer_id/external-users", api.CreateExternalUser)
	e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/password/reset", api.ResetExternalUserPassword)
	e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/login-enabled", api.SetExternalUserLoginEnabled)
	e.GET("/api/customer-processing/portal/fulfillment/overview", api.portalWorkbenchOverview)
	e.GET("/api/customer-processing/portal/fulfillment/billing/summary", api.portalWorkbenchBilling(api.billingSummary))
	e.GET("/api/customer-processing/portal/fulfillment/billing/unbilled", api.portalWorkbenchBilling(api.unbilledCharges))
	e.GET("/api/customer-processing/portal/fulfillment/billing/bills", api.portalWorkbenchBilling(api.issuedBills))
	e.GET("/api/customer-processing/portal/fulfillment/billing/bills/:bill_id", api.portalWorkbenchBilling(api.issuedBill))
	e.GET("/api/customer-processing/portal/overview", api.customerPortalOverview)
	e.GET("/api/customer-processing/portal/options", api.customerPortalOptions)
	e.GET("/api/customer-processing/internal/:customer_id/overview", api.internalCustomerPortalOverview)
	e.GET("/api/customer-processing/internal/:customer_id/options", api.internalCustomerPortalOptions)
	e.POST("/api/customer-processing/portal/work-orders", api.submitCustomerProcessingWorkOrder)
	e.GET("/api/customer-processing/portal/processing-catalog", api.sharedProcessingCatalog)
	e.GET("/api/customer-processing/portal/processing-requests", api.listSharedProcessingRequests)
	e.POST("/api/customer-processing/portal/processing-requests/preview", api.previewSharedProcessingRequest)
	e.POST("/api/customer-processing/portal/processing-requests", api.submitSharedProcessingRequest)
	e.GET("/api/customer-processing/internal/:customer_id/processing-catalog", api.sharedProcessingCatalog)
	e.GET("/api/customer-processing/internal/:customer_id/processing-requests", api.listSharedProcessingRequests)
	e.POST("/api/customer-processing/internal/:customer_id/processing-requests/preview", api.previewSharedProcessingRequest)
	e.POST("/api/customer-processing/internal/:customer_id/processing-requests", api.submitSharedProcessingRequest)
	e.POST("/api/customer-processing/portal/direct-ship-orders", api.submitCustomerDirectShipOrder)
}
