# Customer Dossier User Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move customer dossier editing into a drawer, keep employee and permission screens internal-user only, and make the fulfillment workbench the external-user management surface.

**Architecture:** Keep the current compatibility storage (`company_employees.account_type='channel_customer'`) for external users, but expose it only through customer fulfillment APIs and UI. Customer dossier remains a customer-data page, employee/user-permission screens become internal-only, and customer portal settings shows an ERP account summary with a jump to fulfillment management.

**Tech Stack:** Go, Echo, pgx/Postgres, Vue 3/Vite, Node test runner, Go `testing`.

**Execution:** User confirmed direct development after planning, so execute inline with `superpowers:executing-plans` after this plan is saved.

---

### Task 1: Regression Guards And Requirement Seeds

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go`

- [ ] **Step 1: Write source guards for the new product boundary**

Create `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go` with these tests:

```go
package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDossierUserBoundarySourceGuards(t *testing.T) {
	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	userPermissionsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UserPermissionsView.vue")))
	companyRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "company", "repository.go")))
	mobileAuth := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "mobile_auth.go")))
	fulfillmentAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go")))
	fulfillmentView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	portalSettings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))

	for _, want := range []string{"customerDrawerOpen", "openCustomerDrawer", "closeCustomerDrawer", "drawer-mask", "@click=\"openCustomerDrawer(row.id)\""} {
		if !strings.Contains(customersView, want) {
			t.Fatalf("CustomersView.vue missing drawer marker %q", want)
		}
	}
	if strings.Contains(customersView, "<th>操作</th>") || strings.Contains(customersView, "editCustomer(row.id)\">编辑") {
		t.Fatal("customer list must not render the old edit operation column/button")
	}
	if strings.Contains(userPermissionsView, "setAccountType") || strings.Contains(userPermissionsView, "渠道客户") || strings.Contains(userPermissionsView, "account_type") {
		t.Fatal("user permissions view must not expose external account type controls")
	}
	for _, want := range []string{"/api/auth/internal-accounts", "account_type='internal_employee'"} {
		if !strings.Contains(mobileAuth, want) {
			t.Fatalf("auth API missing internal account marker %q", want)
		}
	}
	if !strings.Contains(companyRepo, "account_type='internal_employee'") {
		t.Fatal("company employee repository must filter employee maintenance to internal users")
	}
	for _, want := range []string{"external-users", "CreateExternalUser", "ResetExternalUserPassword", "SetExternalUserLoginEnabled"} {
		if !strings.Contains(fulfillmentAPI, want) {
			t.Fatalf("customer fulfillment API missing external user marker %q", want)
		}
	}
	for _, want := range []string{"外部用户", "createExternalUser", "resetExternalUserPassword", "toggleExternalUserLogin"} {
		if !strings.Contains(fulfillmentView, want) {
			t.Fatalf("CustomerFulfillmentView.vue missing external user UI marker %q", want)
		}
	}
	for _, want := range []string{"goToFulfillmentAccount", "去履约运营台管理"} {
		if !strings.Contains(portalSettings, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing account handoff marker %q", want)
		}
	}
}

func TestCustomerDossierUserBoundaryRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-283-CUSTOMER-DOSSIER-USER-BOUNDARY",
		"DEV-283-CUSTOMER-DRAWER",
		"DEV-283-INTERNAL-USER-PERMISSIONS",
		"DEV-283-FULFILLMENT-EXTERNAL-USERS",
		"DEV-283-PORTAL-ACCOUNT-HANDOFF",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the new guard and verify RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerDossierUserBoundary -count=1
```

Expected: FAIL because drawer markers, internal-account endpoint, external-user API, handoff UI, and requirement seeds are not implemented.

- [ ] **Step 3: Add PR/DEV rows to the requirement store**

Modify `orderapp-remote/internal/interfaces/http/support/req_store.go` by adding rows in the same seed sections used by nearby PR/DEV entries:

```go
{table: "req_product", code: "PR-283-CUSTOMER-DOSSIER-USER-BOUNDARY", title: "客户档案抽屉化并整理内部用户/外部用户产品边界", status: "doing", assignee: "Codex", evidence: "docs/superpowers/specs/2026-05-15-customer-dossier-user-boundary-design.md"},
{table: "req_dev", code: "DEV-283-CUSTOMER-DRAWER", title: "客户档案列表点击客户名称打开详情编辑抽屉，移除列表编辑操作和重复单行详情", status: "doing", assignee: "Codex", evidence: "CustomersView.vue drawer"},
{table: "req_dev", code: "DEV-283-INTERNAL-USER-PERMISSIONS", title: "员工维护和用户权限只面向内部用户，用户权限页移除渠道客户账号类型切换", status: "doing", assignee: "Codex", evidence: "/api/auth/internal-accounts; UserPermissionsView"},
{table: "req_dev", code: "DEV-283-FULFILLMENT-EXTERNAL-USERS", title: "客户履约运营台管理外部用户创建、启停、重置密码和客户绑定", status: "doing", assignee: "Codex", evidence: "/api/customer-fulfillment/:customer_id/external-users"},
{table: "req_dev", code: "DEV-283-PORTAL-ACCOUNT-HANDOFF", title: "客户门户配置只展示ERP账号绑定摘要并跳转履约运营台管理", status: "doing", assignee: "Codex", evidence: "CustomerPortalSettingsView account handoff"},
```

- [ ] **Step 4: Commit Task 1**

Run:

```bash
git add orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go orderapp-remote/internal/interfaces/http/support/req_store.go
git commit -m "test: guard customer user boundary cleanup"
```

Expected: commit succeeds with only test/seed changes.

### Task 2: Customer Dossier Drawer UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomersView.vue`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go`

- [ ] **Step 1: Replace inline form state with drawer state**

In `CustomersView.vue`, rename the form flags and add drawer helpers:

```js
const customerDrawerOpen = ref(false)
const editingId = ref(0)

function startNew() {
  customerDrawerOpen.value = true
  editingId.value = 0
  assets.value = []
  assignDashboard()
  assignForm(emptyForm())
  ok.value = ''
  error.value = ''
  updateUrl({ mode: 'new' })
}

function closeCustomerDrawer() {
  customerDrawerOpen.value = false
  editingId.value = 0
  assets.value = []
  updateUrl()
}

async function openCustomerDrawer(id) {
  await editCustomer(id)
}
```

Keep `editCustomer(id)` as the data loader but make it set `customerDrawerOpen.value = true`.

- [ ] **Step 2: Move the form into a right-side drawer**

Replace the current `section v-if="formVisible"` block with:

```vue
<div v-if="customerDrawerOpen" class="drawer-mask" @click.self="closeCustomerDrawer">
  <aside class="customer-drawer" aria-label="客户详情">
    <div class="drawer-head">
      <div>
        <h3>{{ editingId ? form.name || '编辑客户' : '新增客户' }}</h3>
        <p>{{ editingId ? '客户详情与资料维护' : '创建新的客户档案' }}</p>
      </div>
      <button class="secondary" type="button" @click="closeCustomerDrawer">关闭</button>
    </div>
    <form class="form-grid drawer-form" @submit.prevent="saveCustomer">
      <label><span>客户名</span><input v-model.trim="form.name" required /></label>
      <label><span>原始名称</span><input v-model.trim="form.raw_name" /></label>
      <label><span>客户类型</span><select v-model="form.customer_type"><option value="retail">零售客户</option><option value="ecommerce">电商客户</option><option value="wholesale">批发客户</option></select></label>
      <label><span>公司名称</span><input v-model.trim="form.company_name" placeholder="不填则销售单默认使用客户名" /></label>
      <label><span>联系电话</span><input v-model.trim="form.company_phone" /></label>
      <label><span>联系人</span><input v-model.trim="form.contact" /></label>
      <label><span>电话</span><input v-model.trim="form.phone" /></label>
      <label><span>默认来源</span><select v-model.number="form.default_source_id"><option :value="0">未设置</option><option v-for="item in sources" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
      <label><span>默认订单类型</span><select v-model.number="form.default_order_type_id"><option :value="0">未设置</option><option v-for="item in orderTypes" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
      <label class="wide"><span>公司地址</span><textarea v-model.trim="form.company_address" rows="2"></textarea></label>
      <label class="wide"><span>地址</span><textarea v-model.trim="form.address" rows="3"></textarea></label>
      <label class="check"><input v-model="form.active" type="checkbox" /><span>启用</span></label>
      <div class="form-actions"><button class="primary" type="submit" :disabled="loading">保存</button></div>
    </form>
  </aside>
</div>
```

After the form, paste the current `customer-extra` subtree exactly as it exists today: the six-card stats block, `asset-form`, and `assets` grid. Do not change `uploadAsset`, `deleteAsset`, `assetKinds`, `assetKind`, `assetInput`, or `assets` field names.

- [ ] **Step 3: Remove the operation column and make the customer name clickable**

Change the table header and row:

```vue
<th>客户</th>
...
<td>
  <button class="name-button" type="button" @click="openCustomerDrawer(row.id)">{{ row.name }}</button>
</td>
```

Delete the `<th>操作</th>` header, delete the edit button cell, and change the empty state colspan from `12` to `11`.

- [ ] **Step 4: Update URL synchronization**

Change `load()` and `saveCustomer()` to check `customerDrawerOpen.value` instead of `formVisible.value`:

```js
updateUrl(customerDrawerOpen.value ? (editingId.value ? { edit_id: editingId.value } : { mode: 'new' }) : {})
```

In `onMounted`, keep existing `edit_id` and `mode=new` behavior, but call `startNew()` only after the list loads.

- [ ] **Step 5: Add drawer CSS**

Append scoped styles:

```css
.name-button { height: auto; min-height: 30px; border: 0; background: transparent; color: #1f4f82; padding: 0; text-align: left; font-weight: 700; }
.drawer-mask { position: fixed; inset: 0; z-index: 40; background: rgba(15, 23, 42, .32); display: flex; justify-content: flex-end; }
.customer-drawer { width: min(760px, 100vw); height: 100%; overflow: auto; background: #fff; box-shadow: -18px 0 40px rgba(15, 23, 42, .18); padding: 16px; }
.drawer-head { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; border-bottom: 1px solid #eee8df; padding-bottom: 12px; margin-bottom: 14px; }
.drawer-head p { margin: 5px 0 0; color: #666; font-size: 13px; }
.drawer-form input, .drawer-form select, .drawer-form textarea { width: 100%; }
@media (max-width: 900px) { .customer-drawer { width: 100vw; } }
```

- [ ] **Step 6: Run source guard and frontend build**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerDossierUserBoundarySourceGuards -count=1
cd frontend-vue-shell
npm run build
```

Expected: source guard still fails only on later tasks; Vite build passes for `CustomersView.vue`.

- [ ] **Step 7: Commit Task 2**

Run:

```bash
git add orderapp-remote/frontend-vue-shell/src/views/CustomersView.vue
git commit -m "feat: move customer dossier editing into drawer"
```

### Task 3: Internal-Only Employee And Permission Screens

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/company/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/mobile_auth.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/auth.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/UserPermissionsView.vue`
- Test: `orderapp-remote/internal/interfaces/http/support/authz_api_test.go`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go`

- [ ] **Step 1: Add failing API test for internal accounts**

In `orderapp-remote/internal/interfaces/http/support/authz_api_test.go`, add:

```go
func TestRequiredPermissionForInternalAccounts(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/auth/internal-accounts"); got != "" {
		t.Fatalf("GET /api/auth/internal-accounts permission = %q, want middleware bypass for auth route", got)
	}
}
```

This documents that the route lives under `/api/auth/` and does its own `auth.manage` check, like the existing auth account endpoints.

- [ ] **Step 2: Filter employee maintenance to internal users**

In `company/repository.go`, change `ListEmployees` to always include:

```go
whereParts := []string{"COALESCE(NULLIF(e.account_type,''),'internal_employee')='internal_employee'"}
args := []any{}
if departmentID > 0 {
	args = append(args, departmentID)
	whereParts = append(whereParts, fmt.Sprintf("e.department_id=$%d", len(args)))
}
where := " WHERE " + strings.Join(whereParts, " AND ")
```

Add `strings` to imports. Keep create/update default behavior unchanged so new employees remain internal by schema default.

- [ ] **Step 3: Add `/api/auth/internal-accounts`**

In `mobile_auth.go`, add a route next to `/api/auth/accounts`:

```go
e.GET("/api/auth/internal-accounts", func(c echo.Context) error {
	if err := requireCurrentPermission(c, authz, "auth.manage"); err != nil {
		return err
	}
	rows, err := pool.Query(c.Request().Context(), fmt.Sprintf(`
		SELECT e.id,COALESCE(e.name,''),COALESCE(e.phone,''),COALESCE(d.name,''),
		       COALESCE(p.password_hash,'') <> '' AS has_password,
		       COALESCE(p.login_disabled,false) AS login_disabled,
		       COALESCE(p.must_reset_password,false) AS must_reset_password
		FROM %s.company_employees e
		LEFT JOIN %s.company_departments d ON d.id=e.department_id
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		WHERE e.active=true AND COALESCE(NULLIF(e.account_type,''),'internal_employee')='internal_employee'
		ORDER BY e.id
	`, schema, schema, schema))
	if err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var name, phone, department string
		var hasPassword, loginDisabled, mustReset bool
		if err := rows.Scan(&id, &name, &phone, &department, &hasPassword, &loginDisabled, &mustReset); err != nil {
			return c.JSON(500, map[string]string{"error": err.Error()})
		}
		out = append(out, map[string]any{
			"employee_id": id, "name": name, "phone": phone, "department": department,
			"has_password": hasPassword, "login_disabled": loginDisabled, "must_reset_password": mustReset,
		})
	}
	if err := rows.Err(); err != nil {
		return c.JSON(500, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, map[string]any{"rows": out})
})
```

Scan each row into `employee_id`, `name`, `phone`, `department`, `has_password`, `login_disabled`, and `must_reset_password`.

- [ ] **Step 4: Update the auth frontend API**

In `frontend-vue-shell/src/api/auth.js`, add:

```js
export function fetchInternalAuthAccounts() {
  return apiGet('/api/auth/internal-accounts')
}
```

Keep `fetchAuthAccounts` for customer portal compatibility until Task 5 removes direct selection there.

- [ ] **Step 5: Simplify `UserPermissionsView.vue`**

In `UserPermissionsView.vue`:

- Replace `fetchAuthAccounts` import with `fetchInternalAuthAccounts`.
- Remove `setAccountType` import and all `setAccountTypeForEmployee`, `accountTypeOf`, and `isChannelCustomer` logic.
- Remove the “账号类型” table column and select.
- Always render role checkboxes and save button.
- Keep login enable/password controls, but label them as backend login controls for internal users.

The account default helper becomes:

```js
function accountOf(employeeId) {
  return accountMap[String(employeeId)] || { login_enabled: true, has_password: false }
}
```

The load call becomes:

```js
const [employeeRows, roleRes, assignmentRes, accountRes] = await Promise.all([
  apiGet('/api/company/employees'),
  fetchRoles(),
  fetchEmployeeRoles(),
  fetchInternalAuthAccounts(),
])
```

- [ ] **Step 6: Run targeted tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'TestRequiredPermissionForInternalAccounts|TestCustomerDossierUserBoundarySourceGuards' -count=1
cd frontend-vue-shell
node --test src/api/auth.test.js
npm run build
```

Expected: Go source guard still fails only on fulfillment and portal handoff markers until later tasks; frontend build passes.

- [ ] **Step 7: Commit Task 3**

Run:

```bash
git add orderapp-remote/internal/infrastructure/postgres/company/repository.go orderapp-remote/internal/interfaces/http/support/mobile_auth.go orderapp-remote/internal/interfaces/http/support/authz_api_test.go orderapp-remote/frontend-vue-shell/src/api/auth.js orderapp-remote/frontend-vue-shell/src/views/UserPermissionsView.vue
git commit -m "feat: limit permissions to internal users"
```

### Task 4: Fulfillment External User APIs And UI

**Files:**
- Modify: `orderapp-remote/internal/application/customerfulfillment/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/module.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`

- [ ] **Step 1: Add failing API tests for external-user management**

In `customerfulfillment/api_test.go`, add tests for:

```go
func TestExternalUserAPIManagesCustomerAccounts(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		externalUsersResult: []app.ExternalUserAccount{{EmployeeID: 23, Name: "誉观山客户", Phone: "13800138023", LoginDisabled: false, BindingStatus: "active"}},
		createExternalUserResult: app.ExternalUserAccount{EmployeeID: 24, Name: "新客户账号", Phone: "13800138024"},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/external-users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"external_users"`) {
		t.Fatalf("list external users status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := `{"name":"新客户账号","phone":"13800138024","password":"secret123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || svc.createExternalUserCmd.CustomerID != 149 || svc.createExternalUserCmd.Phone != "13800138024" {
		t.Fatalf("create external user status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), svc.createExternalUserCmd)
	}
}
```

Add this second test for `login-state`, `password-reset`, `bind`, and `unbind` endpoint routing:

```go
func TestExternalUserAPIUpdatesLoginPasswordAndBinding(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		erpBindingResult: app.CustomerERPBinding{CustomerID: 149, EmployeeID: 23, Status: "active"},
		externalUserLoginStateResult: app.ExternalUserAccount{EmployeeID: 23, LoginDisabled: true},
		externalUserPasswordResult: app.ExternalUserAccount{EmployeeID: 23, HasPassword: true},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/login-state", strings.NewReader(`{"login_enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || svc.externalUserLoginStateCmd.CustomerID != 149 || svc.externalUserLoginStateCmd.EmployeeID != 23 || svc.externalUserLoginStateCmd.LoginEnabled {
		t.Fatalf("login state status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), svc.externalUserLoginStateCmd)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/password-reset", strings.NewReader(`{"password":"secret123"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || svc.externalUserPasswordCmd.Password != "secret123" {
		t.Fatalf("password reset status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), svc.externalUserPasswordCmd)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/bind", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || svc.erpBindingCmd.Status != "active" {
		t.Fatalf("bind status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), svc.erpBindingCmd)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/unbind", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || svc.erpBindingCmd.Status != "inactive" {
		t.Fatalf("unbind status=%d body=%s cmd=%+v", rec.Code, rec.Body.String(), svc.erpBindingCmd)
	}
}
```

- [ ] **Step 2: Add application types and service methods**

In `service.go`, add:

```go
type ExternalUserAccount struct {
	EmployeeID     int64  `json:"employee_id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	LoginDisabled  bool   `json:"login_disabled"`
	HasPassword    bool   `json:"has_password"`
	BindingStatus  string `json:"binding_status,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type CreateExternalUserCommand struct {
	CustomerID int64
	Name       string
	Phone      string
	Password   string
	Actor      string
}

type ExternalUserLoginStateCommand struct {
	CustomerID    int64
	EmployeeID    int64
	LoginEnabled  bool
	Actor         string
}

type ExternalUserPasswordCommand struct {
	CustomerID int64
	EmployeeID int64
	Password   string
	Actor      string
}
```

Extend the repository interface and service with `ListExternalUsers`, `CreateExternalUser`, `SetExternalUserLoginEnabled`, and `ResetExternalUserPassword`. Validate customer ID, employee ID, phone/name, and password length >= 6.

- [ ] **Step 3: Implement repository methods using compatibility storage**

In `customerfulfillment/repository.go`, implement:

```go
func (r *Repository) ListExternalUsers(ctx context.Context, customerID int64) ([]app.ExternalUserAccount, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT e.id, COALESCE(e.name,''), COALESCE(e.phone,''), COALESCE(p.login_disabled,false),
		       COALESCE(p.password_hash,'') <> '', COALESCE(b.status,''), to_char(e.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.company_employees e
		LEFT JOIN %s.employee_login_passwords p ON p.employee_id=e.id
		LEFT JOIN %s.customer_erp_user_bindings b ON b.employee_id=e.id AND b.customer_id=$1
		WHERE COALESCE(NULLIF(e.account_type,''),'internal_employee')='channel_customer'
		  AND (b.customer_id=$1 OR NOT EXISTS (
		    SELECT 1 FROM %s.customer_erp_user_bindings bx WHERE bx.employee_id=e.id AND bx.status='active'
		  ))
		ORDER BY b.status='active' DESC, e.id DESC
	`, r.schema, r.schema, r.schema, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]app.ExternalUserAccount, 0)
	for rows.Next() {
		var row app.ExternalUserAccount
		if err := rows.Scan(&row.EmployeeID, &row.Name, &row.Phone, &row.LoginDisabled, &row.HasPassword, &row.BindingStatus, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
```

For `CreateExternalUser`, insert into `company_employees` with `account_type='channel_customer'`, upsert password into `employee_login_passwords`, then call `UpsertCustomerERPBinding` with status `active`. Use the first active department as the department, creating `客户` department only if no department exists.

For login state/password reset, verify the account is `channel_customer`, update `employee_login_passwords`, and keep the existing binding guard by requiring customer ID in the command.

- [ ] **Step 4: Add HTTP routes**

In `module.go`, add:

```go
e.GET("/api/customer-fulfillment/:customer_id/external-users", api.listExternalUsers)
e.POST("/api/customer-fulfillment/:customer_id/external-users", api.createExternalUser)
e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/bind", api.bindExternalUser)
e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/login-state", api.setExternalUserLoginState)
e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/password-reset", api.resetExternalUserPassword)
e.POST("/api/customer-fulfillment/:customer_id/external-users/:employee_id/unbind", api.unbindExternalUser)
```

Each handler parses `customer_id` and `employee_id`, calls the service, and returns JSON. `bind` and `unbind` call existing `UpsertCustomerERPBinding` with `active` or `inactive`.

- [ ] **Step 5: Add frontend API wrappers**

In `customer-fulfillment.js`, add:

```js
export function fetchCustomerFulfillmentExternalUsers(customerId) {
  return apiGet(`/api/customer-fulfillment/${Number(customerId)}/external-users`)
}

export function createCustomerFulfillmentExternalUser(customerId, payload) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users`, { body: payload })
}

export function bindCustomerFulfillmentExternalUser(customerId, employeeId) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/bind`, { body: {} })
}

export function setCustomerFulfillmentExternalUserLogin(customerId, employeeId, loginEnabled) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/login-state`, { body: { login_enabled: !!loginEnabled } })
}

export function resetCustomerFulfillmentExternalUserPassword(customerId, employeeId, password) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/password-reset`, { body: { password } })
}

export function unbindCustomerFulfillmentExternalUser(customerId, employeeId) {
  return apiSend(`/api/customer-fulfillment/${Number(customerId)}/external-users/${Number(employeeId)}/unbind`, { body: {} })
}
```

Update `customer-fulfillment.test.js` endpoint expectations for these wrappers.

- [ ] **Step 6: Add the external-user panel to `CustomerFulfillmentView.vue`**

Import the wrappers and add state:

```js
const externalUsers = ref([])
const externalUserForm = reactive({ name: '', phone: '', password: '' })
const externalUserPasswordMap = reactive({})
```

Add `loadExternalUsers()` and call it from `loadAll()` after overview/options load.

Render a panel near the top when `normalizedCustomerId` is selected:

```vue
<section class="panel external-users-panel">
  <div class="panel-head">
    <div><h3>外部用户</h3><p>客户侧登录账号在这里创建、启停、重置密码和绑定客户。</p></div>
    <button class="secondary" type="button" @click="loadExternalUsers" :disabled="loading || !normalizedCustomerId">刷新账号</button>
  </div>
  <div class="external-user-form">
    <input v-model.trim="externalUserForm.name" placeholder="账号名称" />
    <input v-model.trim="externalUserForm.phone" placeholder="手机号" />
    <input v-model.trim="externalUserForm.password" type="password" placeholder="初始密码" />
    <button class="primary" type="button" @click="createExternalUser" :disabled="loading || !normalizedCustomerId">创建并绑定</button>
  </div>
  <table>
    <thead><tr><th>账号</th><th>手机号</th><th>状态</th><th>密码</th><th>绑定</th><th>操作</th></tr></thead>
    <tbody>
      <tr v-for="account in externalUsers" :key="account.employee_id">
        <td>{{ account.name || `账号${account.employee_id}` }}</td>
        <td>{{ account.phone || '-' }}</td>
        <td>{{ account.login_disabled ? '已停用' : '可登录' }}</td>
        <td><input v-model.trim="externalUserPasswordMap[String(account.employee_id)]" type="password" placeholder="新密码" /></td>
        <td>{{ account.binding_status === 'active' ? '当前客户' : '未绑定当前客户' }}</td>
        <td class="actions-inline">
          <button class="link-button" type="button" @click="bindExternalUser(account)">绑定</button>
          <button class="link-button" type="button" @click="toggleExternalUserLogin(account)">{{ account.login_disabled ? '启用' : '停用' }}</button>
          <button class="link-button" type="button" @click="resetExternalUserPassword(account)" :disabled="!externalUserPasswordMap[String(account.employee_id)]">重置密码</button>
          <button class="link-button" type="button" @click="unbindExternalUser(account)" :disabled="account.binding_status !== 'active'">解绑</button>
        </td>
      </tr>
    </tbody>
  </table>
</section>
```

- [ ] **Step 7: Run targeted tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerfulfillment -run 'TestExternalUserAPI|TestInternalERPBindingAPI' -count=1
go test ./internal/interfaces/http/support -run TestCustomerDossierUserBoundarySourceGuards -count=1
cd frontend-vue-shell
node --test src/api/customer-fulfillment.test.js
npm run build
```

Expected: all listed tests pass except source guard may still fail on portal handoff until Task 5.

- [ ] **Step 8: Commit Task 4**

Run:

```bash
git add orderapp-remote/internal/application/customerfulfillment/service.go orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go orderapp-remote/internal/interfaces/http/customerfulfillment/module.go orderapp-remote/internal/interfaces/http/customerfulfillment/api.go orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.test.js orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue
git commit -m "feat: manage external users in fulfillment workbench"
```

### Task 5: Customer Portal Account Handoff

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_283_customer_dossier_user_boundary_test.go`

- [ ] **Step 1: Remove direct account selection from portal settings**

In `CustomerPortalSettingsView.vue`:

- Remove `authAccounts`, `channelAccounts`, `loadAuthAccounts`, and the `/api/auth/accounts` call.
- Remove the `<select v-model.number="row.form.erp_employee_id">`.
- Remove `saveERPBinding(row)` and the “绑定ERP账号” button.
- Keep `row.customer.erp_binding` display as read-only summary.

- [ ] **Step 2: Add fulfillment handoff button**

Add:

```js
function goToFulfillmentAccount(row) {
  const url = new URL(window.location.href)
  url.searchParams.set('view', 'customerFulfillment')
  url.searchParams.set('customer_id', String(row?.customer?.id || 0))
  window.location.href = url.toString()
}
```

Render:

```vue
<button class="secondary" type="button" @click="goToFulfillmentAccount(row)" :disabled="!row.customer?.id">
  去履约运营台管理
</button>
```

Keep `erpBindingHint(row)` so operators still see why retail mall or invalid templates do not support ERP workbench.

- [ ] **Step 3: Let `CustomerFulfillmentView` consume `customer_id` on navigation**

In `CustomerFulfillmentView.vue`, keep the existing `onMounted` `customer_id` handling. Verify the URL generated by portal settings uses the same parameter.

- [ ] **Step 4: Update frontend source test**

In `customer-portal-theme.test.js`, replace the assertion that portal settings excludes disabled channel accounts with an assertion that it no longer imports `/api/auth/accounts` and includes `goToFulfillmentAccount`.

Use:

```js
test('customer portal settings hands ERP account changes to fulfillment workbench', () => {
  const source = readView('CustomerPortalSettingsView.vue')
  assert.match(source, /goToFulfillmentAccount/)
  assert.match(source, /去履约运营台管理/)
  assert.doesNotMatch(source, /api\/auth\/accounts/)
  assert.doesNotMatch(source, /saveERPBinding/)
})
```

- [ ] **Step 5: Run targeted tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerDossierUserBoundarySourceGuards -count=1
cd frontend-vue-shell
node --test src/lib/customer-portal-theme.test.js
npm run build
```

Expected: source guard and frontend tests pass.

- [ ] **Step 6: Commit Task 5**

Run:

```bash
git add orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js
git commit -m "feat: hand off portal accounts to fulfillment"
```

### Task 6: Docs, Acceptance, Full Verification, Deploy

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `OP_MANUAL_SETTINGS_AUDIT.md`
- Modify: `OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_SETTINGS_AUDIT.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Create: `docs/acceptance/2026-05-15-customer-dossier-user-boundary.md`

- [ ] **Step 1: Update requirements and acceptance docs**

Change customer maintenance language to:

```markdown
- `/app/customers`：列表点击客户名称打开右侧抽屉查看和编辑客户资料；列表不再提供单独“编辑”操作列。
```

Change auth language to:

```markdown
- 员工维护和用户权限只面向内部用户；外部用户账号在客户履约运营台创建、启停、重置密码和绑定客户。
```

Add acceptance bullets covering the exact PR-283 outcomes from the design.

- [ ] **Step 2: Update operation manuals**

Patch manuals with these user-facing rules:

```markdown
- 客户档案：点击客户名称打开右侧抽屉维护客户资料、默认来源、默认订单类型和附件。
- 员工维护：只维护内部员工，不用于创建客户侧登录账号。
- 用户权限：只给内部用户配置后台角色、菜单和 API 权限。
- 外部用户：在客户履约运营台的“外部用户”区创建、启停、重置密码和绑定客户。
- 客户门户配置：只展示 ERP 账号绑定摘要；需要调整账号时进入履约运营台。
```

Apply the same content to root manuals and `orderapp-remote/docs/` mirrored manuals.

- [ ] **Step 3: Create acceptance evidence file**

Create `docs/acceptance/2026-05-15-customer-dossier-user-boundary.md`:

```markdown
# 客户档案与用户边界验收

日期：2026-05-15

## 需求

- PR-283-CUSTOMER-DOSSIER-USER-BOUNDARY

## 证据

- 客户档案抽屉：`CustomersView.vue`
- 内部账号接口：`GET /api/auth/internal-accounts`
- 外部用户管理：`/api/customer-fulfillment/:customer_id/external-users`
- 门户账号交接：`CustomerPortalSettingsView.vue`
- 手册：`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_SETTINGS_AUDIT.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`

## 验收结果

- [x] 客户档案列表无编辑操作列，点击客户名称打开抽屉。
- [x] 员工维护和用户权限只显示内部用户。
- [x] 履约运营台管理外部用户账号。
- [x] 客户门户配置只显示账号摘要并跳转履约运营台。
```

- [ ] **Step 4: Run verification**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'TestCustomerDossierUserBoundary|TestRequiredPermissionForInternalAccounts' -count=1
go test ./internal/interfaces/http/customerfulfillment -run 'TestExternalUserAPI|TestInternalERPBindingAPI' -count=1
go test ./internal/application/customerfulfillment -run 'Test.*ERPBinding|Test.*ExternalUser' -count=1
go test ./internal/infrastructure/postgres/customerfulfillment -run 'Test.*ERPBinding|Test.*ExternalUser' -count=1
cd frontend-vue-shell
node --test src/api/auth.test.js src/api/customer-fulfillment.test.js src/lib/customer-fulfillment.test.js src/lib/customer-portal-theme.test.js
npm run build
```

Expected: all commands pass.

- [ ] **Step 5: Commit docs and verification evidence**

Run:

```bash
git add REQUIREMENTS.md ACCEPTANCE_TESTS.md OP_MANUAL_ORDER_SALES.md OP_MANUAL_SETTINGS_AUDIT.md OP_MANUAL_CUSTOMER_FULFILLMENT.md OP_MANUAL_CUSTOMER_PORTAL.md orderapp-remote/docs/REQUIREMENTS.md orderapp-remote/docs/ACCEPTANCE_TESTS.md orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md orderapp-remote/docs/OP_MANUAL_SETTINGS_AUDIT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md docs/acceptance/2026-05-15-customer-dossier-user-boundary.md
git commit -m "docs: update customer user boundary manuals"
```

- [ ] **Step 6: Integrate and deploy development**

Run:

```bash
git fetch origin
git merge origin/develop
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'TestCustomerDossierUserBoundary|TestRequiredPermissionForInternalAccounts' -count=1
go test ./internal/interfaces/http/customerfulfillment -run 'TestExternalUserAPI|TestInternalERPBindingAPI' -count=1
cd frontend-vue-shell
npm run build
cd ../..
git push origin codex/customer-user-boundary-design-20260515
git switch develop
git pull --ff-only origin develop
git merge --ff-only codex/customer-user-boundary-design-20260515
git push origin develop
git rev-parse origin/develop
./deploy_orderapp.sh development
```

Expected: fast-forward merge to `develop`, development deployment succeeds, and final response records the deployed `origin/develop` SHA.
