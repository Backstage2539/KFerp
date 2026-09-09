package sales

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	excelinfra "orderapp/internal/infrastructure/excel"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

const accountPrefix = "/api/customer-processing/portal"

type customerAccountReader interface {
	CustomerAccount(context.Context, app.AccountQuery) (app.AccountData, error)
}

func registerCustomerAccountRoutes(e *echo.Echo, h orderAPIHandler) {
	for _, name := range []string{"orders", "order-fees", "statements", "statements.xlsx", "statements.pdf", "settlements"} {
		e.GET(accountPrefix+"/"+name, h.customerAccount)
	}
	e.GET(accountPrefix+"/orders/:id/detail", h.customerAccount)
	e.GET(accountPrefix+"/settlements/:settlement", h.customerAccount)
	for _, key := range []string{"sales-orders", "sales-order-preview", "sales-order-preview.pdf", "sales-order-images"} {
		e.GET(accountPrefix+"/orders/:id/"+key, h.customerDocument)
	}
	e.POST(accountPrefix+"/orders/:id/sales-orders", h.customerDocument)
	e.POST(accountPrefix+"/orders/:id/sales-order-images", h.customerDocument)
	e.GET(accountPrefix+"/orders/:id/sales-files/:file", h.customerDocument)
}
func (h orderAPIHandler) accountScope(c echo.Context, finance bool) (int64, error) {
	data, err := h.portalContext(c, "context", 0)
	if err != nil {
		return 0, echo.NewHTTPError(403, err.Error())
	}
	id, _ := data["customer_id"].(int64)
	if id <= 0 {
		return 0, echo.NewHTTPError(403, "客户未绑定")
	}
	if raw := c.QueryParam("customer_id"); raw != "" && raw != strconv.FormatInt(id, 10) {
		return 0, echo.NewHTTPError(403, "客户范围不匹配")
	}
	codes, _ := data["capabilities"].([]string)
	for _, code := range codes {
		if finance && code == "settlement" || !finance && (code == "direct_ship" || code == "product_order" || code == "processing" || code == "mall") {
			return id, nil
		}
	}
	return 0, echo.NewHTTPError(403, "客户能力未开通")
}
func (h orderAPIHandler) readAccount(c echo.Context, q app.AccountQuery) (app.AccountData, error) {
	reader, ok := h.customerScope.(customerAccountReader)
	if !ok {
		return app.AccountData{}, echo.NewHTTPError(503, "账目服务不可用")
	}
	return reader.CustomerAccount(c.Request().Context(), q)
}
func (h orderAPIHandler) customerAccount(c echo.Context) error {
	path := c.Request().URL.Path
	finance := !strings.Contains(path, "/orders")
	id, err := h.accountScope(c, finance)
	if err != nil {
		return err
	}
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	q, err := app.NormalizeAccountQuery(app.AccountQuery{CurrentVersion: !finance, CustomerID: id, Query: strings.TrimSpace(c.QueryParam("q")), DateFrom: c.QueryParam("date_from"), DateTo: c.QueryParam("date_to"), Period: c.QueryParam("period"), Anchor: c.QueryParam("anchor"), PayStatus: c.QueryParam("pay_status"), ShipStatus: c.QueryParam("ship_status"), IncludeVoid: c.QueryParam("include_void") == "true", Page: page, Limit: limit})
	if err != nil {
		return echo.NewHTTPError(400, err.Error())
	}
	if c.Param("id") != "" {
		q.OrderID, err = strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || q.OrderID <= 0 {
			return echo.NewHTTPError(400, "订单编号无效")
		}
		q.IncludeVoid = true
	}
	data, err := h.readAccount(c, q)
	if err != nil {
		return echo.NewHTTPError(400, err.Error())
	}
	if q.OrderID > 0 {
		if len(data.Rows) != 1 {
			return echo.NewHTTPError(404, "订单不存在或不属于当前客户")
		}
		return c.JSON(200, data.Rows[0])
	}
	format := ""
	if strings.HasSuffix(path, ".xlsx") {
		format = "xlsx"
	}
	if strings.HasSuffix(path, ".pdf") {
		format = "pdf"
	}
	if raw := c.Param("settlement"); raw != "" {
		pieces := strings.Split(raw, ".")
		billID, _ := strconv.ParseInt(pieces[0], 10, 64)
		if len(pieces) == 2 {
			format = pieces[1]
		}
		var found *app.AccountSettlement
		for i := range data.Settlements {
			if data.Settlements[i].ID == billID {
				found = &data.Settlements[i]
				break
			}
		}
		if found == nil {
			return echo.NewHTTPError(404, "结算单不存在或不属于当前客户")
		}
		if format == "" {
			return c.JSON(200, found)
		}
		data.Rows = []app.AccountOrder{}
		data.Summary = app.AccountSummary{}
		data.Fees = found.Fees
		data.Settlements = []app.AccountSettlement{*found}
		data.DateFrom = found.PeriodFrom
		data.DateTo = found.PeriodTo
	}
	if format != "" {
		var content []byte
		mime := "application/pdf"
		switch format {
		case "pdf":
			content, err = pdfinfra.RenderCustomerAccount(data)
		case "xlsx":
			mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			content, err = excelinfra.RenderCustomerAccount(data)
		default:
			return echo.NewHTTPError(400, "不支持的文件格式")
		}
		if err != nil {
			return echo.NewHTTPError(500, "账单生成失败："+err.Error())
		}
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="customer-statement.%s"`, format))
		return c.Blob(200, mime, content)
	}
	if !finance {
		data.Fees = nil
		data.Settlements = nil
	}
	data.Paginate(q.Page, q.Limit)
	return c.JSON(200, data)
}

// Validate every selected order before any document lookup, rendering or generation.
func (h orderAPIHandler) customerDocument(c echo.Context) error {
	customerID, err := h.accountScope(c, false)
	if err != nil {
		return err
	}
	combined := c.Param("id") == "combined"
	ids := []int64{}
	if combined {
		if c.Request().Method == http.MethodPost {
			ids, err = parseCombinedDocumentBodyOrderIDs(c)
		} else {
			ids, err = parseCombinedDocumentOrderIDs(c)
		}
	} else {
		var id int64
		id, err = parseSalesOrderID(c)
		ids = []int64{id}
	}
	if err != nil {
		return echo.NewHTTPError(400, err.Error())
	}
	if len(ids) == 0 || len(ids) > 100 {
		return echo.NewHTTPError(400, "每次请选择1至100笔订单")
	}
	names := []string{}
	customerName := ""
	for _, id := range ids {
		d, e := h.readAccount(c, app.AccountQuery{CustomerID: customerID, OrderID: id, Page: 1, Limit: 1})
		if e != nil {
			return echo.NewHTTPError(500, "订单校验失败")
		}
		if len(d.Rows) != 1 || d.Rows[0].IsVoid {
			return echo.NewHTTPError(403, "订单不存在、已作废或不属于当前客户")
		}
		names = append(names, d.Rows[0].OrderNo)
		customerName = d.CustomerName
	}
	path := c.Request().URL.Path
	single := salesOrderDocumentHandler{sales: h.sales}
	multi := combinedDocumentHandler{sales: h.sales}
	if strings.HasSuffix(path, "sales-order-preview.pdf") {
		if combined {
			return multi.salesOrderPreviewPDF(c)
		}
		return single.previewPDF(c)
	}
	if strings.HasSuffix(path, "sales-order-preview") {
		// Customers only need the rendered preview; settings, source assets and internal metadata stay private.
		version := 0
		if combined {
			p, e := h.sales.PreviewCombinedSalesOrderDocument(c.Request().Context(), ids)
			err = e
			version = p.NextVersionNo
		} else {
			p, e := h.sales.PreviewSalesOrderDocument(c.Request().Context(), ids[0])
			err = e
			version = p.NextVersionNo
		}
		if err != nil {
			return echo.NewHTTPError(400, err.Error())
		}
		return c.JSON(200, map[string]any{"next_version_no": version, "snapshot": map[string]any{}})
	}
	isImage := strings.HasSuffix(path, "sales-order-images") || strings.HasSuffix(path, ".png")
	if c.Request().Method == http.MethodPost {
		version := 0
		docID := int64(0)
		if combined {
			cmd := salesapp.CombinedDocumentCommand{Actor: support.ActorOf(c), OrderIDs: ids}
			if isImage {
				v, e := h.sales.GenerateCombinedSalesOrderImage(c.Request().Context(), cmd)
				err = e
				docID = v.Document.ID
				version = v.Document.VersionNo
			} else {
				v, e := h.sales.GenerateCombinedSalesOrderDocument(c.Request().Context(), cmd)
				err = e
				docID = v.Document.ID
				version = v.Document.VersionNo
			}
		} else {
			if isImage {
				v, e := h.sales.GenerateSalesOrderImage(c.Request().Context(), salesapp.GenerateSalesOrderImageCommand{Actor: support.ActorOf(c), OrderID: ids[0]})
				err = e
				docID = v.Document.ID
				version = v.Document.VersionNo
			} else {
				v, e := h.sales.GenerateSalesOrderDocument(c.Request().Context(), salesapp.GenerateSalesOrderDocumentCommand{Actor: support.ActorOf(c), OrderID: ids[0]})
				err = e
				docID = v.Document.ID
				version = v.Document.VersionNo
			}
		}
		if err != nil {
			return echo.NewHTTPError(400, err.Error())
		}
		return c.JSON(200, map[string]any{"id": docID, "version_no": version})
	}
	query := ""
	if combined {
		parts := []string{}
		for _, id := range ids {
			parts = append(parts, strconv.FormatInt(id, 10))
		}
		query = "?order_ids=" + strings.Join(parts, ",")
	}
	docs := []map[string]any{}
	images := []map[string]any{}
	meta := func(id int64, version int, date string, latest bool, image bool) map[string]any {
		ext := "pdf"
		if image {
			ext = "png"
		}
		return map[string]any{"id": id, "version_no": version, "order_no": strings.Join(names, ", "), "created_at": date, "is_latest": latest, "download_url": fmt.Sprintf("%s/orders/%s/sales-files/%d.%s%s", accountPrefix, c.Param("id"), id, ext, query)}
	}
	if combined {
		ds, e := h.sales.ListCombinedSalesOrderDocuments(c.Request().Context(), ids)
		if e != nil {
			return echo.NewHTTPError(400, e.Error())
		}
		for _, d := range ds {
			docs = append(docs, meta(d.ID, d.VersionNo, d.CreatedAt, d.IsLatest, false))
		}
		ims, e := h.sales.ListCombinedSalesOrderImageDocuments(c.Request().Context(), ids)
		if e != nil {
			return echo.NewHTTPError(400, e.Error())
		}
		for _, d := range ims {
			images = append(images, meta(d.ID, d.VersionNo, d.CreatedAt, d.IsLatest, true))
		}
	} else {
		ds, e := h.sales.ListSalesOrderDocuments(c.Request().Context(), ids[0])
		if e != nil {
			return echo.NewHTTPError(400, e.Error())
		}
		for _, d := range ds {
			docs = append(docs, meta(d.ID, d.VersionNo, d.CreatedAt, d.IsLatest, false))
		}
		ims, e := h.sales.ListSalesOrderImageDocuments(c.Request().Context(), ids[0])
		if e != nil {
			return echo.NewHTTPError(400, e.Error())
		}
		for _, d := range ims {
			images = append(images, meta(d.ID, d.VersionNo, d.CreatedAt, d.IsLatest, true))
		}
	}
	if raw := c.Param("file"); raw != "" {
		parts := strings.Split(raw, ".")
		fileID, _ := strconv.ParseInt(parts[0], 10, 64)
		set := docs
		if isImage {
			set = images
		}
		allowed := false
		for _, d := range set {
			if d["id"] == fileID {
				allowed = true
			}
		}
		if !allowed || len(parts) != 2 || (parts[1] != "pdf" && parts[1] != "png") {
			return echo.NewHTTPError(404, "文件不存在或不属于所选订单")
		}
		c.SetParamNames("id", "doc_id", "image_id")
		c.SetParamValues(strconv.FormatInt(ids[0], 10), strconv.FormatInt(fileID, 10), strconv.FormatInt(fileID, 10))
		if combined {
			if isImage {
				return multi.downloadSalesOrderImage(c)
			}
			return multi.downloadSalesOrder(c)
		}
		if isImage {
			return single.downloadImage(c)
		}
		return single.download(c)
	}
	return c.JSON(200, map[string]any{"rows": docs, "image_rows": images, "order": map[string]any{"customer": map[string]any{"id": customerID, "name": customerName}}})
}
