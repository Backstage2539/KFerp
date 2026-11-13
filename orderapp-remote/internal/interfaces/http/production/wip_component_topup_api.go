package production

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/production"
	"orderapp/internal/interfaces/http/support"
)

func registerWIPComponentTopUpAPI(e *echo.Echo, s *app.Service) {
	e.GET("/api/job-cards/:id/wip-top-up", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的工位任务"})
		}
		out, err := s.PreviewWIPTopUp(c.Request().Context(), id)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, out)
	})
	e.POST("/api/job-cards/:id/wip-top-up", func(c echo.Context) error {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的工位任务"})
		}
		var cmd app.WIPTopUpCommand
		if err = c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "补料信息无效"})
		}
		cmd.ID = id
		cmd.EmployeeID = support.CurrentEmployeeID(c)
		cmd.Operator = support.ActorOf(c)
		out, err := s.ConfirmWIPTopUp(c.Request().Context(), cmd)
		if err != nil {
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusOK, out)
	})
}
