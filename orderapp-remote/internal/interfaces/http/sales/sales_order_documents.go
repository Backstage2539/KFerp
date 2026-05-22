package sales

import (
	"fmt"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"path"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type salesOrderDocumentHandler struct {
	sales *salesapp.Service
}

func registerSalesOrderDocumentRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := salesOrderDocumentHandler{sales: salesSvc}
	e.GET("/orders/:id/sales-order", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=salesOrder&order_id="+c.Param("id")))
	})
	e.GET("/api/orders/:id/sales-orders", h.list)
	e.GET("/api/orders/:id/sales-order-preview", h.preview)
	e.GET("/api/orders/:id/sales-order-preview.pdf", h.previewPDF)
	e.PUT("/api/orders/:id/sales-order-note", h.saveNote)
	e.POST("/api/orders/:id/sales-orders", h.generate)
	e.GET("/api/orders/:id/sales-order-images", h.listImages)
	e.POST("/api/orders/:id/sales-order-images", h.generateImage)
	e.GET("/orders/:id/sales-orders/:doc_id.pdf", h.download)
	e.GET("/orders/:id/sales-order-latest.pdf", h.downloadLatest)
	e.GET("/orders/:id/sales-order-images/:image_id.png", h.downloadImage)
	e.GET("/orders/:id/sales-order-image-latest.png", h.downloadLatestImage)
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
	images, err := h.sales.ListSalesOrderImageDocuments(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": docs, "image_rows": images, "order": ctx})
}

func (h salesOrderDocumentHandler) listImages(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	images, err := h.sales.ListSalesOrderImageDocuments(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	ctx, err := h.sales.LoadSalesOrderContext(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": images, "order": ctx})
}

func (h salesOrderDocumentHandler) preview(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewSalesOrderDocument(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (h salesOrderDocumentHandler) previewPDF(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewSalesOrderPDF(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, preview.Filename))
	return c.Blob(http.StatusOK, "application/pdf", preview.Data)
}

func (h salesOrderDocumentHandler) saveNote(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	if err := h.sales.SaveSalesOrderNote(c.Request().Context(), salesapp.SaveSalesOrderNoteCommand{Actor: support.ActorOf(c), OrderID: orderID, Note: req.Note}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewSalesOrderDocument(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
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

func (h salesOrderDocumentHandler) generateImage(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	res, err := h.sales.GenerateSalesOrderImage(c.Request().Context(), salesapp.GenerateSalesOrderImageCommand{Actor: support.ActorOf(c), OrderID: orderID})
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
	docID, err := parseSalesOrderDocumentID(c.Param("doc_id"), c.Request().URL.Path)
	if err != nil {
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

func (h salesOrderDocumentHandler) downloadImage(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	imageID, err := parseSalesOrderImageID(c.Param("image_id"), c.Request().URL.Path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid image id"})
	}
	return h.downloadImageFile(c, orderID, imageID, false)
}

func (h salesOrderDocumentHandler) downloadLatestImage(c echo.Context) error {
	orderID, err := parseSalesOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return h.downloadImageFile(c, orderID, 0, true)
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

func (h salesOrderDocumentHandler) downloadImageFile(c echo.Context, orderID, imageID int64, latest bool) error {
	file, err := h.sales.LoadSalesOrderImageFile(c.Request().Context(), orderID, imageID, latest)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentType, "image/png")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	return c.File(file.Path)
}

func parseSalesOrderDocumentID(rawParam, requestPath string) (int64, error) {
	raw := strings.TrimSpace(rawParam)
	if raw == "" {
		raw = path.Base(strings.TrimSpace(requestPath))
	}
	raw = strings.TrimSuffix(raw, ".pdf")
	docID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || docID <= 0 {
		return 0, fmt.Errorf("invalid document id")
	}
	return docID, nil
}

func parseSalesOrderImageID(rawParam, requestPath string) (int64, error) {
	raw := strings.TrimSpace(rawParam)
	if raw == "" {
		raw = path.Base(strings.TrimSpace(requestPath))
	}
	raw = strings.TrimSuffix(raw, ".png")
	imageID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || imageID <= 0 {
		return 0, fmt.Errorf("invalid image id")
	}
	return imageID, nil
}

func parseSalesOrderID(c echo.Context) (int64, error) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return 0, fmt.Errorf("invalid order id")
	}
	return orderID, nil
}
