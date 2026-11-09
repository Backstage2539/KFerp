package customerportal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	fulfillment "orderapp/internal/application/customerfulfillment"
	portal "orderapp/internal/application/customerportal"
	excelinfra "orderapp/internal/infrastructure/excel"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"

	"github.com/labstack/echo/v4"
)

type miniIssuedBillReader interface {
	ListIssuedBills(context.Context, fulfillment.BillingQuery) (fulfillment.BillingBillsPage, error)
	GetIssuedBill(context.Context, int64, int64) (fulfillment.BillingBill, error)
	GetBillingPDFSettings(context.Context, int64, int64) (fulfillment.BillingPDFSettings, error)
}

func registerMiniIssuedBills(e *echo.Echo, svc Service, source MiniCustomerFulfillment, sales any, assetDir string) {
	e.GET("/api/mini/customer-issued-bills", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, svc, portal.CapabilitySettlement)
		if err != nil || !allowed {
			return err
		}
		reader, ok := source.(miniIssuedBillReader)
		if !ok {
			return miniInternalError(c)
		}
		from, to, status := strings.TrimSpace(c.QueryParam("date_from")), strings.TrimSpace(c.QueryParam("date_to")), strings.TrimSpace(c.QueryParam("status"))
		for _, d := range []string{from, to} {
			if d != "" {
				if _, e := time.Parse("2006-01-02", d); e != nil {
					return c.JSON(400, map[string]string{"error": "日期无效"})
				}
			}
		}
		if from != "" && to != "" && from > to || status != "" && status != "unpaid" && status != "paid" {
			return c.JSON(400, map[string]string{"error": "筛选条件无效"})
		}
		page, _ := strconv.Atoi(c.QueryParam("page"))
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		rows, err := reader.ListIssuedBills(c.Request().Context(), fulfillment.BillingQuery{CustomerID: current.CurrentCustomerID, DateFrom: from, DateTo: to, Status: status, Page: page, Limit: limit})
		if err != nil {
			return miniCustomerFulfillmentError(c, err)
		}
		return c.JSON(200, rows)
	})
	e.GET("/api/mini/customer-issued-bills/:id", func(c echo.Context) error {
		current, allowed, err := requireMiniCustomerFulfillmentContext(c, svc, portal.CapabilitySettlement)
		if err != nil || !allowed {
			return err
		}
		reader, ok := source.(miniIssuedBillReader)
		if !ok {
			return miniInternalError(c)
		}
		raw := c.Param("id")
		kind := ""
		for _, ext := range []string{".pdf", ".xlsx"} {
			if strings.HasSuffix(raw, ext) {
				kind = ext
				raw = strings.TrimSuffix(raw, ext)
				break
			}
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(400, map[string]string{"error": "账单编号无效"})
		}
		bill, err := reader.GetIssuedBill(c.Request().Context(), current.CurrentCustomerID, id)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "账单不存在"})
		}
		if kind == "" {
			return c.JSON(200, bill)
		}
		var body []byte
		mime := "application/pdf"
		if kind == ".xlsx" {
			body, err = excelinfra.RenderCustomerBill(bill)
			mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else {
			settings, settingsErr := reader.GetBillingPDFSettings(c.Request().Context(), current.CurrentCustomerID, id)
			if settingsErr != nil {
				return miniInternalError(c)
			}
			seal, sealErr := fulfillmenthttp.ReadBillingSeal(c.Request().Context(), sales, assetDir, settings.SealAssetID)
			if sealErr != nil {
				return miniInternalError(c)
			}
			body, err = pdfinfra.RenderCustomerBillStamped(bill, settings, seal)
		}
		if err != nil {
			return c.JSON(500, map[string]string{"error": "账单生成失败"})
		}
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="bill-%d%s"`, id, kind))
		return c.Blob(200, mime, body)
	})
}
