package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

type externalUserAuditRecord struct {
	EmployeeID int64
	Action     string
	Field      string
	OldValue   string
	NewValue   string
	Meta       map[string]any
	Raw        string
}

func TestExternalUserWritesAreAuditedWithoutPasswordsAndTrackSideEffects(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name,customer_type,active)
		VALUES('审计客户','wholesale',true)
		RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id,enabled,capability_template_key)
		VALUES($1,true,'')
	`, schema), customerID); err != nil {
		t.Fatalf("insert customer portal profile: %v", err)
	}

	firstPassword := "first-secret-123"
	first, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "外部用户甲",
		Phone:      "13940000001",
		Password:   firstPassword,
		Actor:      "审计管理员",
	})
	if err != nil {
		t.Fatalf("CreateExternalUser first: %v", err)
	}
	createAudit := externalUserAuditByAction(t, ctx, pool, schema, first.EmployeeID, "create")
	assertExternalUserAuditMeta(t, createAudit, customerID, first.EmployeeID, "外部用户甲")
	for key, want := range map[string]bool{
		"reused_existing":      false,
		"name_changed":         false,
		"password_initialized": true,
		"password_reset":       false,
		"login_reenabled":      false,
	} {
		if got, ok := createAudit.Meta[key].(bool); !ok || got != want {
			t.Fatalf("create meta[%s] = %#v, want %v", key, createAudit.Meta[key], want)
		}
	}

	if _, err := repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   first.EmployeeID,
		LoginEnabled: false,
		Actor:        "审计管理员",
	}); err != nil {
		t.Fatalf("SetExternalUserLoginEnabled false: %v", err)
	}
	disableAudit := externalUserAuditByAction(t, ctx, pool, schema, first.EmployeeID, "set_login_enabled")
	if disableAudit.Field != "login_enabled" || disableAudit.OldValue != "true" || disableAudit.NewValue != "false" {
		t.Fatalf("disable audit = %+v", disableAudit)
	}

	resetPassword := "reset-secret-456"
	if _, err := repo.ResetExternalUserPassword(ctx, app.ResetExternalUserPasswordCommand{
		CustomerID: customerID,
		EmployeeID: first.EmployeeID,
		Password:   resetPassword,
		Actor:      "审计管理员",
	}); err != nil {
		t.Fatalf("ResetExternalUserPassword: %v", err)
	}
	resetAudit := externalUserAuditByAction(t, ctx, pool, schema, first.EmployeeID, "reset_password")
	if resetAudit.Field != "login_enabled" || resetAudit.OldValue != "false" || resetAudit.NewValue != "true" {
		t.Fatalf("reset re-enable audit = %+v", resetAudit)
	}
	if resetAudit.Meta["login_reenabled"] != true {
		t.Fatalf("reset meta = %#v, want login_reenabled", resetAudit.Meta)
	}
	if _, err := repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   first.EmployeeID,
		LoginEnabled: false,
		Actor:        "审计管理员",
	}); err != nil {
		t.Fatalf("SetExternalUserLoginEnabled before reuse: %v", err)
	}

	reusePassword := "reuse-secret-567"
	reused, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "外部用户甲新名",
		Phone:      first.Phone,
		Password:   reusePassword,
		Actor:      "审计管理员",
	})
	if err != nil {
		t.Fatalf("CreateExternalUser reuse: %v", err)
	}
	if reused.EmployeeID != first.EmployeeID || reused.Name != "外部用户甲新名" || !reused.LoginEnabled || !reused.HasPassword {
		t.Fatalf("reused external user = %+v, want same employee with new name and enabled login", reused)
	}
	reuseAudit := externalUserAuditByAction(t, ctx, pool, schema, first.EmployeeID, "create")
	if reuseAudit.Field != "name" || reuseAudit.OldValue != "外部用户甲" || reuseAudit.NewValue != "外部用户甲新名" {
		t.Fatalf("reuse name audit = %+v", reuseAudit)
	}
	for key, want := range map[string]bool{
		"reused_existing":      true,
		"name_changed":         true,
		"password_initialized": false,
		"password_reset":       true,
		"login_reenabled":      true,
	} {
		if got, ok := reuseAudit.Meta[key].(bool); !ok || got != want {
			t.Fatalf("reuse meta[%s] = %#v, want %v; all meta=%#v", key, reuseAudit.Meta[key], want, reuseAudit.Meta)
		}
	}
	if reuseAudit.Meta["previous_external_user_name"] != "外部用户甲" || reuseAudit.Meta["external_user_name"] != "外部用户甲新名" {
		t.Fatalf("reuse name meta = %#v", reuseAudit.Meta)
	}
	var reusedHash string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT password_hash FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), first.EmployeeID).Scan(&reusedHash); err != nil {
		t.Fatalf("read reused password hash: %v", err)
	}
	if reusedHash != hashCustomerFulfillmentPassword(reusePassword) {
		t.Fatalf("reused password was not reset")
	}

	secondPassword := "second-secret-789"
	second, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "外部用户乙",
		Phone:      "13940000002",
		Password:   secondPassword,
		Actor:      "审计管理员",
	})
	if err != nil {
		t.Fatalf("CreateExternalUser second: %v", err)
	}
	secondAudit := externalUserAuditByAction(t, ctx, pool, schema, second.EmployeeID, "create")
	replaced := numericAuditMetaList(secondAudit.Meta["replaced_employee_ids"])
	if len(replaced) != 1 || replaced[0] != first.EmployeeID {
		t.Fatalf("second create replaced ids = %v, want [%d]", replaced, first.EmployeeID)
	}
	replacedUsers := externalUserReplacementAuditMeta(secondAudit.Meta["replaced_external_users"])
	if len(replacedUsers) != 1 || replacedUsers[0].EmployeeID != first.EmployeeID || replacedUsers[0].Name != "外部用户甲新名" {
		t.Fatalf("second create replaced users = %+v, want %d/外部用户甲新名", replacedUsers, first.EmployeeID)
	}
	var firstBindingStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status FROM %s.customer_erp_user_bindings
		WHERE customer_id=$1 AND employee_id=$2
	`, schema), customerID, first.EmployeeID).Scan(&firstBindingStatus); err != nil {
		t.Fatalf("read replaced binding: %v", err)
	}
	if firstBindingStatus != "inactive" {
		t.Fatalf("replaced binding status = %q", firstBindingStatus)
	}

	var auditDump string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(string_agg(concat_ws('|',actor,entity_type,entity_id::text,action,field,old_value,new_value,meta::text), E'\n'),'')
		FROM %s.audit_logs
		WHERE entity_type='customer_external_user'
	`, schema)).Scan(&auditDump); err != nil {
		t.Fatalf("read audit dump: %v", err)
	}
	for _, secret := range []string{
		firstPassword,
		resetPassword,
		reusePassword,
		secondPassword,
		hashCustomerFulfillmentPassword(firstPassword),
		hashCustomerFulfillmentPassword(resetPassword),
		hashCustomerFulfillmentPassword(reusePassword),
		hashCustomerFulfillmentPassword(secondPassword),
	} {
		if strings.Contains(auditDump, secret) {
			t.Fatalf("audit leaked password material %q: %s", secret, auditDump)
		}
	}
}

func TestExternalUserAuditFailureRollsBackBusinessChanges(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name,customer_type,active)
		VALUES('审计回滚客户','wholesale',true)
		RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id,enabled,capability_template_key)
		VALUES($1,true,'')
	`, schema), customerID); err != nil {
		t.Fatalf("insert customer portal profile: %v", err)
	}

	existing, err := repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "已有外部用户",
		Phone:      "13940000003",
		Password:   "existing-secret",
		Actor:      "审计管理员",
	})
	if err != nil {
		t.Fatalf("seed external user: %v", err)
	}
	var originalHash string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT password_hash FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), existing.EmployeeID).Scan(&originalHash); err != nil {
		t.Fatalf("read original hash: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.audit_logs RENAME TO audit_logs_unavailable`, schema)); err != nil {
		t.Fatalf("rename audit table: %v", err)
	}

	_, err = repo.CreateExternalUser(ctx, app.CreateExternalUserCommand{
		CustomerID: customerID,
		Name:       "应回滚外部用户",
		Phone:      "13940000004",
		Password:   "rollback-create-secret",
		Actor:      "审计管理员",
	})
	if err == nil {
		t.Fatal("CreateExternalUser should fail when audit insert fails")
	}
	var createdCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.company_employees WHERE phone='13940000004'`, schema)).Scan(&createdCount); err != nil {
		t.Fatalf("count rolled back employee: %v", err)
	}
	if createdCount != 0 {
		t.Fatalf("rolled back create left %d employee rows", createdCount)
	}

	_, err = repo.SetExternalUserLoginEnabled(ctx, app.SetExternalUserLoginEnabledCommand{
		CustomerID:   customerID,
		EmployeeID:   existing.EmployeeID,
		LoginEnabled: false,
		Actor:        "审计管理员",
	})
	if err == nil {
		t.Fatal("SetExternalUserLoginEnabled should fail when audit insert fails")
	}
	var loginDisabled bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT login_disabled FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), existing.EmployeeID).Scan(&loginDisabled); err != nil {
		t.Fatalf("read login_disabled after rollback: %v", err)
	}
	if loginDisabled {
		t.Fatal("failed login state audit did not roll back login_disabled")
	}

	_, err = repo.ResetExternalUserPassword(ctx, app.ResetExternalUserPasswordCommand{
		CustomerID: customerID,
		EmployeeID: existing.EmployeeID,
		Password:   "rollback-reset-secret",
		Actor:      "审计管理员",
	})
	if err == nil {
		t.Fatal("ResetExternalUserPassword should fail when audit insert fails")
	}
	var hashAfter string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT password_hash FROM %s.employee_login_passwords WHERE employee_id=$1`, schema), existing.EmployeeID).Scan(&hashAfter); err != nil {
		t.Fatalf("read hash after rollback: %v", err)
	}
	if hashAfter != originalHash {
		t.Fatalf("failed reset audit changed password hash: before=%s after=%s", originalHash, hashAfter)
	}
}

func externalUserAuditByAction(t *testing.T, ctx context.Context, pool queryRower, schema string, employeeID int64, action string) externalUserAuditRecord {
	t.Helper()
	var record externalUserAuditRecord
	var metaJSON []byte
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(entity_id,0), action, COALESCE(field,''), COALESCE(old_value,''), COALESCE(new_value,''), COALESCE(meta,'{}'::jsonb)::text,
		       concat_ws('|',actor,entity_type,entity_id::text,action,field,old_value,new_value,meta::text)
		FROM %s.audit_logs
		WHERE entity_type='customer_external_user' AND entity_id=$1 AND action=$2
		ORDER BY id DESC LIMIT 1
	`, schema), employeeID, action).Scan(&record.EmployeeID, &record.Action, &record.Field, &record.OldValue, &record.NewValue, &metaJSON, &record.Raw); err != nil {
		t.Fatalf("read %s audit for employee %d: %v", action, employeeID, err)
	}
	if err := json.Unmarshal(metaJSON, &record.Meta); err != nil {
		t.Fatalf("decode audit meta: %v", err)
	}
	return record
}

func assertExternalUserAuditMeta(t *testing.T, record externalUserAuditRecord, customerID, employeeID int64, name string) {
	t.Helper()
	if record.EmployeeID != employeeID || record.Meta["customer_name"] != "审计客户" || record.Meta["external_user_name"] != name || record.Meta["binding_status"] != "active" {
		t.Fatalf("audit record = %+v", record)
	}
	if got := int64(record.Meta["customer_id"].(float64)); got != customerID {
		t.Fatalf("customer_id meta = %d, want %d", got, customerID)
	}
	if got := int64(record.Meta["employee_id"].(float64)); got != employeeID {
		t.Fatalf("employee_id meta = %d, want %d", got, employeeID)
	}
}

func numericAuditMetaList(value any) []int64 {
	items, _ := value.([]any)
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if number, ok := item.(float64); ok {
			out = append(out, int64(number))
		}
	}
	return out
}

type externalUserReplacementAudit struct {
	EmployeeID int64
	Name       string
}

func externalUserReplacementAuditMeta(value any) []externalUserReplacementAudit {
	items, _ := value.([]any)
	out := make([]externalUserReplacementAudit, 0, len(items))
	for _, item := range items {
		row, _ := item.(map[string]any)
		employeeID, _ := row["employee_id"].(float64)
		name, _ := row["external_user_name"].(string)
		out = append(out, externalUserReplacementAudit{EmployeeID: int64(employeeID), Name: name})
	}
	return out
}
