package manufacturing

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	manufacturingapp "orderapp/internal/application/manufacturing"
	"orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type industryTemplateRequest struct {
	ID          int64                                      `json:"id"`
	Name        string                                     `json:"name"`
	IndustryKey string                                     `json:"industry_key"`
	Description string                                     `json:"description"`
	Status      string                                     `json:"status"`
	Fields      []manufacturingapp.IndustryFieldDefinition `json:"fields"`
}

type processTemplateRequest struct {
	ID                 int64                                       `json:"id"`
	Name               string                                      `json:"name"`
	ProductID          int64                                       `json:"product_id"`
	BomVersionID       int64                                       `json:"bom_version_id"`
	IndustryTemplateID int64                                       `json:"industry_template_id"`
	Status             string                                      `json:"status"`
	DefaultEquipment   string                                      `json:"default_equipment"`
	DefaultMinutes     int                                         `json:"default_minutes"`
	KeyParamsJSON      string                                      `json:"key_params_json"`
	Note               string                                      `json:"note"`
	Operations         []manufacturingapp.ProcessTemplateOperation `json:"operations"`
}

type processRouteRequest struct {
	ID               int64                                    `json:"id"`
	Name             string                                   `json:"name"`
	Status           string                                   `json:"status"`
	DefaultEquipment string                                   `json:"default_equipment"`
	DefaultMinutes   int                                      `json:"default_minutes"`
	Note             string                                   `json:"note"`
	Operations       []manufacturingapp.ProcessRouteOperation `json:"operations"`
}

func registerAPI(e *echo.Echo, svc *manufacturingapp.Service) {
	e.GET("/api/industry-field-templates", func(c echo.Context) error {
		rows, err := svc.ListIndustryTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/industry-field-templates", func(c echo.Context) error {
		var req industryTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveIndustryTemplate(c.Request().Context(), manufacturingapp.SaveIndustryTemplateCommand{
			ID:          req.ID,
			Name:        req.Name,
			IndustryKey: req.IndustryKey,
			Description: req.Description,
			Status:      req.Status,
			Fields:      req.Fields,
			Actor:       support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/industry-field-templates/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateIndustryTemplate(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/process-templates", func(c echo.Context) error {
		productID := int64(0)
		if raw := strings.TrimSpace(c.QueryParam("product_id")); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid product_id"})
			}
			productID = n
		}
		rows, err := svc.ListProcessTemplates(c.Request().Context(), manufacturingapp.ProcessTemplateQuery{
			ProductID: productID,
			Status:    strings.TrimSpace(c.QueryParam("status")),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/process-templates", func(c echo.Context) error {
		var req processTemplateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveProcessTemplate(c.Request().Context(), manufacturingapp.SaveProcessTemplateCommand{
			ID:                 req.ID,
			Name:               req.Name,
			ProductID:          req.ProductID,
			BomVersionID:       req.BomVersionID,
			IndustryTemplateID: req.IndustryTemplateID,
			Status:             req.Status,
			DefaultEquipment:   req.DefaultEquipment,
			DefaultMinutes:     req.DefaultMinutes,
			KeyParamsJSON:      req.KeyParamsJSON,
			Note:               req.Note,
			Operations:         req.Operations,
			Actor:              support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/process-templates/:id/publish", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.PublishProcessTemplate(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/process-templates/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateProcessTemplate(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/process-routes", func(c echo.Context) error {
		rows, err := svc.ListProcessRoutes(c.Request().Context(), manufacturingapp.ProcessRouteQuery{
			Status: strings.TrimSpace(c.QueryParam("status")),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/process-routes", func(c echo.Context) error {
		var req processRouteRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveProcessRoute(c.Request().Context(), manufacturingapp.SaveProcessRouteCommand{
			ID:               req.ID,
			Name:             req.Name,
			Status:           req.Status,
			DefaultEquipment: req.DefaultEquipment,
			DefaultMinutes:   req.DefaultMinutes,
			Note:             req.Note,
			Operations:       req.Operations,
			Actor:            support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/process-routes/:id/publish", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.PublishProcessRoute(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.POST("/api/process-routes/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateProcessRoute(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})
}

func parseIDParam(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}
