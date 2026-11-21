package messagecenter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	wechat "orderapp/internal/infrastructure/wechataccount"
)

func TestPartialShipmentRequiresCurrentShipmentDetails(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	config := DefaultExternalConfig()
	config.OfficialEnabled = true
	config.OfficialEvents = []string{"shipment_created"}
	config.OfficialEventSince = map[string]time.Time{"shipment_created": time.Now().Add(-time.Hour)}
	config.Templates["shipment_created"] = wechat.TemplateMapping{ID: "ship", Fields: map[string]string{"thing5": "items"}}
	configRaw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, fmt.Sprintf(`UPDATE %s.message_external_settings SET config=$1 WHERE id=1`, schema), configRaw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.message_business_test_verifications(event,status_value,template_fingerprint,delivery_id,verified_by,detail_verified)
	 VALUES('shipment_created','部分发货',$1,1,'test',true),('shipment_created','已发货',$1,2,'test',true)`, schema), businessTemplateFingerprint(config.Templates["shipment_created"]))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(42,'TEST42',8,1,1,1,false,'')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE "+schema+".orders SET settlement_reason='shipment-committed' WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, schema, "shipment_created", "partial-missing", 42, 8, map[string]any{"items": "整单共20袋", "shipping_status": "部分发货", "tracking_no": "SF1"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var reason, payload string
	var processed bool
	err = p.QueryRow(ctx, "SELECT failure_reason,payload::text,processed_at IS NOT NULL FROM "+schema+".message_business_events WHERE source_key='partial-missing'").Scan(&reason, &payload, &processed)
	if err != nil || !processed || !strings.Contains(reason, "本次发货商品明细") || payload != "{}" {
		t.Fatalf("partial missing: reason=%q payload=%s processed=%v err=%v", reason, payload, processed, err)
	}
	var committed string
	if err = p.QueryRow(ctx, "SELECT settlement_reason FROM "+schema+".orders WHERE id=42").Scan(&committed); err != nil || committed != "shipment-committed" {
		t.Fatalf("business transaction: %q %v", committed, err)
	}
	for _, tc := range []struct{ key, status, items string }{{"partial-known", "部分发货", "本次2袋"}, {"complete", "已发货", ""}} {
		tx, err = p.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = RecordBusinessEvent(ctx, tx, schema, "shipment_created", tc.key, 42, 8, map[string]any{"items": "整单共20袋", "shipment_items": tc.items, "shipping_status": tc.status, "tracking_no": "SF2"}); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var value string
		err = p.QueryRow(ctx, "SELECT payload->>'items' FROM "+schema+".message_business_events WHERE source_key=$1 AND processed_at IS NULL", tc.key).Scan(&value)
		want := "整单共20袋"
		if tc.status == "部分发货" {
			want = tc.items
		}
		if err != nil || value != want {
			t.Fatalf("%s items=%q err=%v", tc.key, value, err)
		}
	}
	_, err = p.Exec(ctx, "INSERT INTO "+schema+".message_business_events(event_type,source_key,order_id,customer_id,payload) VALUES('shipment_created','old-partial',42,8,$1::jsonb)", `{"items":"整单共20袋","shipping_status":"部分发货","tracking_no":"SF3"}`)
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{Repo: NewRepository(p, schema), AppID: "app"}
	if err = worker.CaptureBusinessCommitted(ctx); err != nil {
		t.Fatal(err)
	}
	err = p.QueryRow(ctx, "SELECT failure_reason,payload::text,processed_at IS NOT NULL FROM "+schema+".message_business_events WHERE source_key='old-partial'").Scan(&reason, &payload, &processed)
	if err != nil || !processed || !strings.Contains(reason, "本次发货商品明细") || payload != "{}" {
		t.Fatalf("old partial: reason=%q payload=%s processed=%v err=%v", reason, payload, processed, err)
	}
}
