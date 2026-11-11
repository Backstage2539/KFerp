package messagecenter

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNotificationPromptChoiceAndReselect(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Version != 1 || state.Choice != nil {
		t.Fatalf("initial=%+v err=%v", state, err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"order_accepted", "receipt_received"}); err != nil {
		t.Fatal(err)
	}
	state, err = r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || state.Required || state.Choice == nil || !state.Choice.Enabled || len(state.Choice.Events) != 2 {
		t.Fatalf("saved=%+v err=%v", state, err)
	}
	other, err := r.NotificationPrompt(ctx, "app", 1, 9, 0)
	if err != nil || !other.Required || other.Choice != nil {
		t.Fatalf("cross customer=%+v err=%v", other, err)
	}
	if err = r.RequireNotificationReselect(ctx, "admin"); err != nil {
		t.Fatal(err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, false, nil); err == nil {
		t.Fatal("stale version accepted")
	}
	state, err = r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Version != 2 || state.Choice == nil || !state.Choice.Enabled {
		t.Fatalf("reselect=%+v err=%v", state, err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 2, false, nil); err != nil {
		t.Fatal(err)
	}
	state, err = r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || state.Required || state.Choice == nil || state.Choice.Enabled {
		t.Fatalf("declined=%+v err=%v", state, err)
	}
	var count int
	if err = p.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='notification_prompt_reselect'`, s)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit=%d err=%v", count, err)
	}
}

func TestPromptConcurrentReselectNeverLosesNewRequest(t *testing.T) {
	p, s := notificationDB(t)
	_ = p
	ctx := context.Background()
	r := NewRepository(p, s)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var saveErr, adminErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		saveErr = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"})
	}()
	go func() { defer wg.Done(); <-start; adminErr = r.RequireNotificationReselect(ctx, "admin") }()
	close(start)
	wg.Wait()
	if adminErr != nil {
		t.Fatal(adminErr)
	}
	if saveErr != nil && saveErr != ErrNotificationChoiceStale {
		t.Fatal(saveErr)
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || state.Version != 2 || !state.Required {
		t.Fatalf("lost reselect: %+v %v", state, err)
	}
}

func TestConcurrentUnsubscribeAndChoiceNeverDeadlockOrSend(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1); INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','old',1,8,0,true,true,true,now())`, s)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := p.Exec(ctx, fmt.Sprintf(`UPDATE %s.wechat_bindings SET active=true,subscribed=true,notifications_enabled=true,notification_subscription_time=0`, s)); err != nil {
			t.Fatal(err)
		}
		if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		errs := make(chan error, 2)
		go func() { <-start; errs <- r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}) }()
		go func() { <-start; errs <- r.SubscriptionEvent(ctx, "app", "old", false, 1) }()
		close(start)
		for j := 0; j < 2; j++ {
			select {
			case err := <-errs:
				if err != nil {
					t.Fatalf("concurrent op %d: %v", i, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("unsubscribe and choice deadlocked")
			}
		}
		var active, subscribed, enabled bool
		if err := p.QueryRow(ctx, "SELECT active,subscribed,notifications_enabled FROM "+s+".wechat_bindings WHERE app_id='app' AND openid='old'").Scan(&active, &subscribed, &enabled); err != nil || active || subscribed || enabled {
			t.Fatalf("unsubscribe left delivery enabled: %v %v %v %v", active, subscribed, enabled, err)
		}
	}
}

func TestFirstBindingUnsubscribeCannotBeLost(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, "INSERT INTO "+s+".mini_users VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err != nil {
		t.Fatal(err)
	}
	token, err := r.StartNotificationBinding(ctx, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RecordNotificationScan(ctx, token, "app", "openid", 100); err != nil {
		t.Fatal(err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "openid", false, 101); err != nil {
		t.Fatal(err)
	}
	if err = r.ConfirmNotificationBinding(ctx, token, "app", 1, 8, 0); err == nil {
		t.Fatal("binding ignored unsubscribe before row exists")
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+s+".wechat_bindings WHERE active AND notifications_enabled").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unexpected enabled binding: %d %v", count, err)
	}
}

func TestOlderUnsubscribeDoesNotDisableNewlyConfirmedBinding(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, "INSERT INTO "+s+".mini_users VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err != nil {
		t.Fatal(err)
	}
	token, err := r.StartNotificationBinding(ctx, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RecordNotificationScan(ctx, token, "app", "openid", 101); err != nil {
		t.Fatal(err)
	}
	if err = r.ConfirmNotificationBinding(ctx, token, "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	if err = r.SubscriptionEvent(ctx, "app", "openid", false, 100); err != nil {
		t.Fatal(err)
	}
	var active, enabled bool
	var subscriptionTime int64
	if err = p.QueryRow(ctx, "SELECT active,notifications_enabled,notification_subscription_time FROM "+s+".wechat_bindings WHERE openid='openid'").Scan(&active, &enabled, &subscriptionTime); err != nil || !active || !enabled || subscriptionTime != 101 {
		t.Fatalf("old unsubscribe disabled current binding: active=%t enabled=%t time=%d err=%v", active, enabled, subscriptionTime, err)
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || state.Required || state.Choice == nil || !state.Choice.Enabled {
		t.Fatalf("old unsubscribe reset current choice: %+v %v", state, err)
	}
}

func TestFirstBindingConcurrentUnsubscribeNeverSends(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, "INSERT INTO "+s+".mini_users VALUES(1)"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err != nil {
		t.Fatal(err)
	}
	token, err := r.StartNotificationBinding(ctx, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RecordNotificationScan(ctx, token, "app", "openid", 100); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	confirmDone := make(chan error, 1)
	unsubscribeDone := make(chan error, 1)
	go func() { <-start; confirmDone <- r.ConfirmNotificationBinding(ctx, token, "app", 1, 8, 0) }()
	go func() { <-start; unsubscribeDone <- r.SubscriptionEvent(ctx, "app", "openid", false, 101) }()
	close(start)
	select {
	case <-confirmDone:
	case <-time.After(5 * time.Second):
		t.Fatal("confirmation deadlocked")
	}
	select {
	case err := <-unsubscribeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("unsubscribe deadlocked")
	}
	var count int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM "+s+".wechat_bindings WHERE active AND notifications_enabled").Scan(&count); err != nil || count != 0 {
		t.Fatalf("unsubscribe lost: %d %v", count, err)
	}
}

func TestNotificationChoiceLocksCurrentSessionIdentity(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, "CREATE TABLE "+s+".mini_sessions(token TEXT,mini_user_id BIGINT,current_customer_id BIGINT,expire_at TIMESTAMPTZ);INSERT INTO "+s+".mini_sessions VALUES('token',1,8,now()+interval '1 hour')"); err != nil {
		t.Fatal(err)
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE "+s+".mini_sessions SET current_customer_id=9 WHERE token='token'"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}, "token") }()
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err != ErrNotificationIdentityChanged {
			t.Fatalf("changed session accepted: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session lock deadlocked")
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Choice != nil {
		t.Fatalf("stale identity wrote choice: %+v %v", state, err)
	}
}

func TestPromptAuditFailureRollsBackBothVersionAndChoice(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, "DROP TABLE "+s+".audit_logs"); err != nil {
		t.Fatal(err)
	}
	if err := r.RequireNotificationReselect(ctx, "admin"); err == nil {
		t.Fatal("missing audit accepted")
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err == nil {
		t.Fatal("missing audit accepted")
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || state.Version != 1 || !state.Required || state.Choice != nil {
		t.Fatalf("partial commit: %+v %v", state, err)
	}
}

func TestNotificationChoiceAppliedOnlyAfterBinding(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users VALUES(1)`, s)); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"order_accepted"}); err != nil {
		t.Fatal(err)
	}
	token, err := r.StartNotificationBinding(ctx, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RecordNotificationScan(ctx, token, "app", "openid"); err != nil {
		t.Fatal(err)
	}
	if err = r.ConfirmNotificationBinding(ctx, token, "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	prefs, err := r.NotificationPreferences(ctx, "app", 1)
	if err != nil || len(prefs) != 1 || !prefs[0].Enabled || len(prefs[0].Events) != 1 || prefs[0].Events[0] != "order_accepted" {
		t.Fatalf("bound=%+v err=%v", prefs, err)
	}
	if err = r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, false, nil); err != nil {
		t.Fatal(err)
	}
	prefs, err = r.NotificationPreferences(ctx, "app", 1)
	if err != nil || prefs[0].Enabled {
		t.Fatalf("declined bound=%+v err=%v", prefs, err)
	}
}

func TestNotificationReselectDoesNotEraseChoiceAndLegacyCannotAcknowledgeIt(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1); INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,notification_events,notification_consent_at) VALUES('app','old',1,8,0,true,true,true,'["created"]',now())`, s)); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"created"}, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.RequireNotificationReselect(ctx, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationPreference(ctx, "app", 1, 8, 0, false, nil, 1); err != ErrNotificationChoiceStale {
		t.Fatalf("stale=%v", err)
	}
	prefs, err := r.NotificationPreferences(ctx, "app", 1)
	if err != nil || !prefs[0].Enabled || len(prefs[0].Events) != 1 {
		t.Fatalf("reselect changed actual preference: %+v %v", prefs, err)
	}
	if err := r.SaveNotificationPreference(ctx, "app", 1, 8, 0, true, []string{"paid"}); err != nil {
		t.Fatal(err)
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Version != 2 {
		t.Fatalf("legacy suppressed reselect: %+v %v", state, err)
	}
}

func TestNotificationUnsubscribeRequiresNewConsent(t *testing.T) {
	p, s := notificationDB(t)
	ctx := context.Background()
	r := NewRepository(p, s)
	if _, err := p.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.mini_users VALUES(1); INSERT INTO %[1]s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled,notification_consent_at) VALUES('app','old',1,8,0,true,true,true,now())`, s)); err != nil {
		t.Fatal(err)
	}
	if err := r.SaveNotificationChoice(ctx, "app", 1, 8, 0, 1, true, []string{"created"}); err != nil {
		t.Fatal(err)
	}
	if err := r.SubscriptionEvent(ctx, "app", "old", false); err != nil {
		t.Fatal(err)
	}
	state, err := r.NotificationPrompt(ctx, "app", 1, 8, 0)
	if err != nil || !state.Required || state.Choice == nil || state.Choice.Enabled {
		t.Fatalf("unsubscribe consent: %+v %v", state, err)
	}
	token, err := r.StartNotificationBinding(ctx, 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RecordNotificationScan(ctx, token, "app", "old", time.Now().Unix()+1); err != nil {
		t.Fatal(err)
	}
	if err = r.ConfirmNotificationBinding(ctx, token, "app", 1, 8, 0); err != nil {
		t.Fatal(err)
	}
	prefs, err := r.NotificationPreferences(ctx, "app", 1)
	if err != nil || len(prefs) != 1 || prefs[0].Enabled {
		t.Fatalf("consent restored: %+v %v", prefs, err)
	}
}
