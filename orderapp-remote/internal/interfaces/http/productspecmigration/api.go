package productspecmigration

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

func registerMigrationAPI(e *echo.Echo, svc *productspecmigrationapp.Service) {
	get := func(c echo.Context) error {
		id, ok := productID(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
		}
		row, err := svc.Get(c.Request().Context(), id)
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	prepare := func(c echo.Context) error {
		id, ok := productID(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
		}
		row, err := svc.Prepare(c.Request().Context(), productspecmigrationapp.PrepareCommand{ProductID: id, Actor: support.ActorOf(c)})
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	assess := func(c echo.Context) error {
		id, ok := productID(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
		}
		row, err := svc.Assess(c.Request().Context(), productspecmigrationapp.AssessCommand{ProductID: id, Actor: support.ActorOf(c)})
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	cutover := func(c echo.Context) error {
		id, ok := productID(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
		}
		row, err := svc.Cutover(c.Request().Context(), productspecmigrationapp.CutoverCommand{ProductID: id, Actor: support.ActorOf(c)})
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, row)
	}
	options := func(c echo.Context) error {
		id, ok := productID(c)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid product_id"})
		}
		rows, err := svc.ListOptions(c.Request().Context(), id)
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	}

	// Product-scoped routes are canonical. The collection-style aliases keep
	// rollout clients compatible while the admin screen transitions.
	e.GET("/api/products/:product_id/bom-spec-migration", get)
	e.POST("/api/products/:product_id/bom-spec-migration/prepare", prepare)
	e.POST("/api/products/:product_id/bom-spec-migration/readiness", assess)
	e.POST("/api/products/:product_id/bom-spec-migration/cutover", cutover)
	e.GET("/api/products/:product_id/bom-spec-options", options)
	e.GET("/api/product-bom-spec-migrations/:product_id", get)
	e.POST("/api/product-bom-spec-migrations/:product_id/prepare", prepare)
	e.POST("/api/product-bom-spec-migrations/:product_id/readiness", assess)
	e.POST("/api/product-bom-spec-migrations/:product_id/cutover", cutover)
	e.POST("/api/product-business-identities/resolve", func(c echo.Context) error {
		var req struct {
			ProductID    int64  `json:"product_id"`
			BomSpecID    *int64 `json:"bom_spec_id"`
			BomVariantID *int64 `json:"bom_variant_id"`
			LegacySpecG  int64  `json:"legacy_spec_g"`
			Mode         string `json:"mode"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid request"})
		}
		cmd := productspecmigrationapp.ResolveIdentityCommand{
			ProductID: req.ProductID, BomSpecID: req.BomSpecID, BomVariantID: req.BomVariantID, LegacySpecG: req.LegacySpecG,
		}
		var identity productspecmigrationapp.BusinessIdentity
		var err error
		if strings.EqualFold(strings.TrimSpace(req.Mode), string(productspecmigrationapp.ResolveWrite)) {
			identity, err = svc.ResolveForWrite(c.Request().Context(), cmd)
		} else {
			identity, err = svc.ResolveForRead(c.Request().Context(), cmd)
		}
		if err != nil {
			return migrationError(c, err)
		}
		return c.JSON(http.StatusOK, identity)
	})
}

func productID(c echo.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("product_id")), 10, 64)
	return id, err == nil && id > 0
}

func migrationError(c echo.Context, err error) error {
	var blocked *productspecmigrationapp.CutoverBlockedError
	if errors.As(err, &blocked) {
		return c.JSON(http.StatusConflict, map[string]any{
			"error":     err.Error(),
			"readiness": blocked.Readiness,
			"blockers":  blocked.Readiness.Blockers,
		})
	}
	switch {
	case errors.Is(err, productspecmigrationapp.ErrProductRequired), errors.Is(err, productspecmigrationapp.ErrActorRequired):
		return c.JSON(http.StatusBadRequest, map[string]any{"error": err.Error()})
	case errors.Is(err, productspecmigrationapp.ErrMigrationNotFound):
		return c.JSON(http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, productspecmigrationapp.ErrLegacyWriteRejected):
		return c.JSON(http.StatusConflict, map[string]any{"error": err.Error(), "code": "legacy_write_rejected"})
	case errors.Is(err, productspecmigrationapp.ErrBomSpecRequired):
		return c.JSON(http.StatusConflict, map[string]any{"error": err.Error(), "code": "bom_spec_id_required"})
	case errors.Is(err, productspecmigrationapp.ErrBomSpecUnavailable):
		return c.JSON(http.StatusConflict, map[string]any{"error": err.Error(), "code": "bom_spec_not_published"})
	default:
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}
}
