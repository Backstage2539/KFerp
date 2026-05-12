package messagecenter

import (
	"context"
	"testing"
)

type fakeRepository struct {
	published PublishCommand
	query     NotificationQuery
	readID    int64
	readEmp   int64
}

func (r *fakeRepository) Publish(ctx context.Context, cmd PublishCommand) (int64, error) {
	r.published = cmd
	return 1, nil
}

func (r *fakeRepository) ListNotifications(ctx context.Context, query NotificationQuery) ([]Notification, error) {
	r.query = query
	return []Notification{{ID: 1, Title: "新订单"}}, nil
}

func (r *fakeRepository) MarkRead(ctx context.Context, eventID, employeeID int64) error {
	r.readID = eventID
	r.readEmp = employeeID
	return nil
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
