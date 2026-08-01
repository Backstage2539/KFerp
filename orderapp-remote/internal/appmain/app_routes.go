package appmain

import (
	authzapp "orderapp/internal/application/authz"
	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	companyapp "orderapp/internal/application/company"
	contractsapp "orderapp/internal/application/contracts"
	costingapp "orderapp/internal/application/costing"
	customerapp "orderapp/internal/application/customer"
	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"
	financeapp "orderapp/internal/application/finance"
	inventoryapp "orderapp/internal/application/inventory"
	manufacturingapp "orderapp/internal/application/manufacturing"
	materialsapp "orderapp/internal/application/materials"
	messagecenterapp "orderapp/internal/application/messagecenter"
	productionapp "orderapp/internal/application/production"
	purchaseapp "orderapp/internal/application/purchase"
	salesapp "orderapp/internal/application/sales"
	stockapp "orderapp/internal/application/stock"
	docconvert "orderapp/internal/infrastructure/docconvert"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescontracts "orderapp/internal/infrastructure/postgres/contracts"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgrescustomer "orderapp/internal/infrastructure/postgres/customer"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"
	postgresfinance "orderapp/internal/infrastructure/postgres/finance"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresmanufacturing "orderapp/internal/infrastructure/postgres/manufacturing"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgresmessagecenter "orderapp/internal/infrastructure/postgres/messagecenter"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgrespurchase "orderapp/internal/infrastructure/postgres/purchase"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	bomhttp "orderapp/internal/interfaces/http/bom"
	cataloghttp "orderapp/internal/interfaces/http/catalog"
	companyhttp "orderapp/internal/interfaces/http/company"
	contractshttp "orderapp/internal/interfaces/http/contracts"
	costinghttp "orderapp/internal/interfaces/http/costing"
	customerhttp "orderapp/internal/interfaces/http/customer"
	customerfulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
	customerportalhttp "orderapp/internal/interfaces/http/customerportal"
	financehttp "orderapp/internal/interfaces/http/finance"
	inventoryhttp "orderapp/internal/interfaces/http/inventory"
	manufacturinghttp "orderapp/internal/interfaces/http/manufacturing"
	materialshttp "orderapp/internal/interfaces/http/materials"
	messagecenterhttp "orderapp/internal/interfaces/http/messagecenter"
	productionhttp "orderapp/internal/interfaces/http/production"
	purchasehttp "orderapp/internal/interfaces/http/purchase"
	saleshttp "orderapp/internal/interfaces/http/sales"
	stockhttp "orderapp/internal/interfaces/http/stock"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerAppRoutes(e *echo.Echo, pool *pgxpool.Pool, cfg appConfig) {
	schema := cfg.Schema
	assetDir := cfg.AssetDir
	authzSvc := authzapp.NewService(postgresauthz.NewRepository(pool, schema))
	bomSvc := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	catalogSvc := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	companySvc := companyapp.NewService(postgrescompany.NewRepository(pool, schema))
	contractConverter := contractsapp.PDFConverter(docconvert.NewLibreOfficeConverter(cfg.DocxConverterCommand))
	if cfg.DocxConverterURL != "" {
		contractConverter = docconvert.NewGotenbergConverter(cfg.DocxConverterURL)
	}
	contractsSvc := contractsapp.NewService(postgrescontracts.NewRepository(pool, schema, postgrescontracts.WithAssetDir(assetDir)), contractConverter, contractsapp.WithAssetDir(assetDir))
	costingSvc := costingapp.NewService(postgrescosting.NewRepository(pool, schema))
	customerSvc := customerapp.NewService(postgrescustomer.NewRepository(pool, schema, assetDir))
	customerFulfillmentSvc := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
	customerPortalIdentity := customerPortalIdentityProvider(cfg)
	customerPortalSvc := customerportalapp.NewService(postgrescustomerportal.NewRepository(pool, schema), customerPortalIdentity)
	financeSvc := financeapp.NewService(postgresfinance.NewRepository(pool, schema))
	inventorySvc := inventoryapp.NewService(postgresinventory.NewRepository(pool, schema))
	manufacturingSvc := manufacturingapp.NewService(postgresmanufacturing.NewRepository(pool, schema))
	materialsSvc := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	messageCenterSvc := messagecenterapp.NewService(postgresmessagecenter.NewRepository(pool, schema))
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	stockSvc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	purchaseSvc := purchaseapp.NewService(postgrespurchase.NewRepository(pool, schema), stockSvc)
	salesSvc := salesapp.NewService(postgressales.NewRepository(pool, schema, postgressales.WithSalesOrderAssetDir(assetDir)))

	e.Use(supporthttp.AuthorizationMiddleware(authzSvc))

	supporthttp.RegisterRoutes(e, pool, schema, supporthttp.Dependencies{Authz: authzSvc})
	messagecenterhttp.RegisterRoutes(e, messagecenterhttp.Dependencies{MessageCenter: messageCenterSvc})
	customerportalhttp.RegisterRoutes(e, customerportalhttp.Dependencies{CustomerPortal: customerPortalSvc, MessageCenter: messageCenterSvc, SalesDocuments: salesSvc, EmployeeSales: salesSvc, CustomerMaintenance: customerSvc, AssetDir: assetDir})
	customerfulfillmenthttp.RegisterRoutes(e, customerfulfillmenthttp.Dependencies{CustomerFulfillment: customerFulfillmentSvc, Customers: customerSvc, MessageCenter: messageCenterSvc, Sales: salesSvc})
	cataloghttp.RegisterRoutes(e, cataloghttp.Dependencies{Catalog: catalogSvc})
	materialshttp.RegisterRoutes(e, materialshttp.Dependencies{Materials: materialsSvc})
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomSvc})
	costinghttp.RegisterRoutes(e, costinghttp.Dependencies{Costing: costingSvc, Authz: authzSvc})
	inventoryhttp.RegisterRoutes(e, inventoryhttp.Dependencies{Inventory: inventorySvc})
	manufacturinghttp.RegisterRoutes(e, manufacturinghttp.Dependencies{Manufacturing: manufacturingSvc})
	stockhttp.RegisterRoutes(e, stockhttp.Dependencies{Stock: stockSvc})
	purchasehttp.RegisterRoutes(e, purchasehttp.Dependencies{Purchase: purchaseSvc})
	productionhttp.RegisterRoutes(e, productionhttp.Dependencies{Production: productionSvc, Stock: stockSvc, MessageCenter: messageCenterSvc})
	companyhttp.RegisterRoutes(e, companyhttp.Dependencies{Company: companySvc})
	contractshttp.RegisterRoutes(e, contractshttp.Dependencies{Contracts: contractsSvc})
	customerhttp.RegisterRoutes(e, customerhttp.Dependencies{Customer: customerSvc, AssetDir: assetDir})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesSvc, MessageCenter: messageCenterSvc, AssetDir: assetDir})
	financehttp.RegisterRoutes(e, financehttp.Dependencies{Finance: financeSvc, CustomerAccounts: customerFulfillmentSvc})
}

func customerPortalIdentityProvider(cfg appConfig) customerportalapp.IdentityProvider {
	if cfg.CustomerPortalDevLogin {
		return customerportalhttp.StaticIdentityProvider{
			OpenID:  cfg.CustomerPortalDevOpenID,
			UnionID: cfg.CustomerPortalDevUnionID,
		}
	}
	if cfg.WechatMiniAppID != "" && cfg.WechatMiniAppSecret != "" {
		return customerportalhttp.WechatIdentityProvider{
			AppID:     cfg.WechatMiniAppID,
			AppSecret: cfg.WechatMiniAppSecret,
		}
	}
	return customerportalhttp.DisabledIdentityProvider{}
}
