package messagecenter

import (
	"context"
	"testing"
)

type fakeRepository struct {
	published     PublishCommand
	query         NotificationQuery
	readID        int64
	readEmp       int64
	rules         []Rule
	notifications []Notification
	savedRule     SaveRuleCommand
}

func (r *fakeRepository) Publish(ctx context.Context, cmd PublishCommand) (int64, error) {
	r.published = cmd
	return 1, nil
}

func (r *fakeRepository) ListNotifications(ctx context.Context, query NotificationQuery) ([]Notification, error) {
	r.query = query
	if r.notifications != nil {
		return r.notifications, nil
	}
	return []Notification{{ID: 1, Title: "新订单"}}, nil
}

func (r *fakeRepository) MarkRead(ctx context.Context, eventID, employeeID int64) error {
	r.readID = eventID
	r.readEmp = employeeID
	return nil
}

func (r *fakeRepository) ListRules(ctx context.Context) ([]Rule, error) {
	return r.rules, nil
}

func (r *fakeRepository) ListActiveRules(ctx context.Context, query RuleQuery) ([]Rule, error) {
	return r.rules, nil
}

func (r *fakeRepository) SaveRule(ctx context.Context, cmd SaveRuleCommand) (Rule, error) {
	r.savedRule = cmd
	enabled := true
	if cmd.Enabled != nil {
		enabled = *cmd.Enabled
	}
	return Rule{Code: cmd.Code, EventType: cmd.EventType, Channel: cmd.Channel, TargetType: cmd.TargetType, TargetKey: cmd.TargetKey, Enabled: enabled}, nil
}

func TestServiceDefaultsERPPlatformDelivery(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	_, err := svc.Publish(context.Background(), PublishCommand{Title: "新订单"})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.published.Deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(repo.published.Deliveries))
	}
	delivery := repo.published.Deliveries[0]
	if delivery.Channel != ChannelERPPlatform || delivery.TargetType != "permission" || delivery.TargetKey != "orders.read" {
		t.Fatalf("default delivery = %#v", delivery)
	}
}

func TestServicePublishesConfiguredRuleDeliveries(t *testing.T) {
	repo := &fakeRepository{rules: []Rule{
		{Code: "order-created-production", Enabled: true, EventType: "order.created", Channel: ChannelERPPlatform, TargetType: "role", TargetKey: "production", PayloadMatch: map[string]any{"service": "wholesale"}},
		{Code: "order-created-customer-im", Enabled: true, EventType: "order.created", Channel: ChannelExternalIM, TargetType: "order_customer", TargetKey: "customer", AdapterKey: "wechat_service_account", TemplateKey: "order_created_customer"},
		{Code: "ignored", Enabled: true, EventType: "order.created", Channel: ChannelERPPlatform, TargetType: "role", TargetKey: "finance", PayloadMatch: map[string]any{"service": "retail"}},
	}}
	svc := NewService(repo)
	_, err := svc.Publish(context.Background(), PublishCommand{
		Topic:     "orders",
		EventType: "order.created",
		Payload:   map[string]any{"service": "wholesale"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.published.Deliveries) != 2 {
		t.Fatalf("deliveries=%#v, want 2 matching rules", repo.published.Deliveries)
	}
	if repo.published.Deliveries[0].TargetType != "role" || repo.published.Deliveries[0].TargetKey != "production" {
		t.Fatalf("first delivery=%#v", repo.published.Deliveries[0])
	}
	if repo.published.Deliveries[1].Channel != ChannelExternalIM || repo.published.Deliveries[1].AdapterKey != "wechat_service_account" {
		t.Fatalf("external delivery=%#v", repo.published.Deliveries[1])
	}
}

func TestServiceSavesIMNeutralNotificationRule(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	enabled := false
	rule, err := svc.SaveRule(context.Background(), SaveRuleCommand{
		Code:         "production-finished-sales",
		EventType:    "order.production_finished",
		Channel:      "enterprise_wechat",
		TargetType:   "role",
		TargetKey:    "sales",
		TemplateKey:  "production_finished",
		AdapterKey:   "enterprise_wechat",
		Enabled:      &enabled,
		PayloadMatch: map[string]any{"new_status": "生产完成"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.Code != "production-finished-sales" || repo.savedRule.Channel != "enterprise_wechat" || repo.savedRule.TargetType != "role" {
		t.Fatalf("saved rule=%#v command=%#v", rule, repo.savedRule)
	}
	if repo.savedRule.Enabled == nil || *repo.savedRule.Enabled {
		t.Fatalf("enabled should be persisted false: %#v", repo.savedRule.Enabled)
	}
}

func TestServiceNormalizesNotificationQueryAndRead(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(repo)
	if _, err := svc.ListNotifications(context.Background(), NotificationQuery{EmployeeID: 7, Limit: 0}); err != nil {
		t.Fatal(err)
	}
	if repo.query.Channel != ChannelERPPlatform || repo.query.Limit != 20 {
		t.Fatalf("query = %#v", repo.query)
	}
	if err := svc.MarkRead(context.Background(), 9, 7); err != nil {
		t.Fatal(err)
	}
	if repo.readID != 9 || repo.readEmp != 7 {
		t.Fatalf("read = %d/%d, want 9/7", repo.readID, repo.readEmp)
	}
}

func TestServiceDedupesNotificationsByEventID(t *testing.T) {
	repo := &fakeRepository{notifications: []Notification{
		{ID: 11, EventType: "order.created", SourceType: "order", SourceID: 71, Title: "新订单 SO-001"},
		{ID: 11, EventType: "order.created", SourceType: "order", SourceID: 71, Title: "新订单 SO-001"},
		{ID: 12, EventType: "order.created", SourceType: "order", SourceID: 72, Title: "新订单 SO-002"},
	}}
	svc := NewService(repo)
	rows, err := svc.ListNotifications(context.Background(), NotificationQuery{EmployeeID: 7, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != 11 || rows[1].ID != 12 {
		t.Fatalf("notifications=%#v, want unique events 11 and 12", rows)
	}
}
