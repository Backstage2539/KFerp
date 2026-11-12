package messagecenter

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func eligibilityFixtures(t *testing.T, p *pgxpool.Pool, s string) {
	t.Helper()
	_, err := p.Exec(context.Background(), fmt.Sprintf(`ALTER TABLE %[1]s.customer_portal_user_bindings ADD COLUMN approved_by text NOT NULL DEFAULT '';
	CREATE TABLE %[1]s.customer_portal_profiles(customer_id bigint,enabled bool);INSERT INTO %[1]s.customer_portal_profiles VALUES(8,true),(9,true);
	CREATE TABLE %[1]s.customer_erp_user_bindings(employee_id bigint,customer_id bigint,status text);
	CREATE TABLE %[1]s.employee_roles(employee_id bigint,role_code text);
	CREATE TABLE %[1]s.auth_role_permissions(role_code text,permission_code text);`, s))
	if err != nil {
		t.Fatal(err)
	}
}

func TestIndividualNotificationReselectPreservesChoicesAndRejectsOldVersion(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.mini_users ADD COLUMN active bool DEFAULT true, ADD COLUMN openid text DEFAULT '', ADD COLUMN phone text DEFAULT '';
	ALTER TABLE %[1]s.customers ADD COLUMN active bool DEFAULT true;
	ALTER TABLE %[1]s.company_employees ADD COLUMN active bool DEFAULT true, ADD COLUMN account_type text DEFAULT 'internal_employee', ADD COLUMN phone text DEFAULT '';
	CREATE TABLE %[1]s.employee_login_passwords(employee_id bigint,login_disabled boolean);
	CREATE TABLE %[1]s.customer_portal_user_bindings(mini_user_id bigint,customer_id bigint,status text);
	INSERT INTO %[1]s.mini_users(id) VALUES(1),(2);
	INSERT INTO %[1]s.customer_portal_user_bindings VALUES(1,8,'approved'),(2,8,'approved');
	INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_events) VALUES('app','one',1,8,true,true,true,'["shipment_created"]'),('app','two',2,8,true,true,true,'["shipment_created"]')`, s)); err != nil {
		t.Fatal(err)
	}
	eligibilityFixtures(t, p, s)
	for _, id := range []int64{1, 2} {
		if err := r.SaveNotificationChoice(ctx, "app", id, 8, 0, 1, true, []string{"shipment_created"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	a, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !a.Required || a.Version != 2 || a.Choice == nil || !a.Choice.Enabled {
		t.Fatalf("target=%+v %v", a, err)
	}
	b, err := r.NotificationPrompt(ctx, "app", 2, 8, 0)
	if err != nil || b.Required || b.Version != 1 {
		t.Fatalf("other=%+v %v", b, err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, false, nil); err != ErrNotificationChoiceStale {
		t.Fatalf("old popup save: %v", err)
	}
	if err = r.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	a, err = r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !a.Required || !a.Choice.Enabled {
		t.Fatalf("legacy consumed request: %+v %v", a, err)
	}
	if err = r.RequireNotificationReselect(ctx, "admin"); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 2, true, []string{"shipment_created"}); err != ErrNotificationChoiceStale {
		t.Fatalf("global stale=%v", err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 3, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err = r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	a, err = r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !a.Required || a.Version != 4 {
		t.Fatalf("repeat=%+v %v", a, err)
	}
	var count int
	if err = p.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='notification_identity_reselect'`, s)).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit %d %v", count, err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+s+".customer_portal_user_bindings SET status='revoked' WHERE mini_user_id=1"); err != nil {
		t.Fatal(err)
	}
	if err = r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err == nil {
		t.Fatal("revoked identity allowed")
	}
}

func TestConcurrentIndividualReselectKeepsBothRequests(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.mini_users ADD COLUMN active bool DEFAULT true, ADD COLUMN openid text DEFAULT '', ADD COLUMN phone text DEFAULT ''; ALTER TABLE %[1]s.customers ADD COLUMN active bool DEFAULT true; ALTER TABLE %[1]s.company_employees ADD COLUMN active bool DEFAULT true, ADD COLUMN account_type text DEFAULT 'internal_employee', ADD COLUMN phone text DEFAULT ''; CREATE TABLE %[1]s.employee_login_passwords(employee_id bigint,login_disabled boolean); CREATE TABLE %[1]s.customer_portal_user_bindings(mini_user_id bigint,customer_id bigint,status text); INSERT INTO %[1]s.mini_users(id) VALUES(1);INSERT INTO %[1]s.customer_portal_user_bindings VALUES(1,8,'approved')`, s))
	if err != nil {
		t.Fatal(err)
	}
	eligibilityFixtures(t, p, s)
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 3)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0)
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); <-start; errs <- r.RequireNotificationReselect(ctx, "admin") }()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Version != 4 {
		t.Fatalf("concurrent=%+v %v", state, err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 3, true, []string{"shipment_created"}); err != ErrNotificationChoiceStale {
		t.Fatalf("stale accepted: %v", err)
	}
}

func TestNotificationSubscribersShowsUnboundAndMultipleAccounts(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	_, err := p.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.customers ADD COLUMN active boolean DEFAULT true, ADD COLUMN phone text DEFAULT '';
	ALTER TABLE %[1]s.company_employees ADD COLUMN name text DEFAULT '',ADD COLUMN phone text DEFAULT '',ADD COLUMN active boolean DEFAULT true,ADD COLUMN account_type text DEFAULT 'internal_employee';
	ALTER TABLE %[1]s.mini_users ADD COLUMN openid text DEFAULT '',ADD COLUMN phone text DEFAULT '',ADD COLUMN nickname text DEFAULT '',ADD COLUMN active boolean DEFAULT true;
	CREATE TABLE %[1]s.customer_portal_user_bindings(mini_user_id bigint,customer_id bigint,status text);
	CREATE TABLE %[1]s.employee_login_passwords(employee_id bigint,login_disabled boolean);
	UPDATE %[1]s.customers SET phone='13800000000' WHERE id=8;
	INSERT INTO %[1]s.customers(id,name,active,phone) VALUES(9,'未注册客户',true,'13800000009');
	INSERT INTO %[1]s.company_employees(id,name,phone) VALUES(7,'销售员工','13800000007');
	INSERT INTO %[1]s.mini_users(id,openid,phone,nickname) VALUES(1,'person-one','13900000001','第一账号'),(2,'person-two','13900000002','第二账号');
	INSERT INTO %[1]s.customer_portal_user_bindings VALUES(1,8,'approved'),(2,8,'approved');
	INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,active,subscribed,notifications_enabled,notification_events) VALUES('app','official-one',1,8,true,true,true,'["shipment_created"]'),('app','official-two',1,8,true,true,false,'[]'),('app','official-old',1,8,false,false,false,'[]');`, s))
	if err != nil {
		t.Fatal(err)
	}
	eligibilityFixtures(t, p, s)
	items, err := r.NotificationSubscribers(ctx, "app", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 2 customer accounts, unregistered customer and employee: %+v", items)
	}
	byID := map[int64]NotificationSubscriber{}
	for _, v := range items {
		if v.CustomerID == 8 {
			byID[v.MiniUserID] = v
		}
	}
	if !byID[1].HasBinding || !byID[1].Subscribed || !byID[1].NotificationsEnabled || byID[1].BindingCount != 3 || byID[1].ActiveBindingCount != 2 || byID[1].SubscribedBindingCount != 2 || byID[1].EnabledBindingCount != 1 || byID[1].CustomerPhone != "13800000000" || byID[1].AccountPhone != "13900000001" {
		t.Fatalf("bound=%+v", byID[1])
	}
	if byID[2].HasBinding || !byID[2].IdentityActive || !byID[2].PromptRequired {
		t.Fatalf("unbound=%+v", byID[2])
	}
	_, err = p.Exec(ctx, "UPDATE "+s+".customer_portal_profiles SET enabled=false WHERE customer_id=8")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err == nil {
		t.Fatal("disabled portal permitted")
	}
	_, err = p.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.customer_portal_profiles SET enabled=true WHERE customer_id=8; INSERT INTO %[1]s.company_employees(id,name,phone,account_type) VALUES(10,'客户账户','13810000010','channel_customer'); INSERT INTO %[1]s.customer_erp_user_bindings VALUES(10,8,'active');UPDATE %[1]s.mini_users SET openid='erp-employee:10' WHERE id=1;UPDATE %[1]s.customer_portal_user_bindings SET approved_by='erp-password-login:10' WHERE mini_user_id=1`, s))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	_, err = p.Exec(ctx, "UPDATE "+s+".customer_erp_user_bindings SET status='revoked'")
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RequireIdentityNotificationReselect(ctx, "admin", "app", 1, 8, 0); err == nil {
		t.Fatal("revoked projected account permitted")
	}
	filtered, err := r.NotificationSubscribers(ctx, "app", "13900000001")
	if err != nil || len(filtered) != 1 || filtered[0].IdentityActive {
		t.Fatalf("revoked projected=%+v err=%v", filtered, err)
	}
	filtered, err = r.NotificationSubscribers(ctx, "app", "13800000009")
	if err != nil || len(filtered) != 1 || filtered[0].MiniUserID != 0 || filtered[0].CustomerID != 9 {
		t.Fatalf("unregistered=%+v err=%v", filtered, err)
	}
	_, err = p.Exec(ctx, "UPDATE "+s+".customer_portal_user_bindings SET status='revoked' WHERE mini_user_id=1")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err = r.NotificationSubscribers(ctx, "app", "13900000001")
	if err != nil || len(filtered) != 1 || filtered[0].IdentityActive {
		t.Fatalf("revoked=%+v err=%v", filtered, err)
	}
}
