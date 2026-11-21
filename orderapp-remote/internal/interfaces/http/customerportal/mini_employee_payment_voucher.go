package customerportal

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	supporthttp "orderapp/internal/interfaces/http/support"
)

func registerMiniEmployeePaymentVoucherAPI(e *echo.Echo, portal Service, sales EmployeeSales, assetDir string) {
	if strings.TrimSpace(assetDir) == "" {
		assetDir = "/app/data/assets"
	}
	h := miniEmployeeOrderHandler{portal: portal, sales: sales}
	upload := func(c echo.Context, actor string) error {
		saver, ok := sales.(supporthttp.SalesOrderAssetSaver)
		if !ok {
			return miniInternalError(c)
		}
		asset, err := supporthttp.SaveUploadedSalesOrderAsset(c, saver, assetDir, "payment_voucher", actor)
		if err != nil {
			var httpError *echo.HTTPError
			if errors.As(err, &httpError) && httpError.Code == http.StatusBadRequest {
				message, _ := httpError.Message.(string)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
			}
			return miniInternalError(c)
		}
		return c.JSON(http.StatusCreated, map[string]any{"asset": asset})
	}
	e.POST("/api/mini/employee/order/payment-voucher", func(c echo.Context) error {
		employee, err := requireMiniEmployee(c.Request().Context(), c.Request().Header.Get(echo.HeaderAuthorization), portal, "orders.write")
		if err != nil {
			return miniEmployeeAuthError(c, err)
		}
		return upload(c, miniEmployeeActor(employee))
	})
	e.POST("/api/mini/employee/orders/:id/payment-voucher", func(c echo.Context) error {
		access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.write")
		if !ok {
			return err
		}
		if access.row.IsVoid {
			return c.JSON(http.StatusConflict, map[string]string{"error": "作废订单不能上传收款凭证"})
		}
		return upload(c, miniEmployeeActor(access.employee))
	})
}
