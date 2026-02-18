package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// JSON API response types
type BomListItem struct {
	ProductID  int64   `json:"product_id"`
	Product    string  `json:"product"`
	YieldRate  float64 `json:"yield_rate"`
	ItemCount  int     `json:"item_count"`
	UpdatedAt  string  `json:"updated_at"`
}

type BomItemJSON struct {
	ID           int64   `json:"id"`
	MaterialID   int64   `json:"material_id"`
	MaterialName string  `json:"material_name"`
	RatioPct     float64 `json:"ratio_pct"`
}

type BomDetailResponse struct {
	ProductID   int64         `json:"product_id"`
	ProductName string        `json:"product_name"`
	YieldRate   float64       `json:"yield_rate"`
	Items       []BomItemJSON `json:"items"`
	TotalRatio  float64       `json:"total_ratio"`
	UpdatedAt   string        `json:"updated_at"`
}

type OptionItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SaveBomRequest struct {
	ProductID int64   `json:"product_id"`
	YieldRate float64 `json:"yield_rate"`
}

type SaveBomItemRequest struct {
	ProductID  int64   `json:"product_id"`
	MaterialID int64   `json:"material_id"`
	RatioPct   float64 `json:"ratio_pct"`
}

type DeleteBomItemRequest struct {
	ProductID int64 `json:"product_id"`
	ID        int64 `json:"id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func registerBomAPI(e *echo.Echo, pool *pgxpool.Pool, schema string) {
	// GET /api/bom/list
	e.GET("/api/bom/list", func(c echo.Context) error {
		q := fmt.Sprintf(`
			SELECT 
				p.id,
				p.name,
				COALESCE(b.yield_rate, 0.8),
				COALESCE((SELECT COUNT(*) FROM %s.product_bom_items bi WHERE bi.product_id = p.id), 0),
				COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'), '-')
			FROM %s.products p
			LEFT JOIN %s.product_bom b ON b.product_id = p.id
			WHERE p.active = true
			ORDER BY p.name
		`, schema, schema, schema)
		rows, err := pool.Query(c.Request().Context(), q)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		defer rows.Close()

		result := make([]BomListItem, 0)
		for rows.Next() {
			var r BomListItem
			if err := rows.Scan(&r.ProductID, &r.Product, &r.YieldRate, &r.ItemCount, &r.UpdatedAt); err != nil {
				return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
			}
			result = append(result, r)
		}
		return c.JSON(http.StatusOK, result)
	})

	// GET /api/bom/detail/:product_id
	e.GET("/api/bom/detail/:product_id", func(c echo.Context) error {
		pid, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}

		// Get product name and yield rate
		var productName string
		var yieldRate float64
		var updatedAt string
		err = pool.QueryRow(c.Request().Context(),
			"SELECT COALESCE(p.name,''), COALESCE(b.yield_rate,0.8), COALESCE(to_char(b.updated_at,'YYYY-MM-DD HH24:MI'),'-') "+
			"FROM "+schema+".products p LEFT JOIN "+schema+".product_bom b ON b.product_id=p.id "+
			"WHERE p.id=$1", pid).Scan(&productName, &yieldRate, &updatedAt)
		if err != nil {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "product not found"})
		}

		// Get items
		items, total, err := listBomItems(c.Request().Context(), pool, schema, pid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}

		itemsJSON := make([]BomItemJSON, len(items))
		for i, item := range items {
			itemsJSON[i] = BomItemJSON{
				ID:           item.ID,
				MaterialID:   item.MaterialID,
				MaterialName: item.MaterialName,
				RatioPct:     item.RatioPct,
			}
		}

		return c.JSON(http.StatusOK, BomDetailResponse{
			ProductID:   pid,
			ProductName: productName,
			YieldRate:   yieldRate,
			Items:       itemsJSON,
			TotalRatio:  total,
			UpdatedAt:   updatedAt,
		})
	})

	// GET /api/bom/products
	e.GET("/api/bom/products", func(c echo.Context) error {
		opts, err := fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".products WHERE active=true ORDER BY name")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		result := make([]OptionItem, len(opts))
		for i, o := range opts {
			result[i] = OptionItem{ID: o.ID, Name: o.Name}
		}
		return c.JSON(http.StatusOK, result)
	})

	// GET /api/bom/materials - 返回所有物料（不区分生豆/耗材）
	e.GET("/api/bom/materials", func(c echo.Context) error {
		opts, err := fetchOptions(c.Request().Context(), pool, "SELECT id, name FROM "+schema+".materials ORDER BY name")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		result := make([]OptionItem, len(opts))
		for i, o := range opts {
			result[i] = OptionItem{ID: o.ID, Name: o.Name}
		}
		return c.JSON(http.StatusOK, result)
	})

	// POST /api/bom/save
	e.POST("/api/bom/save", func(c echo.Context) error {
		var req SaveBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if req.ProductID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "product required"})
		}
		if req.YieldRate <= 0 || req.YieldRate > 1 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "yield_rate must be (0,1]"})
		}

		q := "INSERT INTO " + schema + ".product_bom(product_id,yield_rate,updated_at) VALUES($1,$2,now()) ON CONFLICT (product_id) DO UPDATE SET yield_rate=excluded.yield_rate, updated_at=now()"
		if _, err := pool.Exec(c.Request().Context(), q, req.ProductID, req.YieldRate); err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	// POST /api/bom/item/save
	e.POST("/api/bom/item/save", func(c echo.Context) error {
		var req SaveBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if req.ProductID <= 0 || req.MaterialID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "product/material required"})
		}
		if req.RatioPct <= 0 || req.RatioPct > 100 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ratio must be (0,100]"})
		}

		// Check total ratio
		_, total, err := listBomItems(c.Request().Context(), pool, schema, req.ProductID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}

		// Get existing ratio for this material (if updating)
		var oldRatio float64
		pool.QueryRow(c.Request().Context(),
			"SELECT COALESCE(ratio_pct,0) FROM "+schema+".product_bom_items WHERE product_id=$1 AND material_id=$2",
			req.ProductID, req.MaterialID).Scan(&oldRatio)

		if total-oldRatio+req.RatioPct > 100.0001 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "ratio sum exceed 100%"})
		}

		q := "INSERT INTO " + schema + ".product_bom_items(product_id,material_id,ratio_pct,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT (product_id,material_id) DO UPDATE SET ratio_pct=excluded.ratio_pct, updated_at=now()"
		if _, err := pool.Exec(c.Request().Context(), q, req.ProductID, req.MaterialID, req.RatioPct); err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	// POST /api/bom/item/delete
	e.POST("/api/bom/item/delete", func(c echo.Context) error {
		var req DeleteBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if req.ID > 0 {
			_, _ = pool.Exec(c.Request().Context(), "DELETE FROM "+schema+".product_bom_items WHERE id=$1", req.ID)
		}
		return c.NoContent(http.StatusOK)
	})
}
