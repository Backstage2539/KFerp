package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func registerCustomerRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, assetDir string) {
	h := customerHandler{pool: pool, schema: schema, assetDir: assetDir}

	// Customers
	e.GET("/customers", h.index)
	e.GET("/customers/new", h.new)
	e.POST("/customers/new", h.create)
	e.GET("/customers/:id", h.edit)
	// prefs for order auto-fill
	e.GET("/customers/:id/prefs", h.prefs)
	e.POST("/customers/:id", h.update)
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
	pool     *pgxpool.Pool
	schema   string
	assetDir string
}

func (h customerHandler) index(c echo.Context) error {
	data := CustomersPageData{Q: strings.TrimSpace(c.QueryParam("q"))}
	data.Limit = intParam(c, "limit", 10)
	if data.Limit <= 0 || data.Limit > 200 {
		data.Limit = 10
	}
	data.Offset = intParam(c, "offset", 0)
	if data.Offset < 0 {
		data.Offset = 0
	}
	if p := intParam(c, "page", 0); p > 0 {
		data.Offset = (p - 1) * data.Limit
	}
	if data.Limit > 0 {
		data.Page = (data.Offset / data.Limit) + 1
	} else {
		data.Page = 1
	}

	sources, _ := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", h.schema))
	types, _ := fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", h.schema))
	data.Sources = sources
	data.OrderTypes = types

	rows, hasNext, err := fetchCustomers(c.Request().Context(), h.pool, h.schema, data.Q, data.Limit, data.Offset)
	if err != nil {
		data.Error = err.Error()
	} else {
		data.Rows = rows
		data.HasPrev = data.Offset > 0
		data.HasNext = hasNext
		if data.Limit > 0 {
			data.Page = (data.Offset / data.Limit) + 1
		} else {
			data.Page = 1
		}
	}
	return c.Render(http.StatusOK, "customers.html", data)

}

func (h customerHandler) new(c echo.Context) error {
	data := CustomerEditData{Active: true}
	data.Sources, _ = fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", h.schema))
	data.OrderTypes, _ = fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", h.schema))
	// allow returning back to order entry
	if strings.TrimSpace(c.QueryParam("from")) == "order" {
		data.From = "order"
	}
	return c.Render(http.StatusOK, "customer_edit.html", data)

}

func (h customerHandler) create(c echo.Context) error {
	var req CustomerUpsertRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	id, err := upsertCustomer(c.Request().Context(), h.pool, h.schema, actorOf(c), nil, &req)
	if err != nil {
		data := CustomerEditData{Active: true, Name: req.Name, RawName: req.RawName, Contact: req.Contact, Phone: req.Phone, Address: req.Address, Error: err.Error()}
		if strings.TrimSpace(c.QueryParam("from")) == "order" {
			data.From = "order"
		}
		return c.Render(http.StatusOK, "customer_edit.html", data)
	}
	// if created from order entry, go back to order page
	if strings.TrimSpace(c.QueryParam("from")) == "order" {
		return c.Redirect(http.StatusSeeOther, "/app/order")
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", id))

}

func (h customerHandler) edit(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	data, err := fetchCustomerByID(c.Request().Context(), h.pool, h.schema, id)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	if data == nil {
		return c.String(http.StatusNotFound, "not found")
	}
	if msg := strings.TrimSpace(c.QueryParam("err")); msg != "" {
		data.Error = msg
	}
	if strings.TrimSpace(c.QueryParam("ok")) != "" {
		data.Ok = true
	}
	data.Sources, _ = fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", h.schema))
	data.OrderTypes, _ = fetchOptions(c.Request().Context(), h.pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", h.schema))
	if assets, err := fetchCustomerAssets(c.Request().Context(), h.pool, h.schema, id); err == nil {
		data.Assets = assets
	}
	if dash, err := fetchCustomerDashboard(c.Request().Context(), h.pool, h.schema, id); err == nil {
		data.Dash = dash
	}
	return c.Render(http.StatusOK, "customer_edit.html", data)

}

func (h customerHandler) prefs(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	p, err := fetchCustomerPrefs(c.Request().Context(), h.pool, h.schema, id)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, p)

}

func (h customerHandler) update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	var req CustomerUpsertRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	_, err = upsertCustomer(c.Request().Context(), h.pool, h.schema, actorOf(c), &id, &req)
	if err != nil {
		data := CustomerEditData{ID: id, Active: strings.TrimSpace(req.Active) != "", Name: req.Name, RawName: req.RawName, Contact: req.Contact, Phone: req.Phone, Address: req.Address, Error: err.Error()}
		return c.Render(http.StatusOK, "customer_edit.html", data)
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", id))

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
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape("读取文件失败："+err.Error())))
	}
	log.Printf("asset upload start customer_id=%d kind=%s filename=%s size=%d", id, kind, fh.Filename, fh.Size)
	f, err := fh.Open()
	if err != nil {
		return c.String(http.StatusBadRequest, "bad file")
	}
	defer func() { _ = f.Close() }()

	// sniff content type
	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	ct := http.DetectContentType(head[:n])
	// reset reader by chaining
	r := io.MultiReader(bytes.NewReader(head[:n]), f)

	obj, size, sha, err := saveCustomerAssetFile(h.assetDir, id, kind, r, ct, 100*1024*1024, fh.Filename)
	if err != nil {
		log.Printf("asset upload save error customer_id=%d kind=%s ct=%s err=%v", id, kind, ct, err)
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape(err.Error())))
	}
	if _, err := insertCustomerAsset(c.Request().Context(), h.pool, h.schema, actorOf(c), id, kind, obj, ct, size, sha); err != nil {
		log.Printf("asset upload db error customer_id=%d kind=%s err=%v", id, kind, err)
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape("保存记录失败："+err.Error())))
	}
	log.Printf("asset upload ok customer_id=%d kind=%s obj=%s bytes=%d", id, kind, obj, size)
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?ok=1", id))

}

func (h customerHandler) deleteAsset(c echo.Context) error {
	assetID, err := strconv.ParseInt(strings.TrimSpace(c.FormValue("asset_id")), 10, 64)
	if err != nil || assetID <= 0 {
		return c.String(http.StatusBadRequest, "asset_id required")
	}
	cid, _, obj, err := deleteCustomerAssetByID(c.Request().Context(), h.pool, h.schema, actorOf(c), assetID)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if obj != "" {
		_ = os.Remove(filepath.Join(h.assetDir, obj))
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", cid))

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
	if err := inlineUpdateCustomer(c.Request().Context(), h.pool, h.schema, actorOf(c), id, &req); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.String(http.StatusOK, "ok")

}

func (h customerHandler) delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.String(http.StatusBadRequest, "invalid id")
	}
	if err := deleteCustomer(c.Request().Context(), h.pool, h.schema, actorOf(c), id); err != nil {
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
	q := fmt.Sprintf(`SELECT object_key, content_type FROM %s.customer_assets WHERE id=$1`, h.schema)
	if err := h.pool.QueryRow(c.Request().Context(), q, assetID).Scan(&obj, &ct); err != nil {
		return c.String(http.StatusNotFound, "not found")
	}
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
