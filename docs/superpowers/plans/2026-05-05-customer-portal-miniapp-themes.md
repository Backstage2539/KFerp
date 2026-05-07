# Customer Portal Miniapp Themes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add customer-level miniapp theme selection in ERP customer portal settings and apply the selected built-in theme across the customer miniapp.

**Architecture:** Store `theme_key` on `customer_portal_profiles` next to `display_name` and `enabled`. Backend normalizes the theme and returns it in admin, login, current-context, customer-switch, and service-page API responses. ERP Vue lets admins choose one of three built-in themes, and the uni-app miniapp maps `theme_key` to root classes and tokenized visual styles.

**Tech Stack:** Go/Echo/Postgres, Vue 3 + Vite ERP shell, uni-app + Vue 3 + TypeScript miniapp, Go unit/API tests, Node/Vitest frontend tests.

---

## File Structure

- Modify `orderapp-remote/internal/application/customerportal/service.go`
  - Owns theme constants, normalization, DTO fields, and service propagation from current customer context into service pages.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
  - Adds `theme_key` with a default and migration-safe `ADD COLUMN IF NOT EXISTS`.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
  - Loads the current customer theme during login and token context resolution.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`
  - Lists, details, and saves the customer portal `theme_key`.
- Modify `orderapp-remote/internal/interfaces/http/customerportal/admin_api.go`
  - Accepts `theme_key` in the customer portal visibility payload.
- Modify `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`
  - API-level assertions for theme propagation and save payload binding.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`
  - Source guard for `theme_key` schema.
- Modify `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`
  - DB-backed tests for default theme, selected theme, and switching current customer.
- Create `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.js`
  - ERP shell source of truth for theme options and normalization.
- Create `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`
  - Node tests for ERP theme options and fallback.
- Modify `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`
  - Adds theme cards and includes `theme_key` in save payloads.
- Create `miniapp/src/utils/themes.ts`
  - Miniapp theme key normalization, metadata, and root class mapping.
- Create `miniapp/src/utils/themes.test.ts`
  - Vitest coverage for the miniapp theme helper.
- Modify `miniapp/src/api/customerPortal.ts`
  - Adds `theme_key` to miniapp API types.
- Modify `miniapp/src/stores/session.ts`
  - Persists the current customer theme in session state.
- Modify `miniapp/src/pages/login/login.vue`
  - Uses default theme on the unauthenticated login screen.
- Modify `miniapp/src/pages/home/home.vue`
  - Applies the selected theme to customer home cards and hero.
- Modify `miniapp/src/pages/service/service.vue`
  - Applies the selected theme to service pages, filters, forms, lists, metrics, and bean-list shell.
- Modify `miniapp/src/App.vue`
  - Sets global page background defaults that do not fight per-theme page roots.
- Modify `orderapp-remote/internal/interfaces/http/support/req_store.go`
  - Adds PR/DEV/UT/API/REV seed rows.
- Create `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_miniapp_themes_test.go`
  - Requirement seed and source guard tests.
- Modify `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/docs/REQUIREMENTS.md`, `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
  - Adds product requirement and acceptance criteria.
- Modify `orderapp-remote/docs/customer-portal-miniapp-test.md`
  - Adds operation manual steps for selecting and validating themes.

---

### Task 1: Backend Theme Model And API Contract

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/admin_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] **Step 1: Write failing API tests for theme fields**

Patch `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`:

```go
type fakeService struct {
	login      customerportalapp.LoginResult
	me         customerportalapp.CurrentContext
	service    customerportalapp.ServicePage
	filter     *customerportalapp.ServicePageFilter
	customers  []customerportalapp.PortalAdminCustomer
	detail     customerportalapp.PortalAdminDetail
	saveCmd    *customerportalapp.UpdatePortalVisibilityCommand
	directShip customerportalapp.DirectShipBatch
	processing customerportalapp.ProcessingRequest
	beanList   customerportalapp.BeanListSummary
	err        error
}

func (s fakeService) UpdatePortalVisibility(_ context.Context, cmd customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	if s.saveCmd != nil {
		*s.saveCmd = cmd
	}
	return s.detail, nil
}
```

Update `TestMiniLoginAndMeAPI` setup:

```go
RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
	login: customerportalapp.LoginResult{
		Token:             "mini-token",
		MiniUserID:        3,
		ThemeKey:          customerportalapp.PortalThemePremiumPartner,
		CurrentCustomerID: 8,
	},
	me: customerportalapp.CurrentContext{
		MiniUserID: 3, CurrentCustomerID: 8, CurrentCustomerName: "客户A",
		ThemeKey:     customerportalapp.PortalThemePremiumPartner,
		Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
	},
}})
```

Update assertions in the same test:

```go
if loginRec.Code != http.StatusOK ||
	!strings.Contains(loginRec.Body.String(), `"token":"mini-token"`) ||
	!strings.Contains(loginRec.Body.String(), `"theme_key":"premium_partner"`) {
	t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
}

if meRec.Code != http.StatusOK ||
	!strings.Contains(meRec.Body.String(), `"current_customer_name":"客户A"`) ||
	!strings.Contains(meRec.Body.String(), `"theme_key":"premium_partner"`) ||
	!strings.Contains(meRec.Body.String(), customerportalapp.CapabilityDirectShip) {
	t.Fatalf("me status=%d body=%s", meRec.Code, meRec.Body.String())
}
```

Update `TestMiniServicePageAPIRequiresTokenAndReturnsScopedData` service fixture and assertion:

```go
service: customerportalapp.ServicePage{
	Key:                 customerportalapp.ServiceKeyShipping,
	Title:               "物流查询",
	ThemeKey:            customerportalapp.PortalThemeCleanOps,
	CurrentCustomerID:   8,
	CurrentCustomerName: "客户A",
	Orders: []customerportalapp.CustomerOrderSummary{{
		OrderNo: "SO-1", ShipTrackingNo: "SF123", GrandTotal: "137.00",
		Items: []customerportalapp.CustomerOrderItemSummary{{ItemName: "乌拉嘎", Spec: "454g", Qty: "2", UnitPrice: "68.50", LineTotal: "137.00"}},
	}},
}
```

Add to the service response assertion:

```go
!strings.Contains(rec.Body.String(), `"theme_key":"clean_ops"`) ||
```

Update `TestPortalAdminVisibilityAPIsExposeAndSaveCustomerCapabilities`:

```go
var saveCmd customerportalapp.UpdatePortalVisibilityCommand
e := echo.New()
RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
	saveCmd:   &saveCmd,
	customers: []customerportalapp.PortalAdminCustomer{{ID: 147, Name: "13800138075", PortalEnabled: true, BindingCount: 1, ThemeKey: customerportalapp.PortalThemeCoffeeFactory}},
	detail: customerportalapp.PortalAdminDetail{
		Customer: customerportalapp.PortalAdminCustomer{ID: 147, Name: "13800138075", DisplayName: "测试客户", PortalEnabled: true, ThemeKey: customerportalapp.PortalThemePremiumPartner},
		Bindings: []customerportalapp.PortalUserBinding{{MiniUserID: 1, Phone: "13800138075", Role: "owner", Status: "approved"}},
		Capabilities: []customerportalapp.CapabilityOption{
			{Code: customerportalapp.CapabilityBeanList, Label: "我的豆单", Enabled: true},
			{Code: customerportalapp.CapabilityDirectShip, Label: "一件代发", Enabled: true},
		},
	},
}})
```

Update list/detail/save assertions:

```go
if listRec.Code != http.StatusOK ||
	!strings.Contains(listRec.Body.String(), `"name":"13800138075"`) ||
	!strings.Contains(listRec.Body.String(), `"theme_key":"coffee_factory"`) ||
	!strings.Contains(listRec.Body.String(), `"binding_count":1`) {
	t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
}

if detailRec.Code != http.StatusOK ||
	!strings.Contains(detailRec.Body.String(), `"theme_key":"premium_partner"`) ||
	!strings.Contains(detailRec.Body.String(), `"bindings":[`) ||
	!strings.Contains(detailRec.Body.String(), `"capabilities":[`) {
	t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
}

body := strings.NewReader(`{"display_name":"测试客户","enabled":true,"theme_key":"premium_partner","capabilities":[{"code":"bean_list","enabled":true},{"code":"direct_ship","enabled":false}]}`)
```

Add after save response assertion:

```go
if saveCmd.ThemeKey != customerportalapp.PortalThemePremiumPartner {
	t.Fatalf("save theme_key=%q, want premium_partner", saveCmd.ThemeKey)
}
```

- [ ] **Step 2: Run API tests to verify they fail**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerportal -run 'TestMiniLoginAndMeAPI|TestMiniServicePageAPIRequiresTokenAndReturnsScopedData|TestPortalAdminVisibilityAPIsExposeAndSaveCustomerCapabilities' -count=1
```

Expected: FAIL with compile errors mentioning `ThemeKey` and `PortalThemePremiumPartner` are undefined.

- [ ] **Step 3: Add theme constants, fields, and normalization**

Patch `orderapp-remote/internal/application/customerportal/service.go` after service key constants:

```go
const (
	PortalThemeCoffeeFactory  = "coffee_factory"
	PortalThemeCleanOps       = "clean_ops"
	PortalThemePremiumPartner = "premium_partner"
)
```

Add `ThemeKey` to DTOs:

```go
type LoginResult struct {
	Token             string            `json:"token"`
	MiniUserID        int64             `json:"mini_user_id"`
	CurrentCustomerID int64             `json:"current_customer_id"`
	ThemeKey          string            `json:"theme_key"`
	Bindings          []CustomerBinding `json:"bindings"`
	Capabilities      []Capability      `json:"capabilities"`
}

type CurrentContext struct {
	MiniUserID          int64             `json:"mini_user_id"`
	CurrentCustomerID   int64             `json:"current_customer_id"`
	CurrentCustomerName string            `json:"current_customer_name"`
	ThemeKey            string            `json:"theme_key"`
	Bindings            []CustomerBinding `json:"bindings"`
	Capabilities        []Capability      `json:"capabilities"`
}

type ServicePage struct {
	Key                 string                 `json:"key"`
	Title               string                 `json:"title"`
	Capability          string                 `json:"capability"`
	ThemeKey            string                 `json:"theme_key"`
	CurrentCustomerID   int64                  `json:"current_customer_id"`
	CurrentCustomerName string                 `json:"current_customer_name"`
	Summary             []ServiceMetric        `json:"summary"`
	BeanLists           []BeanListSummary      `json:"bean_lists,omitempty"`
	Products            []ProductSummary       `json:"products,omitempty"`
	Orders              []CustomerOrderSummary `json:"orders,omitempty"`
	DirectShipBatches   []DirectShipBatch      `json:"direct_ship_batches,omitempty"`
	Inventory           []InventoryItem        `json:"inventory,omitempty"`
	ProcessingRequests  []ProcessingRequest    `json:"processing_requests,omitempty"`
	FeeItems            []FeeItem              `json:"fee_items,omitempty"`
	SettlementBatches   []SettlementBatch      `json:"settlement_batches,omitempty"`
}

type PortalAdminCustomer struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	CompanyName   string `json:"company_name"`
	DisplayName   string `json:"display_name"`
	PortalEnabled bool   `json:"portal_enabled"`
	PortalStatus  string `json:"portal_status"`
	ThemeKey      string `json:"theme_key"`
	BindingCount  int    `json:"binding_count"`
}

type UpdatePortalVisibilityCommand struct {
	CustomerID   int64
	DisplayName  string
	Enabled      bool
	ThemeKey     string
	Capabilities []CapabilityOption
	UpdatedBy    string
}
```

Add this function near normalization helpers:

```go
func NormalizePortalThemeKey(value string) string {
	switch strings.TrimSpace(value) {
	case PortalThemeCoffeeFactory:
		return PortalThemeCoffeeFactory
	case PortalThemeCleanOps:
		return PortalThemeCleanOps
	case PortalThemePremiumPartner:
		return PortalThemePremiumPartner
	default:
		return PortalThemeCoffeeFactory
	}
}
```

In `GetServicePage`, set the page theme after current context is loaded:

```go
page.ThemeKey = NormalizePortalThemeKey(current.ThemeKey)
```

In `UpdatePortalVisibility`, normalize the command before passing it to the repo:

```go
cmd.ThemeKey = NormalizePortalThemeKey(cmd.ThemeKey)
```

- [ ] **Step 4: Accept `theme_key` in admin API**

Patch `orderapp-remote/internal/interfaces/http/customerportal/admin_api.go`:

```go
type portalVisibilityRequest struct {
	DisplayName  string                               `json:"display_name"`
	Enabled      *bool                                `json:"enabled"`
	ThemeKey     string                               `json:"theme_key"`
	Capabilities []customerportalapp.CapabilityOption `json:"capabilities"`
}
```

And include `ThemeKey` in `UpdatePortalVisibilityCommand`:

```go
detail, err := svc.UpdatePortalVisibility(c.Request().Context(), customerportalapp.UpdatePortalVisibilityCommand{
	CustomerID:   id,
	DisplayName:  req.DisplayName,
	Enabled:      enabled,
	ThemeKey:     req.ThemeKey,
	Capabilities: req.Capabilities,
	UpdatedBy:    support.ActorOf(c),
})
```

- [ ] **Step 5: Run API tests to verify they pass**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerportal -run 'TestMiniLoginAndMeAPI|TestMiniServicePageAPIRequiresTokenAndReturnsScopedData|TestPortalAdminVisibilityAPIsExposeAndSaveCustomerCapabilities' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit backend contract**

Run:

```bash
git add orderapp-remote/internal/application/customerportal/service.go orderapp-remote/internal/interfaces/http/customerportal/admin_api.go orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go
git commit -m "feat: add customer portal theme contract"
```

---

### Task 2: Postgres Theme Persistence

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository_source_test.go`

- [ ] **Step 1: Write failing schema source test**

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go` in `TestCustomerPortalSchemaDefinesP0Tables` expected strings:

```go
"theme_key TEXT NOT NULL DEFAULT 'coffee_factory'",
"ADD COLUMN IF NOT EXISTS theme_key TEXT NOT NULL DEFAULT 'coffee_factory'",
```

- [ ] **Step 2: Write failing repository tests**

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go` before `TestEnsureSchemaRejectsNonObjectCapabilityConfig`:

```go
func TestCurrentContextByTokenReturnsCurrentCustomerTheme(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerAID, customerBID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-theme') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('主题客户A') RETURNING id
	`, schema)).Scan(&customerAID); err != nil {
		t.Fatalf("insert customer A: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('主题客户B') RETURNING id
	`, schema)).Scan(&customerBID); err != nil {
		t.Fatalf("insert customer B: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, display_name, theme_key)
		VALUES($1,'主题A展示名','clean_ops'),($2,'主题B展示名','premium_partner')
	`, schema), customerAID, customerBID); err != nil {
		t.Fatalf("insert profiles: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved'),($1,$3,'member','approved')
	`, schema), miniUserID, customerAID, customerBID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES('token-theme',$1,$2,now() + INTERVAL '1 day')
	`, schema), miniUserID, customerBID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	got, err := repo.CurrentContextByToken(ctx, "token-theme")
	if err != nil {
		t.Fatalf("CurrentContextByToken: %v", err)
	}
	if got.CurrentCustomerID != customerBID || got.ThemeKey != customerportalapp.PortalThemePremiumPartner {
		t.Fatalf("current=%d theme=%q, want customerB premium_partner", got.CurrentCustomerID, got.ThemeKey)
	}
}

func TestCreateLoginSessionReturnsDefaultThemeForUnconfiguredCustomer(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerPortalTestDB(t)
	repo := NewRepository(pool, schema)

	var miniUserID, customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid) VALUES('openid-default-theme') RETURNING id
	`, schema)).Scan(&miniUserID); err != nil {
		t.Fatalf("insert mini user: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('默认主题客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_user_bindings(mini_user_id, customer_id, role, status)
		VALUES($1,$2,'owner','approved')
	`, schema), miniUserID, customerID); err != nil {
		t.Fatalf("insert binding: %v", err)
	}

	got, err := repo.CreateLoginSession(ctx, customerportalapp.CreateLoginSessionCommand{OpenID: "openid-default-theme"})
	if err != nil {
		t.Fatalf("CreateLoginSession: %v", err)
	}
	if got.CurrentCustomerID != customerID || got.ThemeKey != customerportalapp.PortalThemeCoffeeFactory {
		t.Fatalf("current=%d theme=%q, want default coffee_factory", got.CurrentCustomerID, got.ThemeKey)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/customerportal -run 'TestCustomerPortalSchemaDefinesP0Tables|TestCurrentContextByTokenReturnsCurrentCustomerTheme|TestCreateLoginSessionReturnsDefaultThemeForUnconfiguredCustomer' -count=1
```

Expected: schema source test fails because `theme_key` is missing; DB-backed tests fail to compile until Task 1 exists, then fail because repository does not populate `ThemeKey`.

- [ ] **Step 4: Add schema column and migration-safe alter**

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go` inside `customer_portal_profiles`:

```go
	theme_key TEXT NOT NULL DEFAULT 'coffee_factory',
```

After the `customer_service_capabilities_customer_code_uq` index statement in the same SQL string, add:

```go
ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS theme_key TEXT NOT NULL DEFAULT 'coffee_factory';
```

Update the `fmt.Sprintf` argument list for the extra `%s` with another `schema`.

- [ ] **Step 5: Load theme in repository context and admin queries**

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`.

In `CreateLoginSession`, after `capabilities` are loaded:

```go
themeKey, err := r.themeForCustomerTx(ctx, tx, currentCustomerID)
if err != nil {
	return customerportalapp.LoginResult{}, err
}
```

Include it in the return:

```go
ThemeKey:          themeKey,
```

In `CurrentContextByToken`, after capabilities are loaded:

```go
themeKey, err := r.themeForCustomerTx(ctx, tx, currentCustomerID)
if err != nil {
	return customerportalapp.CurrentContext{}, err
}
```

Include it in the return:

```go
ThemeKey:            themeKey,
```

Add helper below `capabilitiesForCustomerTx`:

```go
func (r Repository) themeForCustomerTx(ctx context.Context, q txQuerier, customerID int64) (string, error) {
	if customerID <= 0 {
		return customerportalapp.PortalThemeCoffeeFactory, nil
	}
	var raw string
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(theme_key,''),'coffee_factory')
		FROM %s.customer_portal_profiles
		WHERE customer_id=$1
	`, r.schema), customerID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return customerportalapp.PortalThemeCoffeeFactory, nil
	}
	if err != nil {
		return "", err
	}
	return customerportalapp.NormalizePortalThemeKey(raw), nil
}
```

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`.

In `ListPortalAdminCustomers`, add selected column:

```sql
COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
```

Update `GROUP BY` to include `p.theme_key`.

Update scan:

```go
if err := rows.Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.BindingCount); err != nil {
	return nil, err
}
row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
```

In `UpdatePortalVisibility`, change insert/upsert:

```sql
INSERT INTO %s.customer_portal_profiles(customer_id, display_name, enabled, status, theme_key, updated_at, updated_by)
VALUES($1,$2,$3,'active',$4,now(),$5)
ON CONFLICT(customer_id) DO UPDATE SET
	display_name=excluded.display_name,
	enabled=excluded.enabled,
	status='active',
	theme_key=excluded.theme_key,
	updated_at=now(),
	updated_by=excluded.updated_by
```

Update exec args:

```go
cmd.CustomerID, strings.TrimSpace(cmd.DisplayName), cmd.Enabled, customerportalapp.NormalizePortalThemeKey(cmd.ThemeKey), strings.TrimSpace(cmd.UpdatedBy)
```

In `portalAdminCustomer`, add selected column:

```sql
COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
```

Update the full query and scan:

```go
err := r.pool.QueryRow(ctx, fmt.Sprintf(`
	SELECT c.id,
	       COALESCE(c.name,''),
	       COALESCE(c.phone,''),
	       COALESCE(c.company_name,''),
	       COALESCE(p.display_name,''),
	       COALESCE(p.enabled,true),
	       COALESCE(p.status,'active'),
	       COALESCE(NULLIF(p.theme_key,''),'coffee_factory'),
	       COALESCE((SELECT COUNT(*)::int FROM %s.customer_portal_user_bindings b WHERE b.customer_id=c.id AND b.status='approved'),0)
	FROM %s.customers c
	LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=c.id
	WHERE c.id=$1
`, r.schema, r.schema, r.schema), customerID).Scan(&row.ID, &row.Name, &row.Phone, &row.CompanyName, &row.DisplayName, &row.PortalEnabled, &row.PortalStatus, &row.ThemeKey, &row.BindingCount)
if err == nil {
	row.ThemeKey = customerportalapp.NormalizePortalThemeKey(row.ThemeKey)
}
```

- [ ] **Step 6: Update admin repository source guard**

Patch `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository_source_test.go` expected strings:

```go
"theme_key",
"NormalizePortalThemeKey",
```

- [ ] **Step 7: Run repository tests**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/customerportal -count=1
```

Expected: PASS when `ORDERAPP_TEST_DATABASE_URL` or `DATABASE_URL` is configured; otherwise DB-backed tests may skip and source tests pass.

- [ ] **Step 8: Commit persistence**

Run:

```bash
git add orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository_source_test.go
git commit -m "feat: persist customer portal themes"
```

---

### Task 3: ERP Customer Portal Theme Selector

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`

- [ ] **Step 1: Write failing ERP theme helper test**

Create `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`:

```js
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { customerPortalThemeOptions, defaultCustomerPortalThemeKey, normalizeCustomerPortalThemeKey } from './customer-portal-theme.js'

test('customer portal exposes the three built-in miniapp themes', () => {
  assert.equal(defaultCustomerPortalThemeKey, 'coffee_factory')
  assert.deepEqual(customerPortalThemeOptions.map((item) => item.key), [
    'coffee_factory',
    'clean_ops',
    'premium_partner',
  ])
  assert.ok(customerPortalThemeOptions.every((item) => item.label && item.description && item.swatchClass))
})

test('customer portal theme normalization falls back to coffee factory', () => {
  assert.equal(normalizeCustomerPortalThemeKey('clean_ops'), 'clean_ops')
  assert.equal(normalizeCustomerPortalThemeKey('premium_partner'), 'premium_partner')
  assert.equal(normalizeCustomerPortalThemeKey(''), 'coffee_factory')
  assert.equal(normalizeCustomerPortalThemeKey('unknown'), 'coffee_factory')
})
```

- [ ] **Step 2: Run helper test to verify it fails**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/customer-portal-theme.test.js
```

Expected: FAIL because `customer-portal-theme.js` does not exist.

- [ ] **Step 3: Implement ERP theme helper**

Create `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.js`:

```js
export const defaultCustomerPortalThemeKey = 'coffee_factory'

export const customerPortalThemeOptions = [
  {
    key: 'coffee_factory',
    label: '咖啡工厂专业风',
    description: '暖咖啡色，品牌感强，适合大多数客户',
    swatchClass: 'theme-swatch-coffee',
  },
  {
    key: 'clean_ops',
    label: '清爽业务工具风',
    description: '克制清楚，适合高频订单、物流、库存查询',
    swatchClass: 'theme-swatch-clean',
  },
  {
    key: 'premium_partner',
    label: '品牌会员高级风',
    description: '质感更强，适合合作伙伴和对外展示',
    swatchClass: 'theme-swatch-premium',
  },
]

const customerPortalThemeKeys = new Set(customerPortalThemeOptions.map((item) => item.key))

export function normalizeCustomerPortalThemeKey(value) {
  const key = String(value || '').trim()
  return customerPortalThemeKeys.has(key) ? key : defaultCustomerPortalThemeKey
}
```

- [ ] **Step 4: Run helper test to verify it passes**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/customer-portal-theme.test.js
```

Expected: PASS.

- [ ] **Step 5: Update CustomerPortalSettingsView**

Patch `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue` script imports:

```js
import { customerPortalThemeOptions, normalizeCustomerPortalThemeKey } from '../lib/customer-portal-theme'
```

In `createPortalRow`, add `theme_key`:

```js
form: {
  display_name: customer.display_name || '',
  enabled: customer.portal_enabled !== false,
  theme_key: normalizeCustomerPortalThemeKey(customer.theme_key),
},
```

In `assignRowDetail`, add:

```js
row.form.theme_key = normalizeCustomerPortalThemeKey(row.customer.theme_key)
```

In `saveVisibility` body, add:

```js
theme_key: normalizeCustomerPortalThemeKey(row.form.theme_key),
```

In the template, inside `.config-cell` after the enabled checkbox and before the save button, add:

```vue
<div class="theme-picker">
  <span>小程序主题</span>
  <div class="theme-options">
    <button
      v-for="theme in customerPortalThemeOptions"
      :key="`${row.customer.id}-${theme.key}`"
      type="button"
      class="theme-option"
      :class="{ selected: row.form.theme_key === theme.key }"
      @click="row.form.theme_key = theme.key">
      <i :class="['theme-swatch', theme.swatchClass]"></i>
      <strong>{{ theme.label }}</strong>
      <small>{{ theme.description }}</small>
    </button>
  </div>
</div>
```

Add CSS before the media query:

```css
.theme-picker { display: flex; flex-direction: column; gap: 8px; }
.theme-picker > span { color: #666; font-size: 12px; }
.theme-options { display: grid; grid-template-columns: 1fr; gap: 8px; }
.theme-option {
  min-height: 72px;
  display: grid;
  grid-template-columns: 28px 1fr;
  column-gap: 8px;
  row-gap: 3px;
  align-items: start;
  width: 100%;
  height: auto;
  padding: 9px;
  border: 1px solid #e4e7ec;
  border-radius: 8px;
  background: #fff;
  color: #171717;
  text-align: left;
}
.theme-option.selected { border-color: #1f1f1f; box-shadow: 0 0 0 2px rgba(31,31,31,.08); }
.theme-option strong { font-size: 13px; line-height: 1.3; }
.theme-option small { grid-column: 2; color: #666; font-size: 12px; line-height: 1.35; }
.theme-swatch { width: 22px; height: 22px; border-radius: 999px; }
.theme-swatch-coffee { background: linear-gradient(135deg, #2b2118, #9b7141); }
.theme-swatch-clean { background: linear-gradient(135deg, #e7f0eb, #28624a); }
.theme-swatch-premium { background: linear-gradient(135deg, #111, #b88a46); }
```

- [ ] **Step 6: Add a source guard test for the Vue page**

Append to `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`:

```js
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const currentDir = path.dirname(fileURLToPath(import.meta.url))

test('customer portal settings view saves selected miniapp theme', () => {
  const source = fs.readFileSync(path.join(currentDir, '..', 'views', 'CustomerPortalSettingsView.vue'), 'utf8')
  for (const want of [
    'customerPortalThemeOptions',
    'normalizeCustomerPortalThemeKey',
    'theme_key',
    '小程序主题',
    'theme-option',
    'theme-swatch-coffee',
    'theme-swatch-clean',
    'theme-swatch-premium',
  ]) {
    assert.ok(source.includes(want), `missing ${want}`)
  }
})
```

- [ ] **Step 7: Run ERP frontend tests**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/customer-portal-theme.test.js
npm run build
```

Expected: PASS and Vite build succeeds.

- [ ] **Step 8: Commit ERP selector**

Run:

```bash
git add orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.js orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue
git commit -m "feat: add customer portal theme selector"
```

---

### Task 4: Miniapp Theme Application

**Files:**
- Create: `miniapp/src/utils/themes.ts`
- Create: `miniapp/src/utils/themes.test.ts`
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/stores/session.ts`
- Modify: `miniapp/src/pages/login/login.vue`
- Modify: `miniapp/src/pages/home/home.vue`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/App.vue`

- [ ] **Step 1: Write failing miniapp theme helper test**

Create `miniapp/src/utils/themes.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import {
  defaultMiniappThemeKey,
  miniappThemeClass,
  miniappThemeMeta,
  miniappThemeOptions,
  normalizeMiniappThemeKey,
} from './themes'

describe('miniapp themes', () => {
  it('exposes the three built-in customer portal themes', () => {
    expect(defaultMiniappThemeKey).toBe('coffee_factory')
    expect(miniappThemeOptions.map((item) => item.key)).toEqual([
      'coffee_factory',
      'clean_ops',
      'premium_partner',
    ])
  })

  it('normalizes invalid theme keys to the default coffee factory theme', () => {
    expect(normalizeMiniappThemeKey('clean_ops')).toBe('clean_ops')
    expect(normalizeMiniappThemeKey('premium_partner')).toBe('premium_partner')
    expect(normalizeMiniappThemeKey('')).toBe('coffee_factory')
    expect(normalizeMiniappThemeKey('unknown')).toBe('coffee_factory')
  })

  it('maps theme keys to stable page classes and display metadata', () => {
    expect(miniappThemeClass('coffee_factory')).toBe('theme-coffee-factory')
    expect(miniappThemeClass('clean_ops')).toBe('theme-clean-ops')
    expect(miniappThemeClass('premium_partner')).toBe('theme-premium-partner')
    expect(miniappThemeMeta('premium_partner').eyebrow).toBe('ROASTERY PARTNER')
    expect(miniappThemeMeta('unknown').eyebrow).toBe('QACOOHEE SERVICE')
  })
})
```

- [ ] **Step 2: Run miniapp test to verify it fails**

Run:

```bash
npm test --prefix miniapp -- src/utils/themes.test.ts
```

Expected: FAIL because `themes.ts` does not exist.

- [ ] **Step 3: Implement miniapp theme helper**

Create `miniapp/src/utils/themes.ts`:

```ts
export type MiniappThemeKey = 'coffee_factory' | 'clean_ops' | 'premium_partner'

export type MiniappThemeOption = {
  key: MiniappThemeKey
  className: string
  label: string
  eyebrow: string
  subtitle: string
}

export const defaultMiniappThemeKey: MiniappThemeKey = 'coffee_factory'

export const miniappThemeOptions: MiniappThemeOption[] = [
  {
    key: 'coffee_factory',
    className: 'theme-coffee-factory',
    label: '咖啡工厂专业风',
    eyebrow: 'QACOOHEE SERVICE',
    subtitle: '豆单、订单、代发、库存和结算集中处理。',
  },
  {
    key: 'clean_ops',
    className: 'theme-clean-ops',
    label: '清爽业务工具风',
    eyebrow: '客户服务台',
    subtitle: '高频业务优先，订单、物流和库存更容易扫读。',
  },
  {
    key: 'premium_partner',
    className: 'theme-premium-partner',
    label: '品牌会员高级风',
    eyebrow: 'ROASTERY PARTNER',
    subtitle: '围绕豆单、定制服务和结算的合作伙伴入口。',
  },
]

const themesByKey = new Map(miniappThemeOptions.map((item) => [item.key, item]))

export function normalizeMiniappThemeKey(value?: string): MiniappThemeKey {
  const key = String(value || '').trim() as MiniappThemeKey
  return themesByKey.has(key) ? key : defaultMiniappThemeKey
}

export function miniappThemeMeta(value?: string): MiniappThemeOption {
  return themesByKey.get(normalizeMiniappThemeKey(value)) || miniappThemeOptions[0]
}

export function miniappThemeClass(value?: string): string {
  return miniappThemeMeta(value).className
}
```

- [ ] **Step 4: Run helper test to verify it passes**

Run:

```bash
npm test --prefix miniapp -- src/utils/themes.test.ts
```

Expected: PASS.

- [ ] **Step 5: Add theme_key to miniapp API types and session**

Patch `miniapp/src/api/customerPortal.ts`:

```ts
import type { MiniappThemeKey } from '../utils/themes'
```

Add to `LoginResponse`, `MeResponse`, and `ServicePageResponse`:

```ts
theme_key?: MiniappThemeKey | string
```

Patch `miniapp/src/stores/session.ts` imports:

```ts
import { defaultMiniappThemeKey, normalizeMiniappThemeKey, type MiniappThemeKey } from '../utils/themes'
```

Add state:

```ts
themeKey: defaultMiniappThemeKey as MiniappThemeKey,
```

In `clearSession`, add:

```ts
this.themeKey = defaultMiniappThemeKey
```

In `applyContext` argument type, add:

```ts
theme_key?: string
```

At the end of `applyContext`, add:

```ts
this.themeKey = normalizeMiniappThemeKey(context.theme_key)
```

- [ ] **Step 6: Apply themes to login page**

Patch `miniapp/src/pages/login/login.vue` script:

```ts
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const themeClass = miniappThemeClass()
const themeMeta = miniappThemeMeta()
```

Patch template:

```vue
<view class="page" :class="themeClass">
  <view class="hero">
    <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
    <text class="title">客户中心</text>
    <text class="subtitle">{{ themeMeta.subtitle }}</text>
  </view>
```

Replace login page CSS with theme-aware selectors:

```css
.page {
  min-height: 100vh;
  padding: 56rpx 32rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
}

.hero {
  display: flex;
  flex-direction: column;
  gap: 18rpx;
  padding: 72rpx 0 48rpx;
}

.eyebrow {
  color: #6f5d2e;
  font-size: 26rpx;
  font-weight: 800;
}

.theme-clean-ops .eyebrow {
  color: #28624a;
}

.theme-premium-partner .eyebrow {
  color: #8a5c20;
}

.title {
  color: #171717;
  font-size: 52rpx;
  font-weight: 900;
  line-height: 1.14;
}

.subtitle {
  color: #5f5a52;
  font-size: 28rpx;
  line-height: 1.6;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 24rpx;
}

.login-button {
  width: 100%;
  min-height: 88rpx;
  background: #2b2118;
  border-radius: 10rpx;
  color: #ffffff;
  font-size: 30rpx;
  font-weight: 800;
}

.theme-clean-ops .login-button {
  background: #173b2e;
}

.theme-premium-partner .login-button {
  background: #17120d;
  color: #f8ddb0;
}

.error {
  color: #b42318;
  font-size: 26rpx;
  line-height: 1.5;
}
```

- [ ] **Step 7: Apply themes to home page**

Patch `miniapp/src/pages/home/home.vue` script imports:

```ts
import { computed, ref } from 'vue'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'
```

Add computed values:

```ts
const themeClass = computed(() => miniappThemeClass(session.themeKey))
const themeMeta = computed(() => miniappThemeMeta(session.themeKey))
```

Patch template root and header:

```vue
<view class="page" :class="themeClass">
  <view class="header">
    <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
    <text class="title">{{ customerName }}</text>
    <text class="subtitle">{{ themeMeta.subtitle }}</text>
  </view>
```

Replace the `.page`, `.header`, `.eyebrow`, `.entry`, `.entry-pressed`, `.entry-label` CSS blocks with:

```css
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 30rpx 28rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #2b2118 0%, #6b4b2b 100%);
}

.theme-clean-ops .header {
  background: #ffffff;
  border: 1rpx solid #dfe7e2;
}

.theme-premium-partner .header {
  background: linear-gradient(135deg, #111111 0%, #513018 55%, #b88a46 100%);
}

.eyebrow {
  color: rgba(255, 248, 235, 0.78);
  font-size: 24rpx;
  font-weight: 900;
}

.theme-clean-ops .eyebrow {
  color: #28624a;
}

.title {
  color: #fff8eb;
  font-size: 42rpx;
  font-weight: 900;
  line-height: 1.18;
}

.theme-clean-ops .title {
  color: #14201a;
}

.subtitle {
  color: rgba(255, 248, 235, 0.82);
  font-size: 26rpx;
  line-height: 1.55;
}

.theme-clean-ops .subtitle {
  color: #66756c;
}

.entry {
  min-height: 168rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24rpx;
  background: #fffaf2;
  border: 1rpx solid #ead9bd;
  border-radius: 16rpx;
  box-sizing: border-box;
}

.theme-clean-ops .entry {
  background: #ffffff;
  border-color: #dde7e1;
}

.theme-premium-partner .entry {
  background: #fffdf8;
  border-color: #eadab7;
}

.entry-pressed {
  transform: scale(.99);
  opacity: .86;
}

.entry-label {
  color: #171717;
  font-size: 30rpx;
  font-weight: 800;
}
```

- [ ] **Step 8: Apply themes to service page**

Patch `miniapp/src/pages/service/service.vue` imports:

```ts
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'
```

Add computed values:

```ts
const activeThemeKey = computed(() => page.value?.theme_key || session.themeKey)
const themeClass = computed(() => miniappThemeClass(activeThemeKey.value))
const themeMeta = computed(() => miniappThemeMeta(activeThemeKey.value))
```

After `page.value = await fetchServicePage(...)`, add:

```ts
if (page.value.theme_key) {
  session.applyContext({
    mini_user_id: session.miniUserID,
    current_customer_id: page.value.current_customer_id || session.currentCustomerID,
    current_customer_name: page.value.current_customer_name || session.currentCustomerName,
    theme_key: page.value.theme_key,
    bindings: session.bindings,
    capabilities: session.capabilities,
  })
}
```

Patch template root and header:

```vue
<view class="page" :class="themeClass">
  <view class="header">
    <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
    <text class="title">{{ title }}</text>
    <text class="subtitle">{{ page?.current_customer_name || session.currentCustomerName || '客户中心' }}</text>
  </view>
```

Patch existing service CSS by adding theme-aware variants after base card styles:

```css
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-clean-ops {
  background: #f5f7f6;
}

.page.theme-premium-partner {
  background: #fbf7ef;
}

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 30rpx 28rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #2b2118 0%, #6b4b2b 100%);
}

.theme-clean-ops .header {
  background: #ffffff;
  border: 1rpx solid #dfe7e2;
}

.theme-premium-partner .header {
  background: linear-gradient(135deg, #111111 0%, #513018 55%, #b88a46 100%);
}

.theme-clean-ops .metric,
.theme-clean-ops .panel,
.theme-clean-ops .section-row {
  border-color: #dde7e1;
}

.theme-premium-partner .metric,
.theme-premium-partner .panel,
.theme-premium-partner .section-row {
  border-color: #eadab7;
  background: #fffdf8;
}

.theme-clean-ops .primary {
  background: #173b2e;
}

.theme-premium-partner .primary {
  background: #17120d;
  color: #f8ddb0;
}

.theme-clean-ops .section-count {
  color: #28624a;
}

.theme-premium-partner .section-count {
  color: #8a5c20;
}

.theme-clean-ops .bean-list-native {
  border-radius: 16rpx;
}

.theme-premium-partner .bean-list-native {
  border-radius: 16rpx;
}
```

Keep the existing bean-list internals intact; only add theme variants around the outer shell, panels, filters, and buttons.

- [ ] **Step 9: Add source guard tests for page wiring**

Append to `miniapp/src/utils/themes.test.ts`:

```ts
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('miniapp theme source wiring', () => {
  it('applies theme classes to login, home, and service pages', () => {
    for (const file of [
      'src/pages/login/login.vue',
      'src/pages/home/home.vue',
      'src/pages/service/service.vue',
    ]) {
      const source = readFileSync(resolve(file), 'utf8')
      expect(source).toContain('miniappThemeClass')
      expect(source).toContain('theme-coffee-factory')
      expect(source).toContain('theme-clean-ops')
      expect(source).toContain('theme-premium-partner')
    }
  })
})
```

- [ ] **Step 10: Run miniapp tests and build**

Run:

```bash
npm test --prefix miniapp -- src/utils/themes.test.ts src/utils/capabilities.test.ts src/utils/servicePage.test.ts
npm run typecheck --prefix miniapp
npm run build:mp-weixin --prefix miniapp
```

Expected: PASS and `miniapp/dist/build/mp-weixin` generated.

- [ ] **Step 11: Commit miniapp theming**

Run:

```bash
git add miniapp/src/utils/themes.ts miniapp/src/utils/themes.test.ts miniapp/src/api/customerPortal.ts miniapp/src/stores/session.ts miniapp/src/pages/login/login.vue miniapp/src/pages/home/home.vue miniapp/src/pages/service/service.vue miniapp/src/App.vue
git commit -m "feat: theme customer miniapp pages"
```

---

### Task 5: Requirements, Tests Table Seeds, And Manuals

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/customer-portal-miniapp-test.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_miniapp_themes_test.go`

- [ ] **Step 1: Write failing requirement seed/source test**

Create `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_miniapp_themes_test.go`:

```go
package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerPortalMiniappThemeRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-MINIAPP-THEMES",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-02",
		"DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-03",
		"UT-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"API-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
		"REV-CUSTOMER-PORTAL-MINIAPP-THEMES-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal miniapp theme seed missing %q", want)
		}
	}
}

func TestCustomerPortalMiniappThemeSourceWiring(t *testing.T) {
	miniRoot := filepath.Join("..", "miniapp", "src")
	servicePath := filepath.Join(miniRoot, "pages", "service", "service.vue")
	homePath := filepath.Join(miniRoot, "pages", "home", "home.vue")
	themePath := filepath.Join(miniRoot, "utils", "themes.ts")
	for _, path := range []string{servicePath, homePath, themePath} {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				t.Skip("miniapp source is not present in the orderapp-only Docker build context")
			}
			t.Fatalf("stat %s: %v", path, err)
		}
	}
	for _, file := range []string{servicePath, homePath, themePath} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(body)
		for _, want := range []string{"coffee_factory", "clean_ops", "premium_partner", "theme-coffee-factory", "theme-clean-ops", "theme-premium-partner"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing %q", file, want)
			}
		}
	}
}
```

- [ ] **Step 2: Run seed test to verify it fails**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerPortalMiniappTheme -count=1
```

Expected: FAIL because requirement seeds are not present.

- [ ] **Step 3: Add requirement seeds**

Patch `orderapp-remote/internal/interfaces/http/support/req_store.go` immediately after `PR-CUSTOMER-PORTAL-NATIVE-BEANLIST-CACHE` rows:

```go
{table: "req_product", code: "PR-CUSTOMER-PORTAL-MINIAPP-THEMES", title: "客户门户小程序样式优化：ERP 客户门户配置可按客户选择三套内置小程序主题，并在小程序首页、服务页、订单、表单和豆单外层生效", status: "review", assignee: "VA", evidence: "docs/superpowers/specs/2026-05-05-customer-portal-miniapp-themes-design.md"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-01", title: "客户门户 profile 新增 theme_key，后台配置 API、mini 登录/上下文/服务页 API 返回并保存客户主题", status: "done", assignee: "Codex", evidence: "customer_portal_profiles.theme_key; /api/customer-portal/admin/customers/:id/visibility; /api/mini/me; /api/mini/services/:key"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-02", title: "ERP 客户门户配置页新增三套主题单选卡：咖啡工厂专业风、清爽业务工具风、品牌会员高级风", status: "done", assignee: "Codex", evidence: "CustomerPortalSettingsView theme-options; customer-portal-theme.js"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-MINIAPP-THEMES-03", title: "uni-app 小程序新增主题 helper 并在登录页、首页、服务页、订单/筛选/表单/豆单外层应用当前客户主题", status: "done", assignee: "Codex", evidence: "miniapp/src/utils/themes.ts; home.vue; service.vue; login.vue"},
{table: "req_unit", code: "UT-CUSTOMER-PORTAL-MINIAPP-THEMES-01", title: "单元测试覆盖主题归一化、schema 字段、仓储主题读取、ERP 主题选项、小程序主题 helper、源码守卫和需求种子", status: "done", assignee: "Codex", evidence: "go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support; node --test customer-portal-theme.test.js; npm test --prefix miniapp"},
{table: "req_api", code: "API-CUSTOMER-PORTAL-MINIAPP-THEMES-01", title: "API 测试覆盖后台保存 theme_key、客户详情返回 theme_key、/api/mini/me 和 /api/mini/services/:key 返回当前客户主题", status: "done", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1"},
{table: "req_review", code: "REV-CUSTOMER-PORTAL-MINIAPP-THEMES-01", prCode: "PR-CUSTOMER-PORTAL-MINIAPP-THEMES", title: "验收：同一客户可在 ERP 门户设置中切换三套主题；小程序登录该客户后首页和服务页跟随主题；未配置客户默认咖啡工厂专业风", status: "todo", assignee: "VA", evidence: "待 Van ERP 后台和微信开发者工具验收"},
```

- [ ] **Step 4: Update requirements docs**

Append to root `REQUIREMENTS.md` customer portal section, and mirror the same text into `orderapp-remote/docs/REQUIREMENTS.md`:

```markdown
- 客户门户小程序必须支持客户级主题配置。ERP 客户门户配置页可为每个客户选择三套内置主题之一：咖啡工厂专业风、清爽业务工具风、品牌会员高级风；小程序用户端不提供自行切换入口。
- 未配置主题或历史客户必须默认使用咖啡工厂专业风；非法主题值必须回退到默认主题。
- 小程序首页、服务页、订单查询、提交表单、指标卡和豆单外层容器必须跟随当前客户主题；豆单内部发布内容仍按 ERP 豆单发布配置展示。
```

Append to root `ACCEPTANCE_TESTS.md`, and mirror the same text into `orderapp-remote/docs/ACCEPTANCE_TESTS.md`:

```markdown
- [ ] ERP 客户门户配置页中，同一客户可以选择“咖啡工厂专业风 / 清爽业务工具风 / 品牌会员高级风”之一并保存。
- [ ] 小程序登录该客户后，首页和服务页视觉跟随 ERP 选择的主题；切换客户后主题跟随当前客户变化。
- [ ] 未配置主题的客户默认显示咖啡工厂专业风；小程序用户端没有自行切换主题入口。
```

- [ ] **Step 5: Update miniapp operation manual**

Patch `orderapp-remote/docs/customer-portal-miniapp-test.md` after the customer binding SQL section:

```markdown
## 小程序主题配置

ERP 后台路径：`设置 -> 客户门户配置`。

1. 搜索要验收的小程序绑定客户。
2. 在客户行内确认“门户启用”。
3. 在“小程序主题”中选择一套主题：
   - 咖啡工厂专业风：默认主题，暖咖啡色，适合大多数客户。
   - 清爽业务工具风：克制清楚，适合高频查订单、物流和库存。
   - 品牌会员高级风：质感更强，适合合作伙伴入口。
4. 点击“保存配置”。
5. 重新进入微信开发者工具中的小程序，或切换客户后回到首页，确认首页和服务页跟随新主题。

未配置主题的历史客户默认使用“咖啡工厂专业风”。小程序用户端不提供自行切换主题入口。
```

Add to the existing “通过标准” list:

```markdown
- 首页和服务页视觉主题与 ERP 客户门户配置中选择的主题一致。
- 未配置主题时，小程序默认使用咖啡工厂专业风。
```

- [ ] **Step 6: Run seed and docs-related tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'TestCustomerPortalMiniappTheme|TestMiniappServicePageSupportsNativeBeanListCacheAndOrderSearch' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit docs and seeds**

Run:

```bash
git add REQUIREMENTS.md ACCEPTANCE_TESTS.md orderapp-remote/docs/REQUIREMENTS.md orderapp-remote/docs/ACCEPTANCE_TESTS.md orderapp-remote/docs/customer-portal-miniapp-test.md orderapp-remote/internal/interfaces/http/support/req_store.go orderapp-remote/internal/interfaces/http/support/dev_customer_portal_miniapp_themes_test.go
git commit -m "docs: record customer portal miniapp theme requirement"
```

---

### Task 6: Full Verification And Integration Readiness

**Files:**
- Verify all files changed by Tasks 1-5.

- [ ] **Step 1: Run focused Go tests**

Run:

```bash
cd orderapp-remote
go test ./internal/application/customerportal ./internal/interfaces/http/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support -count=1
```

Expected: PASS. DB-backed repository tests may skip only if no test database env is configured; source guards must pass.

- [ ] **Step 2: Run miniapp tests and typecheck**

Run:

```bash
npm test --prefix miniapp
npm run typecheck --prefix miniapp
```

Expected: PASS.

- [ ] **Step 3: Run ERP frontend tests and build**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/customer-portal-theme.test.js src/lib/menu-ia.test.js src/lib/view-routing.test.js
npm run build
```

Expected: PASS and Vite build succeeds.

- [ ] **Step 4: Run miniapp WeChat build**

Run:

```bash
VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp
```

Expected: PASS and `miniapp/dist/build/mp-weixin` exists.

- [ ] **Step 5: Run full backend tests**

Run:

```bash
cd orderapp-remote
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Check formatting and unstaged diff**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors. `git status --short` should show only intentional committed branch changes or be clean.

- [ ] **Step 7: Acceptance review**

Manually verify these requirement outcomes from code and test evidence:

```text
PR-CUSTOMER-PORTAL-MINIAPP-THEMES:
- ERP customer portal settings can save coffee_factory, clean_ops, or premium_partner.
- Admin detail/list API returns theme_key.
- /api/mini/me and /api/mini/services/:key return theme_key.
- Miniapp session applies theme_key.
- Login/home/service pages contain the three theme root classes.
- Missing/unknown theme falls back to coffee_factory.
- Docs and five requirement tables contain PR/DEV/UT/API/REV evidence.
```

- [ ] **Step 8: Final commit if verification edits were needed**

If Task 6 changed files, commit them:

```bash
git add <changed-files>
git commit -m "test: verify customer portal miniapp themes"
```

If Task 6 did not change files, do not create an empty commit.
