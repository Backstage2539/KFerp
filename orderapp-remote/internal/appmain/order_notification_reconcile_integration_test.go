package appmain

import (
	"testing"

	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
)

func TestNotificationReconcileFollowAfterLoginAndConflict(t *testing.T) {
	f := newSampleFixture(t)
	repo := messagepg.NewRepository(f.pool, f.schema)
	const appID = "notify-reconcile-app"

	// The logged-in sales employee chooses first. WeChat follows later, after
	// the verified mini-program identity was recorded by the login flow.
	f.exec("INSERT INTO %s.mini_wechat_identities(mini_user_id,openid,unionid) VALUES($1,'mini-openid-a','union-a')", f.sales.MiniUserID)
	if err := repo.SaveNotificationChoice(f.ctx, appID, f.sales.MiniUserID, 0, f.sales.EmployeeID, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertOfficialFollower(f.ctx, appID, "oa-a", "union-a", true, 100); err != nil {
		t.Fatal(err)
	}
	if err := messagepg.ReconcileWechatUnion(f.ctx, f.pool, f.schema, f.portal, appID, "union-a"); err != nil {
		t.Fatal(err)
	}
	var miniID, employeeID int64
	var active, subscribed, enabled bool
	err := f.pool.QueryRow(f.ctx, "SELECT mini_user_id,employee_id,active,subscribed,notifications_enabled FROM "+f.schema+".wechat_bindings WHERE app_id=$1 AND openid='oa-a'", appID).Scan(&miniID, &employeeID, &active, &subscribed, &enabled)
	if err != nil || miniID != f.sales.MiniUserID || employeeID != f.sales.EmployeeID || !active || !subscribed || !enabled {
		t.Fatalf("follow after selection: mini=%d employee=%d active=%v followed=%v enabled=%v err=%v", miniID, employeeID, active, subscribed, enabled, err)
	}

	// A second WeChat identity is already bound to a different employee.
	// Reconciliation must keep that existing association untouched.
	f.exec("INSERT INTO %s.mini_wechat_identities(mini_user_id,openid,unionid) VALUES($1,'mini-openid-b','union-b')", f.other.MiniUserID)
	if err = repo.SaveNotificationChoice(f.ctx, appID, f.other.MiniUserID, 0, f.other.EmployeeID, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	f.exec("INSERT INTO %s.wechat_bindings(app_id,openid,mini_user_id,customer_id,employee_id,active,subscribed,notifications_enabled) VALUES($1,'oa-b',$2,0,$3,true,true,true)", appID, f.sales.MiniUserID, f.sales.EmployeeID)
	if err = repo.UpsertOfficialFollower(f.ctx, appID, "oa-b", "union-b", true, 200); err != nil {
		t.Fatal(err)
	}
	if err = messagepg.ReconcileWechatUnion(f.ctx, f.pool, f.schema, f.portal, appID, "union-b"); err != nil {
		t.Fatal(err)
	}
	err = f.pool.QueryRow(f.ctx, "SELECT mini_user_id,employee_id,active,subscribed,notifications_enabled FROM "+f.schema+".wechat_bindings WHERE app_id=$1 AND openid='oa-b'", appID).Scan(&miniID, &employeeID, &active, &subscribed, &enabled)
	if err != nil || miniID != f.sales.MiniUserID || employeeID != f.sales.EmployeeID || !active || !subscribed || !enabled {
		t.Fatalf("existing binding overwritten: mini=%d employee=%d active=%v followed=%v enabled=%v err=%v", miniID, employeeID, active, subscribed, enabled, err)
	}

	// A customer who selected an approved account before following also binds
	// through the authenticated customer session without a second scan.
	customer := f.id("INSERT INTO %s.customers(name) VALUES('通知测试客户') RETURNING id")
	f.exec("INSERT INTO %s.customer_portal_profiles(customer_id,enabled) VALUES($1,true)", customer)
	f.exec("INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,status,approved_by) VALUES($1,$2,'approved','test')", f.visitor.MiniUserID, customer)
	f.exec("INSERT INTO %s.customer_service_capabilities(customer_id,capability_code) VALUES($1,'product_order')", customer)
	f.exec("UPDATE %s.mini_users SET unionid='union-customer' WHERE id=$1", f.visitor.MiniUserID)
	f.call("POST", "/api/mini/current-customer", map[string]any{"customer_id": customer}, f.visitorToken, 200)
	if err = repo.SaveNotificationChoice(f.ctx, appID, f.visitor.MiniUserID, customer, 0, 1, true, []string{"shipment_created"}); err != nil {
		t.Fatal(err)
	}
	if err = repo.UpsertOfficialFollower(f.ctx, appID, "oa-customer", "union-customer", true, 300); err != nil {
		t.Fatal(err)
	}
	if err = messagepg.ReconcileWechatUnion(f.ctx, f.pool, f.schema, f.portal, appID, "union-customer"); err != nil {
		t.Fatal(err)
	}
	var customerID int64
	err = f.pool.QueryRow(f.ctx, "SELECT mini_user_id,customer_id,active,subscribed,notifications_enabled FROM "+f.schema+".wechat_bindings WHERE app_id=$1 AND openid='oa-customer'", appID).Scan(&miniID, &customerID, &active, &subscribed, &enabled)
	if err != nil || miniID != f.visitor.MiniUserID || customerID != customer || !active || !subscribed || !enabled {
		t.Fatalf("customer follow after selection: mini=%d customer=%d active=%v followed=%v enabled=%v err=%v", miniID, customerID, active, subscribed, enabled, err)
	}
}
