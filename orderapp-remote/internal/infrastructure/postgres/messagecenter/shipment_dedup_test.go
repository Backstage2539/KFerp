package messagecenter

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
)

func TestShipmentBusinessNoticeDeduplicatesLegacyPerRecipient(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id) VALUES(1),(2);
	INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at,notification_events) VALUES
	('app','both',1,8,true,true,true,now()-interval '1 hour','["shipped","shipment_created"]'),
	('app','legacy',2,8,true,true,true,now()-interval '1 hour','["shipped"]');
	INSERT INTO %[1]s.orders VALUES(42,'TEST-42',8,1,1,1,false,'');
	INSERT INTO %[1]s.message_desktop_devices(id,name,group_name,token_hash) VALUES(1,'test','test','test');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEnabled = true
	cfg.OfficialSince = time.Now().Add(-time.Hour)
	cfg.DesktopEnabled = true
	cfg.DesktopSince = time.Now().Add(-time.Hour)
	cfg.DesktopID = 1
	cfg.Events = []string{"shipped"}
	cfg.OfficialEvents = []string{"shipped", "shipment_created"}
	cfg.OfficialEventSince = map[string]time.Time{"shipment_created": time.Now().Add(-time.Hour)}
	cfg.Templates["shipped"] = wechat.TemplateMapping{ID: "legacy", Fields: map[string]string{"thing1": "status"}}
	cfg.Templates["shipment_created"] = wechat.TemplateMapping{ID: "new", Fields: map[string]string{"thing1": "status"}}
	configRaw, _ := json.Marshal(cfg)
	_, err = p.Exec(ctx, "UPDATE "+schema+".message_external_settings SET config=$1 WHERE id=1", configRaw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, "INSERT INTO "+schema+".message_business_test_verifications(event,status_value,template_fingerprint,delivery_id,verified_by,detail_verified) VALUES('shipment_created','已发货',$1,1,'test',true)", businessTemplateFingerprint(cfg.Templates["shipment_created"]))
	if err != nil {
		t.Fatal(err)
	}
	w := Worker{Repo: repo, AppID: "app", Sender: &noticeSender{}, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	other := Worker{Repo: repo, AppID: "app", Sender: &noticeSender{}, Authorize: w.Authorize}
	runConcurrent := func() {
		t.Helper()
		var workers sync.WaitGroup
		errors := make(chan error, 2)
		for _, worker := range []*Worker{&w, &other} {
			workers.Add(1)
			go func(worker *Worker) { defer workers.Done(); errors <- worker.Tick(ctx) }(worker)
		}
		workers.Wait()
		close(errors)
		for workerErr := range errors {
			if workerErr != nil {
				t.Fatal(workerErr)
			}
		}
		if err = w.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	runConcurrent()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE "+schema+".orders SET ship_status_id=3 WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, schema, "shipment_created", "shipment_created:42:SF1", 42, 8, map[string]any{"order_id": 42, "order_no": "TEST-42", "customer_id": 8, "items": "coffee", "shipping_status": "已发货", "tracking_no": "SF1"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	runConcurrent()
	var bothSent, legacySent, groupCount int
	if err = p.QueryRow(ctx, `SELECT count(*) FILTER(WHERE d.target_key='app:both' AND d.status IN ('accepted','delivered')), count(*) FILTER(WHERE d.target_key='app:legacy' AND d.status IN ('accepted','delivered')), count(*) FILTER(WHERE d.channel='wechat_desktop') FROM `+schema+`.message_deliveries d JOIN `+schema+`.message_events e ON e.id=d.event_id WHERE e.source_id=42`).Scan(&bothSent, &legacySent, &groupCount); err != nil {
		t.Fatal(err)
	}
	if bothSent != 1 || legacySent != 1 || groupCount != 1 {
		t.Fatalf("both=%d legacy=%d group=%d", bothSent, legacySent, groupCount)
	}
	// When the shipment has no business event (for example, no tracking number),
	// legacy subscribers still receive the original status notice.
	_, err = p.Exec(ctx, `INSERT INTO `+schema+`.orders VALUES(43,'TEST-43',8,1,1,1,false,'')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, `UPDATE `+schema+`.orders SET ship_status_id=3 WHERE id=43`)
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	err = p.QueryRow(ctx, `SELECT count(*) FILTER(WHERE d.target_key='app:both' AND d.status IN ('accepted','delivered')), count(*) FILTER(WHERE d.target_key='app:legacy' AND d.status IN ('accepted','delivered')) FROM `+schema+`.message_deliveries d JOIN `+schema+`.message_events e ON e.id=d.event_id WHERE e.source_id=43`).Scan(&bothSent, &legacySent)
	if err != nil || bothSent != 1 || legacySent != 1 {
		t.Fatalf("legacy fallback both=%d legacy=%d err=%v", bothSent, legacySent, err)
	}
	_, err = p.Exec(ctx, `INSERT INTO `+schema+`.orders VALUES(44,'TEST-44',8,1,1,1,false,'')`)
	if err != nil {
		t.Fatal(err)
	}
	runConcurrent()
	tx, err = p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE "+schema+".orders SET ship_status_id=2 WHERE id=44"); err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, schema, "shipment_created", "shipment_created:44:SF1", 44, 8, map[string]any{"items": "全单商品", "shipping_status": "部分发货", "tracking_no": "SF1"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	runConcurrent()
	err = p.QueryRow(ctx, `SELECT count(*) FILTER(WHERE d.target_key='app:both' AND d.status IN ('accepted','delivered')), count(*) FILTER(WHERE d.target_key='app:legacy' AND d.status IN ('accepted','delivered')) FROM `+schema+`.message_deliveries d JOIN `+schema+`.message_events e ON e.id=d.event_id WHERE e.source_id=44`).Scan(&bothSent, &legacySent)
	if err != nil || bothSent != 1 || legacySent != 1 {
		t.Fatalf("partial missing details fallback both=%d legacy=%d err=%v", bothSent, legacySent, err)
	}
	// Legacy templates that display the full order's items cannot describe a
	// partial shipment; those deliveries stop with an actionable reason.
	cfg.Templates["shipped"] = wechat.TemplateMapping{ID: "legacy", Fields: map[string]string{"thing1": "items"}}
	configRaw, _ = json.Marshal(cfg)
	if _, err = p.Exec(ctx, "UPDATE "+schema+".message_external_settings SET config=$1 WHERE id=1", configRaw); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `INSERT INTO `+schema+`.orders VALUES(45,'TEST-45',8,1,1,1,false,'')`); err != nil {
		t.Fatal(err)
	}
	runConcurrent()
	if _, err = p.Exec(ctx, `UPDATE `+schema+`.orders SET ship_status_id=2 WHERE id=45`); err != nil {
		t.Fatal(err)
	}
	runConcurrent()
	var stopped, groupStillSent int
	err = p.QueryRow(ctx, `SELECT count(*) FILTER(WHERE d.channel='wechat_service_account' AND d.status='skipped' AND d.error LIKE '%本次发货商品明细%'), count(*) FILTER(WHERE d.channel='wechat_desktop') FROM `+schema+`.message_deliveries d JOIN `+schema+`.message_events e ON e.id=d.event_id WHERE e.source_id=45`).Scan(&stopped, &groupStillSent)
	if err != nil || stopped != 2 || groupStillSent != 1 {
		t.Fatalf("unsafe partial template stopped=%d group=%d err=%v", stopped, groupStillSent, err)
	}
}
