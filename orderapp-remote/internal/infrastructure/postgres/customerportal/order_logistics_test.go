package customerportal

import (
	"context"
	"fmt"
	"testing"
)

func TestCustomerOrderLogisticsEventsLatestAndScoped(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.order_shipping_tracking_events (id bigserial PRIMARY KEY,order_id bigint NOT NULL,tracking_no text NOT NULL DEFAULT '',event_time timestamptz NOT NULL,status text NOT NULL DEFAULT '',description text NOT NULL DEFAULT '',location text NOT NULL DEFAULT '',source text NOT NULL DEFAULT '')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_shipping_tracking_events(order_id,tracking_no,event_time,status,description,location) VALUES (10,'SF1','2026-11-08 10:00+08','picked_up','已揽收','昆明'),(10,'SF1','2026-11-09 11:00+08','delivered','已签收','大理'),(11,'OTHER','2026-11-09 11:00+08','delivered','其他客户轨迹','')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.loadCustomerOrderLogisticsEvents(ctx, []int64{10, 12})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[10].Description != "已签收" || got[10].TrackingNo != "SF1" || got[10].EventTime != "2026-11-09 11:00:00" {
		t.Fatalf("events=%+v", got)
	}
	if _, ok := got[11]; ok {
		t.Fatal("unexpected foreign order event")
	}
}
