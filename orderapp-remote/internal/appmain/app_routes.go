package appmain

import (
	authzapp "orderapp/internal/application/authz"
	bomapp "orderapp/internal/application/bom"
	catalogapp "orderapp/internal/application/catalog"
	companyapp "orderapp/internal/application/company"
	costingapp "orderapp/internal/application/costing"
	customerapp "orderapp/internal/application/customer"
	inventoryapp "orderapp/internal/application/inventory"
	materialsapp "orderapp/internal/application/materials"
	productionapp "orderapp/internal/application/production"
	purchaseapp "orderapp/internal/application/purchase"
	salesapp "orderapp/internal/application/sales"
	stockapp "orderapp/internal/application/stock"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgresbom "orderapp/internal/infrastructure/postgres/bom"
	postgrescatalog "orderapp/internal/infrastructure/postgres/catalog"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescosting "orderapp/internal/infrastructure/postgres/costing"
	postgrescustomer "orderapp/internal/infrastructure/postgres/customer"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgrespurchase "orderapp/internal/infrastructure/postgres/purchase"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	bomhttp "orderapp/internal/interfaces/http/bom"
	cataloghttp "orderapp/internal/interfaces/http/catalog"
	companyhttp "orderapp/internal/interfaces/http/company"
	costinghttp "orderapp/internal/interfaces/http/costing"
	customerhttp "orderapp/internal/interfaces/http/customer"
	inventoryhttp "orderapp/internal/interfaces/http/inventory"
	materialshttp "orderapp/internal/interfaces/http/materials"
	productionhttp "orderapp/internal/interfaces/http/production"
	purchasehttp "orderapp/internal/interfaces/http/purchase"
	saleshttp "orderapp/internal/interfaces/http/sales"
	stockhttp "orderapp/internal/interfaces/http/stock"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerAppRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, assetDir string) {
	authzSvc := authzapp.NewService(postgresauthz.NewRepository(pool, schema))
	bomSvc := bomapp.NewService(postgresbom.NewRepository(pool, schema))
	catalogSvc := catalogapp.NewService(postgrescatalog.NewRepository(pool, schema))
	companySvc := companyapp.NewService(postgrescompany.NewRepository(pool, schema))
	costingSvc := costingapp.NewService(postgrescosting.NewRepository(pool, schema))
	customerSvc := customerapp.NewService(postgrescustomer.NewRepository(pool, schema, assetDir))
	inventorySvc := inventoryapp.NewService(postgresinventory.NewRepository(pool, schema))
	materialsSvc := materialsapp.NewService(postgresmaterials.NewRepository(pool, schema))
	productionSvc := productionapp.NewService(postgresproduction.NewRepository(pool, schema))
	stockSvc := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	purchaseSvc := purchaseapp.NewService(postgrespurchase.NewRepository(pool, schema), stockSvc)
	salesSvc := salesapp.NewService(postgressales.NewRepository(pool, schema))

	e.Use(supporthttp.AuthorizationMiddleware(authzSvc))

	supporthttp.RegisterRoutes(e, pool, schema, supporthttp.Dependencies{Authz: authzSvc})
	cataloghttp.RegisterRoutes(e, cataloghttp.Dependencies{Catalog: catalogSvc})
	materialshttp.RegisterRoutes(e, materialshttp.Dependencies{Materials: materialsSvc})
	bomhttp.RegisterRoutes(e, bomhttp.Dependencies{Bom: bomSvc})
	costinghttp.RegisterRoutes(e, costinghttp.Dependencies{Costing: costingSvc})
	inventoryhttp.RegisterRoutes(e, inventoryhttp.Dependencies{Inventory: inventorySvc})
	stockhttp.RegisterRoutes(e, stockhttp.Dependencies{Stock: stockSvc})
	purchasehttp.RegisterRoutes(e, purchasehttp.Dependencies{Purchase: purchaseSvc})
	productionhttp.RegisterRoutes(e, productionhttp.Dependencies{Production: productionSvc})
	companyhttp.RegisterRoutes(e, companyhttp.Dependencies{Company: companySvc})
	customerhttp.RegisterRoutes(e, customerhttp.Dependencies{Customer: customerSvc, AssetDir: assetDir})
	saleshttp.RegisterRoutes(e, saleshttp.Dependencies{Sales: salesSvc})
}
