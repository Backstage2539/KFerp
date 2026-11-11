package messagecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
	"os"
	"strings"
	"testing"
	"time"
)

type noticeSender struct {
	count   int
	err     error
	notices []app.OrderNotice
}

func (s *noticeSender) SendOrderTemplate(_ context.Context, _ string, _ wechat.TemplateMapping, n app.OrderNotice, _ bool) (string, error) {
	s.count++
	s.notices = append(s.notices, n)
	return fmt.Sprint(100 + s.count), s.err
}

func TestLifecycleWorkerAllSixAndCallbackBeforeAcknowledgement(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()-interval '1 minute')`, s))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEnabled = true
	for _, event := range cfg.Events {
		cfg.Templates[event] = wechat.TemplateMapping{ID: "t", Fields: map[string]string{"thing1": "status"}}
	}
	if err = repo.SaveExternalConfig(ctx, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	worker := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	for _, sql := range []string{`INSERT INTO %s.orders VALUES(42,'TEST-42',8,1,1,1,false,'')`, `UPDATE %s.orders SET pay_status_id=2 WHERE id=42`, `UPDATE %s.orders SET process_status_id=2 WHERE id=42`, `UPDATE %s.orders SET process_status_id=3 WHERE id=42`, `UPDATE %s.orders SET ship_status_id=3 WHERE id=42`, `UPDATE %s.orders SET is_void=true WHERE id=42`} {
		if _, err = p.Exec(ctx, fmt.Sprintf(sql, s)); err != nil {
			t.Fatal(err)
		}
		if err = worker.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if sender.count != 6 {
		t.Fatalf("sent %d, wanted six", sender.count)
	}
	for i, event := range cfg.Events {
		if sender.notices[i].Milestones[0] != event {
			t.Fatal(sender.notices[i])
		}
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 6 {
		t.Fatal("duplicate send")
	}
	ds, err := repo.ExternalDeliveries(ctx)
	if err != nil || len(ds) != 6 {
		t.Fatalf("%v %v", ds, err)
	}
	if err = repo.RecordProviderResult(ctx, "app", "106", "success"); err != nil {
		t.Fatal(err)
	}
	ds, _ = repo.ExternalDeliveries(ctx)
	if ds[0].Status != "delivered" {
		t.Fatal(ds[0])
	}
	// A signed callback can reach us before the send response is saved.
	tx, _ := p.Begin(ctx)
	recipients, _ := repo.NotificationRecipients(ctx, "app")
	n := sender.notices[0]
	n.Test = true
	_, err = repo.enqueueNotice(ctx, tx, n, recipients, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	d, err := repo.ClaimExternal(ctx, "wechat_service_account", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RecordProviderResult(ctx, "app", "early", "success"); err != nil {
		t.Fatal(err)
	}
	if err = repo.FinishExternal(ctx, d, "accepted", "early", ""); err != nil {
		t.Fatal(err)
	}
	ds, _ = repo.ExternalDeliveries(ctx)
	if ds[0].Status != "delivered" {
		t.Fatalf("early callback was lost: %s", ds[0].Status)
	}
}

func TestLifecycleStopUnsubscribedAndLeaseUnknown(t *testing.T) {
	p, s := notificationDB(t)
	repo := NewRepository(p, s)
	ctx := context.Background()
	tx, _ := p.Begin(ctx)
	_, err := repo.enqueueNotice(ctx, tx, app.OrderNotice{Test: true, Milestones: []string{"created"}}, []Recipient{{AppID: "app", OpenID: "gone", MiniUserID: 1, Events: []string{"created"}}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit(ctx)
	sender := &noticeSender{}
	w := Worker{Repo: repo, Sender: sender, AppID: "app"}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ds, _ := repo.ExternalDeliveries(ctx)
	if sender.count != 0 || ds[0].Status != "skipped" {
		t.Fatal(ds)
	}
	tx, _ = p.Begin(ctx)
	repo.enqueueNotice(ctx, tx, app.OrderNotice{Test: true, Milestones: []string{"created"}}, nil, 7)
	tx.Commit(ctx)
	d, err := repo.ClaimExternal(ctx, "wechat_desktop", "7")
	if err != nil {
		t.Fatal(err)
	}
	p.Exec(ctx, "UPDATE "+s+".message_deliveries SET lease_until=now()-interval '1 minute' WHERE id=$1", d.ID)
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ds, _ = repo.ExternalDeliveries(ctx)
	if ds[0].Status != "unknown" {
		t.Fatal(ds[0])
	}
	if repo.RetryExternal(ctx, d.ID, "test", false) == nil {
		t.Fatal("unknown blindly retried")
	}
}

func notificationDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated test database required")
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	s := fmt.Sprintf("test_notifications_%d", time.Now().UnixNano())
	_, e = p.Exec(ctx, "CREATE SCHEMA "+s)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { p.Exec(ctx, "DROP SCHEMA "+s+" CASCADE"); p.Close() })
	_, e = p.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.company_employees(id BIGINT PRIMARY KEY);
 CREATE TABLE %[1]s.mini_users(id BIGINT PRIMARY KEY);
 CREATE TABLE %[1]s.wechat_bindings(app_id TEXT,openid TEXT,mini_user_id BIGINT,customer_id BIGINT,active BOOLEAN DEFAULT true,bound_at TIMESTAMPTZ DEFAULT now(),PRIMARY KEY(app_id,openid));
 CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY,name TEXT,responsible_employee_id BIGINT);
 CREATE TABLE %[1]s.pay_statuses(id BIGINT PRIMARY KEY,name TEXT);INSERT INTO %[1]s.pay_statuses VALUES(1,'未付款'),(2,'已付款');
 CREATE TABLE %[1]s.ship_statuses(id BIGINT PRIMARY KEY,name TEXT);INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'部分发货'),(3,'已发货');
 CREATE TABLE %[1]s.order_process_statuses(id BIGINT PRIMARY KEY,name TEXT);INSERT INTO %[1]s.order_process_statuses VALUES(1,'待生产'),(2,'生产中'),(3,'生产完成');
 CREATE TABLE %[1]s.orders(id BIGINT PRIMARY KEY,order_no TEXT,customer_id BIGINT,pay_status_id BIGINT,ship_status_id BIGINT,process_status_id BIGINT,is_void BOOLEAN DEFAULT false,settlement_reason TEXT DEFAULT '');
 CREATE TABLE %[1]s.order_items(id BIGINT,order_id BIGINT,item_name TEXT,spec TEXT,qty NUMERIC,unit TEXT);
 INSERT INTO %[1]s.customers VALUES(8,'测试客户',7);
 CREATE TABLE %[1]s.audit_logs(actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
 `, s))
	if e != nil {
		t.Fatal(e)
	}
	if e = EnsureSchema(ctx, p, s); e != nil {
		t.Fatal(e)
	}
	if e = EnsureExternalSchema(ctx, p, s); e != nil {
		t.Fatal(e)
	}
	return p, s
}

func TestOrderOutboxCommitCoalescesAndRollbackIsSilent(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders VALUES(42,'TEST-42',8,1,1,1,false,'');INSERT INTO %[1]s.order_items VALUES(1,42,'测试商品','250g',2,'袋');UPDATE %[1]s.orders SET pay_status_id=2 WHERE id=42`, s))
	if e != nil {
		t.Fatal(e)
	}
	var count int
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".message_order_changes").Scan(&count)
	if count != 0 {
		t.Fatal("uncommitted event visible")
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	var created bool
	var after, items string
	e = p.QueryRow(ctx, "SELECT created,after_state->>'payment',snapshot->>'items' FROM "+s+".message_order_changes").Scan(&created, &after, &items)
	if e != nil || !created || after != "已付款" || items == "" {
		t.Fatalf("%v %t %s %s", e, created, after, items)
	}
	tx, _ = p.Begin(ctx)
	tx.Exec(ctx, "UPDATE "+s+".orders SET ship_status_id=2 WHERE id=42")
	tx.Rollback(ctx)
	p.Exec(ctx, "UPDATE "+s+".orders SET pay_status_id=2 WHERE id=42")
	p.QueryRow(ctx, "SELECT count(*) FROM "+s+".message_order_changes").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	p.Exec(ctx, "UPDATE "+s+".orders SET pay_status_id=1 WHERE id=42")
	p.Exec(ctx, "UPDATE "+s+".orders SET pay_status_id=2 WHERE id=42")
	p.QueryRow(ctx, "SELECT count(DISTINCT id) FROM "+s+".message_order_changes").Scan(&count)
	if count != 3 {
		t.Fatal(count)
	}
}

func TestLifecycleTransactionRevertedAndSubscriptionReplay(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(42,'TEST-42',8,2,1,1,false,'')`, s)); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{`UPDATE %s.orders SET pay_status_id=1 WHERE id=42`, `UPDATE %s.orders SET pay_status_id=2 WHERE id=42`} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(sql, s)); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var before, after []byte
	if err = p.QueryRow(ctx, "SELECT before_state,after_state FROM "+s+".message_order_changes WHERE NOT created ORDER BY id DESC LIMIT 1").Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("reverted transaction emitted change: %s => %s", before, after)
	}
	if _, err = p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now())`, s)); err != nil {
		t.Fatal(err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "openid", true); err != nil {
		t.Fatal(err)
	}
	prefs, err := r.NotificationPreferences(ctx, "app", 1)
	if err != nil || len(prefs) != 1 || !prefs[0].Enabled {
		t.Fatalf("replayed subscribe disabled consent: %v %v", prefs, err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "openid", false); err != nil {
		t.Fatal(err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "openid", true); err != nil {
		t.Fatal(err)
	}
	prefs, _ = r.NotificationPreferences(ctx, "app", 1)
	if prefs[0].Enabled {
		t.Fatal("resubscribe restored old consent")
	}
}

func TestOfficialEventsAreIndependentAndLegacyCompatible(t *testing.T) {
	legacy := ExternalConfig{Events: []string{"created", "shipped"}}
	if got := legacy.ServiceEvents(); len(got) != 2 || got[0] != "created" {
		t.Fatalf("legacy events: %v", got)
	}
	legacy.OfficialEvents = []string{}
	if got := legacy.ServiceEvents(); len(got) != 0 {
		t.Fatalf("explicit empty must stay empty: %v", got)
	}
	legacy.OfficialEvents = []string{"paid"}
	if got := legacy.ServiceEvents(); len(got) != 1 || got[0] != "paid" {
		t.Fatalf("independent: %v", got)
	}
}

func TestDisabledBusinessEventsDiscardSensitivePayloadAndNeverReplay(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, schema, "receipt_received", "receipt_received:1", 0, 8, map[string]any{"amount": 12345, "customer": "sensitive"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var payload string
	var processed bool
	if err = p.QueryRow(ctx, "SELECT payload::text,processed_at IS NOT NULL FROM "+schema+".message_business_events WHERE source_key='receipt_received:1'").Scan(&payload, &processed); err != nil {
		t.Fatal(err)
	}
	if payload != "{}" || !processed {
		t.Fatalf("inactive event retained sensitive data or could replay: %s %v", payload, processed)
	}
	tx, err = p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, schema, "receipt_received", "receipt_received:2", 0, 8, map[string]any{"amount": 1}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".message_business_events").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback: %d %v", count, err)
	}
}

func TestIndependentChannelEventsCaptureAndDispatch(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);
 INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()-interval '1 minute');
 INSERT INTO %[1]s.message_desktop_devices(id,name,group_name,token_hash,enabled,verified,health,last_seen) VALUES(7,'test','test','hash',true,true,'ready',now())`, schema))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEnabled = true
	cfg.DesktopEnabled = true
	cfg.DesktopID = 7
	cfg.Events = []string{"created"}
	cfg.OfficialEvents = []string{"paid"}
	cfg.Templates["paid"] = wechat.TemplateMapping{ID: "t", Fields: map[string]string{"thing1": "status"}}
	if err = repo.SaveExternalConfig(ctx, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	worker := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	if _, err = p.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(42,'TEST-42',8,1,1,1,false,'')`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET pay_status_id=2 WHERE id=42`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	deliveries, err := repo.ExternalDeliveries(ctx)
	if err != nil || len(deliveries) != 2 || sender.count != 1 {
		t.Fatalf("deliveries=%+v sent=%d err=%v", deliveries, sender.count, err)
	}
	if sender.notices[0].Milestones[0] != "paid" {
		t.Fatalf("wrong official milestone %+v", sender.notices[0])
	}
	for _, delivery := range deliveries {
		if delivery.Channel == "wechat_desktop" && (len(delivery.Notice.Milestones) != 1 || delivery.Notice.Milestones[0] != "created") {
			t.Fatalf("group leaked unrelated event %+v", delivery.Notice)
		}
	}
}

func TestBusinessTestRequiresRecipientOptInAndApprovedTemplate(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1); INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()); INSERT INTO %[1]s.orders VALUES(42,'TEST-42',8,1,1,1,false,'')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEvents = []string{}
	cfg.Templates["order_accepted"] = wechat.TemplateMapping{ID: "t", Fields: map[string]string{"const8": "status", "thing2": "items"}}
	if err = repo.SaveExternalConfig(ctx, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"order_accepted"}); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	worker := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	if _, err = repo.QueueTest(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "test"); err != nil {
		t.Fatal(err)
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	deliveries, err := repo.ExternalDeliveries(ctx)
	if err != nil || len(deliveries) != 1 || deliveries[0].Status != "failed" || sender.count != 0 {
		t.Fatalf("unapproved: %+v sent=%d err=%v", deliveries, sender.count, err)
	}
	cfg.Templates["order_accepted"] = wechat.TemplateMapping{ID: "t", Fields: map[string]string{"const8": "status", "thing2": "items"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}}
	if err = repo.SaveExternalConfig(ctx, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.QueueTestDetail(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "test", "", 42); err != nil {
		t.Fatal(err)
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 1 || !sender.notices[0].Test {
		t.Fatalf("no opted in test: %+v", sender)
	}
	deliveries, err = repo.ExternalDeliveries(ctx)
	if err != nil || len(deliveries) < 1 {
		t.Fatal(err)
	}
	testID := deliveries[0].ID
	if err = repo.VerifyBusinessTest(ctx, testID, "admin"); err == nil {
		t.Fatal("accepted is not delivered")
	}
	if err = repo.RecordProviderResult(ctx, "app", "101", "success"); err != nil {
		t.Fatal(err)
	}
	if err = repo.VerifyBusinessTest(ctx, testID, "admin"); err != nil {
		t.Fatal(err)
	}
	changed := cfg
	changed.Templates = map[string]wechat.TemplateMapping{"order_accepted": {ID: "other", Fields: map[string]string{"const8": "status", "thing2": "items"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}}}
	if err = repo.SaveExternalConfig(ctx, changed, "test"); err != nil {
		t.Fatal(err)
	}
	if err = repo.VerifyBusinessTest(ctx, testID, "admin"); err == nil {
		t.Fatal("different template accepted")
	}
	if err = repo.SaveExternalConfig(ctx, cfg, "test"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"created"}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.QueueTest(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "test"); err != nil {
		t.Fatal(err)
	}
	if err = worker.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 1 {
		t.Fatal("sent to opted-out recipient")
	}
	if _, err = repo.QueueTest(ctx, "order_accepted", "wechat_desktop", 0, 7, "app", "test"); err == nil {
		t.Fatal("business event accepted by group")
	}
}

func TestBusinessEventNeedsVerifiedMatchingTemplate(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	cfg := DefaultExternalConfig()
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"order_accepted"}
	cfg.Templates["order_accepted"] = wechat.TemplateMapping{ID: "accepted", Fields: map[string]string{"const8": "status", "thing2": "items"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}}
	if err := r.SaveExternalConfig(ctx, cfg, "test"); err == nil {
		t.Fatal("enabled without an actual verified test")
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "order_accepted", "order_accepted:55", 55, 8, map[string]any{"order_no": "SECRET"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var payload string
	var processed bool
	if err = p.QueryRow(ctx, "SELECT payload::text,processed_at IS NOT NULL FROM "+s+".message_business_events WHERE source_key='order_accepted:55'").Scan(&payload, &processed); err != nil {
		t.Fatal(err)
	}
	if payload != "{}" || !processed {
		t.Fatal("disabled event retained for future replay")
	}
}

func TestBusinessOrderEventRequiresRealTestAndRespectsConsent(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()); INSERT INTO %[1]s.orders VALUES(42,'TEST-42',8,1,1,1,false,'')`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"order_accepted"}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEvents = []string{}
	cfg.Templates["order_accepted"] = wechat.TemplateMapping{ID: "accept", Fields: map[string]string{"const8": "status", "thing2": "items"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	w := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	if _, err = repo.QueueTestDetail(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "admin", "", 42); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ds, err := repo.ExternalDeliveries(ctx)
	if err != nil || len(ds) != 1 {
		t.Fatal(err)
	}
	if err = repo.RecordProviderResult(ctx, "app", "101", "success"); err != nil {
		t.Fatal(err)
	}
	if err = repo.VerifyBusinessTest(ctx, ds[0].ID, "admin"); err != nil {
		t.Fatal(err)
	}
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"order_accepted"}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	event := map[string]any{"order_id": 42, "order_no": "TEST42", "customer_id": 8, "customer_name": "测试客户", "items": "咖啡", "occurred_at": "2026-11-11 10:00:00"}
	if err = RecordBusinessEvent(ctx, tx, s, "order_accepted", "order_accepted:42", 42, 8, event); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	transient := errors.New("temporary authorization failure")
	w.Authorize = func(context.Context, Recipient, app.OrderNotice) (bool, error) { return false, transient }
	if err = w.Tick(ctx); !errors.Is(err, transient) {
		t.Fatalf("transient error lost: %v", err)
	}
	var pending int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+s+".message_business_events WHERE processed_at IS NULL").Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("failed auth lost event: %d %v", pending, err)
	}
	w.Authorize = func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 2 || sender.notices[1].OrderNo != "TEST42" || sender.notices[1].Test {
		t.Fatalf("event not sent once: %+v", sender)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 2 {
		t.Fatal("event replayed")
	}
	tx, err = p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "order_accepted", "order_accepted:43", 43, 8, event); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	cfg.OfficialEnabled = false
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	cfg.OfficialEnabled = true
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 2 {
		t.Fatal("replayed event across disable and enable")
	}
	tx, err = p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "order_accepted", "order_accepted:44", 44, 8, event); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = w.CaptureBusinessCommitted(ctx); err != nil {
		t.Fatal(err)
	}
	cfg.OfficialEnabled = false
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	cfg.OfficialEnabled = true
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	// Simulate the race where a stale capture creates its delivery after re-enabling.
	if _, err = p.Exec(ctx, "UPDATE "+s+".message_deliveries SET created_at=now()+interval '1 second' WHERE status='ready'"); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 2 {
		t.Fatal("stale event sent after re-enable")
	}
}

func TestShipmentPerStatusProofAndNoHistoricalReplay(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()); INSERT INTO %[1]s.orders VALUES(42,'TEST-42',8,1,1,1,false,'')`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEvents = []string{}
	cfg.Templates["shipment_created"] = wechat.TemplateMapping{ID: "ship", Fields: map[string]string{"const14": "status", "thing5": "items", "character_string2": "tracking_no"}, ApprovedEnums: map[string][]string{"const14": {"已发货", "部分发货"}}}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	w := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	testStatus := func(value, msgID string) {
		t.Helper()
		_, e := repo.QueueTestDetail(ctx, "shipment_created", "wechat_service_account", 1, 0, "app", "admin", value, 42)
		if e != nil {
			t.Fatal(e)
		}
		if e = w.Tick(ctx); e != nil {
			t.Fatal(e)
		}
		ds, e := repo.ExternalDeliveries(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if e = repo.RecordProviderResult(ctx, "app", msgID, "success"); e != nil {
			t.Fatal(e)
		}
		if e = repo.VerifyBusinessTest(ctx, ds[0].ID, "admin"); e != nil {
			t.Fatal(e)
		}
	}
	testStatus("已发货", "101")
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"shipment_created"}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	record := func(key, status string) {
		t.Helper()
		tx, e := p.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		payload := map[string]any{"order_id": 42, "order_no": "TEST42", "customer_id": 8, "items": "测试", "tracking_no": key, "shipping_status": status}
		if e = RecordBusinessEvent(ctx, tx, s, "shipment_created", key, 42, 8, payload); e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
	record("partial1", "部分发货")
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 1 {
		t.Fatal("partial sent without proof")
	}
	testStatus("部分发货", "102")
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 2 {
		t.Fatal("historical partial replayed")
	}
	record("partial2", "部分发货")
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 3 || sender.notices[2].StatusText() != "部分发货" {
		t.Fatalf("partial not delivered correctly: %+v", sender.notices)
	}
	if _, err = repo.QueueTestStatus(ctx, "shipment_created", "wechat_service_account", 1, 0, "app", "admin", "已签收"); err == nil {
		t.Fatal("invalid test status accepted")
	}
}

func TestInvalidNotificationConfigDoesNotAbortBusinessTransaction(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	if _, err := p.Exec(ctx, "UPDATE "+s+".message_external_settings SET config='{"+`"official_enabled":"bad"`+"}'::jsonb WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "order_accepted", "invalid-config", 42, 8, map[string]any{"order_no": "SECRET"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var reason, payload string
	if err = p.QueryRow(ctx, "SELECT failure_reason,payload::text FROM "+s+".message_business_events WHERE source_key='invalid-config'").Scan(&reason, &payload); err != nil {
		t.Fatal(err)
	}
	if reason == "" || payload != "{}" {
		t.Fatalf("invalid config was silently lost: %q %q", reason, payload)
	}
}

func TestTestRecipientMustBeUnique(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','one',1,8,true,true,true,now()),('app','two',1,8,true,true,true,now())`, s))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.QueueTestStatus(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "test", ""); err == nil {
		t.Fatal("ambiguous test recipient accepted")
	}
}

func TestFinancialDetailRequiresExactCustomerAndActiveRecord(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.cash_receipts(id bigint,receipt_no text,customer_id bigint,receipt_date date,actual_cents bigint,receivable_cents bigint,reversed_at timestamptz,allocations jsonb,source_type text DEFAULT 'single_payment',source_id bigint DEFAULT 1);
 CREATE TABLE %[1]s.customer_settlement_payments(id bigint,customer_id bigint,settlement_batch_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_settlement_lines(id bigint,customer_id bigint,settlement_batch_id bigint,source_type text,source_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_fee_items(id bigint,customer_id bigint,source_type text,source_id bigint);
 CREATE TABLE %[1]s.freight_actual_entries(id bigint,shipment_id bigint);
 CREATE TABLE %[1]s.freight_actual_shipments(id bigint,order_id bigint,customer_id bigint);
 CREATE TABLE %[1]s.customer_settlement_batches(id bigint,customer_id bigint,settlement_no text,period_from date,period_to date,total_amount numeric,status text,processing_billing_run_id bigint);
 INSERT INTO %[1]s.orders VALUES(42,'ORDER42',8,1,1,1,false,'');
 INSERT INTO %[1]s.cash_receipts(id,receipt_no,customer_id,receipt_date,actual_cents,receivable_cents,reversed_at,allocations) VALUES(1,'RC1',8,'2026-11-11',100,200,NULL,'[{"order_id":42}]'),(2,'RC2',9,'2026-11-11',500,500,NULL,'[]'),(3,'RC3',8,'2026-11-11',500,500,now(),'[]');
 INSERT INTO %[1]s.customer_settlement_batches VALUES(5,8,'BILL5','2026-11-01','2026-11-11',10,'confirmed',NULL),(6,9,'BILL6','2026-11-01','2026-11-11',10,'confirmed',NULL),(7,8,'EMPTY','2026-11-01','2026-11-11',10,'confirmed',NULL); INSERT INTO %[1]s.customer_settlement_lines VALUES(50,8,5,'order',42,NULL);
 INSERT INTO %[1]s.customer_fee_items VALUES(71,8,'order',42),(72,8,'freight_actual',73);
 INSERT INTO %[1]s.freight_actual_entries VALUES(73,74);
 INSERT INTO %[1]s.freight_actual_shipments VALUES(74,42,8);
 INSERT INTO %[1]s.customer_settlement_lines VALUES(51,8,5,'customer_fee_item',71,NULL),(52,8,5,'customer_fee_item',72,NULL);
 INSERT INTO %[1]s.customer_settlement_payments VALUES(15,8,5,NULL);
 INSERT INTO %[1]s.cash_receipts VALUES(4,'RC4',8,'2026-11-11',100,200,NULL,'[{"line_id":50},{"line_id":51},{"line_id":52}]','billing_payment',15)`, s))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := repo.ReceiptDetail(ctx, 8, 1)
	if err != nil || receipt.ReceiptNo != "RC1" || receipt.ActualCents != 100 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 2); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-customer receipt: %v", err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 3); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reversed receipt: %v", err)
	}
	bill, err := repo.BillDetail(ctx, 8, 5)
	if err != nil || bill.BillNo != "BILL5" || bill.AmountCents != 1000 {
		t.Fatalf("bill=%+v err=%v", bill, err)
	}
	if _, err = repo.BillDetail(ctx, 8, 6); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-customer bill: %v", err)
	}
	if _, err = repo.BillDetail(ctx, 8, 7); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("empty source bill allowed: %v", err)
	}

	if _, err = repo.ReceiptDetail(ctx, 8, 4); err != nil {
		t.Fatalf("mixed bill receipt not visible: %v", err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".customer_fee_items SET customer_id=9 WHERE id=71"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.BillDetail(ctx, 8, 5); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign fee leaked bill: %v", err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 4); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign fee leaked receipt: %v", err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".customer_fee_items SET customer_id=8 WHERE id=71; UPDATE "+s+".freight_actual_shipments SET customer_id=NULL WHERE id=74"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.BillDetail(ctx, 8, 5); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("null freight owner leaked bill: %v", err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".freight_actual_shipments SET customer_id=8 WHERE id=74"); err != nil {
		t.Fatal(err)
	}

	if _, err = p.Exec(ctx, "UPDATE "+s+".orders SET customer_id=9 WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 1); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("source ownership changed: %v", err)
	}
	if _, err = repo.BillDetail(ctx, 8, 5); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign order leaked bill: %v", err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 4); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("foreign order leaked bill receipt: %v", err)
	}

	if _, err = p.Exec(ctx, "UPDATE "+s+".orders SET customer_id=NULL WHERE id=42"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 1); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("null source ownership allowed: %v", err)
	}
	if _, err = repo.BillDetail(ctx, 8, 5); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("null order owner leaked bill: %v", err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 4); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("null order owner leaked bill receipt: %v", err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".customer_settlement_lines SET reversed_at=now() WHERE id=50"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.BillDetail(ctx, 8, 5); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reversed historical order leaked bill: %v", err)
	}
	if _, err = repo.ReceiptDetail(ctx, 8, 4); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reversed historical order leaked receipt: %v", err)
	}

}

func TestFinancialBusinessEventsRequireProofAndRedactAdminOutput(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now());
 CREATE TABLE %[1]s.cash_receipts(id bigint,receipt_no text,customer_id bigint,receipt_date date,actual_cents bigint,receivable_cents bigint,reversed_at timestamptz,allocations jsonb,source_type text,source_id bigint);
 CREATE TABLE %[1]s.customer_settlement_payments(id bigint,customer_id bigint,settlement_batch_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_settlement_lines(id bigint,customer_id bigint,settlement_batch_id bigint,source_type text,source_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_fee_items(id bigint,customer_id bigint,source_type text,source_id bigint);
 CREATE TABLE %[1]s.freight_actual_entries(id bigint,shipment_id bigint);
 CREATE TABLE %[1]s.freight_actual_shipments(id bigint,order_id bigint,customer_id bigint);
 CREATE TABLE %[1]s.customer_settlement_batches(id bigint,customer_id bigint,settlement_no text,period_from date,period_to date,total_amount numeric,status text,processing_billing_run_id bigint);
 INSERT INTO %[1]s.orders VALUES(42,'ORDER42',8,1,1,1,false,'');
 INSERT INTO %[1]s.cash_receipts VALUES(11,'RC11',8,'2026-11-11',2500,10000,NULL,'[{"order_id":42}]','single_payment',1);
 INSERT INTO %[1]s.customer_settlement_batches VALUES(12,8,'BILL12','2026-11-01','2026-11-11',100,'confirmed',NULL); INSERT INTO %[1]s.customer_settlement_lines VALUES(50,8,12,'order',42,NULL)`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"receipt_received", "bill_issued"}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEvents = []string{}
	cfg.Templates["receipt_received"] = wechat.TemplateMapping{ID: "receipt", Fields: map[string]string{"thing1": "customer_name", "time2": "receipt_date", "amount3": "receivable_amount", "character_string4": "receipt_no", "amount6": "received_amount"}}
	cfg.Templates["bill_issued"] = wechat.TemplateMapping{ID: "bill", Fields: map[string]string{"thing1": "customer_name", "character_string2": "bill_no", "amount3": "bill_amount", "time4": "bill_period"}}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	w := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(ctx context.Context, v Recipient, n app.OrderNotice) (bool, error) {
		if v.CustomerID != n.CustomerID {
			return false, nil
		}
		if n.ReceiptID > 0 {
			_, e := repo.ReceiptDetail(ctx, v.CustomerID, n.ReceiptID)
			if errors.Is(e, pgx.ErrNoRows) {
				return false, nil
			}
			return e == nil, e
		}
		if n.BillID > 0 {
			_, e := repo.BillDetail(ctx, v.CustomerID, n.BillID)
			if errors.Is(e, pgx.ErrNoRows) {
				return false, nil
			}
			return e == nil, e
		}
		return false, nil
	}}
	for i, event := range []string{"receipt_received", "bill_issued"} {
		detailID := int64(11)
		if event == "bill_issued" {
			detailID = 12
		}
		if _, err = repo.QueueTestDetail(ctx, event, "wechat_service_account", 1, 0, "app", "admin", "", detailID); err != nil {
			t.Fatal(err)
		}
		if err = w.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		ds, e := repo.ExternalDeliveries(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if err = repo.RecordProviderResult(ctx, "app", fmt.Sprint(101+i), "success"); err != nil {
			t.Fatal(err)
		}
		if err = repo.VerifyBusinessTest(ctx, ds[0].ID, "admin"); err != nil {
			t.Fatal(err)
		}
	}
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"receipt_received", "bill_issued"}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "receipt_received", "receipt:11", 42, 8, map[string]any{"receipt_id": 11}); err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "bill_issued", "bill:12", 0, 8, map[string]any{"bill_id": 12}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 4 {
		t.Fatalf("expected two financial events: %+v", sender.notices)
	}
	if sender.notices[2].ReceiptNo != "RC11" || sender.notices[2].ReceivedAmount != "25.00" || sender.notices[2].ReceivableAmount != "100.00" {
		t.Fatalf("receipt fields: %+v", sender.notices[2])
	}
	if sender.notices[3].BillNo != "BILL12" || sender.notices[3].BillPeriod != "2026-11-01~2026-11-11" {
		t.Fatalf("bill fields: %+v", sender.notices[3])
	}
	ds, err := repo.ExternalDeliveries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if !d.Notice.Test && (d.Notice.ReceiptNo != "" || d.Notice.BillNo != "" || d.Notice.ReceivedAmount != "" || d.Notice.CustomerName != "") {
			t.Fatalf("financial data leaked to settings: %+v", d.Notice)
		}
	}
}

func TestFinancialSnapshotTransientFailurePreservesReferenceAndRetries(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1);INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()-interval '1 minute');
 CREATE TABLE %[1]s.customer_settlement_payments(id bigint,customer_id bigint,settlement_batch_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_settlement_batches(id bigint,customer_id bigint,settlement_no text,period_from date,period_to date,total_amount numeric,status text,processing_billing_run_id bigint);
 CREATE TABLE %[1]s.customer_settlement_lines(id bigint,customer_id bigint,settlement_batch_id bigint,source_type text,source_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_fee_items(id bigint,customer_id bigint,source_type text,source_id bigint);
 CREATE TABLE %[1]s.freight_actual_entries(id bigint,shipment_id bigint);
 CREATE TABLE %[1]s.freight_actual_shipments(id bigint,order_id bigint,customer_id bigint);
 INSERT INTO %[1]s.orders VALUES(42,'ORDER42',8,1,1,1,false,'')`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"receipt_received"}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEnabled = true
	cfg.OfficialSince = time.Now().Add(-time.Minute)
	cfg.OfficialEvents = []string{"receipt_received"}
	cfg.OfficialEventSince = map[string]time.Time{"receipt_received": time.Now().Add(-time.Minute)}
	cfg.Templates["receipt_received"] = wechat.TemplateMapping{ID: "receipt", Fields: map[string]string{"thing1": "customer_name", "time2": "receipt_date", "amount3": "receivable_amount", "character_string4": "receipt_no", "amount6": "received_amount"}}
	raw, _ := json.Marshal(cfg)
	_, err = p.Exec(ctx, "UPDATE "+s+".message_external_settings SET config=$1 WHERE id=1", raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, "INSERT INTO "+s+".message_business_test_verifications(event,status_value,template_fingerprint,delivery_id,verified_by,detail_verified) VALUES($1,$2,$3,1,'test',true)", "receipt_received", "货款到账", businessTemplateFingerprint(cfg.Templates["receipt_received"]))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = RecordBusinessEvent(ctx, tx, s, "receipt_received", "receipt:11", 42, 8, map[string]any{"receipt_id": 11, "amount_cents": 12345, "customer_name": "secret"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var pending bool
	var eventRaw string
	if err = p.QueryRow(ctx, "SELECT snapshot_pending,payload::text FROM "+s+".message_business_events WHERE source_key='receipt:11'").Scan(&pending, &eventRaw); err != nil {
		t.Fatal(err)
	}
	if !pending || strings.Contains(eventRaw, "secret") || strings.Contains(eventRaw, "12345") {
		t.Fatalf("unsafe deferred payload: %s", eventRaw)
	}
	sender := &noticeSender{}
	w := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return true, nil }}
	if err = w.Tick(ctx); err == nil {
		t.Fatal("database failure should retry")
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+s+".message_business_events WHERE processed_at IS NULL").Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed snapshot lost event: %d %v", count, err)
	}
	_, err = p.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.cash_receipts(id bigint,receipt_no text,customer_id bigint,receipt_date date,actual_cents bigint,receivable_cents bigint,reversed_at timestamptz,allocations jsonb,source_type text,source_id bigint);
 INSERT INTO %[1]s.cash_receipts VALUES(11,'RC11',8,'2026-11-11',2500,10000,NULL,'[{"order_id":42}]','single_payment',1)`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 1 || sender.notices[0].ReceiptNo != "RC11" {
		t.Fatalf("event did not recover: %+v", sender.notices)
	}
}

func TestVirtualTestCannotEnableAndRealDetailMustBeAuthorized(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	repo := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1); INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','openid',1,8,true,true,true,now()); INSERT INTO %[1]s.orders VALUES(42,'TEST42',8,1,1,1,false,'')`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"order_accepted"}); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultExternalConfig()
	cfg.OfficialEvents = []string{}
	cfg.Templates["order_accepted"] = wechat.TemplateMapping{ID: "t", Fields: map[string]string{"const8": "status", "thing2": "items"}, ApprovedEnums: map[string][]string{"const8": {"已接单"}}}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err != nil {
		t.Fatal(err)
	}
	sender := &noticeSender{}
	w := Worker{Repo: repo, AppID: "app", Sender: sender, Authorize: func(context.Context, Recipient, app.OrderNotice) (bool, error) { return false, nil }}
	if _, err = repo.QueueTest(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "admin"); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	ds, err := repo.ExternalDeliveries(ctx)
	if err != nil || len(ds) != 1 {
		t.Fatal(err)
	}
	if err = repo.RecordProviderResult(ctx, "app", "101", "success"); err != nil {
		t.Fatal(err)
	}
	if err = repo.VerifyBusinessTest(ctx, ds[0].ID, "admin"); err == nil {
		t.Fatal("virtual preference link accepted as actual detail proof")
	}
	cfg.OfficialEnabled = true
	cfg.OfficialEvents = []string{"order_accepted"}
	if err = repo.SaveExternalConfig(ctx, cfg, "admin"); err == nil {
		t.Fatal("virtual test enabled event")
	}
	if _, err = repo.QueueTestDetail(ctx, "order_accepted", "wechat_service_account", 1, 0, "app", "admin", "", 42); err != nil {
		t.Fatal(err)
	}
	if err = w.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if sender.count != 1 {
		t.Fatal("unauthorized real-detail test sent")
	}
	ds, err = repo.ExternalDeliveries(ctx)
	if err != nil || len(ds) != 2 || ds[0].Status != "skipped" {
		t.Fatalf("unauthorized detail test: %+v %v", ds, err)
	}
}
