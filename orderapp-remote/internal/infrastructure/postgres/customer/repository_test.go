package customer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	customerapp "orderapp/internal/application/customer"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgrescustomerfulfillment "orderapp/internal/infrastructure/postgres/customerfulfillment"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFetchCustomerDashboardCoalescesEmptyAggregates(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0) <> 2 THEN 1 ELSE 0 END),0) AS unpaid",
		"COALESCE(SUM(CASE WHEN COALESCE(o.ship_status_id,0) IN (0,1,2) THEN 1 ELSE 0 END),0) AS unshipped",
		"COALESCE(SUM(CASE WHEN $2>0 AND COALESCE(o.process_status_id,0) = $2 THEN 1 ELSE 0 END),0) AS in_prod",
		"COALESCE(SUM(CASE WHEN $3>0 AND COALESCE(o.process_status_id,0) = $3 THEN 1 ELSE 0 END),0) AS in_ship",
		"COALESCE(SUM(CASE WHEN COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4) THEN 1 ELSE 0 END),0) AS completed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("fetchCustomerDashboard missing aggregate null guard %q", want)
		}
	}
}

func TestCustomerUpsertWritesCanonicalEmptyStringsForOptionalContactFields(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, forbidden := range []string{
		"nullText(contact)",
		"nullText(phone)",
		"nullText(address)",
		"nullText(next.contact)",
		"nullText(next.phone)",
		"nullText(next.address)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("customer optional text must use canonical empty strings, found %q", forbidden)
		}
	}
}

func TestSaveAssetCleansFileWhenMetadataInsertFails(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_asset_cleanup_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.customers (
			id BIGINT PRIMARY KEY
		);
		CREATE TABLE %s.customer_assets (
			id BIGSERIAL PRIMARY KEY,
			customer_id BIGINT NOT NULL REFERENCES %s.customers(id),
			kind TEXT NOT NULL,
			object_key TEXT NOT NULL,
			content_type TEXT NOT NULL,
			bytes BIGINT NOT NULL,
			sha256 TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			created_by TEXT NOT NULL
		);
	`, schema, schema, schema)); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, assetDir)
	_, err := repo.SaveAsset(ctx, customerapp.SaveAssetCommand{
		CustomerID:  404,
		Kind:        "logo",
		Reader:      bytes.NewReader([]byte("image bytes")),
		ContentType: "image/png",
		MaxBytes:    8 << 20,
		Filename:    "logo.png",
		Actor:       "tester",
	})
	if err == nil {
		t.Fatal("SaveAsset missing customer error = nil")
	}
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed customer asset metadata insert left orphan asset entries: %+v", entries)
	}
}

func TestCustomerUpsertRequiresActiveInternalResponsibleEmployeeAndAuditsChanges(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_responsible_employee_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	var departmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.company_departments WHERE name='销售' LIMIT 1`, schema)).Scan(&departmentID); err != nil {
		t.Fatalf("load department: %v", err)
	}
	var ownerA, ownerB, channelAccount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('销售甲', '13900001001', $1, 'internal_employee', true)
		RETURNING id
	`, schema), departmentID).Scan(&ownerA); err != nil {
		t.Fatalf("insert owner A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('销售乙', '13900001002', $1, 'internal_employee', true)
		RETURNING id
	`, schema), departmentID).Scan(&ownerB); err != nil {
		t.Fatalf("insert owner B: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('外部客户账号', '13900001003', $1, 'channel_customer', true)
		RETURNING id
	`, schema), departmentID).Scan(&channelAccount); err != nil {
		t.Fatalf("insert channel account: %v", err)
	}

	repo := NewRepository(pool, schema, t.TempDir())
	_, err := repo.Upsert(ctx, "tester", nil, customerapp.UpsertCommand{
		Name:         "无负责人客户",
		CustomerType: customerapp.CustomerTypeWholesale,
		Active:       "on",
	})
	if err == nil || !strings.Contains(err.Error(), "responsible_employee_id required") {
		t.Fatalf("upsert without responsible error=%v, want responsible_employee_id required", err)
	}

	_, err = repo.Upsert(ctx, "tester", nil, customerapp.UpsertCommand{
		Name:                  "外部账号客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", channelAccount),
		Active:                "on",
	})
	if err == nil || !strings.Contains(err.Error(), "responsible employee not found") {
		t.Fatalf("upsert with channel account error=%v, want responsible employee not found", err)
	}

	customerID, err := repo.Upsert(ctx, "tester", nil, customerapp.UpsertCommand{
		Name:                  "必填负责人客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerA),
		Active:                "on",
	})
	if err != nil {
		t.Fatalf("create customer with responsible employee: %v", err)
	}
	if _, err := repo.Upsert(ctx, "tester", &customerID, customerapp.UpsertCommand{
		Name:                  "必填负责人客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerB),
		Active:                "on",
	}); err != nil {
		t.Fatalf("update customer responsible employee: %v", err)
	}

	edit, err := repo.Editor(ctx, customerID)
	if err != nil {
		t.Fatalf("editor customer: %v", err)
	}
	if edit.Customer.ResponsibleEmployeeID != fmt.Sprintf("%d", ownerB) || edit.Customer.ResponsibleEmployeeName != "销售乙" {
		t.Fatalf("editor responsible=%q/%q, want %d/销售乙", edit.Customer.ResponsibleEmployeeID, edit.Customer.ResponsibleEmployeeName, ownerB)
	}
	list, err := repo.List(ctx, customerapp.ListQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if len(list.Rows) != 1 || list.Rows[0].ResponsibleEmployeeID == nil || *list.Rows[0].ResponsibleEmployeeID != int(ownerB) || list.Rows[0].ResponsibleEmployeeName != "销售乙" {
		t.Fatalf("list responsible row=%+v, want owner %d/销售乙", list.Rows, ownerB)
	}
	var oldValue, newValue string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(old_value,''), COALESCE(new_value,'')
		FROM %s.audit_logs
		WHERE entity_type='customer' AND entity_id=$1 AND field='responsible_employee_id'
		ORDER BY id DESC
		LIMIT 1
	`, schema), customerID).Scan(&oldValue, &newValue); err != nil {
		t.Fatalf("query responsible audit: %v", err)
	}
	if oldValue != fmt.Sprintf("%d", ownerA) || newValue != fmt.Sprintf("%d", ownerB) {
		t.Fatalf("responsible audit old/new=%q/%q, want %d/%d", oldValue, newValue, ownerA, ownerB)
	}
}

func TestManagedCustomerUpsertEnforcesOwnershipAndProtectedFieldsInTransaction(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_managed_permissions_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
	}
	if err := postgrescustomerportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	var ownerA, ownerB int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('销售甲', '13900002001', (SELECT id FROM %s.company_departments WHERE name='销售' LIMIT 1), 'internal_employee', true)
		RETURNING id
	`, schema, schema)).Scan(&ownerA); err != nil {
		t.Fatalf("insert owner A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('销售乙', '13900002002', (SELECT id FROM %s.company_departments WHERE name='销售' LIMIT 1), 'internal_employee', true)
		RETURNING id
	`, schema, schema)).Scan(&ownerB); err != nil {
		t.Fatalf("insert owner B: %v", err)
	}

	repo := NewRepository(pool, schema, t.TempDir())
	portalEnabled := true
	ownedID, err := repo.Upsert(ctx, "seed", nil, customerapp.UpsertCommand{
		Name:                  "甲负责客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerA),
		Active:                "on",
		PortalEnabled:         &portalEnabled,
	})
	if err != nil {
		t.Fatalf("seed owned customer: %v", err)
	}
	otherID, err := repo.Upsert(ctx, "seed", nil, customerapp.UpsertCommand{
		Name:                  "乙负责客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerB),
		Active:                "on",
	})
	if err != nil {
		t.Fatalf("seed other customer: %v", err)
	}

	salesA := customerapp.MaintenancePrincipal{EmployeeID: ownerA, EmployeeName: "销售甲"}
	var sourceID, orderTypeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.sources ORDER BY id LIMIT 1`, schema)).Scan(&sourceID); err != nil {
		t.Fatalf("read source option: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_types ORDER BY id LIMIT 1`, schema)).Scan(&orderTypeID); err != nil {
		t.Fatalf("read order type option: %v", err)
	}
	var invalidCustomerBefore, invalidAuditBefore int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.customers`, schema)).Scan(&invalidCustomerBefore); err != nil {
		t.Fatalf("count customers before invalid option: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs`, schema)).Scan(&invalidAuditBefore); err != nil {
		t.Fatalf("count audits before invalid option: %v", err)
	}
	for _, tc := range []struct {
		name        string
		sourceID    int64
		orderTypeID int64
		want        string
	}{
		{name: "missing source", sourceID: 999999, orderTypeID: orderTypeID, want: "来源不存在"},
		{name: "missing order type", sourceID: sourceID, orderTypeID: 999999, want: "订单类型不存在"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.UpsertManaged(ctx, salesA, nil, customerapp.UpsertCommand{
				Name: "无效选项客户", CustomerType: customerapp.CustomerTypeRetail,
				DefaultSourceID: strconv.FormatInt(tc.sourceID, 10), DefaultOrderTypeID: strconv.FormatInt(tc.orderTypeID, 10),
			})
			if message, ok := customerapp.MaintenanceValidationMessage(err); !ok || !strings.Contains(message, tc.want) {
				t.Fatalf("invalid option err=%v message=%q ok=%v", err, message, ok)
			}
		})
	}
	var invalidCustomerAfter, invalidAuditAfter int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.customers`, schema)).Scan(&invalidCustomerAfter); err != nil {
		t.Fatalf("count customers after invalid option: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs`, schema)).Scan(&invalidAuditAfter); err != nil {
		t.Fatalf("count audits after invalid option: %v", err)
	}
	if invalidCustomerAfter != invalidCustomerBefore || invalidAuditAfter != invalidAuditBefore {
		t.Fatalf("invalid options mutated customers/audits: customers %d->%d audits %d->%d", invalidCustomerBefore, invalidCustomerAfter, invalidAuditBefore, invalidAuditAfter)
	}

	portalDisabled := false
	if _, err := repo.UpsertManaged(ctx, salesA, &ownedID, customerapp.UpsertCommand{
		Name:                  "甲负责客户（已维护）",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerB),
		Active:                "",
		PortalEnabled:         &portalDisabled,
	}); err != nil {
		t.Fatalf("sales update own customer: %v", err)
	}
	var name string
	var active bool
	var responsibleID int64
	var savedPortalEnabled bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.name, c.active, c.responsible_employee_id, COALESCE(p.enabled,false)
		FROM %s.customers c
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
		WHERE c.id=$1
	`, schema, schema), ownedID).Scan(&name, &active, &responsibleID, &savedPortalEnabled); err != nil {
		t.Fatalf("read managed customer: %v", err)
	}
	if name != "甲负责客户（已维护）" || !active || responsibleID != ownerA || !savedPortalEnabled {
		t.Fatalf("sales protected fields changed: name=%q active=%v owner=%d portal=%v", name, active, responsibleID, savedPortalEnabled)
	}

	var auditCountBefore int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs`, schema)).Scan(&auditCountBefore); err != nil {
		t.Fatalf("count audits before forbidden update: %v", err)
	}
	if _, err := repo.UpsertManaged(ctx, salesA, &otherID, customerapp.UpsertCommand{
		Name:                  "越权修改",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerA),
		Active:                "on",
	}); !errors.Is(err, customerapp.ErrCustomerMaintenanceForbidden) {
		t.Fatalf("sales update other customer error=%v, want forbidden", err)
	}
	var otherName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, schema), otherID).Scan(&otherName); err != nil {
		t.Fatalf("read other customer: %v", err)
	}
	var auditCountAfter int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.audit_logs`, schema)).Scan(&auditCountAfter); err != nil {
		t.Fatalf("count audits after forbidden update: %v", err)
	}
	if otherName != "乙负责客户" || auditCountAfter != auditCountBefore {
		t.Fatalf("forbidden update mutated data/audit: name=%q audits=%d->%d", otherName, auditCountBefore, auditCountAfter)
	}

	createdID, err := repo.UpsertManaged(ctx, salesA, nil, customerapp.UpsertCommand{
		Name:                  "销售新客户",
		CustomerType:          customerapp.CustomerTypeRetail,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerB),
		Active:                "",
		PortalEnabled:         &portalEnabled,
	})
	if err != nil {
		t.Fatalf("sales create customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active, responsible_employee_id FROM %s.customers WHERE id=$1`, schema), createdID).Scan(&active, &responsibleID); err != nil {
		t.Fatalf("read sales-created customer: %v", err)
	}
	if !active || responsibleID != ownerA {
		t.Fatalf("sales-created protected fields active=%v owner=%d, want true/%d", active, responsibleID, ownerA)
	}

	admin := customerapp.MaintenancePrincipal{EmployeeID: ownerA, EmployeeName: "管理员", IsAdmin: true}
	if _, err := repo.UpsertManaged(ctx, admin, &otherID, customerapp.UpsertCommand{
		Name:                  "管理员已维护",
		CustomerType:          customerapp.CustomerTypeWholesale,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerA),
		Active:                "",
	}); err != nil {
		t.Fatalf("admin update customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT name, active, responsible_employee_id FROM %s.customers WHERE id=$1`, schema), otherID).Scan(&otherName, &active, &responsibleID); err != nil {
		t.Fatalf("read admin-updated customer: %v", err)
	}
	if otherName != "管理员已维护" || active || responsibleID != ownerA {
		t.Fatalf("admin fields not persisted: name=%q active=%v owner=%d", otherName, active, responsibleID)
	}
}

func TestCustomerUpsertPersistsPortalSwitchWithoutBindingTemplate(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_portal_switch_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
	}
	if err := postgrescustomerportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create audit logs: %v", err)
	}
	var ownerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('渠道负责人', '13900003001', (SELECT id FROM %s.company_departments WHERE name='销售' LIMIT 1), 'internal_employee', true)
		RETURNING id
	`, schema, schema)).Scan(&ownerID); err != nil {
		t.Fatalf("insert responsible employee: %v", err)
	}

	repo := NewRepository(pool, schema, t.TempDir())
	customerID, err := repo.Upsert(ctx, "tester", nil, customerapp.UpsertCommand{
		Name:                  "渠道客户",
		CustomerType:          customerapp.CustomerTypeChannel,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerID),
		Active:                "on",
	})
	if err != nil {
		t.Fatalf("create channel customer: %v", err)
	}
	enabled := true
	if _, err := repo.Upsert(ctx, "tester", &customerID, customerapp.UpsertCommand{
		Name:                  "渠道客户",
		CustomerType:          customerapp.CustomerTypeChannel,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerID),
		PortalEnabled:         &enabled,
		Active:                "on",
	}); err != nil {
		t.Fatalf("enable portal switch: %v", err)
	}

	edit, err := repo.Editor(ctx, customerID)
	if err != nil {
		t.Fatalf("editor customer: %v", err)
	}
	if !edit.Customer.PortalEnabled || edit.Customer.CapabilityTemplateKey != "" {
		t.Fatalf("editor portal fields enabled=%v template=%q", edit.Customer.PortalEnabled, edit.Customer.CapabilityTemplateKey)
	}
	list, err := repo.List(ctx, customerapp.ListQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if len(list.Rows) != 1 || !list.Rows[0].PortalEnabled || list.Rows[0].CapabilityTemplateKey != "" {
		t.Fatalf("list portal row=%+v", list.Rows)
	}

	enabled = false
	if _, err := repo.Upsert(ctx, "tester", &customerID, customerapp.UpsertCommand{
		Name:                  "渠道客户",
		CustomerType:          customerapp.CustomerTypeChannel,
		ResponsibleEmployeeID: fmt.Sprintf("%d", ownerID),
		PortalEnabled:         &enabled,
		Active:                "on",
	}); err != nil {
		t.Fatalf("disable portal switch: %v", err)
	}

	var portalEnabled bool
	var templateKey string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT enabled, capability_template_key
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, schema), customerID).Scan(&portalEnabled, &templateKey); err != nil {
		t.Fatalf("load portal profile: %v", err)
	}
	if portalEnabled || templateKey != "" {
		t.Fatalf("profile enabled=%v template=%q, want disabled without customer-profile template binding", portalEnabled, templateKey)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::int
		FROM %s.audit_logs
		WHERE entity_type='customer'
		  AND entity_id=$1
		  AND field='portal_enabled'
	`, schema), customerID).Scan(&auditCount); err != nil {
		t.Fatalf("count portal audit logs: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("portal audit count=%d, want enable/disable logs only", auditCount)
	}
}

func TestUpsertRetailCustomerDoesNotAutoRewritePortalTemplate(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_retail_binding_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
	}
	if err := postgrescustomerportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
	}
	if err := postgrescustomerfulfillment.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerfulfillment.EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	repo := NewRepository(pool, schema, t.TempDir())
	var responsibleEmployeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('销售负责人', '13900002001', (SELECT id FROM %s.company_departments WHERE name='销售' LIMIT 1), 'internal_employee', true)
		RETURNING id
	`, schema, schema)).Scan(&responsibleEmployeeID); err != nil {
		t.Fatalf("insert responsible employee: %v", err)
	}
	customerID, err := repo.Upsert(ctx, "tester", nil, customerapp.UpsertCommand{
		Name:                  "模板切换客户",
		CustomerType:          customerapp.CustomerTypeWholesale,
		Contact:               "测试联系人",
		Phone:                 "13800138000",
		Address:               "测试地址",
		ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID),
		Active:                "on",
	})
	if err != nil {
		t.Fatalf("create wholesale customer: %v", err)
	}
	var departmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.company_departments ORDER BY id LIMIT 1`, schema)).Scan(&departmentID); err != nil {
		t.Fatalf("load department: %v", err)
	}
	var employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name, phone, department_id, account_type, active)
		VALUES('渠道账号', '13800138223', $1, 'channel_customer', true)
		RETURNING id
	`, schema), departmentID).Scan(&employeeID); err != nil {
		t.Fatalf("insert channel customer account: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key, miniapp_entry_mode, theme_key)
		VALUES($1, 'processing_fulfillment', 'services', 'premium_partner')
		ON CONFLICT(customer_id) DO UPDATE SET capability_template_key='processing_fulfillment'
	`, schema), customerID); err != nil {
		t.Fatalf("insert wholesale portal profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, status, updated_by)
		VALUES($1, $2, 'active', 'setup')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert legacy ERP binding: %v", err)
	}

	if _, err := repo.Upsert(ctx, "tester", &customerID, customerapp.UpsertCommand{
		Name:                  "模板切换客户",
		CustomerType:          customerapp.CustomerTypeRetail,
		Contact:               "测试联系人",
		Phone:                 "13800138000",
		Address:               "测试地址",
		ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID),
		Active:                "on",
	}); err != nil {
		t.Fatalf("update customer to retail: %v", err)
	}

	var templateKey string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT capability_template_key
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, schema), customerID).Scan(&templateKey); err != nil {
		t.Fatalf("load portal profile: %v", err)
	}
	if templateKey != "processing_fulfillment" {
		t.Fatalf("capability_template_key=%q, want original processing_fulfillment", templateKey)
	}
	var activeBindings int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::int
		FROM %s.customer_erp_user_bindings
		WHERE customer_id=$1 AND status='active'
	`, schema), customerID).Scan(&activeBindings); err != nil {
		t.Fatalf("count active bindings: %v", err)
	}
	if activeBindings != 1 {
		t.Fatalf("active ERP workbench bindings=%d, want unchanged by customer type alone", activeBindings)
	}
}

func newCustomerRepositoryTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for customer repository tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
