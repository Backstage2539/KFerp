package customerportal

import (
	"fmt"
	"testing"

	authzapp "orderapp/internal/application/authz"
	companyapp "orderapp/internal/application/company"
	customerapp "orderapp/internal/application/customer"
	customerportalapp "orderapp/internal/application/customerportal"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	postgrescustomer "orderapp/internal/infrastructure/postgres/customer"
)

func TestFormalSecurityWritePathsPermanentlyExpireMiniAndERPSessions(t *testing.T) {
	t.Run("company employee active off then on", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "employee-active-token")
		var departmentID int64
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT department_id FROM %s.company_employees WHERE id=$1`, f.schema), f.employeeID).Scan(&departmentID); err != nil {
			t.Fatalf("load employee department: %v", err)
		}
		repo := postgrescompany.NewRepository(f.pool, f.schema)
		for _, active := range []bool{false, true} {
			if err := repo.UpdateEmployee(f.ctx, f.employeeID, companyapp.EmployeeCommand{
				Name: "正式员工启停", Phone: f.login, DepartmentID: departmentID, Active: active,
			}); err != nil {
				t.Fatalf("UpdateEmployee(active=%t): %v", active, err)
			}
		}
		assertSecurityWritePathSessionsExpired(t, f, erpToken)
	})

	t.Run("customer active off then on", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "customer-active-token")
		responsibleEmployeeID := seedCustomerSecurityResponsibleEmployee(t, f)
		repo := postgrescustomer.NewRepository(f.pool, f.schema, t.TempDir())
		for _, active := range []string{"", "true"} {
			if err := repo.InlineUpdate(f.ctx, "security-write-test", f.customerID, customerapp.InlineUpdateCommand{
				Name: "投影会话客户", CustomerType: customerapp.CustomerTypeRetail,
				Contact: "联系人", Phone: "021-60001111", Address: "业务地址",
				ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID), Active: active,
			}); err != nil {
				t.Fatalf("InlineUpdate(customer active=%q): %v", active, err)
			}
		}
		assertSecurityWritePathSessionsExpired(t, f, erpToken)
	})

	t.Run("portal enabled off then on through customer maintenance", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "customer-portal-enabled-token")
		responsibleEmployeeID := seedCustomerSecurityResponsibleEmployee(t, f)
		repo := postgrescustomer.NewRepository(f.pool, f.schema, t.TempDir())
		customerID := f.customerID
		for _, enabled := range []bool{false, true} {
			if _, err := repo.Upsert(f.ctx, "security-write-test", &customerID, customerapp.UpsertCommand{
				Name: "投影会话客户", CustomerType: customerapp.CustomerTypeRetail,
				Contact: "联系人", Phone: "021-60002222", Address: "业务地址",
				ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID), Active: "true",
				PortalEnabled: &enabled,
			}); err != nil {
				t.Fatalf("Upsert(portal enabled=%t): %v", enabled, err)
			}
		}
		assertSecurityWritePathSessionsExpired(t, f, erpToken)
	})

	t.Run("portal enabled off then on through portal admin", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "portal-admin-enabled-token")
		for _, enabled := range []bool{false, true} {
			if _, err := f.repo.UpdatePortalVisibility(f.ctx, customerportalapp.UpdatePortalVisibilityCommand{
				CustomerID: f.customerID, DisplayName: "门户客户", Enabled: enabled,
				ThemeKey: customerportalapp.PortalThemeCleanOps, MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
				UpdatedBy: "security-write-test",
			}); err != nil {
				t.Fatalf("UpdatePortalVisibility(enabled=%t): %v", enabled, err)
			}
		}
		assertSecurityWritePathSessionsExpired(t, f, erpToken)
	})
}

func TestFormalBenignProfileWritesKeepMiniAndERPSessionsActive(t *testing.T) {
	t.Run("company employee profile", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "employee-benign-token")
		var departmentID int64
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
			INSERT INTO %s.company_departments(name,active)
			VALUES('正式资料编辑部',true)
			RETURNING id
		`, f.schema)).Scan(&departmentID); err != nil {
			t.Fatalf("insert employee department: %v", err)
		}
		repo := postgrescompany.NewRepository(f.pool, f.schema)
		if err := repo.UpdateEmployee(f.ctx, f.employeeID, companyapp.EmployeeCommand{
			Name: "正式普通资料新名称", Phone: "13964445555", DepartmentID: departmentID, Active: true,
		}); err != nil {
			t.Fatalf("UpdateEmployee benign profile: %v", err)
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})

	t.Run("historical inactive binding on another customer", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "historical-inactive-binding-token")
		responsibleEmployeeID := seedCustomerSecurityResponsibleEmployee(t, f)
		var historicalCustomerID int64
		if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
			INSERT INTO %s.customers(name,customer_type,active,responsible_employee_id)
			VALUES('历史解绑客户','retail',true,$1)
			RETURNING id
		`, f.schema), responsibleEmployeeID).Scan(&historicalCustomerID); err != nil {
			t.Fatalf("insert historical customer: %v", err)
		}
		if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_erp_user_bindings(customer_id,employee_id,role,status,updated_by)
			VALUES($1,$2,'customer','inactive','historical-test')
		`, f.schema), historicalCustomerID, f.employeeID); err != nil {
			t.Fatalf("insert historical inactive binding: %v", err)
		}
		if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status,approved_by)
			VALUES($1,$2,'member','revoked','historical-manual-binding')
		`, f.schema), f.miniUserID, historicalCustomerID); err != nil {
			t.Fatalf("insert historical revoked mini binding: %v", err)
		}
		repo := postgrescustomer.NewRepository(f.pool, f.schema, t.TempDir())
		for _, active := range []string{"", "true"} {
			if err := repo.InlineUpdate(f.ctx, "historical-test", historicalCustomerID, customerapp.InlineUpdateCommand{
				Name: "历史解绑客户", CustomerType: customerapp.CustomerTypeRetail,
				ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID), Active: active,
			}); err != nil {
				t.Fatalf("toggle historical customer active=%q: %v", active, err)
			}
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})

	t.Run("customer profile", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "customer-benign-token")
		responsibleEmployeeID := seedCustomerSecurityResponsibleEmployee(t, f)
		repo := postgrescustomer.NewRepository(f.pool, f.schema, t.TempDir())
		if err := repo.InlineUpdate(f.ctx, "benign-write-test", f.customerID, customerapp.InlineUpdateCommand{
			Name: "投影会话客户新名称", CustomerType: customerapp.CustomerTypeRetail,
			Contact: "新联系人", Phone: "021-60003333", Address: "新业务地址",
			ResponsibleEmployeeID: fmt.Sprintf("%d", responsibleEmployeeID), Active: "true",
		}); err != nil {
			t.Fatalf("InlineUpdate benign customer profile: %v", err)
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})

	t.Run("portal appearance and capabilities", func(t *testing.T) {
		f := newProjectedPasswordSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "portal-benign-token")
		if _, err := f.repo.UpdatePortalVisibility(f.ctx, customerportalapp.UpdatePortalVisibilityCommand{
			CustomerID: f.customerID, DisplayName: "普通门户新名称", Enabled: true,
			ThemeKey: customerportalapp.PortalThemeCleanOps, MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
			Capabilities: []customerportalapp.CapabilityOption{{Code: "orders", Enabled: true}},
			UpdatedBy:    "benign-write-test",
		}); err != nil {
			t.Fatalf("UpdatePortalVisibility benign appearance: %v", err)
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})
}

func TestCapabilityTemplateSecurityWritesExpireOnlyAffectedERPSessions(t *testing.T) {
	t.Run("active off then on", func(t *testing.T) {
		f, template := newCapabilityTemplateSecurityFixture(t, "active")
		erpToken := seedSecurityWritePathERPSession(t, f, "template-active-erp-token")
		template.Active = false
		if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "security-write-test", ActiveSet: true}); err != nil {
			t.Fatalf("disable capability template: %v", err)
		}
		template.Active = true
		if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "security-write-test", ActiveSet: true}); err != nil {
			t.Fatalf("restore capability template: %v", err)
		}
		assertERPSecuritySessionExpired(t, f, erpToken)
		assertMiniSecuritySessionActive(t, f)
	})

	t.Run("ERP workbench exposure removed then restored", func(t *testing.T) {
		f, template := newCapabilityTemplateSecurityFixture(t, "exposure")
		erpToken := seedSecurityWritePathERPSession(t, f, "template-exposure-erp-token")
		template.ERPPermissions = []string{}
		template.ERPViewKeys = []string{}
		if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "security-write-test", ActiveSet: true}); err != nil {
			t.Fatalf("remove ERP workbench exposure: %v", err)
		}
		template.ERPPermissions = []string{"customer_processing.read"}
		template.ERPViewKeys = []string{"customerProcessingPortal"}
		if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "security-write-test", ActiveSet: true}); err != nil {
			t.Fatalf("restore ERP workbench exposure: %v", err)
		}
		assertERPSecuritySessionExpired(t, f, erpToken)
		assertMiniSecuritySessionActive(t, f)
	})

	t.Run("label description and theme remain benign", func(t *testing.T) {
		f, template := newCapabilityTemplateSecurityFixture(t, "benign")
		erpToken := seedSecurityWritePathERPSession(t, f, "template-benign-erp-token")
		template.Label = "普通新名称"
		template.Description = "普通新说明"
		template.ThemeKey = customerportalapp.PortalThemeCleanOps
		if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "benign-write-test", ActiveSet: true}); err != nil {
			t.Fatalf("save benign capability template fields: %v", err)
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})
}

func TestFormalRoleAssignmentPermanentlyExpiresInternalEmployeeSessionsOnlyWhenChanged(t *testing.T) {
	t.Run("remove then restore before read", func(t *testing.T) {
		f := newInternalMiniSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "internal-role-cycle-token")
		repo := postgresauthz.NewRepository(f.pool, f.schema)
		if err := repo.AssignEmployeeRoles(f.ctx, authzapp.AssignmentCommand{EmployeeID: f.employeeID, RoleCodes: []string{}}); err != nil {
			t.Fatalf("remove internal employee roles: %v", err)
		}
		if err := repo.AssignEmployeeRoles(f.ctx, authzapp.AssignmentCommand{EmployeeID: f.employeeID, RoleCodes: []string{"sales"}}); err != nil {
			t.Fatalf("restore internal employee roles: %v", err)
		}
		assertSecurityWritePathSessionsExpired(t, f, erpToken)
	})

	t.Run("same role set remains benign", func(t *testing.T) {
		f := newInternalMiniSessionFixture(t)
		erpToken := seedSecurityWritePathERPSession(t, f, "internal-role-same-token")
		repo := postgresauthz.NewRepository(f.pool, f.schema)
		if err := repo.AssignEmployeeRoles(f.ctx, authzapp.AssignmentCommand{EmployeeID: f.employeeID, RoleCodes: []string{"sales"}}); err != nil {
			t.Fatalf("save unchanged internal employee roles: %v", err)
		}
		assertSecurityWritePathSessionsActive(t, f, erpToken)
	})
}

func seedSecurityWritePathERPSession(t *testing.T, f *projectedMiniSessionFixture, token string) string {
	t.Helper()
	if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			ts TIMESTAMPTZ NOT NULL DEFAULT now(),
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL
		);
		CREATE TABLE IF NOT EXISTS %s.login_sessions (
			token TEXT PRIMARY KEY,
			employee_id BIGINT NOT NULL REFERENCES %s.company_employees(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			expire_at TIMESTAMPTZ NOT NULL
		)
	`, f.schema, f.schema, f.schema)); err != nil {
		t.Fatalf("create ERP login sessions table: %v", err)
	}
	execProjectedSessionSQL(t, f, `INSERT INTO %s.login_sessions(token,employee_id,expire_at) VALUES($1,$2,'infinity')`, token, f.employeeID)
	return token
}

func seedCustomerSecurityResponsibleEmployee(t *testing.T, f *projectedMiniSessionFixture) int64 {
	t.Helper()
	var employeeID int64
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`
		INSERT INTO %s.company_employees(name,phone,account_type,department_id,active)
		VALUES('客户负责人','13965556666','internal_employee',(SELECT id FROM %s.company_departments WHERE active=true ORDER BY id LIMIT 1),true)
		RETURNING id
	`, f.schema, f.schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert customer responsible employee: %v", err)
	}
	if _, err := f.pool.Exec(f.ctx, fmt.Sprintf(`UPDATE %s.customers SET responsible_employee_id=$2 WHERE id=$1`, f.schema), f.customerID, employeeID); err != nil {
		t.Fatalf("set customer responsible employee: %v", err)
	}
	return employeeID
}

func newCapabilityTemplateSecurityFixture(t *testing.T, miniTokenLabel string) (*projectedMiniSessionFixture, customerportalapp.CapabilityTemplate) {
	t.Helper()
	f := newProjectedPasswordSessionFixture(t)
	template := customerportalapp.CapabilityTemplate{
		Key:   "security_template_" + miniTokenLabel,
		Label: "安全模板", ThemeKey: customerportalapp.PortalThemeCoffeeFactory,
		MiniappEntryMode: customerportalapp.MiniappEntryModeServices,
		ERPPermissions:   []string{"customer_processing.read"}, ERPViewKeys: []string{"customerProcessingPortal"},
		ERPRoleCodes: []string{"customer_processing"}, Capabilities: []customerportalapp.CapabilityOption{}, Active: true,
	}
	if _, err := f.repo.SaveCapabilityTemplate(f.ctx, customerportalapp.SaveCapabilityTemplateCommand{Template: template, UpdatedBy: "fixture", ActiveSet: true}); err != nil {
		t.Fatalf("save initial capability template: %v", err)
	}
	execProjectedSessionSQL(t, f, `UPDATE %s.customer_portal_profiles SET capability_template_key=$2 WHERE customer_id=$1`, f.customerID, template.Key)
	return f, template
}

func assertSecurityWritePathSessionsExpired(t *testing.T, f *projectedMiniSessionFixture, erpToken string) {
	t.Helper()
	assertMiniSecuritySessionExpired(t, f)
	assertERPSecuritySessionExpired(t, f, erpToken)
}

func assertSecurityWritePathSessionsActive(t *testing.T, f *projectedMiniSessionFixture, erpToken string) {
	t.Helper()
	assertMiniSecuritySessionActive(t, f)
	var active bool
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT expire_at>now() FROM %s.login_sessions WHERE token=$1`, f.schema), erpToken).Scan(&active); err != nil {
		t.Fatalf("load ERP session activity: %v", err)
	}
	if !active {
		t.Fatalf("ERP session %q expired after benign write", erpToken)
	}
}

func assertMiniSecuritySessionExpired(t *testing.T, f *projectedMiniSessionFixture) {
	t.Helper()
	var expired bool
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT expire_at<=now() FROM %s.mini_sessions WHERE token=$1`, f.schema), f.token).Scan(&expired); err != nil {
		t.Fatalf("load mini session expiry: %v", err)
	}
	if !expired {
		t.Fatalf("mini session %q remained active after security write", f.token)
	}
}

func assertMiniSecuritySessionActive(t *testing.T, f *projectedMiniSessionFixture) {
	t.Helper()
	var active bool
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT expire_at>now() FROM %s.mini_sessions WHERE token=$1`, f.schema), f.token).Scan(&active); err != nil {
		t.Fatalf("load mini session activity: %v", err)
	}
	if !active {
		t.Fatalf("mini session %q unexpectedly expired", f.token)
	}
}

func assertERPSecuritySessionExpired(t *testing.T, f *projectedMiniSessionFixture, token string) {
	t.Helper()
	var expired bool
	if err := f.pool.QueryRow(f.ctx, fmt.Sprintf(`SELECT expire_at<=now() FROM %s.login_sessions WHERE token=$1`, f.schema), token).Scan(&expired); err != nil {
		t.Fatalf("load ERP session expiry: %v", err)
	}
	if !expired {
		t.Fatalf("ERP session %q remained active after security write", token)
	}
}
