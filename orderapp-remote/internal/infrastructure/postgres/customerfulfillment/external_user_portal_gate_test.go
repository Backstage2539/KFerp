package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

const customerPortalNotEnabledError = "customer portal not enabled"

func TestExternalUserWritesRequireEnabledActiveCustomerPortal(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	insertCustomer := func(name string, active bool, portalEnabled *bool) int64 {
		t.Helper()
		var customerID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.customers(name,customer_type,active)
			VALUES($1,'wholesale',$2)
			RETURNING id
		`, schema), name, active).Scan(&customerID); err != nil {
			t.Fatalf("insert customer %s: %v", name, err)
		}
		if portalEnabled != nil {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.customer_portal_profiles(customer_id,enabled,capability_template_key)
				VALUES($1,$2,'retail_mall')
			`, schema), customerID, *portalEnabled); err != nil {
				t.Fatalf("insert portal profile %s: %v", name, err)
			}
		}
		return customerID
	}

	assertCreateRejected := func(customerID int64, phone string) {
		t.Helper()
		_, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
			CustomerID: customerID,
			Name:       "不应创建的门户账号",
			Phone:      phone,
			Password:   "secret123",
			Actor:      "认证管理员",
		})
		if err == nil || err.Error() != customerPortalNotEnabledError {
			t.Fatalf("CreateExternalUser customer=%d err=%v, want %q", customerID, err, customerPortalNotEnabledError)
		}
		var count int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.company_employees WHERE phone=$1`, schema), phone).Scan(&count); err != nil {
			t.Fatalf("count rejected employee: %v", err)
		}
		if count != 0 {
			t.Fatalf("rejected create left %d employee rows for %s", count, phone)
		}
	}

	disabled := false
	enabled := true
	assertCreateRejected(insertCustomer("无门户配置客户", true, nil), "13950000001")
	assertCreateRejected(insertCustomer("门户关闭客户", true, &disabled), "13950000002")
	assertCreateRejected(insertCustomer("已停用客户", false, &enabled), "13950000003")

	customerID := insertCustomer("门户启用客户", true, &enabled)
	created, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "可用门户账号",
		Phone:      "13950000004",
		Password:   "secret123",
		Actor:      "认证管理员",
	})
	if err != nil {
		t.Fatalf("CreateExternalUser enabled portal: %v", err)
	}
	if _, err := repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID: customerID, EmployeeID: created.EmployeeID, LoginEnabled: false, Actor: "认证管理员",
	}); err != nil {
		t.Fatalf("disable login before portal close: %v", err)
	}
	var hashBefore string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT password_hash FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), created.EmployeeID).Scan(&hashBefore); err != nil {
		t.Fatalf("read password before portal close: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET enabled=false WHERE customer_id=$1`, schema), customerID); err != nil {
		t.Fatalf("disable portal: %v", err)
	}

	if _, err := repo.ResetExternalUserPassword(ctx, app.ResetExternalUserPasswordCommand{
		CustomerID: customerID, EmployeeID: created.EmployeeID, Password: "secret456", Actor: "认证管理员",
	}); err == nil || err.Error() != customerPortalNotEnabledError {
		t.Fatalf("ResetExternalUserPassword closed portal err=%v, want %q", err, customerPortalNotEnabledError)
	}
	if _, err := repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID: customerID, EmployeeID: created.EmployeeID, LoginEnabled: true, Actor: "认证管理员",
	}); err == nil || err.Error() != customerPortalNotEnabledError {
		t.Fatalf("enable login closed portal err=%v, want %q", err, customerPortalNotEnabledError)
	}
	if _, err := repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID: customerID, EmployeeID: created.EmployeeID, LoginEnabled: false, Actor: "认证管理员",
	}); err != nil {
		t.Fatalf("disable login must remain available after portal close: %v", err)
	}

	var hashAfter string
	var loginDisabled bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT password_hash,login_disabled FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), created.EmployeeID).Scan(&hashAfter, &loginDisabled); err != nil {
		t.Fatalf("read account after rejected writes: %v", err)
	}
	if hashAfter != hashBefore || !loginDisabled {
		t.Fatalf("closed portal account changed: hash_before=%q hash_after=%q login_disabled=%v", hashBefore, hashAfter, loginDisabled)
	}
}

func TestInactiveExternalUserBindingCannotMutateAnotherCustomersActiveAccount(t *testing.T) {
	for _, operation := range []string{"reset_password", "disable_login"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			pool, schema := newCustomerFulfillmentTestDB(t)
			repo := NewRepository(pool, schema)

			insertCustomer := func(name string) int64 {
				t.Helper()
				var customerID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`
					INSERT INTO %s.customers(name,customer_type,active)
					VALUES($1,'wholesale',true)
					RETURNING id
				`, schema), name).Scan(&customerID); err != nil {
					t.Fatalf("insert customer %s: %v", name, err)
				}
				if _, err := pool.Exec(ctx, fmt.Sprintf(`
					INSERT INTO %s.customer_portal_profiles(customer_id,display_name,enabled)
					VALUES($1,$2,true)
				`, schema), customerID, name); err != nil {
					t.Fatalf("insert portal profile %s: %v", name, err)
				}
				return customerID
			}

			oldCustomerID := insertCustomer("旧归属客户")
			currentCustomerID := insertCustomer("当前归属客户")
			oldAccount, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
				CustomerID: oldCustomerID, Name: "跨客户历史账号", Phone: "13950000010", Password: "secret123", Actor: "认证管理员",
			})
			if err != nil {
				t.Fatalf("create old customer account: %v", err)
			}
			if _, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
				CustomerID: oldCustomerID, Name: "旧客户替换账号", Phone: "13950000011", Password: "secret123", Actor: "认证管理员",
			}); err != nil {
				t.Fatalf("replace old customer account: %v", err)
			}
			currentAccount, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
				CustomerID: currentCustomerID, Name: "跨客户历史账号", Phone: "13950000010", Password: "current456", Actor: "认证管理员",
			})
			if err != nil {
				t.Fatalf("bind account to current customer: %v", err)
			}
			if currentAccount.EmployeeID != oldAccount.EmployeeID {
				t.Fatalf("reused employee=%d, want original employee %d", currentAccount.EmployeeID, oldAccount.EmployeeID)
			}

			rows, err := repo.ListExternalUsers(ctx, oldCustomerID)
			if err != nil {
				t.Fatalf("list old customer external users: %v", err)
			}
			foundInactive := false
			for _, row := range rows {
				if row.EmployeeID == oldAccount.EmployeeID && row.BindingStatus == "inactive" {
					foundInactive = true
				}
			}
			if !foundInactive {
				t.Fatalf("old customer list=%+v, want inactive history for employee %d", rows, oldAccount.EmployeeID)
			}

			var hashBefore string
			var loginDisabledBefore bool
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				SELECT password_hash,login_disabled
				FROM %s.employee_login_passwords
				WHERE employee_id=$1
			`, schema), oldAccount.EmployeeID).Scan(&hashBefore, &loginDisabledBefore); err != nil {
				t.Fatalf("load current account credentials: %v", err)
			}

			switch operation {
			case "reset_password":
				_, err = repo.ResetExternalUserPassword(ctx, app.ResetExternalUserPasswordCommand{
					CustomerID: oldCustomerID, EmployeeID: oldAccount.EmployeeID, Password: "attacker789", Actor: "认证管理员",
				})
			case "disable_login":
				_, err = repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
					CustomerID: oldCustomerID, EmployeeID: oldAccount.EmployeeID, LoginEnabled: false, Actor: "认证管理员",
				})
			}
			if err == nil || err.Error() != "external user not found" {
				t.Fatalf("inactive %s err=%v, want external user not found", operation, err)
			}

			var hashAfter string
			var loginDisabledAfter bool
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				SELECT password_hash,login_disabled
				FROM %s.employee_login_passwords
				WHERE employee_id=$1
			`, schema), oldAccount.EmployeeID).Scan(&hashAfter, &loginDisabledAfter); err != nil {
				t.Fatalf("load account credentials after rejected mutation: %v", err)
			}
			if hashAfter != hashBefore || loginDisabledAfter != loginDisabledBefore {
				t.Fatalf("inactive %s changed current account: hash %q -> %q, login_disabled %v -> %v", operation, hashBefore, hashAfter, loginDisabledBefore, loginDisabledAfter)
			}

			var activeCustomerID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				SELECT customer_id
				FROM %s.customer_erp_user_bindings
				WHERE employee_id=$1 AND status='active'
			`, schema), oldAccount.EmployeeID).Scan(&activeCustomerID); err != nil {
				t.Fatalf("load active account customer: %v", err)
			}
			if activeCustomerID != currentCustomerID {
				t.Fatalf("active customer=%d, want current customer %d", activeCustomerID, currentCustomerID)
			}
		})
	}
}
