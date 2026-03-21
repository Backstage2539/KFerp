package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type TemplateRenderer struct{ t *template.Template }

func (tr *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return tr.t.ExecuteTemplate(w, name, data)
}

type Option struct {
	ID   int64
	Name string
}

type ProductTierOption struct {
	ID      int64
	MinLb   float64
	MaxLb   *float64
	PriceLb float64
}

type ProductOption struct {
	ID           int64
	Name         string
	DefaultPrice float64
	Tiers        []ProductTierOption
}

type PageData struct {
	Today        string
	Customers    []Option
	Sources      []Option
	ShipStatuses []Option
	PayStatuses  []Option
	OrderTypes   []Option
	Products     []ProductOption
	ProductsJSON template.JS
	EditMode     bool
	EditID       int64
	EditDataJSON template.JS
	Ok           bool
	OrderNo      string
	Error        string
}

type OrderRow struct {
	ID                int64
	OrderNo           string
	OrderDate         string
	CustomerID        int64
	Customer          string
	GrandTotal        string
	OrderType         string
	PayStatus         string
	ShipStatus        string
	OrderTypeID       int64
	PayStatusID       int64
	ShipStatusID      int64
	ProcessStatusID   int64
	ProcessStatus     string
	CreatedByEmployee string
	Notes             string
	IsVoid            bool
}

type OrdersPageData struct {
	Q                string
	From             string
	To               string
	Preset           string
	Void             string // normal|void|all
	CustomerID       int64
	PayStatusFilter  int64
	ShipStatusFilter int64
	ProcStatusFilter int64
	UnproducedOnly   bool
	CompletedOnly    bool
	Summary          OrdersSummary
	Rows             []OrderRow
	OrderTypeOpts    []Option
	PayOpts          []Option
	ShipOpts         []Option
	ProcessOpts      []Option
	Limit            int
	Offset           int
	Page             int
	HasPrev          bool
	HasNext          bool
	Error            string
}

type OrderItemRow struct {
	LineNo    int
	Product   string
	ItemName  string
	Qty       *float64
	Unit      *string
	Spec      *string
	UnitPrice *float64
	LineTotal *float64
}

type OrderDetailData struct {
	ID                    int64
	OrderNo               string
	OrderDate             string
	Customer              string
	Source                string
	OrderType             string
	PayStatus             string
	ShipStatus            string
	OrderTypeID           int64
	PayStatusID           int64
	ShipStatusID          int64
	ProcessStatus         string
	CreatedByEmployee     string
	IsVoid                bool
	VoidedAt              *string
	VoidReason            *string
	Notes                 *string
	TotalAmount           float64
	ShippingAmt           float64
	DiscountAmt           float64
	RoundToInt            bool
	RoundingAmt           float64
	GrandTotal            float64
	ExpressFee            *string
	OutsourceMaterialFee  float64
	OutsourceRoastFee     float64
	OutsourcePackagingFee float64
	OutsourceManualFee    float64
	OutsourceTaxFee       float64
	OutsourceOtherFee     float64
	OutsourceTotalFee     float64
	OrderTypeOpts         []Option
	PayOpts               []Option
	ShipOpts              []Option
	Items                 []OrderItemRow
	Error                 string
}

type CreateOrderRequest struct {
	OrderDate             string `form:"order_date"`
	CustomerID            int64  `form:"customer_id"`
	SourceID              int64  `form:"source_id"`
	OrderTypeID           int64  `form:"order_type_id"`
	PayStatusID           int64  `form:"pay_status_id"`
	ShipStatusID          int64  `form:"ship_status_id"`
	ShipMethod            string `form:"ship_method"`
	ShipTrackingNo        string `form:"ship_tracking_no"`
	Notes                 string `form:"notes"`
	ShippingAmount        string `form:"shipping_amount"`
	DiscountAmount        string `form:"discount_amount"`
	RoundToInt            string `form:"round_to_int"`
	ExpressFee            string `form:"express_fee"`
	OutsourceMaterialFee  string `form:"outsource_material_fee"`
	OutsourceRoastFee     string `form:"outsource_roast_fee"`
	OutsourcePackagingFee string `form:"outsource_packaging_fee"`
	OutsourceManualFee    string `form:"outsource_manual_fee"`
	OutsourceTaxFee       string `form:"outsource_tax_fee"`
	OutsourceOtherFee     string `form:"outsource_other_fee"`

	ProductID []string `form:"product_id[]"`
	TierID    []string `form:"tier_id[]"`
	UnitPrice []string `form:"unit_price[]"`
	ItemName  []string `form:"item_name[]"`
	Qty       []string `form:"qty[]"`
	Unit      []string `form:"unit[]"`
	Spec      []string `form:"spec[]"`
}

type UpdateOrderRequest struct {
	OrderDate             string `form:"order_date"`
	CustomerID            int64  `form:"customer_id"`
	SourceID              int64  `form:"source_id"`
	OrderTypeID           int64  `form:"order_type_id"`
	PayStatusID           int64  `form:"pay_status_id"`
	ShipStatusID          int64  `form:"ship_status_id"`
	ShipMethod            string `form:"ship_method"`
	ShipTrackingNo        string `form:"ship_tracking_no"`
	Notes                 string `form:"notes"`
	ShippingAmount        string `form:"shipping_amount"`
	DiscountAmount        string `form:"discount_amount"`
	RoundToInt            string `form:"round_to_int"`
	ExpressFee            string `form:"express_fee"`
	OutsourceMaterialFee  string `form:"outsource_material_fee"`
	OutsourceRoastFee     string `form:"outsource_roast_fee"`
	OutsourcePackagingFee string `form:"outsource_packaging_fee"`
	OutsourceManualFee    string `form:"outsource_manual_fee"`
	OutsourceTaxFee       string `form:"outsource_tax_fee"`
	OutsourceOtherFee     string `form:"outsource_other_fee"`

	ItemID    []string `form:"item_id[]"`
	Qty       []string `form:"qty[]"`
	UnitPrice []string `form:"unit_price[]"`
}

func env(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func parseNum(s string) (*string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return nil, err
	}
	return &s, nil
}

func parseFee(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}

func calcOutsourceTotal(req interface {
	GetMaterial() string
	GetRoast() string
	GetPackaging() string
	GetManual() string
	GetTax() string
	GetOther() string
}) (float64, [6]float64, error) {
	material, err := parseFee(req.GetMaterial())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_material_fee")
	}
	roast, err := parseFee(req.GetRoast())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_roast_fee")
	}
	packaging, err := parseFee(req.GetPackaging())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_packaging_fee")
	}
	manual, err := parseFee(req.GetManual())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_manual_fee")
	}
	tax, err := parseFee(req.GetTax())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_tax_fee")
	}
	other, err := parseFee(req.GetOther())
	if err != nil {
		return 0, [6]float64{}, fmt.Errorf("invalid outsource_other_fee")
	}
	fees := [6]float64{material, roast, packaging, manual, tax, other}
	return material + roast + packaging + manual + tax + other, fees, nil
}

func (r *CreateOrderRequest) GetMaterial() string  { return r.OutsourceMaterialFee }
func (r *CreateOrderRequest) GetRoast() string     { return r.OutsourceRoastFee }
func (r *CreateOrderRequest) GetPackaging() string { return r.OutsourcePackagingFee }
func (r *CreateOrderRequest) GetManual() string    { return r.OutsourceManualFee }
func (r *CreateOrderRequest) GetTax() string       { return r.OutsourceTaxFee }
func (r *CreateOrderRequest) GetOther() string     { return r.OutsourceOtherFee }

func (r *UpdateOrderRequest) GetMaterial() string  { return r.OutsourceMaterialFee }
func (r *UpdateOrderRequest) GetRoast() string     { return r.OutsourceRoastFee }
func (r *UpdateOrderRequest) GetPackaging() string { return r.OutsourcePackagingFee }
func (r *UpdateOrderRequest) GetManual() string    { return r.OutsourceManualFee }
func (r *UpdateOrderRequest) GetTax() string       { return r.OutsourceTaxFee }
func (r *UpdateOrderRequest) GetOther() string     { return r.OutsourceOtherFee }

// applyRoundToInt: "抹除小数点" => round down to integer (truncate decimal part).
func applyRoundToInt(total float64, enabled bool) (grand float64, rounding float64) {
	if !enabled {
		return total, 0
	}
	grand = float64(int64(total))
	rounding = grand - total
	return grand, rounding
}

func basicAuth(user, pass, schema string, pool *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Path()
			if strings.HasPrefix(path, "/api/auth/") || path == "/login" {
				return next(c)
			}

			if u, p, ok := c.Request().BasicAuth(); ok {
				if subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1 && subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1 {
					c.Set("actor", u)
					return next(c)
				}
			}

			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename != "" {
						c.Set("operator_employee", ename)
						c.Set("actor", ename)
					}
					return next(c)
				}
			}

			c.Response().Header().Set("WWW-Authenticate", `Basic realm="orderapp"`)
			return c.NoContent(http.StatusUnauthorized)
		}
	}
}

func actorOf(c echo.Context) string {
	if v := c.Get("operator_employee"); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if v := c.Get("actor"); v != nil {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return "unknown"
}

func main() {
	dsn := env("DATABASE_URL", "")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}
	schema := env("DB_SCHEMA", "p2rms15pepb5ciz")
	assetDir := env("ASSET_DIR", "/app/data/assets")

	authUser := env("APP_USER", "order")
	authPass := env("APP_PASS", "")
	if authPass == "" {
		log.Fatal("APP_PASS is required")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int {
			if a-b < 0 {
				return 0
			}
			return a - b
		},
		// format helpers for *float64
		"f0p": func(p *float64) string {
			if p == nil {
				return ""
			}
			return fmt.Sprintf("%.0f", *p)
		},
		"f2p": func(p *float64) string {
			if p == nil {
				return ""
			}
			return fmt.Sprintf("%.2f", *p)
		},
		"py":         func(s string) string { return pinyinFull(s) },
		"pyi":        func(s string) string { return pinyinInitials(s) },
		"assetLabel": func(kind string) string { return kindLabel(kind) },
		"custShort": func(s string) string {
			s = strings.TrimSpace(s)
			if s == "" {
				return s
			}
			// keep only first segment before newline/pipe
			for _, sep := range []string{"\n", "|", "｜"} {
				if i := strings.Index(s, sep); i >= 0 {
					s = strings.TrimSpace(s[:i])
				}
			}
			// cut before first phone number
			re := regexp.MustCompile(`1\d{10}`)
			if loc := re.FindStringIndex(s); loc != nil {
				s = strings.TrimSpace(s[:loc[0]])
			}
			// strip some labels
			s = strings.NewReplacer("地址：", "", "地址:", "", "收件人:", "", "收件人：", "", "姓名:", "", "姓名：", "").Replace(s)
			s = strings.Trim(s, " ,，:：")
			// avoid over-long names
			runes := []rune(s)
			if len(runes) > 30 {
				s = string(runes[:30])
			}
			return s
		},
		"eq64": func(a, b int64) bool { return a == b },
		"eqi":  func(a, b int) bool { return a == b },
	}
	// Note: templates are baked into the image at /app/templates
	t := template.Must(template.New("").Funcs(funcs).ParseGlob("/app/templates/*.html"))

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Secure())
	e.Use(basicAuth(authUser, authPass, schema, pool))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if u, _, ok := c.Request().BasicAuth(); ok {
				if eid, err := resolveEmployeeIDByLogin(c, pool, schema, u); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename, err := resolveEmployeeNameByID(c, pool, schema, eid); err == nil && strings.TrimSpace(ename) != "" {
						c.Set("operator_employee", strings.TrimSpace(ename))
					}
				}
			}
			authz := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[7:])
				if eid, ename, err := resolveEmployeeBySessionToken(c, pool, schema, token); err == nil && eid > 0 {
					c.Set("employee_id", eid)
					if ename != "" {
						c.Set("operator_employee", ename)
					}
				}
			}
			return next(c)
		}
	})
	e.Renderer = &TemplateRenderer{t: t}

	if err := ensureReqTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := seedReqWorkflowA(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}

	if err := ensureFinishedInventoryTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureFinishedAllocationLogTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureMaterialTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureBomTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureBagSpecMappingTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureProduceBatchTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureCompanyStaffTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureMachineCapacityTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureMobileAuthTables(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureProductionRunTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureOrderProcessStatuses(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureSenderSettingsTable(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}
	if err := ensureOutsourceFeeColumns(context.Background(), pool, schema); err != nil {
		log.Fatal(err)
	}

	registerShipExportRoutes(e, pool, schema)
	registerRequirementPages(e, pool, schema)
	registerRequirementAPIs(e, pool, schema)
	registerFinishedInventoryPages(e, pool, schema)
	registerMaterialsPages(e, pool, schema)
	registerBomPages(e, pool, schema)
	registerBomAPI(e, pool, schema)

	// Serve React frontend static files for BOM management
	// Note: Caddy strips /app/ prefix, so routes are /bom-react/*
	e.Static("/bom-react/assets", "frontend/dist/assets")
	e.GET("/bom-react", func(c echo.Context) error {
		return c.File("frontend/dist/index.html")
	})
	e.GET("/bom-react/*", func(c echo.Context) error {
		return c.File("frontend/dist/index.html")
	})

	// Vue3 + Vite workspace shell (DEV-046 serial menu migration)
	e.Static("/vue-shell/assets", "frontend-vue-shell/dist/assets")
	e.GET("/vue-shell", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})
	e.GET("/vue-shell/*", func(c echo.Context) error {
		return c.File("frontend-vue-shell/dist/index.html")
	})

	registerUnprodSummaryPages(e, pool, schema)
	registerProducePlanPages(e, pool, schema)
	registerMachineCapacityPages(e, pool, schema)
	registerSenderSettingsPage(e, pool, schema)
	registerProducePlanAllocate(e, pool, schema)
	registerProductionFlowPages(e, pool, schema)
	registerProduceBatchAPI(e, pool, schema)
	registerCompanyStaffPages(e, pool, schema)
	registerCompanyStaffAPI(e, pool, schema)
	registerMobileAuthAPI(e, pool, schema)
	registerAllocationLogPages(e, pool, schema)

	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/orders")
	})
	// legacy aliases for old entrypoints
	e.GET("/order-list", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/orders")
	})
	e.GET("/order-detail/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.Redirect(http.StatusSeeOther, "/orders")
		}
		return c.Redirect(http.StatusSeeOther, "/orders/"+id)
	})
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", map[string]any{})
	})

	// Products
	e.GET("/audit", func(c echo.Context) error {
		data := AuditPageData{
			From:       strings.TrimSpace(c.QueryParam("from")),
			To:         strings.TrimSpace(c.QueryParam("to")),
			Q:          strings.TrimSpace(c.QueryParam("q")),
			EntityType: strings.TrimSpace(c.QueryParam("type")),
		}
		rows, err := fetchAuditPage(c.Request().Context(), pool, schema, data.From, data.To, data.Q, data.EntityType, 200)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Rows = rows
		}
		return c.Render(http.StatusOK, "audit.html", data)
	})

	// Docs
	e.GET("/docs", func(c echo.Context) error {
		data := DocsIndexData{}
		files, err := listDocFiles("/app/docs")
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Files = files
		}
		return c.Render(http.StatusOK, "docs.html", data)
	})
	e.GET("/docs/:name", func(c echo.Context) error {
		name, err := safeDocName(c.Param("name"))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid name")
		}
		bs, err := os.ReadFile(filepath.Join("/app/docs", name))
		if err != nil {
			return c.String(http.StatusNotFound, "not found")
		}
		if c.QueryParam("raw") == "1" {
			c.Response().Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
			return c.String(http.StatusOK, string(bs))
		}
		return c.Render(http.StatusOK, "doc_view.html", DocViewData{Name: name, Content: string(bs)})
	})

	// Customers
	e.GET("/customers", func(c echo.Context) error {
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

		sources, _ := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", schema))
		types, _ := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema))
		data.Sources = sources
		data.OrderTypes = types

		rows, hasNext, err := fetchCustomers(c.Request().Context(), pool, schema, data.Q, data.Limit, data.Offset)
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
	})
	e.GET("/customers/new", func(c echo.Context) error {
		data := CustomerEditData{Active: true}
		data.Sources, _ = fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", schema))
		data.OrderTypes, _ = fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema))
		// allow returning back to order entry
		if strings.TrimSpace(c.QueryParam("from")) == "order" {
			data.From = "order"
		}
		return c.Render(http.StatusOK, "customer_edit.html", data)
	})
	e.POST("/customers/new", func(c echo.Context) error {
		var req CustomerUpsertRequest
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}
		id, err := upsertCustomer(c.Request().Context(), pool, schema, actorOf(c), nil, &req)
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
	})
	e.GET("/customers/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		data, err := fetchCustomerByID(c.Request().Context(), pool, schema, id)
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
		data.Sources, _ = fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", schema))
		data.OrderTypes, _ = fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema))
		if assets, err := fetchCustomerAssets(c.Request().Context(), pool, schema, id); err == nil {
			data.Assets = assets
		}
		if dash, err := fetchCustomerDashboard(c.Request().Context(), pool, schema, id); err == nil {
			data.Dash = dash
		}
		return c.Render(http.StatusOK, "customer_edit.html", data)
	})
	// prefs for order auto-fill
	e.GET("/customers/:id/prefs", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		p, err := fetchCustomerPrefs(c.Request().Context(), pool, schema, id)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, p)
	})
	e.POST("/customers/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		var req CustomerUpsertRequest
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}
		_, err = upsertCustomer(c.Request().Context(), pool, schema, actorOf(c), &id, &req)
		if err != nil {
			data := CustomerEditData{ID: id, Active: strings.TrimSpace(req.Active) != "", Name: req.Name, RawName: req.RawName, Contact: req.Contact, Phone: req.Phone, Address: req.Address, Error: err.Error()}
			return c.Render(http.StatusOK, "customer_edit.html", data)
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", id))
	})
	// customer assets upload
	e.POST("/customers/:id/assets/upload", func(c echo.Context) error {
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

		obj, size, sha, err := saveCustomerAssetFile(assetDir, id, kind, r, ct, 100*1024*1024, fh.Filename)
		if err != nil {
			log.Printf("asset upload save error customer_id=%d kind=%s ct=%s err=%v", id, kind, ct, err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape(err.Error())))
		}
		if _, err := insertCustomerAsset(c.Request().Context(), pool, schema, actorOf(c), id, kind, obj, ct, size, sha); err != nil {
			log.Printf("asset upload db error customer_id=%d kind=%s err=%v", id, kind, err)
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?err=%s", id, url.QueryEscape("保存记录失败："+err.Error())))
		}
		log.Printf("asset upload ok customer_id=%d kind=%s obj=%s bytes=%d", id, kind, obj, size)
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d?ok=1", id))
	})
	e.POST("/customers/:id/assets/delete", func(c echo.Context) error {
		assetID, err := strconv.ParseInt(strings.TrimSpace(c.FormValue("asset_id")), 10, 64)
		if err != nil || assetID <= 0 {
			return c.String(http.StatusBadRequest, "asset_id required")
		}
		cid, _, obj, err := deleteCustomerAssetByID(c.Request().Context(), pool, schema, actorOf(c), assetID)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		if obj != "" {
			_ = os.Remove(filepath.Join(assetDir, obj))
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/app/customers/%d", cid))
	})
	assetServe := func(c echo.Context, headOnly bool) error {
		assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || assetID <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		var obj string
		var ct string
		q := fmt.Sprintf(`SELECT object_key, content_type FROM %s.customer_assets WHERE id=$1`, schema)
		if err := pool.QueryRow(c.Request().Context(), q, assetID).Scan(&obj, &ct); err != nil {
			return c.String(http.StatusNotFound, "not found")
		}
		obj = strings.TrimPrefix(obj, "/")
		path := filepath.Join(assetDir, obj)
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
	e.GET("/assets/customer_assets/:id", func(c echo.Context) error { return assetServe(c, false) })
	e.HEAD("/assets/customer_assets/:id", func(c echo.Context) error { return assetServe(c, true) })

	// inline update (list)
	e.POST("/customers/:id/inline", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		var req CustomerInlineReq
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}
		if err := inlineUpdateCustomer(c.Request().Context(), pool, schema, actorOf(c), id, &req); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.String(http.StatusOK, "ok")
	})
	// delete (soft): active=false
	e.POST("/customers/:id/delete", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		if err := deleteCustomer(c.Request().Context(), pool, schema, actorOf(c), id); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.String(http.StatusOK, "ok")
	})

	e.GET("/products", func(c echo.Context) error {
		data := struct {
			Products []ProductOption
			Error    string
		}{}
		ps, err := fetchProducts(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		data.Products = ps
		return c.Render(http.StatusOK, "products.html", data)
	})
	e.GET("/products/print", func(c echo.Context) error {
		data := struct {
			Products []ProductOption
			Error    string
		}{}
		ps, err := fetchProducts(c.Request().Context(), pool, schema)
		if err != nil {
			data.Error = err.Error()
		}
		data.Products = ps
		return c.Render(http.StatusOK, "products_print.html", data)
	})
	e.GET("/products/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		p, err := fetchProductByID(c.Request().Context(), pool, schema, id)
		if err != nil {
			return err
		}
		if p == nil {
			return c.String(http.StatusNotFound, "not found")
		}
		return c.Render(http.StatusOK, "product_edit.html", p)
	})
	e.POST("/products/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		mins := c.Request().FormValue("min[]")
		_ = mins
		// parse arrays
		minArr := c.Request().PostForm["min[]"]
		maxArr := c.Request().PostForm["max[]"]
		priceArr := c.Request().PostForm["price[]"]

		ctx := c.Request().Context()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Replace tiers for simplicity
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.product_price_tiers WHERE product_id=$1", schema), id); err != nil {
			return err
		}
		ins := fmt.Sprintf("INSERT INTO %s.product_price_tiers(product_id,min_qty_lb,max_qty_lb,price_per_lb,active) VALUES ($1,$2,$3,$4,true)", schema)
		for i := 0; i < len(minArr); i++ {
			mn := strings.TrimSpace(minArr[i])
			if mn == "" {
				continue
			}
			minv, err := strconv.ParseFloat(mn, 64)
			if err != nil {
				continue
			}
			var maxAny any = nil
			if i < len(maxArr) {
				mx := strings.TrimSpace(maxArr[i])
				if mx != "" {
					if mxv, err := strconv.ParseFloat(mx, 64); err == nil {
						maxAny = mxv
					}
				}
			}
			pv := 0.0
			if i < len(priceArr) {
				if f, err := strconv.ParseFloat(strings.TrimSpace(priceArr[i]), 64); err == nil {
					pv = f
				}
			}
			if _, err := tx.Exec(ctx, ins, id, minv, maxAny, pv); err != nil {
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d", id))
	})

	// Orders list
	e.GET("/orders", func(c echo.Context) error {
		data := OrdersPageData{
			Q:      strings.TrimSpace(c.QueryParam("q")),
			From:   strings.TrimSpace(c.QueryParam("from")),
			To:     strings.TrimSpace(c.QueryParam("to")),
			Preset: strings.TrimSpace(c.QueryParam("preset")),
			Void:   strings.TrimSpace(c.QueryParam("void")),
			Limit:  10,
			Offset: 0,
		}
		if v := strings.TrimSpace(c.QueryParam("customer_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				data.CustomerID = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("pay_status_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				data.PayStatusFilter = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("ship_status_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				data.ShipStatusFilter = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("process_status_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				data.ProcStatusFilter = n
			}
		}
		data.CompletedOnly = strings.TrimSpace(c.QueryParam("completed")) == "1"
		if data.Preset == "unprod" {
			data.UnproducedOnly = true
		}
		if data.Void == "" {
			data.Void = "normal"
		}
		if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				data.Limit = n
			}
		}
		if v := strings.TrimSpace(c.QueryParam("offset")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				data.Offset = n
			}
		}
		// page is 1-indexed; overrides offset when provided
		if v := strings.TrimSpace(c.QueryParam("page")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				data.Offset = (n - 1) * data.Limit
			}
		}

		if data.Limit > 0 {
			data.Page = (data.Offset / data.Limit) + 1
		} else {
			data.Page = 1
		}

		rows, hasNext, errOrders := fetchOrders(c.Request().Context(), pool, schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly, data.Limit, data.Offset)
		if opts, err := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema)); err == nil {
			data.OrderTypeOpts = opts
		} else {
			data.OrderTypeOpts = nil
		}
		if opts, err := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", schema)); err == nil {
			data.PayOpts = opts
		} else {
			data.PayOpts = nil
		}
		if opts, err := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", schema)); err == nil {
			data.ShipOpts = opts
		} else {
			data.ShipOpts = nil
		}
		if opts, err := fetchOptions(c.Request().Context(), pool, fmt.Sprintf("SELECT id, name FROM %s.order_process_statuses WHERE active=true ORDER BY sort,id", schema)); err == nil {
			data.ProcessOpts = opts
		} else {
			data.ProcessOpts = nil
		}
		// summary (best effort)
		if s, err := fetchOrdersSummary(c.Request().Context(), pool, schema, data.Q, data.From, data.To, data.Void, data.CustomerID, data.PayStatusFilter, data.ShipStatusFilter, data.ProcStatusFilter, data.UnproducedOnly, data.CompletedOnly); err == nil {
			data.Summary = s
		}

		if errOrders != nil {
			data.Error = errOrders.Error()
			return c.Render(http.StatusOK, "orders.html", data)
		}
		data.Rows = rows
		data.HasPrev = data.Offset > 0
		data.HasNext = hasNext
		if data.Limit > 0 {
			data.Page = (data.Offset / data.Limit) + 1
		} else {
			data.Page = 1
		}
		return c.Render(http.StatusOK, "orders.html", data)
	})

	e.POST("/orders/:id/inline", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		var req InlineUpdateRequest
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}
		ctx := c.Request().Context()
		if err := inlineUpdateOrder(ctx, pool, schema, id, actorOf(c), &req); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.NoContent(http.StatusNoContent)
	})

	e.GET("/orders/:id/audit", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		rows, err := fetchAuditLogs(c.Request().Context(), pool, schema, id, 50)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, rows)
	})

	// Merged detail+edit: clicking order number goes to unified edit page.
	e.GET("/orders/:id", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d/edit", id))
	})

	// Unified edit: reuse /order page logic.
	e.GET("/orders/:id/edit", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/order?edit_id=%d", id))
	})
	e.POST("/orders/:id/edit", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		var req UpdateOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}
		ctx := c.Request().Context()
		if err := updateOrderHeader(ctx, pool, schema, id, &req); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))
	})

	// Void / unvoid
	e.POST("/orders/:id/void", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		reason := strings.TrimSpace(c.FormValue("reason"))
		ctx := c.Request().Context()
		q := fmt.Sprintf("UPDATE %s.orders SET is_void=true, voided_at=now(), void_reason=$2 WHERE id=$1", schema)
		if _, err := pool.Exec(ctx, q, id, nullText(reason)); err != nil {
			return err
		}
		var rv *string
		if strings.TrimSpace(reason) != "" {
			rv = &reason
		}
		auditInsert(ctx, pool, schema, actorOf(c), "order", &id, "void", nil, nil, rv, AuditMeta{"order_id": id})
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))
	})
	e.POST("/orders/:id/unvoid", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.String(http.StatusBadRequest, "invalid id")
		}
		ctx := c.Request().Context()
		q := fmt.Sprintf("UPDATE %s.orders SET is_void=false, voided_at=NULL, void_reason=NULL WHERE id=$1", schema)
		if _, err := pool.Exec(ctx, q, id); err != nil {
			return err
		}
		auditInsert(ctx, pool, schema, actorOf(c), "order", &id, "unvoid", nil, nil, nil, AuditMeta{"order_id": id})
		return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", id))
	})

	// Create order
	e.GET("/order", func(c echo.Context) error {
		data := PageData{Today: time.Now().Format("2006-01-02")}
		if c.QueryParam("ok") == "1" {
			data.Ok = true
			data.OrderNo = c.QueryParam("order_no")
		}
		if err := loadOptions(c.Request().Context(), pool, schema, &data); err != nil {
			data.Error = err.Error()
		}
		if v := strings.TrimSpace(c.QueryParam("edit_id")); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
				ed, ferr := fetchOrderEdit(c.Request().Context(), pool, schema, id)
				if ferr != nil {
					data.Error = ferr.Error()
				} else if ed == nil {
					data.Error = "order not found"
				} else {
					data.EditMode = true
					data.EditID = id
					if ed.OrderDate != "" {
						data.Today = ed.OrderDate
					}
					type editItem struct {
						ProductID   int64  `json:"product_id"`
						ProductName string `json:"product_name"`
						TierID      string `json:"tier_id"`
						UnitPrice   string `json:"unit_price"`
						Qty         string `json:"qty"`
						Unit        string `json:"unit"`
						Spec        string `json:"spec"`
					}
					items := make([]editItem, 0, len(ed.Items))
					for _, it := range ed.Items {
						spec := strings.TrimSuffix(strings.TrimSpace(strings.ToLower(it.Spec)), "g")
						items = append(items, editItem{
							ProductID:   it.ProductID,
							ProductName: it.Product,
							TierID:      "auto",
							UnitPrice:   it.UnitPrice,
							Qty:         it.Qty,
							Unit:        it.Unit,
							Spec:        spec,
						})
					}
					payload := map[string]any{
						"order_date":              ed.OrderDate,
						"customer_id":             strconv.FormatInt(ed.CustomerID, 10),
						"source_id":               strconv.FormatInt(ed.SourceID, 10),
						"order_type_id":           strconv.FormatInt(ed.OrderTypeID, 10),
						"pay_status_id":           strconv.FormatInt(ed.PayStatusID, 10),
						"ship_status_id":          strconv.FormatInt(ed.ShipStatusID, 10),
						"ship_method":             ed.ShipMethod,
						"ship_tracking_no":        ed.ShipTrackingNo,
						"notes":                   ed.Notes,
						"shipping_amount":         ed.ShippingAmount,
						"discount_amount":         ed.DiscountAmount,
						"round_to_int":            ed.RoundToInt,
						"express_fee":             ed.ExpressFee,
						"outsource_material_fee":  ed.OutsourceMaterialFee,
						"outsource_roast_fee":     ed.OutsourceRoastFee,
						"outsource_packaging_fee": ed.OutsourcePackagingFee,
						"outsource_manual_fee":    ed.OutsourceManualFee,
						"outsource_tax_fee":       ed.OutsourceTaxFee,
						"outsource_other_fee":     ed.OutsourceOtherFee,
						"items":                   items,
					}
					if b, err := json.Marshal(payload); err == nil {
						data.EditDataJSON = template.JS(string(b))
					}
				}
			}
		}
		data.ProductsJSON = buildProductsJSON(data.Products)
		return c.Render(http.StatusOK, "order.html", data)
	})

	e.POST("/order", func(c echo.Context) error {
		if err := requireEmployeeBound(c); err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		var req CreateOrderRequest
		if err := c.Bind(&req); err != nil {
			return c.String(http.StatusBadRequest, "bad request")
		}

		orderDate := strings.TrimSpace(req.OrderDate)
		if orderDate == "" {
			orderDate = time.Now().Format("2006-01-02")
		}
		od, err := time.Parse("2006-01-02", orderDate)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid order_date")
		}
		if req.CustomerID <= 0 {
			return c.String(http.StatusBadRequest, "customer required")
		}

		type item struct {
			productID     *int64
			tierID        *int64
			manualPrice   *float64
			name          string
			units         int64
			specG         int64
			unit          *string
			spec          *string
			unitPrice     float64
			lineTotal     float64
			priceOverride bool
		}
		items := make([]item, 0)
		for i := 0; i < maxLen(req.ItemName, req.ProductID, req.TierID, req.UnitPrice, req.Qty, req.Unit, req.Spec); i++ {
			pidStr := strings.TrimSpace(getStr(req.ProductID, i))
			name := strings.TrimSpace(getStr(req.ItemName, i))

			// If no product and no name, skip row.
			if pidStr == "" && name == "" {
				continue
			}

			it := item{name: name}
			if pidStr != "" {
				if pid, err := strconv.ParseInt(pidStr, 10, 64); err == nil && pid > 0 {
					it.productID = &pid
				}
			}
			if tidStr := strings.TrimSpace(getStr(req.TierID, i)); tidStr != "" && tidStr != "auto" {
				if tidStr == "manual" {
					if v := strings.TrimSpace(getStr(req.UnitPrice, i)); v != "" {
						if f, err := strconv.ParseFloat(v, 64); err == nil {
							it.manualPrice = &f
							it.priceOverride = true
						}
					}
				} else {
					if tid, err := strconv.ParseInt(tidStr, 10, 64); err == nil && tid > 0 {
						it.tierID = &tid
					}
				}
			}
			if q := strings.TrimSpace(getStr(req.Qty, i)); q != "" {
				if n, err := strconv.ParseInt(q, 10, 64); err == nil && n > 0 {
					it.units = n
				}
			}
			if sg := strings.TrimSpace(getStr(req.Spec, i)); sg != "" {
				// spec is grams (e.g. "227" or "227g")
				sg = strings.TrimSuffix(strings.ToLower(sg), "g")
				if n, err := strconv.ParseInt(sg, 10, 64); err == nil && n > 0 {
					it.specG = n
					ss := fmt.Sprintf("%dg", n)
					it.spec = &ss
				}
			}
			if u := strings.TrimSpace(getStr(req.Unit, i)); u != "" {
				it.unit = &u
			}
			items = append(items, it)
		}
		// Validate: need at least one item with spec+units
		valid := false
		for _, it := range items {
			if it.productID != nil && it.specG > 0 && it.units > 0 {
				valid = true
				break
			}
		}
		if !valid {
			return c.String(http.StatusBadRequest, "at least one item required")
		}

		ctx := c.Request().Context()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()

		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		if _, err := tx.Exec(ctx, fmt.Sprintf("LOCK TABLE %s.orders IN SHARE ROW EXCLUSIVE MODE", schema)); err != nil {
			return err
		}

		orderNo := ""

		// Pricing: use tier match by qty(lb). Allow manual override.
		totalAmt := 0.0
		orderWeightG := int64(0)
		for idx := range items {
			itemWeightG := items[idx].specG * items[idx].units
			orderWeightG += itemWeightG
			totalG := float64(itemWeightG)
			qtyLb := totalG / 454.0

			if items[idx].manualPrice != nil {
				// manual price is 元/磅
				items[idx].unitPrice = *items[idx].manualPrice
				items[idx].priceOverride = true
			} else if items[idx].productID != nil {
				// If user selected a tier explicitly
				if items[idx].tierID != nil {
					var price float64
					q := fmt.Sprintf(`SELECT price_per_lb FROM %s.product_price_tiers WHERE id=$1 AND active=true`, schema)
					if err := tx.QueryRow(ctx, q, *items[idx].tierID).Scan(&price); err != nil {
						return c.String(http.StatusBadRequest, "invalid tier")
					}
					items[idx].unitPrice = price
				} else {
					// Auto-match tier by qty(lb)
					var tid *int64
					var price float64
					q := fmt.Sprintf(`
						SELECT id, price_per_lb
						FROM %s.product_price_tiers
						WHERE product_id=$1 AND active=true
						  AND min_qty_lb <= $2
						  AND (max_qty_lb IS NULL OR max_qty_lb >= $2)
						ORDER BY min_qty_lb DESC
						LIMIT 1
					`, schema)
					err := tx.QueryRow(ctx, q, *items[idx].productID, qtyLb).Scan(&tid, &price)
					if err != nil {
						// fallback: highest tier with min<=qty
						q2 := fmt.Sprintf(`
							SELECT id, price_per_lb
							FROM %s.product_price_tiers
							WHERE product_id=$1 AND active=true AND min_qty_lb <= $2
							ORDER BY min_qty_lb DESC
							LIMIT 1
						`, schema)
						if err2 := tx.QueryRow(ctx, q2, *items[idx].productID, qtyLb).Scan(&tid, &price); err2 != nil {
							// below minimum tier: use minimum tier price
							q3 := fmt.Sprintf(`
								SELECT id, price_per_lb
								FROM %s.product_price_tiers
								WHERE product_id=$1 AND active=true
								ORDER BY min_qty_lb ASC
								LIMIT 1
							`, schema)
							if err3 := tx.QueryRow(ctx, q3, *items[idx].productID).Scan(&tid, &price); err3 != nil {
								price = 0
								tid = nil
							}
						}
					}
					items[idx].tierID = tid
					items[idx].unitPrice = price
				}
			}

			items[idx].lineTotal = qtyLb * items[idx].unitPrice
			totalAmt += items[idx].lineTotal
		}

		// Amount calculation (items + shipping - discount)
		shippingAmt := 0.0
		if v := strings.TrimSpace(req.ShippingAmount); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return c.String(http.StatusBadRequest, "invalid shipping_amount")
			}
			shippingAmt = f
		}
		discountAmt := 0.0
		if v := strings.TrimSpace(req.DiscountAmount); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return c.String(http.StatusBadRequest, "invalid discount_amount")
			}
			discountAmt = f
		}
		roundToInt := strings.TrimSpace(req.RoundToInt) != ""
		outsourceTotal, outsourceFees, err := calcOutsourceTotal(&req)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		grand0 := totalAmt + shippingAmt - discountAmt + outsourceTotal
		grandTotal, roundingAmt := applyRoundToInt(grand0, roundToInt)

		// 默认发货状态：未选择时自动写入“未发货”。
		shipStatusID := req.ShipStatusID
		if shipStatusID == 0 {
			_ = tx.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s.ship_statuses WHERE name='未发货' ORDER BY id LIMIT 1", schema)).Scan(&shipStatusID)
		}

		shipMethod := strings.TrimSpace(req.ShipMethod)
		if shipMethod == "" {
			if orderWeightG <= 15000 {
				shipMethod = "sf_small"
			} else {
				shipMethod = "sf_large"
			}
		}

		editID := int64(0)
		if v := strings.TrimSpace(c.FormValue("edit_id")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				editID = n
			}
		}

		insertItemSQL := fmt.Sprintf(`INSERT INTO %s.order_items(order_id,line_no,product_id,price_tier_id,price_overridden,item_name,qty,unit,spec,unit_price,line_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, schema)

		var orderID int64
		if editID > 0 {
			if err := tx.QueryRow(ctx, fmt.Sprintf("SELECT id, order_no FROM %s.orders WHERE id=$1 FOR UPDATE", schema), editID).Scan(&orderID, &orderNo); err != nil {
				return c.String(http.StatusBadRequest, "invalid edit_id")
			}
			uq := fmt.Sprintf(`
				UPDATE %s.orders
				SET order_date=$2,
					customer_id=$3,
					source_id=$4,
					order_type_id=$5,
					pay_status_id=$6,
					ship_status_id=$7,
					ship_method=$8,
					ship_tracking_no=$9,
					notes=$10,
					total_amount=$11,
					shipping_amount=$12,
					discount_amount=$13,
					round_to_int=$14,
					rounding_amount=$15,
					grand_total=$16,
					express_fee=$17,
					outsource_material_fee=$18,
					outsource_roast_fee=$19,
					outsource_packaging_fee=$20,
					outsource_manual_fee=$21,
					outsource_tax_fee=$22,
					outsource_other_fee=$23,
					outsource_total_fee=$24
				WHERE id=$1
			`, schema)
			if _, err := tx.Exec(ctx, uq,
				orderID,
				od,
				req.CustomerID,
				nullInt(req.SourceID),
				nullInt(req.OrderTypeID),
				nullInt(req.PayStatusID),
				nullInt(shipStatusID),
				nullText(shipMethod),
				nullText(req.ShipTrackingNo),
				nullText(req.Notes),
				totalAmt,
				shippingAmt,
				discountAmt,
				roundToInt,
				roundingAmt,
				grandTotal,
				nullText(req.ExpressFee),
				outsourceFees[0],
				outsourceFees[1],
				outsourceFees[2],
				outsourceFees[3],
				outsourceFees[4],
				outsourceFees[5],
				outsourceTotal,
			); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s.order_items WHERE order_id=$1", schema), orderID); err != nil {
				return err
			}
		} else {
			orderNo, err = nextOrderNo(ctx, tx, schema, od)
			if err != nil {
				return err
			}
			insertOrderSQL := fmt.Sprintf(`
				INSERT INTO %s.orders(
					order_date, customer_id,
					source_id, order_type_id, pay_status_id, ship_status_id,
					ship_method, ship_tracking_no,
					notes,
					total_amount, shipping_amount, discount_amount,
					round_to_int, rounding_amount, grand_total,
					express_fee,
					outsource_material_fee, outsource_roast_fee, outsource_packaging_fee,
					outsource_manual_fee, outsource_tax_fee, outsource_other_fee, outsource_total_fee,
					order_no
				) VALUES (
					$1,$2,$3,$4,$5,$6,$7,$8,$9,
					$10,$11,$12,
					$13,$14,$15,
					$16,$17,$18,$19,$20,$21,$22,$23,
					$24
				)
				RETURNING id
			`, schema)
			err = tx.QueryRow(ctx, insertOrderSQL,
				od,
				req.CustomerID,
				nullInt(req.SourceID),
				nullInt(req.OrderTypeID),
				nullInt(req.PayStatusID),
				nullInt(shipStatusID),
				nullText(shipMethod),
				nullText(req.ShipTrackingNo),
				nullText(req.Notes),
				totalAmt,
				shippingAmt,
				discountAmt,
				roundToInt,
				roundingAmt,
				grandTotal,
				nullText(req.ExpressFee),
				outsourceFees[0],
				outsourceFees[1],
				outsourceFees[2],
				outsourceFees[3],
				outsourceFees[4],
				outsourceFees[5],
				outsourceTotal,
				orderNo,
			).Scan(&orderID)
			if err != nil {
				return err
			}
		}

		for idx, it := range items {
			qtyAny := any(nil)
			if it.units > 0 {
				qtyAny = it.units
			}
			if _, err := tx.Exec(ctx, insertItemSQL, orderID, idx+1, it.productID, it.tierID, it.priceOverride, it.name, qtyAny, it.unit, it.spec, it.unitPrice, it.lineTotal); err != nil {
				return err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		if editID > 0 {
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/orders/%d", orderID))
		}
		return c.Redirect(http.StatusSeeOther, "/order?ok=1&order_no="+url.QueryEscape(orderNo))
	})

	addr := env("LISTEN", ":8080")
	log.Printf("orderapp listening on %s", addr)
	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func loadOptions(ctx context.Context, pool *pgxpool.Pool, schema string, data *PageData) error {
	var err error
	if data.Customers, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.customers WHERE active=true ORDER BY name", schema)); err != nil {
		return err
	}
	if data.Sources, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.sources ORDER BY id", schema)); err != nil {
		return err
	}
	if data.ShipStatuses, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.ship_statuses ORDER BY id", schema)); err != nil {
		return err
	}
	if data.PayStatuses, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.pay_statuses ORDER BY id", schema)); err != nil {
		return err
	}
	if data.OrderTypes, err = fetchOptions(ctx, pool, fmt.Sprintf("SELECT id, name FROM %s.order_types ORDER BY id", schema)); err != nil {
		return err
	}
	if data.Products, err = fetchProducts(ctx, pool, schema); err != nil {
		return err
	}
	return nil
}

func fetchProducts(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProductOption, error) {
	sqlstr := fmt.Sprintf("SELECT id, name, default_price FROM %s.products WHERE active=true ORDER BY name", schema)
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProductOption, 0)
	for rows.Next() {
		var p ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.DefaultPrice); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load tiers for all products
	tierSQL := fmt.Sprintf(`
		SELECT id, product_id, min_qty_lb, max_qty_lb, price_per_lb
		FROM %s.product_price_tiers
		WHERE active=true
		ORDER BY product_id, min_qty_lb
	`, schema)
	trs, err := pool.Query(ctx, tierSQL)
	if err != nil {
		return out, nil // products without tiers still ok
	}
	defer trs.Close()

	tierMap := map[int64][]ProductTierOption{}
	for trs.Next() {
		var tid, pid int64
		var min float64
		var max *float64
		var price float64
		if err := trs.Scan(&tid, &pid, &min, &max, &price); err != nil {
			return nil, err
		}
		tierMap[pid] = append(tierMap[pid], ProductTierOption{ID: tid, MinLb: min, MaxLb: max, PriceLb: price})
	}
	if err := trs.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Tiers = tierMap[out[i].ID]
	}
	return out, nil
}

func fetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Option, 0)
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func nextOrderNo(ctx context.Context, tx pgx.Tx, schema string, od time.Time) (string, error) {
	ymd := od.Format("20060102")
	prefix := "SO-" + ymd + "-"

	var maxNo int
	q := fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(right(order_no,4) AS INT)), 0)
		FROM %s.orders
		WHERE order_no LIKE $1
	`, schema)

	if err := tx.QueryRow(ctx, q, prefix+"%").Scan(&maxNo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, maxNo+1), nil
}

func getStr(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}
	return s[i]
}

func nullText(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func nullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
