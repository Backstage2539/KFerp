package production

import (
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/production"
	support "orderapp/internal/interfaces/http/support"
	"strconv"
)

func registerReportingRecheckAPI(e *echo.Echo, service *app.Service) {
	for _, entry := range []struct{ path, kind string }{{"/api/work-orders/:id/reporting-recheck", "work_order"}, {"/api/production-plan-items/:id/reporting-recheck", "plan_item"}} {
		entry := entry
		e.GET(entry.path, func(c echo.Context) error {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的重核对象"})
			}
			res, err := service.RecheckReportingTypes(c.Request().Context(), entry.kind, id, "", support.ActorOf(c))
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusOK, res)
		})
		e.POST(entry.path, func(c echo.Context) error {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "无效的重核对象"})
			}
			var req struct {
				Version string `json:"version"`
			}
			if c.Bind(&req) != nil || req.Version == "" {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "请先预览并确认重核内容"})
			}
			res, err := service.RecheckReportingTypes(c.Request().Context(), entry.kind, id, req.Version, support.ActorOf(c))
			if err != nil {
				return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			}
			return c.JSON(http.StatusOK, res)
		})
	}
}
