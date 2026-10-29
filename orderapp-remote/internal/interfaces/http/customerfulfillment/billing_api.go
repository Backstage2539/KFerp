package customerfulfillment

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	excelinfra "orderapp/internal/infrastructure/excel"
	pdfinfra "orderapp/internal/infrastructure/pdf"

	"github.com/labstack/echo/v4"
)

func (a api) billingSummary(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	row, err := a.svc.BillingSummary(c.Request().Context(), customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) unbilledCharges(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	query, err := billingQuery(c, customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	row, err := a.svc.ListUnbilledCharges(c.Request().Context(), query)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) issuedBills(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	query, err := billingQuery(c, customerID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	row, err := a.svc.ListIssuedBills(c.Request().Context(), query)
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) issuedBill(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	rawID := c.Param("bill_id")
	format := ""
	for _, ext := range []string{".xlsx", ".pdf", ".png"} {
		if strings.HasSuffix(rawID, ext) {
			format = strings.TrimPrefix(ext, ".")
			rawID = strings.TrimSuffix(rawID, ext)
			break
		}
	}
	billID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || billID <= 0 {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("bill id invalid"))
	}
	bill, err := a.svc.GetIssuedBill(c.Request().Context(), customerID, billID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusNotFound, err)
	}
	if format == "" {
		return c.JSON(http.StatusOK, bill)
	}
	var content []byte
	var mime string
	switch format {
	case "xlsx":
		content, err = excelinfra.RenderCustomerBill(bill)
		mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "pdf":
		if c.QueryParam("unstamped") == "1" {
			content, err = pdfinfra.RenderCustomerBill(bill)
		} else {
			settings, settingsErr := a.svc.GetBillingPDFSettings(c.Request().Context(), customerID, billID)
			if settingsErr != nil {
				return customerFulfillmentError(c, http.StatusInternalServerError, settingsErr)
			}
			seal, sealErr := a.readBillingSeal(c.Request().Context(), settings.SealAssetID)
			if sealErr != nil {
				return customerFulfillmentError(c, http.StatusInternalServerError, sealErr)
			}
			content, err = pdfinfra.RenderCustomerBillStamped(bill, settings, seal)
		}
		mime = "application/pdf"
	case "png":
		settings, settingsErr := a.svc.GetBillingPDFSettings(c.Request().Context(), customerID, billID)
		if settingsErr != nil {
			return customerFulfillmentError(c, http.StatusInternalServerError, settingsErr)
		}
		seal, sealErr := a.readBillingSeal(c.Request().Context(), settings.SealAssetID)
		if sealErr != nil {
			return customerFulfillmentError(c, http.StatusInternalServerError, sealErr)
		}
		content, err = pdfinfra.RenderCustomerBillPNG(c.Request().Context(), bill, settings, seal)
		mime = "image/png"
	default:
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("unsupported bill format"))
	}
	if err != nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, err)
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, bill.SettlementNo, format))
	return c.Blob(http.StatusOK, mime, content)
}

func (a api) previewBilling(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req app.BillingPreviewCommand
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	req.CustomerID, req.Actor = customerID, currentCustomerFulfillmentActor(c)
	row, err := a.svc.PreviewBilling(c.Request().Context(), req)
	if err != nil {
		return customerFulfillmentError(c, http.StatusConflict, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) previewBillingExcel(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req app.BillingPreviewCommand
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	req.CustomerID, req.Actor = customerID, currentCustomerFulfillmentActor(c)
	preview, err := a.svc.PreviewBilling(c.Request().Context(), req)
	if err != nil {
		return customerFulfillmentError(c, http.StatusConflict, err)
	}
	if expected := strings.Trim(strings.TrimSpace(c.Request().Header.Get("If-Match")), `"`); expected != "" && expected != preview.PreviewVersion {
		return customerFulfillmentError(c, http.StatusConflict, fmt.Errorf("billing preview changed; review again"))
	}
	bill := app.BillingBill{SettlementResult: app.SettlementResult{CustomerID: preview.CustomerID, SettlementNo: "出账预览", PeriodFrom: preview.PeriodFrom, PeriodTo: preview.PeriodTo, FeeItems: preview.FeeItems, TotalAmountCents: preview.TotalAmountCents, DueAmountCents: preview.TotalAmountCents, Lines: preview.Lines}, CustomerName: preview.CustomerName, Status: "preview"}
	body, err := excelinfra.RenderCustomerBill(bill)
	if err != nil {
		return customerFulfillmentError(c, http.StatusInternalServerError, err)
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="customer-bill-preview.xlsx"`)
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", body)
}

func (a api) issueBilling(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var req app.IssueBillingCommand
	if err := c.Bind(&req); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	req.CustomerID, req.Actor = customerID, currentCustomerFulfillmentActor(c)
	row, err := a.svc.IssueBilling(c.Request().Context(), req)
	if err != nil {
		return customerFulfillmentError(c, http.StatusConflict, err)
	}
	return c.JSON(http.StatusOK, row)
}

func (a api) billingPDFSettings(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	billID, err := parseID(c.Param("bill_id"), "bill")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	settings, err := a.svc.GetBillingPDFSettings(c.Request().Context(), customerID, billID)
	if err != nil {
		return customerFulfillmentError(c, http.StatusNotFound, err)
	}
	return c.JSON(http.StatusOK, settings)
}

func (a api) saveBillingPDFSettings(c echo.Context) error {
	customerID, err := parseID(c.Param("customer_id"), "customer")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	billID, err := parseID(c.Param("bill_id"), "bill")
	if err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, err)
	}
	var settings app.BillingPDFSettings
	if err := c.Bind(&settings); err != nil {
		return customerFulfillmentError(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
	}
	settings.CustomerID, settings.SettlementID = customerID, billID
	result, err := a.svc.SaveBillingPDFSettings(c.Request().Context(), settings, currentCustomerFulfillmentActor(c))
	if err != nil {
		return customerFulfillmentError(c, http.StatusConflict, err)
	}
	return c.JSON(http.StatusOK, result)
}

func billingQuery(c echo.Context, customerID int64) (app.BillingQuery, error) {
	page, err := intQuery(c.QueryParam("page"), 1)
	if err != nil {
		return app.BillingQuery{}, fmt.Errorf("page invalid")
	}
	limit, err := intQuery(c.QueryParam("limit"), 50)
	if err != nil || limit > 200 {
		return app.BillingQuery{}, fmt.Errorf("limit invalid")
	}
	from, to := strings.TrimSpace(c.QueryParam("date_from")), strings.TrimSpace(c.QueryParam("date_to"))
	if from != "" {
		if _, err := timeParseDate(from); err != nil {
			return app.BillingQuery{}, fmt.Errorf("date_from invalid")
		}
	}
	if to != "" {
		if _, err := timeParseDate(to); err != nil {
			return app.BillingQuery{}, fmt.Errorf("date_to invalid")
		}
	}
	if from != "" && to != "" && from > to {
		return app.BillingQuery{}, fmt.Errorf("date range invalid")
	}
	return app.BillingQuery{CustomerID: customerID, FeeType: strings.TrimSpace(c.QueryParam("fee_type")), Status: strings.TrimSpace(c.QueryParam("status")), DateFrom: from, DateTo: to, Page: page, Limit: limit}, nil
}

func intQuery(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return v, nil
}

func timeParseDate(value string) (string, error) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}
