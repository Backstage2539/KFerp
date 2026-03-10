package main

import (
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

	// only ship_method=sf_small
	where = append(where, "COALESCE(o.ship_method,'') = 'sf_small'")
	if oneClick {
		// 一键发货：仅生产完成订单
		where = append(where, "EXISTS (SELECT 1 FROM "+schema+".order_process_statuses ops WHERE ops.id=o.process_status_id AND ops.name IN ('生产完成','已生产完成'))")
	}

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}
	having := ""
	if oneClick {
		having = "HAVING COALESCE(SUM( (COALESCE(NULLIF(oi.qty::text,''),'0')::numeric) * (COALESCE(NULLIF(oi.spec::text,''),'0')::numeric) ),0) <= 15000"
	}

	qsql := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(o.customer_id,0) AS customer_id,
			COALESCE(NULLIF(c.contact,''), c.name, '') AS recv_name,
			COALESCE(c.phone,'') AS recv_phone,
			COALESCE(c.address,'') AS recv_addr,
			'' AS recv_company,
			COALESCE(SUM( (COALESCE(NULLIF(oi.qty::text,''),'0')::numeric) * (COALESCE(NULLIF(oi.spec::text,''),'0')::numeric) ),0) AS total_g
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
