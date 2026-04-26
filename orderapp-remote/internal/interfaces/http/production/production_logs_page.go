package production

import (
	"context"
	"net/http"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type ProductionLogRow struct {
	ID                    int64   `json:"id"`
	BatchID               string  `json:"batch_id"`
	ProductID             int64   `json:"product_id"`
	ProductName           string  `json:"product_name"`
	SpecG                 int64   `json:"spec_g"`
	OrderNos              string  `json:"order_nos"`
	PlannedNeedG          int64   `json:"planned_need_g"`
	InputG                int64   `json:"input_g"`
	BomYieldRate          float64 `json:"bom_yield_rate"`
	FinishedUnits         int64   `json:"finished_units"`
	FinishedLooseG        int64   `json:"finished_loose_g"`
	FinishedTotalG        int64   `json:"finished_total_g"`
	ActualYieldRate       float64 `json:"actual_yield_rate"`
	StartedBy             string  `json:"started_by"`
	StartedAt             string  `json:"started_at"`
	FinishedBy            string  `json:"finished_by"`
	FinishedAt            string  `json:"finished_at"`
	InventoryUnitsBefore  int64   `json:"inventory_units_before"`
	InventoryLooseGBefore int64   `json:"inventory_loose_g_before"`
	InventoryUnitsAfter   int64   `json:"inventory_units_after"`
	InventoryLooseGAfter  int64   `json:"inventory_loose_g_after"`
	MaterialSummary       string  `json:"material_summary"`
}

type ProductionLogsPageData struct {
	From      string
	To        string
	ProductID int64
	BatchID   string
	Operator  string
	Products  []productionProductOption
	Rows      []ProductionLogRow
	Error     string
}

type ProductionLogsAPIResponse struct {
	Products []productionProductOption `json:"products"`
	Rows     []ProductionLogRow        `json:"rows"`
}

type productionProductOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func registerProductionLogPages(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	e.GET("/produce/logs", func(c echo.Context) error {
		target := "/vue-shell?view=produceLogs"
		if raw := c.QueryString(); raw != "" {
			target += "&" + raw
		}
		return c.Redirect(http.StatusFound, target)
	})

	e.GET("/api/produce/logs", func(c echo.Context) error {
		data := parseProductionLogsQuery(c)
		if products, err := postgresinfra.FetchProducts(c.Request().Context(), pool, schema); err == nil {
			data.Products = make([]productionProductOption, 0, len(products))
			for _, p := range products {
				data.Products = append(data.Products, productionProductOption{ID: p.ID, Name: p.Name})
			}
		}
		rows, err := listProductionLogs(c.Request().Context(), pool, schema, data.From, data.To, data.ProductID, data.BatchID, data.Operator, 200)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, ProductionLogsAPIResponse{Products: data.Products, Rows: rows})
	})
}

func parseProductionLogsQuery(c echo.Context) ProductionLogsPageData {
	data := ProductionLogsPageData{
		From:     strings.TrimSpace(c.QueryParam("from")),
		To:       strings.TrimSpace(c.QueryParam("to")),
		BatchID:  strings.TrimSpace(c.QueryParam("batch_id")),
		Operator: strings.TrimSpace(c.QueryParam("operator")),
	}
	if v := strings.TrimSpace(c.QueryParam("product_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			data.ProductID = n
		}
	}
	return data
}

func listProductionLogs(ctx context.Context, pool *pgxpool.Pool, schema, from, to string, productID int64, batchID, operator string, limit int) ([]ProductionLogRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args := []any{}
	where := "WHERE 1=1"
	if productID > 0 {
		args = append(args, productID)
		where += " AND product_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(batchID) != "" {
		args = append(args, strings.TrimSpace(batchID))
		where += " AND batch_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(operator) != "" {
		args = append(args, strings.TrimSpace(operator))
		where += " AND finished_by=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(from) != "" {
		args = append(args, strings.TrimSpace(from))
		where += " AND finished_at >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(to) != "" {
		args = append(args, strings.TrimSpace(to))
		where += " AND finished_at < ($" + strconv.Itoa(len(args)) + "::date + INTERVAL '1 day')"
	}
	args = append(args, limit)

	q := `
		SELECT id,batch_id,product_id,product_name,spec_g,order_nos,
		       planned_need_g,input_g,bom_yield_rate,
		       finished_units,finished_loose_g,finished_total_g,actual_yield_rate,
		       started_by,COALESCE(to_char(started_at,'YYYY-MM-DD HH24:MI'),''),
		       finished_by,COALESCE(to_char(finished_at,'YYYY-MM-DD HH24:MI'),''),
		       inventory_units_before,inventory_loose_g_before,
		       inventory_units_after,inventory_loose_g_after,
		       COALESCE(material_summary::text,'[]')
		FROM ` + schema + `.production_logs
		` + where + `
		ORDER BY finished_at DESC NULLS LAST, id DESC
		LIMIT $` + strconv.Itoa(len(args))

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProductionLogRow, 0)
	for rows.Next() {
		var r ProductionLogRow
		if err := rows.Scan(
			&r.ID, &r.BatchID, &r.ProductID, &r.ProductName, &r.SpecG, &r.OrderNos,
			&r.PlannedNeedG, &r.InputG, &r.BomYieldRate,
			&r.FinishedUnits, &r.FinishedLooseG, &r.FinishedTotalG, &r.ActualYieldRate,
			&r.StartedBy, &r.StartedAt,
			&r.FinishedBy, &r.FinishedAt,
			&r.InventoryUnitsBefore, &r.InventoryLooseGBefore,
			&r.InventoryUnitsAfter, &r.InventoryLooseGAfter,
			&r.MaterialSummary,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
