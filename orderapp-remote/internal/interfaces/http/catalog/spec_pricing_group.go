package catalog

import (
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/catalog"
	"orderapp/internal/interfaces/http/support"
	"strconv"
)

func (h productHandler) listSpecPricingGroups(c echo.Context) error {
	rows, err := h.catalog.ListSpecPricingGroups(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "读取规格计价组失败"})
	}
	return c.JSON(http.StatusOK, map[string]any{"groups": rows})
}
func (h productHandler) specPricingGroupOptions(c echo.Context) error {
	out, err := h.catalog.SpecPricingGroupOptions(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "读取规格计价组选项失败"})
	}
	return c.JSON(http.StatusOK, out)
}
func (h productHandler) saveSpecPricingGroup(c echo.Context) error {
	var g app.SpecPricingGroup
	if err := c.Bind(&g); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "请求格式错误"})
	}
	if c.Param("id") != "" {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "计价组编号无效"})
		}
		g.ID = id
	} else {
		g.ID = 0
		g.Revision = 0
	}
	g.Actor = support.ActorOf(c)
	out, err := h.catalog.SaveSpecPricingGroup(c.Request().Context(), g)
	if err != nil {
		if app.IsValidationError(err) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "保存规格计价组失败"})
	}
	return c.JSON(http.StatusOK, map[string]any{"group": out})
}
