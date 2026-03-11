package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
)

type ShipRow struct {
	OrderID     int64
	OrderNo     string
	CustomerID  int64
	RecvName    string
	RecvPhone   string
	RecvAddr    string
	RecvCompany string
	WeightKg    float64
}

type trackingPair struct {
	Phone    string
	Tracking string
}

func fetchShipRowsSFSmall(c echo.Context, pool *pgxpool.Pool, schema string, q, from, to, voidFilter string, customerID int64, payStatusID, shipStatusID, procStatusID int64, completedOnly bool, oneClick bool) ([]ShipRow, error) {
	// reuse same filters as fetchOrders but without pagination; only sf_small
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if q = strings.TrimSpace(q); q != "" {
		where = append(where, fmt.Sprintf("(o.order_no ILIKE $%d OR c.name ILIKE $%d)", argn, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	if customerID > 0 {
		where = append(where, fmt.Sprintf("o.customer_id = $%d", argn))
		args = append(args, customerID)
		argn++
	}
	if strings.TrimSpace(from) != "" {
		where = append(where, fmt.Sprintf("o.order_date >= $%d", argn))
		args = append(args, from)
		argn++
	}
	if strings.TrimSpace(to) != "" {
		where = append(where, fmt.Sprintf("o.order_date <= $%d", argn))
		args = append(args, to)
		argn++
	}

	voidFilter = strings.TrimSpace(voidFilter)
	switch voidFilter {
	case "void":
		where = append(where, "o.is_void = true")
	case "all":
		// no filter
	default:
		where = append(where, "o.is_void = false")
	}

	if payStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.pay_status_id,0) = $%d", argn))
		args = append(args, payStatusID)
		argn++
	}
	if shipStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.ship_status_id,0) = $%d", argn))
		args = append(args, shipStatusID)
		argn++
	}
	if procStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.process_status_id,0) = $%d", argn))
		args = append(args, procStatusID)
		argn++
	}
	if completedOnly {
		where = append(where, "COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4)")
	}

	if oneClick {
		// 一键发货：过滤流程状态=生产完成
		where = append(where, "EXISTS (SELECT 1 FROM "+schema+".order_process_statuses ops WHERE ops.id=o.process_status_id AND ops.name IN ('生产完成','已生产完成'))")
	} else {
		// 常规模板导出：仅 ship_method=sf_small
		where = append(where, "COALESCE(o.ship_method,'') = 'sf_small'")
	}

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}
	having := ""

	qsql := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(o.customer_id,0) AS customer_id,
			COALESCE(NULLIF(c.contact,''), c.name, '') AS recv_name,
			COALESCE(c.phone,'') AS recv_phone,
			COALESCE(c.address,'') AS recv_addr,
			'' AS recv_company,
			COALESCE(SUM(
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.qty::text,''), '[^0-9.\-]', '', 'g'), ''), '0')::numeric
				*
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec::text,''), '[^0-9.\-]', '', 'g'), ''), '0')::numeric
			),0) AS total_g
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.order_items oi ON oi.order_id=o.id
		%s
		GROUP BY o.id, o.order_no, o.customer_id, recv_name, recv_phone, recv_addr
		%s
		ORDER BY o.id DESC
	`, schema, schema, schema, wsql, having)

	rows, err := pool.Query(c.Request().Context(), qsql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ShipRow, 0)
	for rows.Next() {
		var r ShipRow
		var totalG float64
		if err := rows.Scan(&r.OrderID, &r.OrderNo, &r.CustomerID, &r.RecvName, &r.RecvPhone, &r.RecvAddr, &r.RecvCompany, &totalG); err != nil {
			return nil, err
		}
		r.WeightKg = totalG / 1000.0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

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

func digitsOnly(s string) string {
	b := strings.Builder{}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
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

func fillTrackingPairs(ctx context.Context, pool *pgxpool.Pool, schema string, pairs []trackingPair) (int, int, error) {
	if len(pairs) == 0 {
		return 0, 0, nil
	}
	group := map[string][]string{}
	for _, p := range pairs {
		ph := digitsOnly(p.Phone)
		if ph == "" || strings.TrimSpace(p.Tracking) == "" {
			continue
		}
		group[ph] = append(group[ph], strings.TrimSpace(p.Tracking))
	}
	updated := 0
	total := len(pairs)
	for phone, tracks := range group {
		rows, err := pool.Query(ctx, fmt.Sprintf(`
			SELECT o.id
			FROM %s.orders o
			JOIN %s.customers c ON c.id=o.customer_id
			WHERE o.is_void=false
			  AND COALESCE(o.ship_tracking_no,'')=''
			  AND regexp_replace(COALESCE(c.phone,''),'\\D','','g') = $1
			ORDER BY o.order_date, o.id
		`, schema, schema), phone)
		if err != nil {
			return updated, total, err
		}
		ids := make([]int64, 0)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		n := len(ids)
		if len(tracks) < n {
			n = len(tracks)
		}
		for i := 0; i < n; i++ {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_tracking_no=$2 WHERE id=$1`, schema), ids[i], tracks[i]); err != nil {
				return updated, total, err
			}
			updated++
		}
	}
	return updated, total, nil
}

func fillTrackingByPhone(ctx context.Context, pool *pgxpool.Pool, schema, raw string) (int, int, error) {
	return fillTrackingPairs(ctx, pool, schema, parseTrackingPairs(raw))
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

func registerShipExportRoutes(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/ship/sf_small.xlsx", func(c echo.Context) error {
		rows, err := fetchShipRowsSFSmall(c, pool, schema,
			strings.TrimSpace(c.QueryParam("q")),
			strings.TrimSpace(c.QueryParam("from")),
			strings.TrimSpace(c.QueryParam("to")),
			strings.TrimSpace(c.QueryParam("void")),
			func() int64 {
				v := strings.TrimSpace(c.QueryParam("customer_id"))
				if v == "" {
					return 0
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				return n
			}(),
			func() int64 {
				v := strings.TrimSpace(c.QueryParam("pay_status_id"))
				if v == "" {
					return 0
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				return n
			}(),
			func() int64 {
				v := strings.TrimSpace(c.QueryParam("ship_status_id"))
				if v == "" {
					return 0
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				return n
			}(),
			func() int64 {
				v := strings.TrimSpace(c.QueryParam("process_status_id"))
				if v == "" {
					return 0
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				return n
			}(),
			strings.TrimSpace(c.QueryParam("completed")) == "1",
			false,
		)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		tmpl := strings.TrimSpace(env("SF_SMALL_TEMPLATE", "/app/docs/ship_temp.xlsx"))
		wb, err := excelize.OpenFile(tmpl)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("open template %s: %v", tmpl, err))
		}
		sheet := wb.GetSheetName(0)
		startRow := 2

		senderName := env("SENDER_NAME", "")
		senderPhone := env("SENDER_PHONE", "")
		senderAddr := env("SENDER_ADDR", "")
		senderCompany := env("SENDER_COMPANY", "")
		goods := env("SENDER_GOODS", "咖啡")
		bizType := env("SF_BIZ_TYPE", "")

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
			updated, total, err := fillTrackingPairs(ctx, pool, schema, pairs)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			}
			return c.JSON(http.StatusOK, map[string]any{"ok": true, "updated": updated, "total": total})
		}

		raw := strings.TrimSpace(c.FormValue("entries"))
		updated, total, err := fillTrackingByPhone(ctx, pool, schema, raw)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "updated": updated, "total": total})
	})

	e.GET("/ship/sf_small_one_click.xlsx", func(c echo.Context) error {
		rows, err := fetchShipRowsSFSmall(c, pool, schema,
			strings.TrimSpace(c.QueryParam("q")),
			strings.TrimSpace(c.QueryParam("from")),
			strings.TrimSpace(c.QueryParam("to")),
			strings.TrimSpace(c.QueryParam("void")),
			func() int64 {
				v := strings.TrimSpace(c.QueryParam("customer_id"))
				if v == "" {
					return 0
				}
				n, _ := strconv.ParseInt(v, 10, 64)
				return n
			}(),
			0, 0, 0,
			false,
			true,
		)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}

		tmpl := strings.TrimSpace(env("SF_SMALL_TEMPLATE", "/app/docs/ship_temp.xlsx"))
		wb, err := excelize.OpenFile(tmpl)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("open template %s: %v", tmpl, err))
		}
		sheet := wb.GetSheetName(0)
		startRow := 2

		senderName := env("SENDER_NAME", "")
		senderPhone := env("SENDER_PHONE", "")
		senderAddr := env("SENDER_ADDR", "")
		senderCompany := env("SENDER_COMPANY", "")
		goods := env("SENDER_GOODS", "咖啡")
		bizType := env("SF_BIZ_TYPE", "")

		r := startRow
		for _, o := range rows {
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
