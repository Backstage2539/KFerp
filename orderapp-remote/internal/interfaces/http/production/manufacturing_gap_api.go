package production

import (
	"net/http"
	"strconv"
	"strings"

	productionapp "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"

	"github.com/labstack/echo/v4"
)

type MaterialPlanAPIResponse struct {
	Rows []productionapp.MaterialPlanRow `json:"rows"`
}

type WIPReservationListAPIResponse struct {
	Rows            []productionapp.WIPReservationRow `json:"rows"`
	TotalReservedG  int64                             `json:"total_reserved_g"`
	TotalConsumedG  int64                             `json:"total_consumed_g"`
	TotalRemainingG int64                             `json:"total_remaining_g"`
}

type WIPReservationAdjustAPIRequest struct {
	ReservationID int64  `json:"reservation_id"`
	ReservedG     int64  `json:"reserved_g"`
	ReservedUnits int64  `json:"reserved_units"`
	Note          string `json:"note"`
}

type WIPReservationAdjustAPIResponse struct {
	OK  bool                            `json:"ok"`
	Row productionapp.WIPReservationRow `json:"row"`
}

type WIPReservationReleaseAPIRequest struct {
	RunningItemID int64  `json:"running_item_id"`
	WorkOrderNo   string `json:"work_order_no"`
	Note          string `json:"note"`
}

type WIPReservationReleaseAPIResponse struct {
	OK     bool                                      `json:"ok"`
	Result productionapp.WIPReservationReleaseResult `json:"result"`
}

type QualityInspectionAPIRequest struct {
	Scope         string `json:"scope"`
	ReferenceType string `json:"reference_type"`
	ReferenceNo   string `json:"reference_no"`
	ItemName      string `json:"item_name"`
	Result        string `json:"result"`
	MetricsJSON   string `json:"metrics_json"`
	Note          string `json:"note"`
}

type QualityInspectionCreateAPIResponse struct {
	OK  bool                               `json:"ok"`
	Row productionapp.QualityInspectionRow `json:"row"`
}

type QualityInspectionListAPIResponse struct {
	Rows []productionapp.QualityInspectionRow `json:"rows"`
}

func registerManufacturingGapAPI(e *echo.Echo, productionSvc *productionapp.Service) {
	e.GET("/api/produce/material-plan", func(c echo.Context) error {
		query := productionapp.MaterialPlanQuery{
			From:       c.QueryParam("from"),
			To:         c.QueryParam("to"),
			CustomerID: parseInt64(c.QueryParam("customer_id")),
			Selected:   parseSelectedKeys(c.QueryParam("selected")),
			InputByKey: parseInputByKey(c.QueryParams()),
		}
		res, err := productionSvc.MaterialPlan(c.Request().Context(), query)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, MaterialPlanAPIResponse{Rows: res.Rows})
	})

	e.GET("/api/produce/acceptance-smoke", func(c echo.Context) error {
		res, err := productionSvc.AcceptanceSmoke(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, res)
	})

	e.GET("/api/produce/wip-reservations", func(c echo.Context) error {
		res, err := productionSvc.ListWIPReservations(c.Request().Context(), productionapp.WIPReservationQuery{
			Status:      c.QueryParam("status"),
			WorkOrderNo: c.QueryParam("work_order_no"),
			MaterialID:  parseInt64(c.QueryParam("material_id")),
			Limit:       parseInt(c.QueryParam("limit")),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, WIPReservationListAPIResponse{
			Rows:            res.Rows,
			TotalReservedG:  res.TotalReservedG,
			TotalConsumedG:  res.TotalConsumedG,
			TotalRemainingG: res.TotalRemainingG,
		})
	})

	e.POST("/api/produce/wip-reservations/adjust", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req WIPReservationAdjustAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := productionSvc.AdjustWIPReservation(c.Request().Context(), productionapp.WIPReservationAdjustCommand{
			ReservationID: req.ReservationID,
			ReservedG:     req.ReservedG,
			ReservedUnits: req.ReservedUnits,
			Operator:      support.ActorOf(c),
			Note:          req.Note,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, WIPReservationAdjustAPIResponse{OK: true, Row: row})
	})

	e.POST("/api/produce/wip-reservations/release", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req WIPReservationReleaseAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		result, err := productionSvc.ReleaseWIPReservations(c.Request().Context(), productionapp.WIPReservationReleaseCommand{
			RunningItemID: req.RunningItemID,
			WorkOrderNo:   req.WorkOrderNo,
			Operator:      support.ActorOf(c),
			Note:          req.Note,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, WIPReservationReleaseAPIResponse{OK: true, Result: result})
	})

	e.GET("/api/produce/quality-inspections", func(c echo.Context) error {
		rows, err := productionSvc.ListQualityInspections(c.Request().Context(), productionapp.QualityInspectionQuery{
			Scope:  c.QueryParam("scope"),
			Result: c.QueryParam("result"),
			Limit:  parseInt(c.QueryParam("limit")),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, QualityInspectionListAPIResponse{Rows: rows})
	})

	e.POST("/api/produce/quality-inspections", func(c echo.Context) error {
		if err := support.RequireEmployeeBound(c); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		var req QualityInspectionAPIRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
		}
		row, err := productionSvc.CreateQualityInspection(c.Request().Context(), productionapp.QualityInspectionCommand{
			Scope:         req.Scope,
			ReferenceType: req.ReferenceType,
			ReferenceNo:   req.ReferenceNo,
			ItemName:      req.ItemName,
			Result:        req.Result,
			MetricsJSON:   req.MetricsJSON,
			Note:          req.Note,
			Operator:      support.ActorOf(c),
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, QualityInspectionCreateAPIResponse{OK: true, Row: row})
	})
}

func parseSelectedKeys(raw string) map[string]bool {
	selected := map[string]bool{}
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		key := strings.TrimSpace(part)
		if key != "" {
			selected[key] = true
		}
	}
	return selected
}

func parseInputByKey(values map[string][]string) map[string]int64 {
	out := map[string]int64{}
	for key, rawValues := range values {
		if !strings.HasPrefix(key, "input_") || len(rawValues) == 0 {
			continue
		}
		planKey := strings.TrimPrefix(key, "input_")
		planKey = strings.ReplaceAll(planKey, "_", "-")
		value := parseInt64(rawValues[0])
		if planKey != "" && value > 0 {
			out[planKey] = value
		}
	}
	return out
}

func parseInt64(raw string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return n
}

func parseInt(raw string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	return n
}
