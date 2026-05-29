package customer

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	support "orderapp/internal/interfaces/http/support"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	customerapp "orderapp/internal/application/customer"

	"github.com/labstack/echo/v4"
)

func registerCustomerRoutes(e *echo.Echo, deps Dependencies) {
	h := customerHandler{
		assetDir: deps.AssetDir,
		customer: deps.Customer,
	}

	// Customers
	e.GET("/customers", h.index)
	e.GET("/api/customers", h.indexAPI)
	e.POST("/api/customers", h.createAPI)
	e.POST("/api/customers/customer-types", h.createCustomerTypeAPI)
	e.POST("/api/customers/order-types", h.createOrderTypeAPI)
	e.GET("/api/customers/:id", h.detailAPI)
	e.PUT("/api/customers/:id", h.updateAPI)
	e.GET("/customers/new", h.new)
	e.GET("/customers/:id", h.edit)
	// prefs for order auto-fill
	e.GET("/customers/:id/prefs", h.prefs)
	// customer assets upload
	e.POST("/customers/:id/assets/upload", h.uploadAsset)
	e.POST("/customers/:id/assets/delete", h.deleteAsset)
	e.GET("/assets/customer_assets/:id", func(c echo.Context) error { return h.asset(c, false) })
	e.HEAD("/assets/customer_assets/:id", func(c echo.Context) error { return h.asset(c, true) })

	// inline update (list)
	e.POST("/customers/:id/inline", h.inlineUpdate)
	// delete (soft): active=false
	e.POST("/customers/:id/delete", h.delete)

}

type customerHandler struct {
	assetDir string
	customer *customerapp.Service
}

const maxCustomerAssetUploadBytes = 8 << 20

type customerUpsertAPIRequest struct {
	Name                  string `json:"name"`
	RawName               string `json:"raw_name"`
	CustomerType          string `json:"customer_type"`
	CompanyName           string `json:"company_name"`
	CompanyAddress        string `json:"company_address"`
	CompanyPhone          string `json:"company_phone"`
	Contact               string `json:"contact"`
	Phone                 string `json:"phone"`
	Address               string `json:"address"`
	DefaultSourceID       *int64 `json:"default_source_id"`
	DefaultOrderTypeID    *int64 `json:"default_order_type_id"`
	ResponsibleEmployeeID *int64 `json:"responsible_employee_id"`
	PortalEnabled         *bool  `json:"portal_enabled"`
	CapabilityTemplateKey string `json:"capability_template_key"`
	Active                *bool  `json:"active"`
}

type customerAPIModel struct {
	ID                      int64  `json:"id"`
	Name                    string `json:"name"`
	RawName                 string `json:"raw_name"`
	CustomerType            string `json:"customer_type"`
	CompanyName             string `json:"company_name"`
	CompanyAddress          string `json:"company_address"`
	CompanyPhone            string `json:"company_phone"`
	Contact                 string `json:"contact"`
	Phone                   string `json:"phone"`
	Address                 string `json:"address"`
	DefaultSourceID         *int64 `json:"default_source_id"`
	DefaultOrderTypeID      *int64 `json:"default_order_type_id"`
	ResponsibleEmployeeID   *int64 `json:"responsible_employee_id"`
	ResponsibleEmployeeName string `json:"responsible_employee_name"`
	PortalEnabled           bool   `json:"portal_enabled"`
	CapabilityTemplateKey   string `json:"capability_template_key"`
	Active                  bool   `json:"active"`
}

type customerDashboardAPI struct {
	TotalOrders     int `json:"total_orders"`
	UnpaidOrders    int `json:"unpaid_orders"`
	UnshippedOrders int `json:"unshipped_orders"`
	InProduction    int `json:"in_production"`
	InShipping      int `json:"in_shipping"`
	Completed       int `json:"completed"`
}

type customerAssetAPI struct {
	ID          int64  `json:"id"`
	CustomerID  int64  `json:"customer_id"`
	Kind        string `json:"kind"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	Sha256      string `json:"sha256"`
	CreatedAt   string `json:"created_at"`
	URL         string `json:"url"`
}

type customerEditorAPIResponse struct {
	Customer            customerAPIModel                 `json:"customer"`
	Sources             []apiOption                      `json:"sources"`
	OrderTypes          []apiOption                      `json:"order_types"`
	Employees           []apiOption                      `json:"employees"`
	CustomerTypeOptions []customerapp.CustomerTypeOption `json:"customer_type_options"`
	Assets              []customerAssetAPI               `json:"assets"`
	Dashboard           customerDashboardAPI             `json:"dashboard"`
}

func (h customerHandler) index(c echo.Context) error {
	return support.VueShellRedirect(c, "customers")
}

func (h customerHandler) indexAPI(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	customerType := strings.TrimSpace(c.QueryParam("customer_type"))
	active := parseTriStateBool(c.QueryParam("active"))
	limit := support.IntParam(c, "limit", 10)
	if limit <= 0 {
		limit = 10
	}
	if limit > 200 {
		limit = 200
	}
	offset := support.IntParam(c, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	if page := support.IntParam(c, "page", 0); page > 0 {
		offset = (page - 1) * limit
	}
	result, err := h.customer.List(c.Request().Context(), customerapp.ListQuery{
		Query:         q,
		CustomerType:  customerType,
		Active:        active,
		SortBy:        c.QueryParam("sort_by"),
		SortDirection: c.QueryParam("sort_direction"),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"rows":                  result.Rows,
		"sources":               apiOptions(result.Sources),
		"order_types":           apiOptions(result.OrderTypes),
		"employees":             apiOptions(result.Employees),
		"customer_type_options": result.CustomerTypeOptions,
		"page":                  (offset / limit) + 1,
		"limit":                 limit,
		"offset":                offset,
		"total":                 result.Total,
		"total_pages":           pageCount(result.Total, limit),
		"customer_type":         customerType,
		"active":                formatActiveFilter(active),
		"sort_by":               c.QueryParam("sort_by"),
		"sort_direction":        c.QueryParam("sort_direction"),
		"has_prev":              offset > 0,
		"has_next":              result.HasNext,
	})
}

func (h customerHandler) createCustomerTypeAPI(c echo.Context) error {
	var req struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	option, err := h.customer.CreateCustomerTypeOption(c.Request().Context(), support.ActorOf(c), customerapp.CreateCustomerTypeCommand{
		Label: req.Label,
		Value: req.Value,
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, option)
}

func (h customerHandler) createOrderTypeAPI(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	option, err := h.customer.CreateOrderTypeOption(c.Request().Context(), support.ActorOf(c), customerapp.CreateOrderTypeCommand{Name: req.Name})
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, apiOption{ID: option.ID, Name: option.Name})
}

func pageCount(total, limit int) int {
	if limit <= 0 {
		limit = 10
	}
	if total <= 0 {
		return 1
	}
	return (total + limit - 1) / limit
}

func parseTriStateBool(value string) *bool {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "1", "true", "yes", "on", "enabled", "active", "y":
		b := true
		return &b
	case "0", "false", "no", "off", "disabled", "inactive", "n":
		b := false
		return &b
	default:
		return nil
	}
}

func formatActiveFilter(active *bool) string {
	if active == nil {
		return ""
	}
	if *active {
		return "true"
	}
	return "false"
}

func (h customerHandler) detailAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	payload, err := h.editorPayload(c, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if payload == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, payload)
}

func (h customerHandler) createAPI(c echo.Context) error {
	var req customerUpsertAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	id, err := h.customer.Upsert(c.Request().Context(), support.ActorOf(c), nil, req.toCommand())
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	payload, err := h.editorPayload(c, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, payload)
}

func (h customerHandler) updateAPI(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid id"})
	}
	var req customerUpsertAPIRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad request"})
	}
	if _, err := h.customer.Upsert(c.Request().Context(), support.ActorOf(c), &id, req.toCommand()); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	}
	payload, err := h.editorPayload(c, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
	if payload == nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
	}
	return c.JSON(http.StatusOK, payload)
}

func (h customerHandler) editorPayload(c echo.Context, id int64) (*customerEditorAPIResponse, error) {
	data, err := h.customer.Editor(c.Request().Context(), id)
	if err != nil || data == nil {
		return nil, err
	}
	payload := customerEditorAPIResponse{
		Customer:            customerAPIModelFromEdit(&data.Customer),
		Sources:             apiOptions(data.Sources),
		OrderTypes:          apiOptions(data.OrderTypes),
		Employees:           apiOptions(data.Employees),
		CustomerTypeOptions: data.CustomerTypeOptions,
		Assets:              customerAssetsAPI(data.Assets),
		Dashboard:           customerDashboardAPIFromData(data.Dashboard),
	}
	return &payload, nil
}

func (req customerUpsertAPIRequest) toFormRequest() CustomerUpsertRequest {
	active := "on"
	if req.Active != nil && !*req.Active {
		active = ""
	}
	return CustomerUpsertRequest{
		Name:                  req.Name,
		RawName:               req.RawName,
		CustomerType:          req.CustomerType,
		CompanyName:           req.CompanyName,
		CompanyAddress:        req.CompanyAddress,
		CompanyPhone:          req.CompanyPhone,
		Contact:               req.Contact,
		Phone:                 req.Phone,
		Address:               req.Address,
		DefaultSourceID:       optionalIntString(req.DefaultSourceID),
		DefaultOrderTypeID:    optionalIntString(req.DefaultOrderTypeID),
		ResponsibleEmployeeID: optionalIntString(req.ResponsibleEmployeeID),
		Active:                active,
	}
}

func (req customerUpsertAPIRequest) toCommand() customerapp.UpsertCommand {
	cmd := customerUpsertCommandFromRequest(req.toFormRequest())
	cmd.PortalEnabled = req.PortalEnabled
	return cmd
}

func optionalIntString(v *int64) string {
	if v == nil || *v <= 0 {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func parseOptionalCustomerInt64(v string) *int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n <= 0 {
		return nil
	}
	return &n
}

func customerAPIModelFromEdit(data *CustomerEditData) customerAPIModel {
	return customerAPIModel{
		ID:                      data.ID,
		Name:                    data.Name,
		RawName:                 data.RawName,
		CustomerType:            customerapp.NormalizeCustomerType(data.CustomerType),
		CompanyName:             data.CompanyName,
		CompanyAddress:          data.CompanyAddress,
		CompanyPhone:            data.CompanyPhone,
		Contact:                 data.Contact,
		Phone:                   data.Phone,
		Address:                 data.Address,
		DefaultSourceID:         parseOptionalCustomerInt64(data.DefaultSourceID),
		DefaultOrderTypeID:      parseOptionalCustomerInt64(data.DefaultOrderTypeID),
		ResponsibleEmployeeID:   parseOptionalCustomerInt64(data.ResponsibleEmployeeID),
		ResponsibleEmployeeName: data.ResponsibleEmployeeName,
		PortalEnabled:           data.PortalEnabled,
		CapabilityTemplateKey:   data.CapabilityTemplateKey,
		Active:                  data.Active,
	}
}

func customerDashboardAPIFromData(data CustomerDashboard) customerDashboardAPI {
	return customerDashboardAPI{
		TotalOrders:     data.TotalOrders,
		UnpaidOrders:    data.UnpaidOrders,
		UnshippedOrders: data.UnshippedOrders,
		InProduction:    data.InProduction,
		InShipping:      data.InShipping,
		Completed:       data.Completed,
	}
}

func customerAssetsAPI(assets []CustomerAsset) []customerAssetAPI {
	out := make([]customerAssetAPI, 0, len(assets))
	for _, asset := range assets {
		out = append(out, customerAssetAPI{
			ID:          asset.ID,
			CustomerID:  asset.CustomerID,
			Kind:        asset.Kind,
			ObjectKey:   asset.ObjectKey,
			ContentType: asset.ContentType,
			Bytes:       asset.Bytes,
			Sha256:      asset.Sha256,
			CreatedAt:   asset.CreatedAt,
			URL:         fmt.Sprintf("/assets/customer_assets/%d?v=%s", asset.ID, url.QueryEscape(asset.Sha256)),
		})
	}
	return out
}

func wantsJSON(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get(echo.HeaderAccept), echo.MIMEApplicationJSON) ||
		strings.Contains(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON)
}

func (h customerHandler) new(c echo.Context) error {
	target := map[string]string{"mode": "new"}
	if strings.TrimSpace(c.QueryParam("from")) == "order" {
		target["from"] = "order"
	}
	return support.VueShellRedirectWith(c, "customers", target)
}

func (h customerHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	return support.VueShellRedirectWith(c, "customers", map[string]string{"edit_id": strconv.FormatInt(id, 10)})
}

func (h customerHandler) prefs(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	p, err := h.customer.Prefs(c.Request().Context(), id)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, p)

}

func (h customerHandler) uploadAsset(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	kind := strings.TrimSpace(c.FormValue("kind"))
	if kind == "" {
		return c.String(http.StatusBadRequest, "kind required")
	}
	fh, err := c.FormFile("file")
	if err != nil {
		log.Printf("asset upload formfile error customer_id=%d kind=%s err=%v", id, kind, err)
		if wantsJSON(c) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "读取文件失败：" + err.Error()})
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape("读取文件失败："+err.Error())))
	}
	log.Printf("asset upload start customer_id=%d kind=%s filename=%s size=%d", id, kind, fh.Filename, fh.Size)
	f, err := fh.Open()
	if err != nil {
		if wantsJSON(c) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "bad file"})
		}
		return c.String(http.StatusBadRequest, "bad file")
	}
	defer func() { _ = f.Close() }()

	// sniff content type
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	ct := http.DetectContentType(head[:n])
	// reset reader by chaining
	r := io.MultiReader(bytes.NewReader(head[:n]), f)

	res, err := h.customer.SaveAsset(c.Request().Context(), customerapp.SaveAssetCommand{
		CustomerID:  id,
		Kind:        kind,
		Reader:      r,
		ContentType: ct,
		Filename:    fh.Filename,
		MaxBytes:    maxCustomerAssetUploadBytes,
		Actor:       support.ActorOf(c),
	})
	if err != nil {
		log.Printf("asset upload save error customer_id=%d kind=%s ct=%s err=%v", id, kind, ct, err)
		if wantsJSON(c) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape(err.Error())))
	}
	log.Printf("asset upload ok customer_id=%d kind=%s obj=%s bytes=%d", id, kind, res.ObjectKey, res.Bytes)
	if wantsJSON(c) {
		return c.JSON(http.StatusOK, map[string]any{"customer_id": res.CustomerID, "object_key": res.ObjectKey, "bytes": res.Bytes, "sha256": res.SHA256})
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?ok=1", id))

}

func (h customerHandler) deleteAsset(c echo.Context) error {
	assetID, err := strconv.ParseInt(strings.TrimSpace(c.FormValue("asset_id")), 10, 64)
	if err != nil || assetID <= 0 {
		return c.String(http.StatusBadRequest, "asset_id required")
	}
	res, err := h.customer.DeleteAsset(c.Request().Context(), support.ActorOf(c), assetID)
	if err != nil {
		if wantsJSON(c) {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return c.String(http.StatusBadRequest, err.Error())
	}
	if wantsJSON(c) {
		return c.JSON(http.StatusOK, map[string]any{"customer_id": res.CustomerID, "object_key": res.ObjectKey})
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", res.CustomerID))

}

func (h customerHandler) inlineUpdate(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req CustomerInlineReq
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	if err := h.customer.InlineUpdate(c.Request().Context(), support.ActorOf(c), id, customerInlineCommandFromRequest(req)); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.String(http.StatusOK, "ok")

}

func (h customerHandler) delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	if err := h.customer.Delete(c.Request().Context(), support.ActorOf(c), id); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.String(http.StatusOK, "ok")

}

func (h customerHandler) asset(c echo.Context, headOnly bool) error {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var obj string
	var ct string
	asset, err := h.customer.AssetObject(c.Request().Context(), assetID)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}
	obj = asset.ObjectKey
	ct = asset.ContentType
	obj = strings.TrimPrefix(obj, "/")
	path := filepath.Join(h.assetDir, obj)
	st, err := os.Stat(path)
	if err != nil {
		return c.String(http.StatusNotFound, "not found")
	}
	c.Response().Header().Set(echo.HeaderContentType, ct)
	c.Response().Header().Set("Cache-Control", "private, max-age=60")
	c.Response().Header().Set(echo.HeaderContentLength, fmt.Sprintf("%d", st.Size()))
	if headOnly {
		return c.NoContent(http.StatusOK)
	}
	return c.File(path)

}
