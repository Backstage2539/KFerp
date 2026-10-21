package sales

import (
	"github.com/labstack/echo/v4"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
)

func saveUploadedSalesOrderAsset(c echo.Context, salesSvc *salesapp.Service, assetDir, kind string) (salesapp.SalesOrderAsset, error) {
	return support.SaveUploadedSalesOrderAsset(c, salesSvc, assetDir, kind, support.ActorOf(c))
}
