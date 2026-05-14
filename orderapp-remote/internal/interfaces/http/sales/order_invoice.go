package sales

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

const maxOrderInvoiceFileBytes = 12 << 20

type orderInvoiceHandler struct {
	sales    *salesapp.Service
	assetDir string
}

func registerOrderInvoiceRoutes(e *echo.Echo, salesSvc *salesapp.Service, assetDirs ...string) {
	assetDir := "/app/data/assets"
	if len(assetDirs) > 0 && strings.TrimSpace(assetDirs[0]) != "" {
		assetDir = strings.TrimSpace(assetDirs[0])
	}
	h := orderInvoiceHandler{sales: salesSvc, assetDir: assetDir}
	e.GET("/api/orders/:id/invoice", h.get)
	e.POST("/api/orders/:id/invoice-request", h.request)
	e.POST("/api/orders/:id/invoice-file", h.uploadFile)
}

func (h orderInvoiceHandler) get(c echo.Context) error {
	orderID, err := parseOrderInvoiceID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	invoice, err := h.sales.LoadOrderInvoice(c.Request().Context(), orderID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, invoice)
}

func (h orderInvoiceHandler) request(c echo.Context) error {
	orderID, err := parseOrderInvoiceID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	invoice, err := h.sales.RequestOrderInvoice(c.Request().Context(), salesapp.RequestOrderInvoiceCommand{
		Actor:   support.ActorOf(c),
		OrderID: orderID,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, invoice)
}

func (h orderInvoiceHandler) uploadFile(c echo.Context) error {
	orderID, err := parseOrderInvoiceID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	if _, err := h.sales.LoadOrderInvoice(c.Request().Context(), orderID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "order not found"})
	}
	cmd, err := h.saveUploadedOrderInvoiceFile(c, orderID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	invoice, err := h.sales.SaveOrderInvoiceFile(c.Request().Context(), cmd)
	if err != nil {
		h.cleanupUploadedOrderInvoiceFile(cmd.ObjectKey)
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, invoice)
}

func (h orderInvoiceHandler) saveUploadedOrderInvoiceFile(c echo.Context, orderID int64) (salesapp.SaveOrderInvoiceFileCommand, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, fmt.Errorf("file required")
	}
	src, err := file.Open()
	if err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxOrderInvoiceFileBytes+1))
	if err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, err
	}
	if len(data) == 0 {
		return salesapp.SaveOrderInvoiceFileCommand{}, fmt.Errorf("empty file")
	}
	if len(data) > maxOrderInvoiceFileBytes {
		return salesapp.SaveOrderInvoiceFileCommand{}, fmt.Errorf("file too large")
	}
	contentType, err := classifyOrderInvoiceFile(file.Filename, file.Header.Get("Content-Type"), data)
	if err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, err
	}
	filename := cleanOrderInvoiceFilename(file.Filename)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", "order_invoices", fmt.Sprintf("%d", orderID), fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	path := filepath.Join(h.assetDir, objectKey)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return salesapp.SaveOrderInvoiceFileCommand{}, err
	}
	sum := sha256.Sum256(data)
	return salesapp.SaveOrderInvoiceFileCommand{
		Actor:       support.ActorOf(c),
		OrderID:     orderID,
		Filename:    filename,
		ContentType: contentType,
		Bytes:       int64(len(data)),
		SHA256:      hex.EncodeToString(sum[:]),
		ObjectKey:   objectKey,
	}, nil
}

func classifyOrderInvoiceFile(filename, headerType string, data []byte) (string, error) {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("%PDF-")) {
		return "application/pdf", nil
	}
	detected := http.DetectContentType(data)
	if salesapp.IsOrderInvoiceContentTypeAllowed(detected) && strings.HasPrefix(detected, "image/") {
		return detected, nil
	}
	return "", fmt.Errorf("only PDF and image files are allowed")
}

func cleanOrderInvoiceFilename(raw string) string {
	raw = strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	filename := filepath.Base(raw)
	if filename == "" || filename == "." || filename == "/" {
		return "invoice"
	}
	return filename
}

func (h orderInvoiceHandler) cleanupUploadedOrderInvoiceFile(objectKey string) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return
	}
	assetDir := filepath.Clean(h.assetDir)
	path := filepath.Join(assetDir, clean)
	if err := os.Remove(path); err != nil {
		return
	}
	for dir := filepath.Dir(path); dir != "." && dir != assetDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

func parseOrderInvoiceID(c echo.Context) (int64, error) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		return 0, fmt.Errorf("invalid order id")
	}
	return orderID, nil
}
