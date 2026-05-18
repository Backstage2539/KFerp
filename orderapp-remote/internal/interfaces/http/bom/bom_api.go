package bom

import (
	"errors"
	"net/http"
	"strconv"

	bomapp "orderapp/internal/application/bom"
	"orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

type SaveBomRequest struct {
	ProductID int64 `json:"product_id"`
}

type SaveBomItemRequest struct {
	ProductID          int64   `json:"product_id"`
	MaterialID         int64   `json:"material_id"`
	ComponentType      string  `json:"component_type"`
	ComponentProductID int64   `json:"component_product_id"`
	ComponentSpecG     int64   `json:"component_spec_g"`
	ConsumeUnit        string  `json:"consume_unit"`
	QtyPerUnit         float64 `json:"qty_per_unit"`
	RatioPct           float64 `json:"ratio_pct"`
}

type DeleteBomItemRequest struct {
	ProductID int64 `json:"product_id"`
	ID        int64 `json:"id"`
}

type SaveBagSpecMappingRequest struct {
	SpecG      int64 `json:"spec_g"`
	MaterialID int64 `json:"material_id"`
}

type DeleteBagSpecMappingRequest struct {
	SpecG int64 `json:"spec_g"`
}

type CreateBomVersionRequest struct {
	ProductID int64  `json:"product_id"`
	Note      string `json:"note"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func registerBomAPI(e *echo.Echo, bomSvc *bomapp.Service) {
	e.GET("/api/bom/list", func(c echo.Context) error {
		rows, err := bomSvc.List(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/detail/:product_id", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		detail, err := bomSvc.Detail(c.Request().Context(), productID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.JSON(http.StatusNotFound, ErrorResponse{Error: "product not found"})
			}
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, detail)
	})

	e.GET("/api/bom/products", func(c echo.Context) error {
		rows, err := bomSvc.Products(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/materials", func(c echo.Context) error {
		rows, err := bomSvc.Materials(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/bag-spec-mappings", func(c echo.Context) error {
		rows, err := bomSvc.BagSpecMappings(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, rows)
	})

	e.GET("/api/bom/versions", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.QueryParam("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		rows, err := bomSvc.ListVersions(c.Request().Context(), productID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/bom/versions", func(c echo.Context) error {
		var req CreateBomVersionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		version, err := bomSvc.CreateVersion(c.Request().Context(), bomapp.CreateVersionCommand{ProductID: req.ProductID, Note: req.Note})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, version)
	})

	e.POST("/api/bom/versions/:id/activate", func(c echo.Context) error {
		versionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || versionID <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid version_id"})
		}
		if err := bomSvc.ActivateVersion(c.Request().Context(), versionID); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/bom/save", func(c echo.Context) error {
		var req SaveBomRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.SyncProductYield(c.Request().Context(), req.ProductID); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.DELETE("/api/bom/:product_id", func(c echo.Context) error {
		productID, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
		}
		if err := bomSvc.DeactivateBom(c.Request().Context(), productID); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/bom/item/save", func(c echo.Context) error {
		var req SaveBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		err := bomSvc.SaveItem(c.Request().Context(), bomapp.SaveItemCommand{
			ProductID:          req.ProductID,
			MaterialID:         req.MaterialID,
			ComponentType:      req.ComponentType,
			ComponentProductID: req.ComponentProductID,
			ComponentSpecG:     req.ComponentSpecG,
			ConsumeUnit:        req.ConsumeUnit,
			QtyPerUnit:         req.QtyPerUnit,
			RatioPct:           req.RatioPct,
			Actor:              support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/item/delete", func(c echo.Context) error {
		var req DeleteBomItemRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.DeleteItem(c.Request().Context(), bomapp.DeleteItemCommand{ProductID: req.ProductID, ID: req.ID, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/bag-spec-mappings/save", func(c echo.Context) error {
		var req SaveBagSpecMappingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		err := bomSvc.SaveBagSpecMapping(c.Request().Context(), bomapp.SaveBagSpecMappingCommand{
			SpecG:      req.SpecG,
			MaterialID: req.MaterialID,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})

	e.POST("/api/bom/bag-spec-mappings/delete", func(c echo.Context) error {
		var req DeleteBagSpecMappingRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		if err := bomSvc.DeleteBagSpecMapping(c.Request().Context(), req.SpecG); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.NoContent(http.StatusOK)
	})
}
