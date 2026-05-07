package sales

import (
	"fmt"
	"math"
	"net/http"
	salesapp "orderapp/internal/application/sales"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type ShipRow = salesapp.ShippingExportRow
type shippingExportRow = salesapp.ShippingExportRow
type trackingPair = salesapp.TrackingPair

func parseTrackingPairs(raw string) []trackingPair {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]trackingPair, 0)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var a, b string
		if strings.Contains(ln, "\t") {
			ss := strings.SplitN(ln, "\t", 2)
			a, b = ss[0], ss[1]
		} else if strings.Contains(ln, ",") {
			ss := strings.SplitN(ln, ",", 2)
			a, b = ss[0], ss[1]
		} else if strings.Contains(ln, "，") {
			ss := strings.SplitN(ln, "，", 2)
			a, b = ss[0], ss[1]
		} else {
			ss := strings.Fields(ln)
			if len(ss) >= 2 {
				a, b = ss[0], ss[1]
			}
		}
		a = strings.TrimSpace(a)
		b = strings.TrimSpace(b)
		if a == "" || b == "" {
			continue
		}
		out = append(out, trackingPair{Phone: a, Tracking: b})
	}
	return out
}

func parseTrackingPairsExcel(f *excelize.File) []trackingPair {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) == 0 {
		return nil
	}
	header := rows[0]
	phoneCol, trackCol := -1, -1
	for i, h := range header {
		x := strings.TrimSpace(h)
		if x == "" {
			continue
		}
		if phoneCol < 0 && (strings.Contains(x, "手机") || strings.Contains(x, "电话")) {
			phoneCol = i
		}
		if trackCol < 0 && (strings.Contains(x, "单号") || strings.Contains(x, "运单")) {
			trackCol = i
		}
	}
	start := 1
	if phoneCol < 0 || trackCol < 0 {
		start = 0
	}
	out := make([]trackingPair, 0)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		var p, t string
		if phoneCol >= 0 && phoneCol < len(row) {
			p = strings.TrimSpace(row[phoneCol])
		}
		if trackCol >= 0 && trackCol < len(row) {
			t = strings.TrimSpace(row[trackCol])
		}
		if p == "" || t == "" {
			vals := make([]string, 0)
			for _, c := range row {
				c = strings.TrimSpace(c)
				if c != "" {
					vals = append(vals, c)
				}
			}
			if len(vals) >= 2 {
				if p == "" {
					p = vals[0]
				}
				if t == "" {
					t = vals[1]
				}
			}
		}
		p = strings.TrimSpace(p)
		t = strings.TrimSpace(t)
		if p == "" || t == "" {
			continue
		}
		out = append(out, trackingPair{Phone: p, Tracking: t})
	}
	return out
}

func shipSplitCountSFSmall(weightKg float64) int {
	if weightKg <= 0 {
		return 1
	}
	// rule: <=5 => 1, 6=>2, 10=>2, 11=>3, 15=>3 ... => ceil(weight/5)
	n := int(math.Ceil(weightKg / 5.0))
	if n < 1 {
		return 1
	}
	return n
}

func shippingExportQueryFromContext(c echo.Context, oneClick bool) salesapp.ShippingExportQuery {
	parseIntParam := func(name string) int64 {
		v := strings.TrimSpace(c.QueryParam(name))
		if v == "" {
			return 0
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	query := salesapp.ShippingExportQuery{
		Q:             strings.TrimSpace(c.QueryParam("q")),
		From:          strings.TrimSpace(c.QueryParam("from")),
		To:            strings.TrimSpace(c.QueryParam("to")),
		Void:          strings.TrimSpace(c.QueryParam("void")),
		CustomerID:    parseIntParam("customer_id"),
		CompletedOnly: strings.TrimSpace(c.QueryParam("completed")) == "1",
		OneClick:      oneClick,
	}
	if !oneClick {
		query.PayStatusID = parseIntParam("pay_status_id")
		query.ShipStatusID = parseIntParam("ship_status_id")
		query.ProcessStatusID = parseIntParam("process_status_id")
	}
	return query
}

func registerShipExportRoutes(e *echo.Echo, salesSvc *salesapp.Service) {
	e.GET("/ship/sf_small.xlsx", func(c echo.Context) error {
		rows, err := salesSvc.ListSFSmallShippingRows(c.Request().Context(), shippingExportQueryFromContext(c, false))
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		tmpl, err := orderShippingTemplatePath()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		wb, err := excelize.OpenFile(tmpl)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("open template %s: %v", tmpl, err))
		}
		sheet := wb.GetSheetName(0)
		startRow := 2
		wb.SetCellValue(sheet, "R1", "快递单号")

		sender, err := salesSvc.LoadSenderProfile(c.Request().Context())
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		senderName := sender.Name
		senderPhone := sender.Phone
		senderAddr := sender.Addr
		senderCompany := sender.Company
		goods := sender.Goods
		bizType := sender.BizType

		r := startRow
		for _, o := range rows {
			cnt := shipSplitCountSFSmall(o.WeightKg)
			for i := 1; i <= cnt; i++ {
				remark := fmt.Sprintf("%s %d/%d", o.OrderNo, i, cnt)
				// columns (A-Q):
				// A收件人 B收件人手机/电话 C收件地址 D寄件人 E寄件人手机/电话 F寄件地址
				// G收件公司 H包裹件数 I托寄物 J重量 K长 L宽 M高 N备注 O寄件公司 P业务类型 Q包装服务费
				wb.SetCellValue(sheet, fmt.Sprintf("A%d", r), o.RecvName)
				wb.SetCellValue(sheet, fmt.Sprintf("B%d", r), o.RecvPhone)
				wb.SetCellValue(sheet, fmt.Sprintf("C%d", r), o.RecvAddr)
				wb.SetCellValue(sheet, fmt.Sprintf("D%d", r), senderName)
				wb.SetCellValue(sheet, fmt.Sprintf("E%d", r), senderPhone)
				wb.SetCellValue(sheet, fmt.Sprintf("F%d", r), senderAddr)
				wb.SetCellValue(sheet, fmt.Sprintf("G%d", r), o.RecvCompany)
				wb.SetCellValue(sheet, fmt.Sprintf("H%d", r), 1)
				wb.SetCellValue(sheet, fmt.Sprintf("I%d", r), goods)
				wb.SetCellValue(sheet, fmt.Sprintf("J%d", r), 1) // fixed 1kg per package
				wb.SetCellValue(sheet, fmt.Sprintf("N%d", r), remark)
				wb.SetCellValue(sheet, fmt.Sprintf("O%d", r), senderCompany)
				wb.SetCellValue(sheet, fmt.Sprintf("P%d", r), bizType)
				wb.SetCellValue(sheet, fmt.Sprintf("R%d", r), o.TrackingNo)
				r++
			}
		}

		buf, err := wb.WriteToBuffer()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		fn := fmt.Sprintf("sf_small_%s.xlsx", time.Now().Format("20060102_150405"))
		c.Response().Header().Set(echo.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", fn))
		return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
	})

	e.POST("/ship/tracking_fill", func(c echo.Context) error {
		ctx := c.Request().Context()
		if fh, err := c.FormFile("file"); err == nil && fh != nil {
			f, err := fh.Open()
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			}
			defer f.Close()
			x, err := excelize.OpenReader(f)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": "excel解析失败"})
			}
			pairs := parseTrackingPairsExcel(x)
			res, err := salesSvc.FillTrackingPairs(ctx, salesapp.FillTrackingPairsCommand{
				Actor: support.ActorOf(c),
				Pairs: pairs,
			})
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"ok": true, "updated": res.Updated, "total": res.Total})
		}

		raw := strings.TrimSpace(c.FormValue("entries"))
		res, err := salesSvc.FillTrackingPairs(ctx, salesapp.FillTrackingPairsCommand{
			Actor: support.ActorOf(c),
			Pairs: parseTrackingPairs(raw),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "updated": res.Updated, "total": res.Total})
	})

	e.GET("/ship/sf_small_one_click.xlsx", func(c echo.Context) error {
		rows, err := salesSvc.ListSFSmallShippingRows(c.Request().Context(), shippingExportQueryFromContext(c, true))
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		tmpl, err := orderShippingTemplatePath()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		wb, err := excelize.OpenFile(tmpl)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("open template %s: %v", tmpl, err))
		}
		sheet := wb.GetSheetName(0)
		startRow := 2
		wb.SetCellValue(sheet, "R1", "快递单号")

		sender, err := salesSvc.LoadSenderProfile(c.Request().Context())
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		senderName := sender.Name
		senderPhone := sender.Phone
		senderAddr := sender.Addr
		senderCompany := sender.Company
		goods := sender.Goods
		bizType := sender.BizType

		heavyIDs := make([]int64, 0)
		exportRows := make([]shippingExportRow, 0, len(rows))
		for _, o := range rows {
			if o.WeightKg > 15 {
				heavyIDs = append(heavyIDs, o.OrderID)
				continue
			}
			exportRows = append(exportRows, o)
		}
		if len(heavyIDs) > 0 {
			if err := salesSvc.SetShipMethod(c.Request().Context(), salesapp.SetShipMethodCommand{Actor: support.ActorOf(c), OrderIDs: heavyIDs, Method: "sf_large"}); err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
		}

		r := startRow
		for _, o := range exportRows {
			cnt := shipSplitCountSFSmall(o.WeightKg)
			for i := 1; i <= cnt; i++ {
				remark := fmt.Sprintf("%s %d/%d", o.OrderNo, i, cnt)
				wb.SetCellValue(sheet, fmt.Sprintf("A%d", r), o.RecvName)
				wb.SetCellValue(sheet, fmt.Sprintf("B%d", r), o.RecvPhone)
				wb.SetCellValue(sheet, fmt.Sprintf("C%d", r), o.RecvAddr)
				wb.SetCellValue(sheet, fmt.Sprintf("D%d", r), senderName)
				wb.SetCellValue(sheet, fmt.Sprintf("E%d", r), senderPhone)
				wb.SetCellValue(sheet, fmt.Sprintf("F%d", r), senderAddr)
				wb.SetCellValue(sheet, fmt.Sprintf("G%d", r), o.RecvCompany)
				wb.SetCellValue(sheet, fmt.Sprintf("H%d", r), 1)
				wb.SetCellValue(sheet, fmt.Sprintf("I%d", r), goods)
				wb.SetCellValue(sheet, fmt.Sprintf("J%d", r), 1)
				wb.SetCellValue(sheet, fmt.Sprintf("N%d", r), remark)
				wb.SetCellValue(sheet, fmt.Sprintf("O%d", r), senderCompany)
				wb.SetCellValue(sheet, fmt.Sprintf("P%d", r), bizType)
				wb.SetCellValue(sheet, fmt.Sprintf("R%d", r), o.TrackingNo)
				r++
			}
		}

		buf, err := wb.WriteToBuffer()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		fn := fmt.Sprintf("sf_small_one_click_%s.xlsx", time.Now().Format("20060102_150405"))
		c.Response().Header().Set(echo.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", fn))
		return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
	})
}
