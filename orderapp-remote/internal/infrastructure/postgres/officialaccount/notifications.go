package officialaccount

import (
	"context"
	notifications "orderapp/internal/infrastructure/postgres/messagecenter"
)

func (r Repository) NotificationSubscription(ctx context.Context, appID, openid string, subscribed bool, timestamp int64) error {
	return notifications.NewRepository(r.Pool, r.Schema).SubscriptionEvent(ctx, appID, openid, subscribed, timestamp)
}
func (r Repository) NotificationScan(ctx context.Context, token, appID, openid string, timestamp int64) error {
	return notifications.NewRepository(r.Pool, r.Schema).RecordNotificationScan(ctx, token, appID, openid, timestamp)
}
func (r Repository) NotificationReceipt(ctx context.Context, appID, msgID, status string) error {
	return notifications.NewRepository(r.Pool, r.Schema).RecordProviderResult(ctx, appID, msgID, status)
}
