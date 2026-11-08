package messagecenter

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/messagecenter"
	wechat "orderapp/internal/infrastructure/wechataccount"
	"os"
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
