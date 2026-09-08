package costing

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	app "orderapp/internal/application/costing"
	"orderapp/internal/interfaces/http/support"
)

type publicationOrderabilityService interface {
	ValidateBeanListOrderability(context.Context, app.PublishBeanListCommand) error
}

func registerPublicationOrderabilityAPI(e *echo.Echo, svc Service, authz support.AuthzService) {
	e.POST("/api/costing/bean-list/validate-orderability", func(c echo.Context) error {
		validator, ok := svc.(publicationOrderabilityService)
		if !ok {
			return c.JSON(http.StatusNotImplemented, map[string]string{"error": "价格表录单校验暂不可用"})
		}
		var cmd app.PublishBeanListCommand
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		canPublish, err := currentActorCanPublishBeanList(c, authz)
		if err != nil {
			return err
		}
		if !canPublish {
			cmd.Scope = "mine"
			cmd.CustomerID = 0
		}
		cmd.OwnerType, cmd.OwnerKey, err = beanListOwnerFromScope(c, cmd.Scope, cmd.CustomerID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		cmd.Actor = support.ActorOf(c)
		if err = validator.ValidateBeanListOrderability(c.Request().Context(), cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	})
}
