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

type combinedDocumentHandler struct {
	sales *salesapp.Service
}

type combinedDocumentRequest struct {
	OrderIDs []int64 `json:"order_ids"`
}

func registerCombinedDocumentRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := combinedDocumentHandler{sales: salesSvc}
	e.GET("/api/orders/combined/sales-order-preview", h.salesOrderPreview)
	e.GET("/api/orders/combined/sales-order-preview.pdf", h.salesOrderPreviewPDF)
	e.POST("/api/orders/combined/sales-orders", h.generateSalesOrder)
	e.GET("/orders/combined/sales-orders/:doc_id.pdf", h.downloadSalesOrder)
	e.GET("/api/orders/combined/delivery-note-preview", h.deliveryNotePreview)
	e.GET("/api/orders/combined/delivery-note-preview.pdf", h.deliveryNotePreviewPDF)
	e.POST("/api/orders/combined/delivery-notes", h.generateDeliveryNote)
	e.GET("/orders/combined/delivery-notes/:doc_id.pdf", h.downloadDeliveryNote)
}

func (h combinedDocumentHandler) salesOrderPreview(c echo.Context) error {
	ids, err := parseCombinedDocumentOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewCombinedSalesOrderDocument(c.Request().Context(), ids)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (h combinedDocumentHandler) salesOrderPreviewPDF(c echo.Context) error {
	ids, err := parseCombinedDocumentOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewCombinedSalesOrderPDF(c.Request().Context(), ids)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, preview.Filename))
	return c.Blob(http.StatusOK, "application/pdf", preview.Data)
}

func (h combinedDocumentHandler) generateSalesOrder(c echo.Context) error {
	ids, err := parseCombinedDocumentBodyOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	res, err := h.sales.GenerateCombinedSalesOrderDocument(c.Request().Context(), salesapp.CombinedDocumentCommand{Actor: support.ActorOf(c), OrderIDs: ids})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, res.Document)
}

func (h combinedDocumentHandler) downloadSalesOrder(c echo.Context) error {
	docID, err := parseCombinedDocumentID(c.Param("doc_id"), c.Request().URL.Path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid document id"})
	}
	file, err := h.sales.LoadCombinedSalesOrderDocumentFile(c.Request().Context(), docID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	return c.File(file.Path)
}

func (h combinedDocumentHandler) deliveryNotePreview(c echo.Context) error {
	ids, err := parseCombinedDocumentOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewCombinedDeliveryNoteDocument(c.Request().Context(), ids)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (h combinedDocumentHandler) deliveryNotePreviewPDF(c echo.Context) error {
	ids, err := parseCombinedDocumentOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewCombinedDeliveryNotePDF(c.Request().Context(), ids)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, preview.Filename))
	return c.Blob(http.StatusOK, "application/pdf", preview.Data)
}

func (h combinedDocumentHandler) generateDeliveryNote(c echo.Context) error {
	ids, err := parseCombinedDocumentBodyOrderIDs(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	res, err := h.sales.GenerateCombinedDeliveryNoteDocument(c.Request().Context(), salesapp.CombinedDocumentCommand{Actor: support.ActorOf(c), OrderIDs: ids})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, res.Document)
}

func (h combinedDocumentHandler) downloadDeliveryNote(c echo.Context) error {
	docID, err := parseCombinedDocumentID(c.Param("doc_id"), c.Request().URL.Path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid document id"})
	}
	file, err := h.sales.LoadCombinedDeliveryNoteDocumentFile(c.Request().Context(), docID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	return c.File(file.Path)
}

func parseCombinedDocumentOrderIDs(c echo.Context) ([]int64, error) {
	raw := strings.TrimSpace(c.QueryParam("order_ids"))
	if raw == "" {
		raw = strings.TrimSpace(c.QueryParam("order_id"))
	}
	return parseCombinedDocumentOrderIDList(raw)
}

func parseCombinedDocumentBodyOrderIDs(c echo.Context) ([]int64, error) {
	var req combinedDocumentRequest
	if err := c.Bind(&req); err != nil {
		return nil, fmt.Errorf("invalid json")
	}
	return req.OrderIDs, nil
}

func parseCombinedDocumentOrderIDList(raw string) ([]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("order_ids required")
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\t' || r == ' '
	})
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid order id")
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseCombinedDocumentID(rawParam, requestPath string) (int64, error) {
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
