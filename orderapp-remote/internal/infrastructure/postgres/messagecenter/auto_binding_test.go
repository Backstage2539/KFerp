package messagecenter

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestAutoBindHonorsChoiceAndRejectsStaleFollow(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id,unionid) VALUES (1,'union')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union"); err != nil {
		t.Fatal(err)
	}
	var active, enabled bool
	err = p.QueryRow(ctx, "SELECT active,notifications_enabled FROM "+schema+".wechat_bindings WHERE openid='official'").Scan(&active, &enabled)
	if err != nil || !active || !enabled {
		t.Fatalf("binding active=%v enabled=%v err=%v", active, enabled, err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "official", false, 200); err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 150); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union"); err != nil {
		t.Fatal(err)
	}
	err = p.QueryRow(ctx, "SELECT active,notifications_enabled FROM "+schema+".wechat_bindings WHERE openid='official'").Scan(&active, &enabled)
	if err != nil || active || enabled {
		t.Fatalf("stale follow reactivated: active=%v enabled=%v err=%v", active, enabled, err)
	}
}

func TestAutoBindConcurrentIdentitiesCannotReplaceOneAnother(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union'),(2,'union')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = r.AutoBindNotification(ctx, "app", int64(i+1), int64(i+8), 0, "union")
		}(i)
	}
	close(start)
	wg.Wait()
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("expected one winner: %v", errs)
	}
}

func TestAutoBindRejectsExpiredOrSwitchedSession(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.mini_sessions(token TEXT PRIMARY KEY,mini_user_id BIGINT,current_customer_id BIGINT,expire_at timestamptz);
		INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union');
		INSERT INTO %[1]s.mini_sessions VALUES('expired',1,8,now()-interval '1 hour'),('switched',1,9,now()+interval '1 hour')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"expired", "switched"} {
		if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union", token); err != ErrNotificationIdentityChanged {
			t.Fatalf("%s err=%v", token, err)
		}
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestAutoBindRejectsAmbiguousIdentities(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union'),(2,'union')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 2, 8, 0, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union"); err == nil {
		t.Fatal("ambiguous identity accepted")
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatalf("bindings=%d err=%v", count, err)
	}
}

func TestNotificationReportsMultipleWechatIdentitiesAndFollowers(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union');
		INSERT INTO %[1]s.mini_wechat_identities VALUES(1,'wechat-1','union-a'),(1,'wechat-2','union-b')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.NotificationUnionID(ctx, 1, 7); err != ErrNotificationBindingConflict {
		t.Fatalf("multiple employee identities: %v", err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official-1", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official-2", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union"); err != ErrNotificationBindingConflict {
		t.Fatalf("multiple follower identities: %v", err)
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatalf("binding count=%d err=%v", count, err)
	}
}

func TestAutoBindIgnoresPhysicalLoginWithoutBusinessChoice(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union'),(2,'');
		INSERT INTO %[1]s.mini_wechat_identities VALUES(2,'mini-openid','union')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 2, 0, 7, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 2, 0, 7, "union"); err != nil {
		t.Fatal(err)
	}
	var mini int
	if err = p.QueryRow(ctx, "SELECT mini_user_id FROM "+schema+".wechat_bindings WHERE openid='official'").Scan(&mini); err != nil || mini != 2 {
		t.Fatalf("mini=%d err=%v", mini, err)
	}
}

func TestFollowEventCanAddUnionIDWithoutReversingSameSecondUnsubscribe(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	if err := r.SubscriptionEvent(ctx, "app", "official", true, 100); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	var union string
	var subscribed bool
	if err := p.QueryRow(ctx, "SELECT unionid,subscribed FROM "+schema+".message_wechat_followers WHERE app_id='app' AND openid='official'").Scan(&union, &subscribed); err != nil || union != "union" || !subscribed {
		t.Fatalf("union=%q subscribed=%v err=%v", union, subscribed, err)
	}
	if err := r.SubscriptionEvent(ctx, "app", "official", false, 100); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, "SELECT subscribed FROM "+schema+".message_wechat_followers WHERE app_id='app' AND openid='official'").Scan(&subscribed); err != nil || subscribed {
		t.Fatalf("subscribed=%v err=%v", subscribed, err)
	}
}

func TestUnsubscribeInvalidatesPreviouslyDisplayedChoice(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users(id) VALUES(1);
		INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed) VALUES('app','official',1,8,0,true,true)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "official", false, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, state.Version, true, []string{"shipment_created"}); err != ErrNotificationChoiceStale {
		t.Fatalf("old prompt accepted: %v", err)
	}
}

func TestAutoBindRejectsRevokedCustomerAccess(t *testing.T) {
	p, schema := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, schema)
	_, err := p.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.mini_users ADD COLUMN openid text DEFAULT '', ADD COLUMN phone text DEFAULT '';
		ALTER TABLE %[1]s.customers ADD COLUMN active bool DEFAULT true;
		ALTER TABLE %[1]s.company_employees ADD COLUMN active bool DEFAULT true, ADD COLUMN account_type text, ADD COLUMN phone text;
		CREATE TABLE %[1]s.employee_login_passwords(employee_id bigint,login_disabled bool);
		CREATE TABLE %[1]s.customer_portal_user_bindings(mini_user_id bigint,customer_id bigint,status text);
		CREATE TABLE %[1]s.mini_sessions(token text,mini_user_id bigint,current_customer_id bigint,expire_at timestamptz);
		INSERT INTO %[1]s.mini_users(id,unionid) VALUES(1,'union');
		INSERT INTO %[1]s.customer_portal_user_bindings VALUES(1,8,'revoked');
		INSERT INTO %[1]s.mini_sessions VALUES('valid',1,8,now()+interval '1 hour')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	eligibilityFixtures(t, p, schema)
	if err = r.UpsertOfficialFollower(ctx, "app", "official", "union", true, 100); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union", "valid"); err != ErrNotificationIdentityChanged {
		t.Fatalf("revoked identity accepted: %v", err)
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if _, err = p.Exec(ctx, "UPDATE "+schema+".customer_portal_user_bindings SET status='approved' WHERE mini_user_id=1"); err != nil {
		t.Fatal(err)
	}
	if err = r.AutoBindNotification(ctx, "app", 1, 8, 0, "union", "valid"); err != nil {
		t.Fatalf("approved identity: %v", err)
	}
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+schema+".wechat_bindings").Scan(&count); err != nil || count != 1 {
		t.Fatalf("approved count=%d err=%v", count, err)
	}
}
