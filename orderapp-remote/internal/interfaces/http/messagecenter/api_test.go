package messagecenter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "orderapp/internal/application/messagecenter"

	"github.com/labstack/echo/v4"
)

type fakeService struct {
	query  app.NotificationQuery
	readID int64
	readBy int64
}

func (s *fakeService) Publish(ctx context.Context, cmd app.PublishCommand) (int64, error) {
	return 1, nil
}

func (s *fakeService) ListNotifications(ctx context.Context, query app.NotificationQuery) ([]app.Notification, error) {
	s.query = query
	return []app.Notification{{
		ID:         11,
		Topic:      "orders",
		EventType:  "order.created",
		SourceType: "order",
		SourceID:   71,
		Title:      "新订单 SO-001",
		Tone:       "success",
		Payload:    map[string]any{"order_id": 71},
	}}, nil
}

func (s *fakeService) MarkRead(ctx context.Context, eventID, employeeID int64) error {
	s.readID = eventID
	s.readBy = employeeID
	return nil
}

func TestMessageCenterAPIListsUnreadERPNotificationsForCurrentEmployee(t *testing.T) {
	svc := &fakeService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{MessageCenter: svc})

	req := httptest.NewRequest(http.MethodGet, "/api/message-center/notifications?status=unread&limit=5", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.query.EmployeeID != 7 || svc.query.Status != "unread" || svc.query.Limit != 5 {
		t.Fatalf("query=%#v", svc.query)
	}
	for _, want := range []string{`"notifications"`, `"order.created"`, `"新订单 SO-001"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %q: %s", want, rec.Body.String())
		}
	}
}

func TestMessageCenterAPIMarksNotificationRead(t *testing.T) {
	svc := &fakeService{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{MessageCenter: svc})

	req := httptest.NewRequest(http.MethodPost, "/api/message-center/notifications/11/read", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.readID != 11 || svc.readBy != 7 {
		t.Fatalf("read=%d/%d, want 11/7", svc.readID, svc.readBy)
	}
}
