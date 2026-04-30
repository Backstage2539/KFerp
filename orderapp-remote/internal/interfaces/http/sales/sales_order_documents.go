package sales

import (
	"fmt"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"

	"github.com/labstack/echo/v4"
)

type salesOrderDocumentHandler struct {
	sales *salesapp.Service
}

func registerSalesOrderDocumentRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := salesOrderDocumentHandler{sales: salesSvc}
	e.GET("/orders/:id/sales-order", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/vue-shell?view=salesOrder&order_id="+c.Param("id"))
	})
	e.GET("/api/orders/:id/sales-orders", h.list)
	e.POST("/api/orders/:id/sales-orders", h.generate)
	e.GET("/orders/:id/sales-orders/:doc_id.pdf", h.download)
	e.GET("/orders/:id/sales-order-latest.pdf", h.downloadLatest)
}

func (h salesOrderDocumentHandler) list(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	docs, err := h.sales.ListSalesOrderDocuments(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	ctx, err := h.sales.LoadSalesOrderContext(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": docs, "order": ctx})
}

func (h salesOrderDocumentHandler) generate(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	res, err := h.sales.GenerateSalesOrderDocument(c.Request().Context(), salesapp.GenerateSalesOrderDocumentCommand{Actor: support.ActorOf(c), OrderID: orderID})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, res.Document)
}

func (h salesOrderDocumentHandler) download(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	docID, err := strconv.ParseInt(c.Param("doc_id"), 10, 64)
	if err != nil || docID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid document id"})
	}
	return h.downloadFile(c, orderID, docID, false)
}

func (h salesOrderDocumentHandler) downloadLatest(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return h.downloadFile(c, orderID, 0, true)
}

func (h salesOrderDocumentHandler) downloadFile(c echo.Context, orderID, docID int64, latest bool) error {
	file, err := h.sales.LoadSalesOrderDocumentFile(c.Request().Context(), orderID, docID, latest)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	return c.File(file.Path)
}

func parseSalesOrderID(c echo.Context) (int64, error) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return 0, fmt.Errorf("invalid order id")
	}
	return orderID, nil
}
