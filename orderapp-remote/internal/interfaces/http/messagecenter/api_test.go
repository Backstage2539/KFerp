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
	query     app.NotificationQuery
	readID    int64
	readBy    int64
	savedRule app.SaveRuleCommand
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

func (s *fakeService) ListRules(ctx context.Context) ([]app.Rule, error) {
	return []app.Rule{{
		ID:         1,
		Code:       "order-created-production",
		Name:       "订单下单通知烘焙师",
		Enabled:    true,
		EventType:  "order.created",
		Channel:    app.ChannelERPPlatform,
		TargetType: "role",
		TargetKey:  "production",
	}}, nil
}

func (s *fakeService) SaveRule(ctx context.Context, cmd app.SaveRuleCommand) (app.Rule, error) {
	s.savedRule = cmd
	return app.Rule{ID: 9, Code: cmd.Code, Enabled: cmd.Enabled == nil || *cmd.Enabled, EventType: cmd.EventType, Channel: cmd.Channel, TargetType: cmd.TargetType, TargetKey: cmd.TargetKey}, nil
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

func TestMessageCenterAPIListsConfigurableRules(t *testing.T) {
	svc := &fakeService{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{MessageCenter: svc})

	req := httptest.NewRequest(http.MethodGet, "/api/message-center/rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"rules"`, `"order.created"`, `"production"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %q: %s", want, rec.Body.String())
		}
	}
}

func TestMessageCenterAPISavesIMNeutralRule(t *testing.T) {
	svc := &fakeService{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{MessageCenter: svc})

	body := strings.NewReader(`{"code":"order-shipped-customer-im","event_type":"order.shipped","channel":"enterprise_wechat","target_type":"order_customer","target_key":"customer","template_key":"order_shipped"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/message-center/rules", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.savedRule.Channel != "enterprise_wechat" || svc.savedRule.TargetType != "order_customer" {
		t.Fatalf("saved rule=%#v", svc.savedRule)
	}
}
