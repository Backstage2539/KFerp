package messagecenter

import (
	"context"

	app "orderapp/internal/application/messagecenter"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Publish(context.Context, app.PublishCommand) (int64, error)
	ListNotifications(context.Context, app.NotificationQuery) ([]app.Notification, error)
	MarkRead(context.Context, int64, int64) error
}

type Dependencies struct {
	MessageCenter Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	api := api{svc: deps.MessageCenter}
	e.GET("/api/message-center/notifications", api.listNotifications)
	e.POST("/api/message-center/notifications/:id/read", api.markRead)
}
