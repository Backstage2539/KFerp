package freight

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	"io"
	"net/http"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	repo "orderapp/internal/infrastructure/postgres/freight"
	support "orderapp/internal/interfaces/http/support"
	"regexp"
	"strconv"
	"strings"
)

type Quoter interface {
	QuoteFulfillmentFees(context.Context, app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error)
}

func RegisterRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string, quoter Quoter) {
	r := repo.NewRepository(pool, schema)
	fail := func(c echo.Context, err error) error {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	id := func(c echo.Context, key string) (int64, error) {
		v, e := strconv.ParseInt(c.Param(key), 10, 64)
		if e != nil || v <= 0 {
			return 0, fmt.Errorf("请选择有效记录")
		}
		return v, nil
	}
	actor := support.ActorOf
	e.GET("/api/settings/logistics/shipping-templates/:id/versions", func(c echo.Context) error {
		tid, err := id(c, "id")
		if err != nil {
			return fail(c, err)
		}
		rows, err := pool.Query(c.Request().Context(), fmt.Sprintf(`SELECT snapshot_json FROM %s.shipping_fee_template_versions WHERE template_id=$1 ORDER BY revision DESC`, schema), tid)
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		out := []domain.Template{}
		for rows.Next() {
			var raw []byte
			var t domain.Template
			if err = rows.Scan(&raw); err != nil {
				return fail(c, err)
			}
			if err = json.Unmarshal(raw, &t); err != nil {
				return fail(c, err)
			}
			out = append(out, t)
		}
		if err = rows.Err(); err != nil {
			return fail(c, err)
		}
		return c.JSON(200, map[string]any{"versions": out})
	})
	e.GET("/api/settings/logistics/regions", func(c echo.Context) error {
		return c.JSON(200, map[string]any{"version": domain.RegionVersion, "regions": domain.SearchRegions(c.QueryParam("q"))})
	})
	e.GET("/api/customer-fulfillment/:customer_id/shipping-policies", func(c echo.Context) error {
		cid, err := id(c, "customer_id")
		if err != nil {
			return fail(c, err)
		}
		out, err := r.Policies(c.Request().Context(), cid)
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, map[string]any{"policies": out})
	})
	e.PUT("/api/customer-fulfillment/:customer_id/shipping-policies/:scene", func(c echo.Context) error {
		cid, err := id(c, "customer_id")
		if err != nil {
			return fail(c, err)
		}
		var req struct {
			domain.Policy
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if err = c.Bind(&req); err != nil {
			return fail(c, err)
		}
		req.CustomerID = cid
		req.Scene = c.Param("scene")
		out, err := r.SavePolicy(c.Request().Context(), req.Policy, req.ExpectedRevision, actor(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, out)
	})
	e.POST("/api/customer-fulfillment/:customer_id/freight/preview", func(c echo.Context) error {
		cid, err := id(c, "customer_id")
		if err != nil {
			return fail(c, err)
		}
		var req app.FulfillmentFeeQuoteCommand
		if err = c.Bind(&req); err != nil {
			return fail(c, err)
		}
		req.CustomerID = cid
		req.FulfillmentMode = "sale"
		req.EmployeeAdjustment = true
		out, err := quoter.QuoteFulfillmentFees(c.Request().Context(), req)
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, out.Freight)
	})
	e.POST("/api/settings/logistics/freight-imports/preview", func(c echo.Context) error {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 10<<20)
		f, err := c.FormFile("file")
		if err != nil {
			return fail(c, fmt.Errorf("请选择Excel账单"))
		}
		src, err := f.Open()
		if err != nil {
			return fail(c, err)
		}
		defer src.Close()
		raw, err := io.ReadAll(io.LimitReader(src, (10<<20)+1))
		if err != nil || len(raw) > 10<<20 {
			return fail(c, fmt.Errorf("账单文件不能超过10MB"))
		}
		book, err := excelize.OpenReader(bytes.NewReader(raw), excelize.Options{UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 32 << 20})
		if err != nil {
			return fail(c, fmt.Errorf("Excel读取失败"))
		}
		defer book.Close()
		sheets := book.GetSheetList()
		sheet := c.FormValue("sheet")
		if sheet == "" && len(sheets) > 0 {
			sheet = sheets[0]
		}
		rows, err := book.GetRows(sheet, excelize.Options{RawCellValue: true})
		if err != nil || len(rows) == 0 {
			return fail(c, fmt.Errorf("工作表为空或无效"))
		}
		track, _ := strconv.Atoi(c.FormValue("tracking_column"))
		amount, _ := strconv.Atoi(c.FormValue("amount_column"))
		if track <= 0 || amount <= 0 {
			return c.JSON(200, map[string]any{"sheets": sheets, "sheet": sheet, "headers": rows[0]})
		}
		carrier, _ := strconv.ParseInt(c.FormValue("carrier_id"), 10, 64)
		var validCarrier bool
		if err = pool.QueryRow(c.Request().Context(), fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.logistics_companies WHERE id=$1 AND active=true)`, schema), carrier).Scan(&validCarrier); err != nil || !validCarrier {
			return fail(c, fmt.Errorf("请选择启用的物流公司"))
		}
		parsed, err := parseWorkbook(bytes.NewReader(raw), sheet, track, amount)
		if err != nil {
			return fail(c, err)
		}
		h := sha256.New()
		h.Write(raw)
		fmt.Fprintf(h, "|%s|%d|%d", sheet, track, amount)
		out, err := r.PreviewImport(c.Request().Context(), carrier, hex.EncodeToString(h.Sum(nil)), f.Filename, parsed, actor(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, out)
	})
	e.GET("/api/settings/logistics/freight-imports", func(c echo.Context) error {
		rows, err := pool.Query(c.Request().Context(), fmt.Sprintf(`SELECT id,carrier_id,filename,status FROM %s.freight_import_batches ORDER BY id DESC LIMIT 100`, schema))
		if err != nil {
			return fail(c, err)
		}
		defer rows.Close()
		out := []repo.ImportBatch{}
		for rows.Next() {
			var b repo.ImportBatch
			if err = rows.Scan(&b.ID, &b.CarrierID, &b.Filename, &b.Status); err != nil {
				return fail(c, err)
			}
			out = append(out, b)
		}
		if err = rows.Err(); err != nil {
			return fail(c, err)
		}
		return c.JSON(200, map[string]any{"batches": out})
	})
	e.GET("/api/settings/logistics/freight-imports/:id", func(c echo.Context) error {
		bid, err := id(c, "id")
		if err != nil {
			return fail(c, err)
		}
		out, err := r.Batch(c.Request().Context(), bid)
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, out)
	})
	e.POST("/api/settings/logistics/freight-imports/:id/confirm", func(c echo.Context) error {
		bid, err := id(c, "id")
		if err != nil {
			return fail(c, err)
		}
		out, err := r.ConfirmImport(c.Request().Context(), bid, actor(c))
		if err != nil {
			return fail(c, err)
		}
		return c.JSON(200, out)
	})
	e.POST("/api/settings/logistics/freight-entries/:id/reverse", func(c echo.Context) error {
		eid, err := id(c, "id")
		if err != nil {
			return fail(c, err)
		}
		var req struct {
			RequestKey string `json:"request_key"`
			Reason     string `json:"reason"`
		}
		if err = c.Bind(&req); err != nil {
			return fail(c, err)
		}
		if err = r.ReverseEntry(c.Request().Context(), eid, req.RequestKey, req.Reason, actor(c)); err != nil {
			return fail(c, err)
		}
		return c.JSON(200, map[string]bool{"ok": true})
	})
}

var moneyPattern = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)

func parseWorkbook(src io.Reader, sheet string, trackCol, amountCol int) ([]repo.ImportRow, error) {
	if trackCol <= 0 || amountCol <= 0 || trackCol == amountCol {
		return nil, fmt.Errorf("请选择不同的运单号及实际运费列")
	}
	book, e := excelize.OpenReader(src, excelize.Options{UnzipSizeLimit: 64 << 20, UnzipXMLSizeLimit: 32 << 20})
	if e != nil {
		return nil, e
	}
	defer book.Close()
	rows, e := book.GetRows(sheet, excelize.Options{RawCellValue: true})
	if e != nil {
		return nil, e
	}
	if len(rows) > 10001 {
		return nil, fmt.Errorf("每次最多10000行")
	}
	out := []repo.ImportRow{}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		cell := func(col int) string {
			if col > len(row) {
				return ""
			}
			return strings.TrimSpace(row[col-1])
		}
		track, amount := cell(trackCol), cell(amountCol)
		if track == "" && amount == "" {
			continue
		}
		v := repo.ImportRow{RowNo: i + 1, TrackingNo: track}
		ta, _ := excelize.CoordinatesToCellName(trackCol, i+1)
		aa, _ := excelize.CoordinatesToCellName(amountCol, i+1)
		formula, _ := book.GetCellFormula(sheet, ta)
		formula2, _ := book.GetCellFormula(sheet, aa)
		typ, _ := book.GetCellType(sheet, ta)
		if formula != "" || formula2 != "" {
			v.Error = "公式单元格请转换为文本或最终金额"
		} else if strings.ContainsAny(track, "eE") && typ == excelize.CellTypeNumber || typ == excelize.CellTypeNumber && len(track) > 15 {
			v.Error = "长运单号请按文本保存，避免Excel精度丢失"
		} else if !moneyPattern.MatchString(amount) {
			v.Error = "实际运费需为非负金额，最多两位小数"
		} else {
			parts := strings.SplitN(amount, ".", 2)
			whole, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil || whole > domain.MaxCents/100 {
				v.Error = "金额超出范围"
			} else {
				v.ActualCents = whole * 100
				if len(parts) > 1 {
					c, _ := strconv.ParseInt(parts[1]+strings.Repeat("0", 2-len(parts[1])), 10, 64)
					v.ActualCents += c
				}
			}
		}
		out = append(out, v)
	}
	return out, nil
}
