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

type manufacturingOperationRequest struct {
	ID                    int64   `json:"id"`
	Code                  string  `json:"code"`
	Name                  string  `json:"name"`
	Status                string  `json:"status"`
	DefaultMinutes        int     `json:"default_minutes"`
	StandardOperationCost float64 `json:"standard_operation_cost"`
	Note                  string  `json:"note"`
}

type manufacturingWorkstationRequest struct {
	ID                     int64   `json:"id"`
	Code                   string  `json:"code"`
	Name                   string  `json:"name"`
	Status                 string  `json:"status"`
	DefaultMinutes         int     `json:"default_minutes"`
	MachineHourlyCost      float64 `json:"machine_hourly_cost"`
	LaborHourlyCost        float64 `json:"labor_hourly_cost"`
	OverheadHourlyCost     float64 `json:"overhead_hourly_cost"`
	HourlyRate             float64 `json:"hourly_rate"`
	ApplicableOperationIDs []int64 `json:"applicable_operation_ids"`
	Note                   string  `json:"note"`
}

type workstationCapacityRequest struct {
	ID                     int64   `json:"id"`
	WorkstationID          int64   `json:"workstation_id"`
	Code                   string  `json:"code"`
	Name                   string  `json:"name"`
	Status                 string  `json:"status"`
	BatchSizeQty           float64 `json:"batch_size_qty"`
	BatchSizeUnit          string  `json:"batch_size_unit"`
	StandardMinutes        int     `json:"standard_minutes"`
	HourlyRate             float64 `json:"hourly_rate"`
	CostMethod             string  `json:"cost_method"`
	PieceRate              float64 `json:"piece_rate"`
	ProductionCapacity     int     `json:"production_capacity"`
	SortOrder              int     `json:"sort_order"`
	Note                   string  `json:"note"`
	ApplicableOperationIDs []int64 `json:"applicable_operation_ids"`
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
	e.GET("/api/manufacturing-operations", func(c echo.Context) error {
		rows, err := svc.ListManufacturingOperations(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/manufacturing-operations", func(c echo.Context) error {
		var req manufacturingOperationRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveManufacturingOperation(c.Request().Context(), manufacturingapp.SaveManufacturingOperationCommand{
			ID:                    req.ID,
			Code:                  req.Code,
			Name:                  req.Name,
			Status:                req.Status,
			DefaultMinutes:        req.DefaultMinutes,
			StandardOperationCost: req.StandardOperationCost,
			Note:                  req.Note,
			Actor:                 support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/manufacturing-operations/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateManufacturingOperation(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/manufacturing-workstations", func(c echo.Context) error {
		rows, err := svc.ListManufacturingWorkstations(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/manufacturing-workstations", func(c echo.Context) error {
		var req manufacturingWorkstationRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveManufacturingWorkstation(c.Request().Context(), manufacturingapp.SaveManufacturingWorkstationCommand{
			ID:                     req.ID,
			Code:                   req.Code,
			Name:                   req.Name,
			Status:                 req.Status,
			DefaultMinutes:         req.DefaultMinutes,
			MachineHourlyCost:      req.MachineHourlyCost,
			LaborHourlyCost:        req.LaborHourlyCost,
			OverheadHourlyCost:     req.OverheadHourlyCost,
			HourlyRate:             req.HourlyRate,
			ApplicableOperationIDs: req.ApplicableOperationIDs,
			Note:                   req.Note,
			Actor:                  support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/manufacturing-workstations/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateManufacturingWorkstation(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	e.GET("/api/manufacturing-workstation-capacities", func(c echo.Context) error {
		workstationID := int64(0)
		if raw := strings.TrimSpace(c.QueryParam("workstation_id")); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid workstation_id"})
			}
			workstationID = n
		}
		rows, err := svc.ListManufacturingWorkstationCapacities(c.Request().Context(), manufacturingapp.WorkstationCapacityQuery{
			WorkstationID: workstationID,
			Status:        strings.TrimSpace(c.QueryParam("status")),
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"rows": rows})
	})

	e.POST("/api/manufacturing-workstation-capacities", func(c echo.Context) error {
		var req workstationCapacityRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.SaveManufacturingWorkstationCapacity(c.Request().Context(), manufacturingapp.SaveWorkstationCapacityCommand{
			ID:                 req.ID,
			WorkstationID:      req.WorkstationID,
			Code:               req.Code,
			Name:               req.Name,
			Status:             req.Status,
			BatchSizeQty:       req.BatchSizeQty,
			BatchSizeUnit:      req.BatchSizeUnit,
			StandardMinutes:    req.StandardMinutes,
			HourlyRate:         req.HourlyRate,
			CostMethod:         req.CostMethod,
			PieceRate:          req.PieceRate,
			ProductionCapacity: req.ProductionCapacity,
			SortOrder:          req.SortOrder,
			Note:               req.Note,
			Actor:              support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
	})

	e.POST("/api/manufacturing-workstation-capacities/:id/deactivate", func(c echo.Context) error {
		id, err := parseIDParam(c, "id")
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		if err := svc.DeactivateManufacturingWorkstationCapacity(c.Request().Context(), manufacturingapp.TemplateStatusCommand{ID: id, Actor: support.ActorOf(c)}); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

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

	e.POST("/api/industry-calculators/preview", func(c echo.Context) error {
		var req manufacturingapp.IndustryCalculatorPreviewCommand
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := svc.PreviewIndustryCalculator(c.Request().Context(), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, row)
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
