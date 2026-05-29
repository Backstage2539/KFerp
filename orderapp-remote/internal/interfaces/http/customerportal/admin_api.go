package customerportal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type portalVisibilityRequest struct {
	DisplayName           string                               `json:"display_name"`
	DefaultSenderID       int64                                `json:"default_sender_id"`
	Enabled               *bool                                `json:"enabled"`
	ThemeKey              string                               `json:"theme_key"`
	MiniappEntryMode      string                               `json:"miniapp_entry_mode"`
	CapabilityTemplateKey string                               `json:"capability_template_key"`
	Capabilities          []customerportalapp.CapabilityOption `json:"capabilities"`
}

type capabilityTemplateRequest struct {
	TemplateKey string `json:"template_key"`
}

type portalERPBindingRequest struct {
	EmployeeID int64  `json:"employee_id"`
	Status     string `json:"status"`
}

type saveCapabilityTemplateRequest struct {
	Key               string                               `json:"key"`
	ParentTemplateKey string                               `json:"parent_template_key"`
	Label             string                               `json:"label"`
	Description       string                               `json:"description"`
	ThemeKey          string                               `json:"theme_key"`
	MiniappEntryMode  string                               `json:"miniapp_entry_mode"`
	ERPRoleCodes      []string                             `json:"erp_role_codes"`
	ERPPermissions    []string                             `json:"erp_permissions"`
	ERPViewKeys       []string                             `json:"erp_view_keys"`
	Capabilities      []customerportalapp.CapabilityOption `json:"capabilities"`
	Active            *bool                                `json:"active"`
	SortOrder         int                                  `json:"sort_order"`
}

type copyCapabilityTemplateRequest struct {
	NewKey string `json:"new_key"`
	Label  string `json:"label"`
}

type mallProductRequest struct {
	ID          int64   `json:"id"`
	ProductID   int64   `json:"product_id"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Description string  `json:"description"`
	ImageURL    string  `json:"image_url"`
	SpecG       int64   `json:"spec_g"`
	UnitPrice   float64 `json:"unit_price"`
	TemplateKey string  `json:"template_key"`
	Status      string  `json:"status"`
	SortOrder   int     `json:"sort_order"`
}

const maxMallProductImageUploadBytes = 8 << 20

type mallProductImageUpload struct {
	data     []byte
	filename string
}

func registerAdminAPI(e *echo.Echo, svc Service, assetDirs ...string) {
	assetDir := "/app/data/assets"
	if len(assetDirs) > 0 && strings.TrimSpace(assetDirs[0]) != "" {
		assetDir = strings.TrimSpace(assetDirs[0])
	}
	e.GET("/api/customer-portal/admin/capability-templates", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		rows, err := svc.ListCapabilityTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return c.JSON(http.StatusOK, map[string]any{"templates": rows})
	})

	e.PUT("/api/customer-portal/admin/capability-templates/:key", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		key := strings.TrimSpace(c.Param("key"))
		var req saveCapabilityTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		active := true
		activeSet := false
		if req.Active != nil {
			active = *req.Active
			activeSet = true
		}
		row, err := svc.SaveCapabilityTemplate(c.Request().Context(), customerportalapp.SaveCapabilityTemplateCommand{
			Template: customerportalapp.CapabilityTemplate{
				Key:               key,
				ParentTemplateKey: req.ParentTemplateKey,
				Label:             req.Label,
				Description:       req.Description,
				ThemeKey:          req.ThemeKey,
				MiniappEntryMode:  req.MiniappEntryMode,
				ERPRoleCodes:      req.ERPRoleCodes,
				ERPPermissions:    req.ERPPermissions,
				ERPViewKeys:       req.ERPViewKeys,
				Capabilities:      req.Capabilities,
				Active:            active,
				SortOrder:         req.SortOrder,
			},
			UpdatedBy: support.ActorOf(c),
			ActiveSet: activeSet,
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/customer-portal/admin/capability-templates/:key/copy", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		var req copyCapabilityTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		row, err := svc.CopyCapabilityTemplate(c.Request().Context(), customerportalapp.CopyCapabilityTemplateCommand{
			SourceKey: strings.TrimSpace(c.Param("key")),
			NewKey:    req.NewKey,
			Label:     req.Label,
			UpdatedBy: support.ActorOf(c),
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/api/customer-portal/admin/customers", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		rows, err := svc.ListPortalAdminCustomers(c.Request().Context(), customerportalapp.PortalAdminCustomerQuery{
			Query: c.QueryParam("q"),
			Limit: support.IntParam(c, "limit", 20),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.GET("/api/customer-portal/admin/customers/:id", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		detail, err := svc.PortalAdminDetail(c.Request().Context(), id)
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.PUT("/api/customer-portal/admin/customers/:id/visibility", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		var req portalVisibilityRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		detail, err := svc.UpdatePortalVisibility(c.Request().Context(), customerportalapp.UpdatePortalVisibilityCommand{
			CustomerID:            id,
			DisplayName:           req.DisplayName,
			DefaultSenderID:       req.DefaultSenderID,
			Enabled:               enabled,
			ThemeKey:              req.ThemeKey,
			MiniappEntryMode:      req.MiniappEntryMode,
			CapabilityTemplateKey: req.CapabilityTemplateKey,
			Capabilities:          req.Capabilities,
			UpdatedBy:             support.ActorOf(c),
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.PUT("/api/customer-portal/admin/customers/:id/erp-binding", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		var req portalERPBindingRequest
		if err := c.Bind(&req); err != nil || req.EmployeeID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "employee required"})
		}
		detail, err := svc.UpsertPortalERPBinding(c.Request().Context(), customerportalapp.UpsertPortalERPBindingCommand{
			CustomerID: id,
			EmployeeID: req.EmployeeID,
			Status:     req.Status,
			UpdatedBy:  support.ActorOf(c),
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.POST("/api/customer-portal/admin/customers/:id/capability-template", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		var req capabilityTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		detail, err := svc.ApplyCapabilityTemplate(c.Request().Context(), customerportalapp.ApplyCapabilityTemplateCommand{
			CustomerID:  id,
			TemplateKey: req.TemplateKey,
			UpdatedBy:   support.ActorOf(c),
		})
		if err != nil {
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.GET("/api/customer-portal/admin/mall-products", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		rows, options, err := svc.ListMallProducts(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows, "product_options": options})
	})

	e.POST("/api/customer-portal/admin/mall-products", func(c echo.Context) error {
		return saveMallProduct(c, svc, 0)
	})

	e.PUT("/api/customer-portal/admin/mall-products/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "mall product required"})
		}
		return saveMallProduct(c, svc, id)
	})

	e.POST("/api/customer-portal/admin/mall-products/:id/image", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "mall product required"})
		}
		upload, err := readUploadedMallProductImage(c)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := ensureMallProductImageUploadTarget(c.Request().Context(), svc, id); err != nil {
			return portalAdminError(c, err)
		}
		imageURL, assetPath, err := saveMallProductImageData(assetDir, id, upload.filename, upload.data)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		row, err := svc.UpdateMallProductImage(c.Request().Context(), customerportalapp.UpdateMallProductImageCommand{
			ID:       id,
			ImageURL: imageURL,
			Actor:    support.ActorOf(c),
		})
		if err != nil {
			cleanupMallProductImageAsset(assetPath, assetDir)
			return portalAdminError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	})

	e.GET("/assets/mall_products/*", func(c echo.Context) error {
		return serveMallProductAsset(c, assetDir)
	})
	e.HEAD("/assets/mall_products/*", func(c echo.Context) error {
		return serveMallProductAsset(c, assetDir)
	})
}

func saveMallProduct(c echo.Context, svc Service, id int64) error {
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	var req mallProductRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if id > 0 {
		req.ID = id
	}
	row, err := svc.SaveMallProduct(c.Request().Context(), customerportalapp.SaveMallProductCommand{
		ID:          req.ID,
		ProductID:   req.ProductID,
		Title:       req.Title,
		Subtitle:    req.Subtitle,
		Description: req.Description,
		ImageURL:    req.ImageURL,
		SpecG:       req.SpecG,
		UnitPrice:   req.UnitPrice,
		TemplateKey: req.TemplateKey,
		Status:      req.Status,
		SortOrder:   req.SortOrder,
		Actor:       support.ActorOf(c),
	})
	if err != nil {
		return portalAdminError(c, err)
	}
	return c.JSON(http.StatusOK, row)
}

func portalAdminError(c echo.Context, err error) error {
	if errors.Is(err, customerportalapp.ErrPortalCustomerNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "customer not found"})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable) ||
		err.Error() == customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable.Error() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if errors.Is(err, customerportalapp.ErrCapabilityTemplateInvalid) ||
		err.Error() == customerportalapp.ErrCapabilityTemplateInvalid.Error() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if isMiniValidationError(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

func readUploadedMallProductImage(c echo.Context) (mallProductImageUpload, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return mallProductImageUpload{}, fmt.Errorf("file required")
	}
	src, err := file.Open()
	if err != nil {
		return mallProductImageUpload{}, err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxMallProductImageUploadBytes+1))
	if err != nil {
		return mallProductImageUpload{}, err
	}
	if len(data) == 0 {
		return mallProductImageUpload{}, fmt.Errorf("empty file")
	}
	if len(data) > maxMallProductImageUploadBytes {
		return mallProductImageUpload{}, fmt.Errorf("image file too large")
	}
	if !isAllowedMallProductImage(data) {
		return mallProductImageUpload{}, fmt.Errorf("image file required")
	}
	return mallProductImageUpload{data: data, filename: mallAssetFilename(file.Filename)}, nil
}

func ensureMallProductImageUploadTarget(ctx context.Context, svc Service, mallProductID int64) error {
	rows, _, err := svc.ListMallProducts(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID == mallProductID {
			return nil
		}
	}
	return fmt.Errorf("mall product unavailable")
}

func saveMallProductImageData(assetDir string, mallProductID int64, filename string, data []byte) (string, string, error) {
	objectKey := filepath.ToSlash(filepath.Join("mall_products", strconv.FormatInt(mallProductID, 10), fmt.Sprintf("%d-%s", time.Now().UnixNano(), filename)))
	path := filepath.Join(assetDir, objectKey)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", "", err
	}
	return "/" + filepath.ToSlash(filepath.Join("assets", objectKey)), path, nil
}

func cleanupMallProductImageAsset(path string, assetDir string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	if err := os.Remove(path); err != nil {
		return
	}
	productDir := filepath.Dir(path)
	_ = os.Remove(productDir)
	mallProductsDir := filepath.Join(assetDir, "mall_products")
	_ = os.Remove(mallProductsDir)
}

func isAllowedMallProductImage(data []byte) bool {
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func mallAssetFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "" || filename == "." {
		return "mall-product"
	}
	return filename
}

func serveMallProductAsset(c echo.Context, assetDir string) error {
	rel := strings.TrimPrefix(c.Param("*"), "/")
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return c.NoContent(http.StatusNotFound)
	}
	path := filepath.Join(assetDir, "mall_products", clean)
	f, err := os.Open(path)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		return c.NoContent(http.StatusNotFound)
	}
	if contentType := detectMallProductAssetContentType(f); contentType != "" {
		c.Response().Header().Set(echo.HeaderContentType, contentType)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	http.ServeContent(c.Response(), c.Request(), stat.Name(), stat.ModTime(), f)
	return nil
}

func detectMallProductAssetContentType(f *os.File) string {
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return ""
	}
	return http.DetectContentType(buf[:n])
}
