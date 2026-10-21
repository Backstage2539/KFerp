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
	e.POST("/api/mini/employee/orders/:id/payment-voucher", func(c echo.Context) error {
		access, ok, err := h.ensureMiniEmployeeOrderAccess(c, "orders.write")
		if !ok {
			return err
		}
		if access.row.IsVoid {
			return c.JSON(http.StatusConflict, map[string]string{"error": "作废订单不能上传收款凭证"})
		}
		saver, ok := sales.(supporthttp.SalesOrderAssetSaver)
		if !ok {
			return miniInternalError(c)
		}
		asset, err := supporthttp.SaveUploadedSalesOrderAsset(c, saver, assetDir, "payment_voucher", miniEmployeeActor(access.employee))
		if err != nil {
			var httpError *echo.HTTPError
			if errors.As(err, &httpError) && httpError.Code == http.StatusBadRequest {
				message, _ := httpError.Message.(string)
				return c.JSON(http.StatusBadRequest, map[string]string{"error": message})
			}
			return miniInternalError(c)
		}
		return c.JSON(http.StatusCreated, map[string]any{"asset": asset})
	})
}
