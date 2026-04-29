package costing

import (
	"fmt"
	"net/http"
	appcosting "orderapp/internal/application/costing"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

func registerCostingAPI(e *echo.Echo, svc Service) {
	e.GET("/public/bean-list/:list_type", func(c echo.Context) error {
		listType := c.Param("list_type")
		row, err := svc.PublishedBeanList(c.Request().Context(), appcosting.BeanListPublicationQuery{
			ListType:  listType,
			OwnerType: "official",
		})
		if err != nil {
			return c.HTML(http.StatusBadRequest, renderNoPublishedBeanListPage(listType))
		}
		if row == nil {
			return c.HTML(http.StatusNotFound, renderNoPublishedBeanListPage(listType))
		}
		page, err := renderPublicBeanListPage(*row)
		if err != nil {
			return c.HTML(http.StatusInternalServerError, renderNoPublishedBeanListPage(listType))
		}
		return c.HTML(http.StatusOK, page)
	})

	e.GET("/api/costing/parameters", func(c echo.Context) error {
		params, err := svc.Parameters(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, params)
	})

	e.GET("/api/costing/settings", func(c echo.Context) error {
		rows, err := svc.Settings(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/costing/settings/:key", func(c echo.Context) error {
		var req struct {
			Value float64 `json:"value"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		row, err := svc.UpdateSetting(c.Request().Context(), appcosting.UpdateParameterCommand{
			Key:   c.Param("key"),
			Value: req.Value,
			Actor: support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/costing/calculate", func(c echo.Context) error {
		var req appcosting.CalculateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		resp, err := svc.Calculate(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/costing/bean-list", func(c echo.Context) error {
		resp, err := svc.BeanList(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/api/costing/bean-list/publications", func(c echo.Context) error {
		query := beanListPublicationQueryFromRequest(c)
		rows, err := svc.ListBeanListPublications(c.Request().Context(), query)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/costing/bean-list/publications", func(c echo.Context) error {
		var req appcosting.PublishBeanListCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		req.Actor = support.ActorOf(c)
		ownerType, ownerKey := beanListOwnerFromScope(c, req.Scope)
		req.OwnerType = ownerType
		req.OwnerKey = ownerKey
		row, err := svc.PublishBeanList(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/costing/bean-list/publications/:id/withdraw", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		query := beanListPublicationQueryFromRequest(c)
		if err := svc.WithdrawBeanList(c.Request().Context(), appcosting.WithdrawBeanListCommand{
			ID:        id,
			OwnerType: query.OwnerType,
			OwnerKey:  query.OwnerKey,
			Actor:     support.ActorOf(c),
		}); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})

	e.POST("/api/costing/runs", func(c echo.Context) error {
		run, err := svc.CreateRun(c.Request().Context(), support.ActorOf(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, run)
	})

	e.POST("/api/costing/runs/:id/publish", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
		}
		if err := svc.PublishRun(c.Request().Context(), support.ActorOf(c), id); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true, "id": id})
	})
}

func beanListPublicationQueryFromRequest(c echo.Context) appcosting.BeanListPublicationQuery {
	ownerType, ownerKey := beanListOwnerFromScope(c, c.QueryParam("scope"))
	return appcosting.BeanListPublicationQuery{
		ListType:  c.QueryParam("list_type"),
		Scope:     c.QueryParam("scope"),
		OwnerType: ownerType,
		OwnerKey:  ownerKey,
	}
}

func beanListOwnerFromScope(c echo.Context, scope string) (string, string) {
	switch strings.TrimSpace(scope) {
	case "mine":
		if v := c.Get("employee_id"); v != nil {
			if id, ok := v.(int64); ok && id > 0 {
				return "actor", fmt.Sprintf("employee:%d", id)
			}
		}
		return "actor", "actor:" + support.ActorOf(c)
	default:
		return "official", ""
	}
}
