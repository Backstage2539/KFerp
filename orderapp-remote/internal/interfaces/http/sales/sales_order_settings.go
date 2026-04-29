package sales

import (
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

type salesOrderSettingsHandler struct {
	sales    *salesapp.Service
	assetDir string
}

type salesOrderSettingsRequest struct {
	CompanyName string `json:"company_name"`
	Note        string `json:"note"`
	PaymentText string `json:"payment_text"`
}

type salesOrderPaymentCodeRequest struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	AssetID     int64  `json:"asset_id"`
	Sort        int    `json:"sort"`
	Active      bool   `json:"active"`
}

func registerSalesOrderSettingsRoutes(e *echo.Echo, salesSvc *salesapp.Service, assetDirs ...string) {
	assetDir := "/app/data/assets"
	if len(assetDirs) > 0 && strings.TrimSpace(assetDirs[0]) != "" {
		assetDir = strings.TrimSpace(assetDirs[0])
	}
	h := salesOrderSettingsHandler{sales: salesSvc, assetDir: assetDir}
	e.GET("/settings/sales-order", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, "/vue-shell?view=salesOrderSettings")
	})
	e.GET("/api/settings/sales-order", h.get)
	e.POST("/api/settings/sales-order", h.save)
	e.POST("/api/settings/sales-order/payment-codes", h.uploadPaymentCode)
	e.PUT("/api/settings/sales-order/payment-codes/:id", h.updatePaymentCode)
	e.DELETE("/api/settings/sales-order/payment-codes/:id", h.deletePaymentCode)
	e.POST("/api/settings/sales-order/seal", h.uploadSeal)
}

func (h salesOrderSettingsHandler) get(c echo.Context) error {
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, settings)
}

func (h salesOrderSettingsHandler) save(c echo.Context) error {
	var req salesOrderSettingsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if err := h.sales.SaveSalesOrderSettings(c.Request().Context(), salesapp.SaveSalesOrderSettingsCommand{
		Actor:       support.ActorOf(c),
		CompanyName: req.CompanyName,
		Note:        req.Note,
		PaymentText: req.PaymentText,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, settings)
}

func (h salesOrderSettingsHandler) uploadPaymentCode(c echo.Context) error {
	asset, err := h.saveUploadedSalesOrderAsset(c, "payment_code")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	sort, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("sort")))
	code, err := h.sales.SaveSalesOrderPaymentCode(c.Request().Context(), salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor:       support.ActorOf(c),
		Label:       c.FormValue("label"),
		Description: c.FormValue("description"),
		AssetID:     asset.ID,
		Sort:        sort,
		Active:      true,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"asset": asset, "payment_code": code})
}

func (h salesOrderSettingsHandler) updatePaymentCode(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req salesOrderPaymentCodeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	code, err := h.sales.SaveSalesOrderPaymentCode(c.Request().Context(), salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor: support.ActorOf(c), ID: id, Label: req.Label, Description: req.Description, AssetID: req.AssetID, Sort: req.Sort, Active: req.Active,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, code)
}

func (h salesOrderSettingsHandler) deletePaymentCode(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	if err := h.sales.DeleteSalesOrderPaymentCode(c.Request().Context(), id, support.ActorOf(c)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h salesOrderSettingsHandler) uploadSeal(c echo.Context) error {
	asset, err := h.saveUploadedSalesOrderAsset(c, "seal")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	if err := h.sales.SetSalesOrderSealAsset(c.Request().Context(), asset.ID, support.ActorOf(c)); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"asset": asset})
}

func (h salesOrderSettingsHandler) saveUploadedSalesOrderAsset(c echo.Context, kind string) (salesapp.SalesOrderAsset, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("file required")
	}
	src, err := file.Open()
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, 8<<20))
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if len(data) == 0 {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("empty file")
	}
	sum := sha256.Sum256(data)
	filename := filepath.Base(file.Filename)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", kind, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(h.assetDir, objectKey)), 0755); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if err := os.WriteFile(filepath.Join(h.assetDir, objectKey), data, 0644); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	return h.sales.SaveSalesOrderAsset(c.Request().Context(), salesapp.SaveSalesOrderAssetCommand{
		Actor:       support.ActorOf(c),
		Kind:        kind,
		Filename:    filename,
		ContentType: file.Header.Get("Content-Type"),
		Bytes:       int64(len(data)),
		SHA256:      hex.EncodeToString(sum[:]),
		ObjectKey:   objectKey,
	})
}
