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

type deliveryNoteDocumentHandler struct {
	sales *salesapp.Service
}

type deliveryNoteFormRequest struct {
	PostingDate     string `json:"posting_date"`
	SourceWarehouse string `json:"source_warehouse"`
	DeliveryMethod  string `json:"delivery_method"`
	TrackingNo      string `json:"tracking_no"`
	Note            string `json:"note"`
}

func registerDeliveryNoteDocumentRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	h := deliveryNoteDocumentHandler{sales: salesSvc}
	e.GET("/orders/:id/delivery-note", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/vue-shell?view=deliveryNote&order_id="+c.Param("id"))
	})
	e.GET("/api/orders/:id/delivery-notes", h.list)
	e.GET("/api/orders/:id/delivery-note-preview", h.preview)
	e.POST("/api/orders/:id/delivery-note", h.saveForm)
	e.POST("/api/orders/:id/delivery-notes", h.generate)
	e.GET("/orders/:id/delivery-notes/:doc_id.pdf", h.download)
	e.GET("/orders/:id/delivery-note-latest.pdf", h.downloadLatest)
}

func (h deliveryNoteDocumentHandler) list(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	docs, err := h.sales.ListDeliveryNoteDocuments(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	ctx, err := h.sales.LoadDeliveryNoteContext(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	form, err := h.sales.LoadDeliveryNoteForm(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"rows": docs, "order": ctx, "form": form})
}

func (h deliveryNoteDocumentHandler) saveForm(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	var req deliveryNoteFormRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid json"})
	}
	if err := h.sales.SaveDeliveryNoteForm(c.Request().Context(), salesapp.SaveDeliveryNoteFormCommand{
		Actor:           support.ActorOf(c),
		OrderID:         orderID,
		PostingDate:     req.PostingDate,
		SourceWarehouse: req.SourceWarehouse,
		DeliveryMethod:  req.DeliveryMethod,
		TrackingNo:      req.TrackingNo,
		Note:            req.Note,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	form, err := h.sales.LoadDeliveryNoteForm(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, form)
}

func (h deliveryNoteDocumentHandler) preview(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	preview, err := h.sales.PreviewDeliveryNoteDocument(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, preview)
}

func (h deliveryNoteDocumentHandler) generate(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	res, err := h.sales.GenerateDeliveryNoteDocument(c.Request().Context(), salesapp.GenerateDeliveryNoteDocumentCommand{Actor: support.ActorOf(c), OrderID: orderID})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, res.Document)
}

func (h deliveryNoteDocumentHandler) download(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	docID, err := parseDeliveryNoteDocumentID(c.Param("doc_id"), c.Request().URL.Path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid document id"})
	}
	return h.downloadFile(c, orderID, docID, false)
}

func (h deliveryNoteDocumentHandler) downloadLatest(c echo.Context) error {
	orderID, err := parseDeliveryNoteOrderID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return h.downloadFile(c, orderID, 0, true)
}

func (h deliveryNoteDocumentHandler) downloadFile(c echo.Context, orderID, docID int64, latest bool) error {
	file, err := h.sales.LoadDeliveryNoteDocumentFile(c.Request().Context(), orderID, docID, latest)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	return c.File(file.Path)
}

func parseDeliveryNoteDocumentID(rawParam, requestPath string) (int64, error) {
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

func parseDeliveryNoteOrderID(c echo.Context) (int64, error) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return 0, fmt.Errorf("invalid order id")
	}
	return orderID, nil
}
