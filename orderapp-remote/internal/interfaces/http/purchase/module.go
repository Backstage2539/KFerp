package purchase

import (
	purchaseapp "orderapp/internal/application/purchase"

	"github.com/labstack/echo/v4"
)

type Dependencies struct {
	Purchase *purchaseapp.Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerPurchaseAPI(e, deps.Purchase)
}
