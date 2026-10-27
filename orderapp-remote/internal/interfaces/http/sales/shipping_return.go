package sales

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/sales"
	d "orderapp/internal/domain/shippingreturn"
	"orderapp/internal/interfaces/http/support"
	"strconv"
	"strings"
)

func shippingReturnStaff(c echo.Context) error {
	if support.CustomerFulfillmentOrderScopeLimited(c) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "寄件回传仅限内部订单操作人员"})
	}
	if e := support.RequireEmployeeBound(c); e != nil {
		return c.JSON(http.StatusForbidden, map[string]string{"error": e.Error()})
	}
	return nil
}
func registerShippingReturnRoutes(e *echo.Echo, s *app.Service, m MessagePublisher) {
	list := func(c echo.Context) error {
		if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
			return err
		}
		r, err := s.ShippingReturns()
		if err != nil {
			return c.JSON(503, map[string]string{"error": err.Error()})
		}
		v, err := r.ShippingReturnTemplates(c.Request().Context())
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{"templates": v})
	}
	e.POST("/api/orders/shipping-return-rematch", func(c echo.Context) error {
		if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
			return err
		}
		var body struct {
			BatchID int64   `json:"batch_id"`
			RowIDs  []int64 `json:"row_ids"`
		}
		if c.Bind(&body) != nil || body.BatchID <= 0 || len(body.RowIDs) > 10000 {
			return c.JSON(400, map[string]string{"error": "请选择有效导入批次和寄件行"})
		}
		repo, err := s.ShippingReturns()
		if err != nil {
			return c.JSON(503, map[string]string{"error": err.Error()})
		}
		result, err := repo.RematchShippingReturn(c.Request().Context(), support.ActorOf(c), body.BatchID, body.RowIDs)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, result)
	})
	e.GET("/api/orders/shipping-return-templates", list)
	e.GET("/api/settings/logistics/shipping-return-templates", list)
	e.PUT("/api/settings/logistics/shipping-return-templates", func(c echo.Context) error {
		if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
			return err
		}
		var body struct {
			Templates []d.Template `json:"templates"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(400, map[string]string{"error": "模板格式错误"})
		}
		r, err := s.ShippingReturns()
		if err == nil {
			err = r.SaveShippingReturnTemplates(c.Request().Context(), support.ActorOf(c), body.Templates)
		}
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return list(c)
	})
	e.GET("/api/orders/shipping-return-candidates", func(c echo.Context) error {
		if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
			return err
		}
		r, err := s.ShippingReturns()
		if err != nil {
			return c.JSON(503, map[string]string{"error": err.Error()})
		}
		v, err := r.ShippingReturnCandidates(c.Request().Context(), 0)
		if err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(200, map[string]any{"rows": v})
	})
	e.POST("/api/orders/:id/shipping-return-selection", func(c echo.Context) error {
		if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
			return err
		}
		oid, err := strconv.ParseInt(c.Param("id"), 10, 64)
		var body struct {
			CandidateID int64  `json:"candidate_id"`
			Version     string `json:"version"`
		}
		if err != nil || oid <= 0 || c.Bind(&body) != nil || body.CandidateID <= 0 || body.Version == "" {
			return c.JSON(400, map[string]string{"error": "请选择有效快递单"})
		}
		r, err := s.ShippingReturns()
		if err != nil {
			return c.JSON(503, map[string]string{"error": err.Error()})
		}
		v, err := r.SelectShippingReturn(c.Request().Context(), support.ActorOf(c), oid, body.CandidateID, body.Version)
		if err != nil {
			return c.JSON(409, map[string]string{"error": err.Error()})
		}
		publishOrdersShipped(c, m, v)
		return c.JSON(200, v)
	})
}
func importShippingReturn(c echo.Context, s *app.Service, m MessagePublisher, data []byte, name string) error {
	if err := shippingReturnStaff(c); err != nil || c.Response().Committed {
		return err
	}
	r, err := s.ShippingReturns()
	if err != nil {
		return c.JSON(503, map[string]string{"error": err.Error()})
	}
	templates, err := r.ShippingReturnTemplates(c.Request().Context())
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	var selected *d.Template
	for _, t := range templates {
		if t.ID == c.FormValue("template_id") && t.Active {
			v := t
			selected = &v
			break
		}
	}
	if selected == nil {
		return c.JSON(400, map[string]string{"error": "模板已停用或不存在，请刷新"})
	}
	if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
		return c.JSON(400, map[string]string{"error": "请上传 xlsx 文件"})
	}
	rows, err := d.Parse(bytes.NewReader(data), *selected)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	v, err := r.ImportShippingReturn(c.Request().Context(), support.ActorOf(c), name, hash, *selected, rows)
	if err != nil {
		c.Logger().Error(err)
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	publishOrdersShipped(c, m, app.FillShipmentTrackingResult{Updated: len(v.OrderIDs), OrderIDs: v.OrderIDs})
	return c.JSON(200, v)
}
