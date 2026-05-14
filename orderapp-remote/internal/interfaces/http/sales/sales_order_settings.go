package sales

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
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

const maxSalesOrderSettingsAssetUploadBytes = 8 << 20

type salesOrderSettingsRequest struct {
	CompanyName     string  `json:"company_name"`
	Note            string  `json:"note"`
	PaymentText     string  `json:"payment_text"`
	BankAccountName string  `json:"bank_account_name"`
	BankName        string  `json:"bank_name"`
	BankAccountNo   string  `json:"bank_account_no"`
	SealXMM         float64 `json:"seal_x_mm"`
	SealYMM         float64 `json:"seal_y_mm"`
	SealWidthMM     float64 `json:"seal_width_mm"`
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
		return c.Redirect(http.StatusFound, support.PrefixRelativeLocation(c, "/vue-shell?view=salesOrderSettings"))
	})
	e.GET("/api/settings/sales-order", h.get)
	e.POST("/api/settings/sales-order", h.save)
	e.POST("/api/settings/sales-order/seal-position", h.saveSealPosition)
	e.GET("/api/settings/sales-order/seals", h.listSeals)
	e.POST("/api/settings/sales-order/payment-codes", h.uploadPaymentCode)
	e.PUT("/api/settings/sales-order/payment-codes/:id", h.updatePaymentCode)
	e.DELETE("/api/settings/sales-order/payment-codes/:id", h.deletePaymentCode)
	e.POST("/api/settings/sales-order/seal", h.uploadSeal)
	e.POST("/api/settings/sales-order/seal/remove-background", h.removeSealBackground)
	e.GET("/assets/sales_order_assets/*", h.serveSalesOrderAsset)
	e.HEAD("/assets/sales_order_assets/*", h.serveSalesOrderAsset)
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
		Actor:           support.ActorOf(c),
		CompanyName:     req.CompanyName,
		Note:            req.Note,
		PaymentText:     req.PaymentText,
		BankAccountName: req.BankAccountName,
		BankName:        req.BankName,
		BankAccountNo:   req.BankAccountNo,
		SealXMM:         req.SealXMM,
		SealYMM:         req.SealYMM,
		SealWidthMM:     req.SealWidthMM,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, settings)
}

func (h salesOrderSettingsHandler) saveSealPosition(c echo.Context) error {
	var req salesOrderSettingsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if req.SealWidthMM <= 0 {
		req.SealWidthMM = settings.SealWidthMM
	}
	if err := h.sales.SaveSalesOrderSettings(c.Request().Context(), salesapp.SaveSalesOrderSettingsCommand{
		Actor:           support.ActorOf(c),
		CompanyName:     settings.CompanyName,
		Note:            settings.Note,
		PaymentText:     settings.PaymentText,
		BankAccountName: settings.BankAccountName,
		BankName:        settings.BankName,
		BankAccountNo:   settings.BankAccountNo,
		SealXMM:         req.SealXMM,
		SealYMM:         req.SealYMM,
		SealWidthMM:     req.SealWidthMM,
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	settings, err = h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, settings)
}

func (h salesOrderSettingsHandler) listSeals(c echo.Context) error {
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	rows, err := h.sales.ListSalesOrderSealAssets(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	var currentID int64
	if settings.Seal != nil {
		currentID = settings.Seal.ID
	}
	return c.JSON(http.StatusOK, map[string]any{"current_id": currentID, "rows": rows})
}

func (h salesOrderSettingsHandler) uploadPaymentCode(c echo.Context) error {
	label := strings.TrimSpace(c.FormValue("label"))
	if label == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "label required"})
	}
	asset, err := h.saveUploadedSalesOrderAsset(c, "payment_code")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	sort, _ := strconv.Atoi(strings.TrimSpace(c.FormValue("sort")))
	code, err := h.sales.SaveSalesOrderPaymentCode(c.Request().Context(), salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor:       support.ActorOf(c),
		Label:       label,
		Description: c.FormValue("description"),
		AssetID:     asset.ID,
		Sort:        sort,
		Active:      true,
	})
	if err != nil {
		h.cleanupSavedSalesOrderAsset(c, asset)
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
		h.cleanupSavedSalesOrderAsset(c, asset)
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"asset": asset})
}

func (h salesOrderSettingsHandler) removeSealBackground(c echo.Context) error {
	settings, err := h.sales.LoadSalesOrderSettings(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if settings.Seal == nil || strings.TrimSpace(settings.Seal.ObjectKey) == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "seal required"})
	}
	path, ok := h.storedSalesOrderAssetPath(settings.Seal.ObjectKey)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid seal asset"})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "seal file not found"})
	}
	transparent, err := removeSealImageBackground(data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	filename := transparentSealFilename(settings.Seal.Filename)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", "seal", fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(h.assetDir, objectKey)), 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if err := os.WriteFile(filepath.Join(h.assetDir, objectKey), transparent, 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	sum := sha256.Sum256(transparent)
	asset, err := h.sales.SaveSalesOrderAsset(c.Request().Context(), salesapp.SaveSalesOrderAssetCommand{
		Actor:       support.ActorOf(c),
		Kind:        "seal",
		Filename:    filename,
		ContentType: "image/png",
		Bytes:       int64(len(transparent)),
		SHA256:      hex.EncodeToString(sum[:]),
		ObjectKey:   objectKey,
	})
	if err != nil {
		h.cleanupSalesOrderAssetFile(objectKey)
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	if err := h.sales.SetSalesOrderSealAsset(c.Request().Context(), asset.ID, support.ActorOf(c)); err != nil {
		h.cleanupSavedSalesOrderAsset(c, asset)
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{"asset": asset})
}

func (h salesOrderSettingsHandler) serveSalesOrderAsset(c echo.Context) error {
	rel := strings.TrimPrefix(c.Param("*"), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return c.NoContent(http.StatusNotFound)
	}
	path := filepath.Join(h.assetDir, "sales_order_assets", clean)
	f, err := os.Open(path)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		return c.NoContent(http.StatusNotFound)
	}
	if contentType := detectSalesOrderAssetContentType(f); contentType != "" {
		c.Response().Header().Set(echo.HeaderContentType, contentType)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	http.ServeContent(c.Response(), c.Request(), stat.Name(), stat.ModTime(), f)
	return nil
}

func detectSalesOrderAssetContentType(f *os.File) string {
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}
	return http.DetectContentType(buf[:n])
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
	data, err := io.ReadAll(io.LimitReader(src, maxSalesOrderSettingsAssetUploadBytes+1))
	if err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if len(data) == 0 {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("empty file")
	}
	if len(data) > maxSalesOrderSettingsAssetUploadBytes {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("image file too large")
	}
	filename := filepath.Base(file.Filename)
	contentType := http.DetectContentType(data)
	if kind == "payment_code" && !isAllowedSalesOrderSettingsImage(data) {
		return salesapp.SalesOrderAsset{}, fmt.Errorf("image file required")
	}
	if kind == "seal" {
		normalized, err := removeSealImageBackground(data)
		if err != nil {
			return salesapp.SalesOrderAsset{}, err
		}
		data = normalized
		filename = transparentSealFilename(filename)
		contentType = "image/png"
	}
	sum := sha256.Sum256(data)
	objectKey := filepath.ToSlash(filepath.Join("sales_order_assets", kind, fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(h.assetDir, objectKey)), 0755); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	if err := os.WriteFile(filepath.Join(h.assetDir, objectKey), data, 0644); err != nil {
		return salesapp.SalesOrderAsset{}, err
	}
	asset, err := h.sales.SaveSalesOrderAsset(c.Request().Context(), salesapp.SaveSalesOrderAssetCommand{
		Actor:       support.ActorOf(c),
		Kind:        kind,
		Filename:    filename,
		ContentType: contentType,
		Bytes:       int64(len(data)),
		SHA256:      hex.EncodeToString(sum[:]),
		ObjectKey:   objectKey,
	})
	if err != nil {
		h.cleanupSalesOrderAssetFile(objectKey)
		return salesapp.SalesOrderAsset{}, err
	}
	return asset, nil
}

func isAllowedSalesOrderSettingsImage(data []byte) bool {
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func (h salesOrderSettingsHandler) storedSalesOrderAssetPath(objectKey string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.Join(h.assetDir, clean), true
}

func (h salesOrderSettingsHandler) cleanupSavedSalesOrderAsset(c echo.Context, asset salesapp.SalesOrderAsset) {
	if asset.ID > 0 {
		if err := h.sales.DeleteSalesOrderAsset(c.Request().Context(), asset.ID, support.ActorOf(c)); err != nil {
			return
		}
	}
	h.cleanupSalesOrderAssetFile(asset.ObjectKey)
}

func (h salesOrderSettingsHandler) cleanupSalesOrderAssetFile(objectKey string) {
	path, ok := h.storedSalesOrderAssetPath(objectKey)
	if !ok {
		return
	}
	assetDir := filepath.Clean(h.assetDir)
	path = filepath.Clean(path)
	if err := os.Remove(path); err != nil {
		return
	}
	for dir := filepath.Dir(path); dir != "." && dir != assetDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

func transparentSealFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if base == "" || base == "." {
		base = "seal"
	}
	return base + "-transparent.png"
}

func removeSealImageBackground(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("unsupported seal image")
	}
	out := normalizeSealImage(img)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normalizeSealImage(img image.Image) *image.NRGBA {
	bounds := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	foreground := image.Rectangle{}
	hasForeground := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			r := uint8(r16 >> 8)
			g := uint8(g16 >> 8)
			b := uint8(b16 >> 8)
			a := uint8(a16 >> 8)
			if a > 0 && isLightNeutralSealBackground(r, g, b) {
				a = 0
			}
			dstX := x - bounds.Min.X
			dstY := y - bounds.Min.Y
			out.SetNRGBA(dstX, dstY, color.NRGBA{R: r, G: g, B: b, A: a})
			if a > 0 {
				if !hasForeground {
					foreground = image.Rect(dstX, dstY, dstX+1, dstY+1)
					hasForeground = true
					continue
				}
				if dstX < foreground.Min.X {
					foreground.Min.X = dstX
				}
				if dstY < foreground.Min.Y {
					foreground.Min.Y = dstY
				}
				if dstX+1 > foreground.Max.X {
					foreground.Max.X = dstX + 1
				}
				if dstY+1 > foreground.Max.Y {
					foreground.Max.Y = dstY + 1
				}
			}
		}
	}
	if hasForeground && shouldCropSealPadding(out.Bounds(), foreground) {
		return cropSealImage(out, paddedSealForegroundBounds(out.Bounds(), foreground))
	}
	return out
}

func isLightNeutralSealBackground(r, g, b uint8) bool {
	maxV := max(max(r, g), b)
	minV := min(min(r, g), b)
	return minV >= 238 && maxV-minV <= 24
}

func shouldCropSealPadding(bounds, foreground image.Rectangle) bool {
	if bounds.Empty() || foreground.Empty() {
		return false
	}
	minPadding := max(3, min(bounds.Dx(), bounds.Dy())/20)
	return foreground.Min.X > minPadding ||
		foreground.Min.Y > minPadding ||
		bounds.Max.X-foreground.Max.X > minPadding ||
		bounds.Max.Y-foreground.Max.Y > minPadding
}

func paddedSealForegroundBounds(bounds, foreground image.Rectangle) image.Rectangle {
	padding := max(2, min(foreground.Dx(), foreground.Dy())/20)
	crop := image.Rect(
		max(bounds.Min.X, foreground.Min.X-padding),
		max(bounds.Min.Y, foreground.Min.Y-padding),
		min(bounds.Max.X, foreground.Max.X+padding),
		min(bounds.Max.Y, foreground.Max.Y+padding),
	)
	if crop.Empty() {
		return bounds
	}
	return crop
}

func cropSealImage(src *image.NRGBA, crop image.Rectangle) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	for y := crop.Min.Y; y < crop.Max.Y; y++ {
		for x := crop.Min.X; x < crop.Max.X; x++ {
			dst.SetNRGBA(x-crop.Min.X, y-crop.Min.Y, src.NRGBAAt(x, y))
		}
	}
	return dst
}
