package production

import (
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/production"
	"orderapp/internal/interfaces/http/support"
	"strconv"
)

func registerPlanRemainderAPI(e *echo.Echo, s *app.Service) {
	e.GET("/api/packaging-remainders", func(c echo.Context) error {
		groups, err := s.ListPlanRemainderGroups(c.Request().Context())
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"groups": groups})
	})
	e.GET("/api/production-plans/:id/packaging-remainders", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的生产计划"})
		}
		product, err := strconv.ParseInt(c.QueryParam("product_id"), 10, 64)
		if err != nil || product <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请选择产品"})
		}
		out, err := s.GetPlanRemainder(c.Request().Context(), id, product)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, out)
	})
	e.POST("/api/production-plans/:id/packaging-remainders", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的生产计划"})
		}
		var cmd app.PlanRemainderCommand
		if err = c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "余料信息无效"})
		}
		cmd.PlanID = id
		cmd.Operator = support.ActorOf(c)
		result, err := s.ConfirmPlanRemainder(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})
	e.POST("/api/production-plans/:id/packaging-remainders/recheck", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的生产计划"})
		}
		var cmd app.PlanRemainderRecheckCommand
		if err = c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的重新核对请求"})
		}
		cmd.PlanID = id
		cmd.Operator = support.ActorOf(c)
		result, err := s.RecheckPlanRemainder(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/production-plans/:id/packaging-remainders/resolve-discrepancies", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的生产计划"})
		}
		var cmd app.PlanRemainderResolveCommand
		if err = c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的差额核对请求"})
		}
		cmd.PlanID = id
		cmd.Operator = support.ActorOf(c)
		result, err := s.ResolvePlanRemainderDiscrepancies(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

}
