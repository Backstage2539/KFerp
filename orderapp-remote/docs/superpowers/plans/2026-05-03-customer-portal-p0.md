# Customer Portal P0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first customer portal foundation: mini-program login, customer binding, service capability configuration, `/api/mini/me`, and a uni-app customer home skeleton.

**Architecture:** Add a new `customerportal` bounded context beside existing application/infrastructure/interface modules. Mini-program requests use their own `/api/mini/*` auth path and never reuse employee-only ERP API permissions. P0 deliberately stops at identity, binding, capability visibility, and home scaffolding; bean lists, orders, direct ship import, processing jobs, and settlement details are separate P1-P4 plans.

**Tech Stack:** Go/Echo/Postgres for ERP APIs, source-inspection and service unit tests, `httptest` API tests, `uni-app + Vue 3 + TypeScript + Pinia` for the mini-program shell.

---

## Scope Check

The approved design covers five independent subsystems. This plan implements only P0:

- mini users and mini sessions
- customer portal profiles
- mini user to customer bindings
- customer service capabilities
- `/api/mini/login`, `/api/mini/me`, and customer switch
- mini-program login and home shell
- PR/DEV/UT/API/REV seeds for P0

P1-P4 must be planned separately after P0 is merged and verified.

## File Structure

- Create `orderapp-remote/internal/application/customerportal/service.go`: domain-facing service types and P0 customer portal use cases.
- Create `orderapp-remote/internal/application/customerportal/service_test.go`: service validation and capability aggregation unit tests.
- Create `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`: P0 DDL.
- Create `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`: Postgres repository for login/session/context.
- Create `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`: schema source guard for required tables/indexes.
- Create `orderapp-remote/internal/interfaces/http/customerportal/module.go`: route registration contract.
- Create `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`: `/api/mini/*` handlers.
- Create `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`: API-level handler tests with fake service.
- Modify `orderapp-remote/internal/appmain/schema_setup.go`: include customer portal schema.
- Modify `orderapp-remote/internal/appmain/app_routes.go`: wire customer portal service and routes.
- Modify `orderapp-remote/internal/interfaces/http/support/auth_middleware.go`: allow `/api/mini/*` through BasicAuth so mini handlers can authenticate mini tokens themselves.
- Modify `orderapp-remote/internal/interfaces/http/support/auth_middleware_test.go`: cover mini API auth boundary.
- Modify `orderapp-remote/internal/interfaces/http/support/req_store.go`: seed P0 requirement rows.
- Create `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_p0_test.go`: source guard for requirement seeds and wiring.
- Create `miniapp/`: uni-app project root.
- Create `miniapp/package.json`, `miniapp/vite.config.ts`, `miniapp/tsconfig.json`, `miniapp/src/main.ts`, `miniapp/src/App.vue`, `miniapp/src/pages.json`, `miniapp/src/manifest.json`.
- Create `miniapp/src/api/client.ts`, `miniapp/src/api/customerPortal.ts`: typed mini API client.
- Create `miniapp/src/stores/session.ts`: token/current-customer state.
- Create `miniapp/src/pages/login/login.vue`, `miniapp/src/pages/home/home.vue`: first login and home pages.
- Create `miniapp/src/utils/capabilities.ts`, `miniapp/src/utils/capabilities.test.ts`: module visibility logic.

### Task 1: Requirement Seeds

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_p0_test.go`

- [ ] **Step 1: Write the failing seed guard**

Create `orderapp-remote/internal/interfaces/http/support/dev_customer_portal_p0_test.go`:

```go
package support

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalP0RequirementSeeds(t *testing.T) {
	body, err := os.ReadFile("req_store.go")
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-CUSTOMER-PORTAL-P0",
		"DEV-CUSTOMER-PORTAL-P0-01",
		"DEV-CUSTOMER-PORTAL-P0-02",
		"DEV-CUSTOMER-PORTAL-P0-03",
		"DEV-CUSTOMER-PORTAL-P0-04",
		"UT-CUSTOMER-PORTAL-P0-01",
		"API-CUSTOMER-PORTAL-P0-01",
		"REV-CUSTOMER-PORTAL-P0-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal P0 seed missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the failing seed guard**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerPortalP0RequirementSeeds -count=1
```

Expected: `FAIL` because the seed codes are not present yet.

- [ ] **Step 3: Add seed rows**

In `orderapp-remote/internal/interfaces/http/support/req_store.go`, append these rows inside the existing `for _, row := range []reqSeedRow{ ... }` seed block:

```go
{table: "req_product", code: "PR-CUSTOMER-PORTAL-P0", title: "客户服务平台 P0：小程序客户登录、客户绑定、服务能力配置和客户首页底座", status: "review", assignee: "VA", evidence: "docs/superpowers/specs/2026-05-03-customer-service-platform-design.md; docs/superpowers/plans/2026-05-03-customer-portal-p0.md"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-01", title: "新增 customerportal application service，统一 mini user、客户绑定、当前客户和服务能力聚合规则", status: "todo", assignee: "Codex", evidence: "internal/application/customerportal"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-02", title: "新增 customerportal Postgres schema/repository，包含 mini_users、mini_sessions、customer_portal_profiles、customer_portal_user_bindings、customer_service_capabilities", status: "todo", assignee: "Codex", evidence: "internal/infrastructure/postgres/customerportal"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-03", title: "新增 /api/mini/login、/api/mini/me、/api/mini/current-customer，并让 /api/mini/* 使用小程序 token 自行鉴权", status: "todo", assignee: "Codex", evidence: "internal/interfaces/http/customerportal; auth_middleware.go"},
{table: "req_dev", code: "DEV-CUSTOMER-PORTAL-P0-04", title: "新增 uni-app 微信小程序骨架，包含登录页、客户首页、token 存储和按服务能力显示入口", status: "todo", assignee: "Codex", evidence: "miniapp"},
{table: "req_unit", code: "UT-CUSTOMER-PORTAL-P0-01", title: "单元测试覆盖客户门户服务能力聚合、schema 表定义、miniapp 能力入口显示逻辑和需求种子", status: "todo", assignee: "Codex", evidence: "go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support; npm test --prefix miniapp"},
{table: "req_api", code: "API-CUSTOMER-PORTAL-P0-01", title: "API 测试覆盖 /api/mini/login、/api/mini/me、/api/mini/current-customer 和未绑定客户的 401/403 行为", status: "todo", assignee: "Codex", evidence: "go test ./internal/interfaces/http/customerportal -count=1"},
{table: "req_review", code: "REV-CUSTOMER-PORTAL-P0-01", prCode: "PR-CUSTOMER-PORTAL-P0", title: "验收：小程序用户登录后只能看到已绑定客户和该客户开通的服务能力入口", status: "todo", assignee: "VA", evidence: "待 Van 小程序开发工具/接口验收"},
```

- [ ] **Step 4: Verify seed guard passes**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestCustomerPortalP0RequirementSeeds -count=1
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/internal/interfaces/http/support/req_store.go orderapp-remote/internal/interfaces/http/support/dev_customer_portal_p0_test.go
git commit -m "test: seed customer portal p0 requirements"
```

### Task 2: Application Service

**Files:**
- Create: `orderapp-remote/internal/application/customerportal/service.go`
- Create: `orderapp-remote/internal/application/customerportal/service_test.go`

- [ ] **Step 1: Write service tests**

Create `orderapp-remote/internal/application/customerportal/service_test.go`:

```go
package customerportal

import (
	"context"
	"errors"
	"testing"
)

type fakeIdentityProvider struct {
	identity MiniIdentity
	err      error
}

func (p fakeIdentityProvider) Resolve(ctx context.Context, code string) (MiniIdentity, error) {
	if p.err != nil {
		return MiniIdentity{}, p.err
	}
	return p.identity, nil
}

type fakeRepository struct {
	loginResult LoginResult
	context     CurrentContext
	session     string
	err         error
	switchErr   error
}

func (r *fakeRepository) CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error) {
	if r.err != nil {
		return LoginResult{}, r.err
	}
	return r.loginResult, nil
}

func (r *fakeRepository) CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error) {
	r.session = token
	if r.err != nil {
		return CurrentContext{}, r.err
	}
	return r.context, nil
}

func (r *fakeRepository) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	r.session = token
	if r.switchErr != nil {
		return CurrentContext{}, r.switchErr
	}
	r.context.CurrentCustomerID = customerID
	return r.context, nil
}

func TestLoginRejectsEmptyCode(t *testing.T) {
	svc := NewService(&fakeRepository{}, fakeIdentityProvider{})
	_, err := svc.Login(context.Background(), LoginCommand{})
	if err == nil || err.Error() != "code required" {
		t.Fatalf("Login() err=%v, want code required", err)
	}
}

func TestLoginCreatesSessionFromResolvedIdentity(t *testing.T) {
	repo := &fakeRepository{loginResult: LoginResult{Token: "mini-token", MiniUserID: 9}}
	svc := NewService(repo, fakeIdentityProvider{identity: MiniIdentity{OpenID: "openid-1", UnionID: "union-1"}})
	got, err := svc.Login(context.Background(), LoginCommand{Code: "wx-code", Phone: "13800138000", Nickname: "客户"})
	if err != nil {
		t.Fatalf("Login() err=%v", err)
	}
	if got.Token != "mini-token" || got.MiniUserID != 9 {
		t.Fatalf("Login()=%+v", got)
	}
}

func TestMeRequiresTokenAndReturnsBoundCapabilities(t *testing.T) {
	repo := &fakeRepository{context: CurrentContext{
		MiniUserID:         8,
		CurrentCustomerID:  7,
		CurrentCustomerName: "品牌客户",
		Bindings: []CustomerBinding{{CustomerID: 7, CustomerName: "品牌客户", Role: "owner", Status: "approved"}},
		Capabilities: []Capability{{Code: CapabilityDirectShip, Enabled: true}, {Code: CapabilitySettlement, Enabled: true}},
	}}
	svc := NewService(repo, fakeIdentityProvider{})
	got, err := svc.Me(context.Background(), "mini-token")
	if err != nil {
		t.Fatalf("Me() err=%v", err)
	}
	if got.CurrentCustomerID != 7 || !got.HasCapability(CapabilityDirectShip) || !got.HasCapability(CapabilitySettlement) {
		t.Fatalf("Me()=%+v", got)
	}
}

func TestSwitchCustomerRejectsUnauthorizedBinding(t *testing.T) {
	repo := &fakeRepository{switchErr: ErrCustomerBindingNotFound}
	svc := NewService(repo, fakeIdentityProvider{})
	_, err := svc.SwitchCurrentCustomer(context.Background(), "mini-token", 99)
	if !errors.Is(err, ErrCustomerBindingNotFound) {
		t.Fatalf("SwitchCurrentCustomer() err=%v", err)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd orderapp-remote
go test ./internal/application/customerportal -count=1
```

Expected: package does not compile because `service.go` does not exist.

- [ ] **Step 3: Implement the application service**

Create `orderapp-remote/internal/application/customerportal/service.go`:

```go
package customerportal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	CapabilityBeanList         = "bean_list"
	CapabilityProductOrder     = "product_order"
	CapabilityDirectShip       = "direct_ship"
	CapabilityProcessing       = "processing"
	CapabilityInventoryCustody = "inventory_custody"
	CapabilityShippingQuery    = "shipping_query"
	CapabilitySettlement       = "settlement"
)

var ErrCustomerBindingNotFound = errors.New("customer binding not found")

type LoginCommand struct {
	Code     string
	Phone    string
	Nickname string
}

type MiniIdentity struct {
	OpenID  string
	UnionID string
}

type CreateLoginSessionCommand struct {
	OpenID   string
	UnionID  string
	Phone    string
	Nickname string
}

type LoginResult struct {
	Token             string            `json:"token"`
	MiniUserID        int64             `json:"mini_user_id"`
	CurrentCustomerID int64             `json:"current_customer_id"`
	Bindings          []CustomerBinding `json:"bindings"`
	Capabilities      []Capability      `json:"capabilities"`
}

type CustomerBinding struct {
	CustomerID   int64  `json:"customer_id"`
	CustomerName string `json:"customer_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
}

type Capability struct {
	Code    string         `json:"code"`
	Enabled bool          `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

type CurrentContext struct {
	MiniUserID           int64             `json:"mini_user_id"`
	CurrentCustomerID    int64             `json:"current_customer_id"`
	CurrentCustomerName  string            `json:"current_customer_name"`
	Bindings             []CustomerBinding `json:"bindings"`
	Capabilities         []Capability      `json:"capabilities"`
}

func (c CurrentContext) HasCapability(code string) bool {
	code = strings.TrimSpace(code)
	for _, capability := range c.Capabilities {
		if capability.Enabled && capability.Code == code {
			return true
		}
	}
	return false
}

type IdentityProvider interface {
	Resolve(ctx context.Context, code string) (MiniIdentity, error)
}

type Repository interface {
	CreateLoginSession(ctx context.Context, cmd CreateLoginSessionCommand) (LoginResult, error)
	CurrentContextByToken(ctx context.Context, token string) (CurrentContext, error)
	SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error)
}

type Service struct {
	repo     Repository
	identity IdentityProvider
}

func NewService(repo Repository, identity IdentityProvider) *Service {
	return &Service{repo: repo, identity: identity}
}

func (s *Service) Login(ctx context.Context, cmd LoginCommand) (LoginResult, error) {
	code := strings.TrimSpace(cmd.Code)
	if code == "" {
		return LoginResult{}, fmt.Errorf("code required")
	}
	if s.repo == nil {
		return LoginResult{}, fmt.Errorf("repository required")
	}
	if s.identity == nil {
		return LoginResult{}, fmt.Errorf("identity provider required")
	}
	identity, err := s.identity.Resolve(ctx, code)
	if err != nil {
		return LoginResult{}, err
	}
	identity.OpenID = strings.TrimSpace(identity.OpenID)
	if identity.OpenID == "" {
		return LoginResult{}, fmt.Errorf("openid required")
	}
	return s.repo.CreateLoginSession(ctx, CreateLoginSessionCommand{
		OpenID:   identity.OpenID,
		UnionID:  strings.TrimSpace(identity.UnionID),
		Phone:    strings.TrimSpace(cmd.Phone),
		Nickname: strings.TrimSpace(cmd.Nickname),
	})
}

func (s *Service) Me(ctx context.Context, token string) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	return s.repo.CurrentContextByToken(ctx, token)
}

func (s *Service) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (CurrentContext, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return CurrentContext{}, fmt.Errorf("mini token required")
	}
	if customerID <= 0 {
		return CurrentContext{}, fmt.Errorf("customer required")
	}
	if s.repo == nil {
		return CurrentContext{}, fmt.Errorf("repository required")
	}
	return s.repo.SwitchCurrentCustomer(ctx, token, customerID)
}
```

- [ ] **Step 4: Verify service tests pass**

Run:

```bash
cd orderapp-remote
go test ./internal/application/customerportal -count=1
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/internal/application/customerportal
git commit -m "feat: add customer portal application service"
```

### Task 3: Postgres Schema And Repository

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`
- Modify: `orderapp-remote/internal/appmain/schema_setup.go`

- [ ] **Step 1: Write schema guard test**

Create `orderapp-remote/internal/infrastructure/postgres/customerportal/schema_test.go`:

```go
package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalSchemaDefinesP0Tables(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %s.mini_users",
		"CREATE TABLE IF NOT EXISTS %s.mini_sessions",
		"CREATE TABLE IF NOT EXISTS %s.customer_portal_profiles",
		"CREATE TABLE IF NOT EXISTS %s.customer_portal_user_bindings",
		"CREATE TABLE IF NOT EXISTS %s.customer_service_capabilities",
		"mini_users_openid_uq",
		"customer_portal_user_bindings_user_customer_uq",
		"customer_service_capabilities_customer_code_uq",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run schema guard and verify failure**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/customerportal -run TestCustomerPortalSchemaDefinesP0Tables -count=1
```

Expected: package does not compile because the package does not exist.

- [ ] **Step 3: Implement schema**

Create `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`:

```go
package customerportal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.mini_users (
	id BIGSERIAL PRIMARY KEY,
	openid TEXT NOT NULL,
	unionid TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	nickname TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_login_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS mini_users_openid_uq ON %s.mini_users(openid);

CREATE TABLE IF NOT EXISTS %s.mini_sessions (
	token TEXT PRIMARY KEY,
	mini_user_id BIGINT NOT NULL REFERENCES %s.mini_users(id) ON DELETE CASCADE,
	current_customer_id BIGINT NULL REFERENCES %s.customers(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	expire_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS mini_sessions_user_idx ON %s.mini_sessions(mini_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.customer_portal_profiles (
	customer_id BIGINT PRIMARY KEY REFERENCES %s.customers(id) ON DELETE CASCADE,
	display_name TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	default_settlement_cycle TEXT NOT NULL DEFAULT 'monthly',
	default_payment_terms TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS %s.customer_portal_user_bindings (
	id BIGSERIAL PRIMARY KEY,
	mini_user_id BIGINT NOT NULL REFERENCES %s.mini_users(id) ON DELETE CASCADE,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	role TEXT NOT NULL DEFAULT 'member',
	status TEXT NOT NULL DEFAULT 'approved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	approved_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_portal_user_bindings_user_customer_uq
	ON %s.customer_portal_user_bindings(mini_user_id, customer_id);

CREATE TABLE IF NOT EXISTS %s.customer_service_capabilities (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	capability_code TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_service_capabilities_customer_code_uq
	ON %s.customer_service_capabilities(customer_id, capability_code);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
```

- [ ] **Step 4: Implement repository**

Create `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`:

```go
package customerportal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) CreateLoginSession(ctx context.Context, cmd customerportalapp.CreateLoginSessionCommand) (customerportalapp.LoginResult, error) {
	openID := strings.TrimSpace(cmd.OpenID)
	if openID == "" {
		return customerportalapp.LoginResult{}, fmt.Errorf("openid required")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var miniUserID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_users(openid,unionid,phone,nickname,last_login_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT (openid) DO UPDATE SET
			unionid=COALESCE(NULLIF(excluded.unionid,''), %s.mini_users.unionid),
			phone=COALESCE(NULLIF(excluded.phone,''), %s.mini_users.phone),
			nickname=COALESCE(NULLIF(excluded.nickname,''), %s.mini_users.nickname),
			active=true,
			last_login_at=now()
		RETURNING id
	`, r.schema, r.schema, r.schema, r.schema), openID, strings.TrimSpace(cmd.UnionID), strings.TrimSpace(cmd.Phone), strings.TrimSpace(cmd.Nickname)).Scan(&miniUserID); err != nil {
		return customerportalapp.LoginResult{}, err
	}

	bindings, err := r.listBindingsTx(ctx, tx, miniUserID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	currentCustomerID := int64(0)
	if len(bindings) > 0 {
		currentCustomerID = bindings[0].CustomerID
	}
	token, err := randomToken(24)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.mini_sessions(token, mini_user_id, current_customer_id, expire_at)
		VALUES($1,$2,NULLIF($3,0),now()+interval '30 days')
	`, r.schema), token, miniUserID, currentCustomerID); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	capabilities, err := r.capabilitiesForCustomerTx(ctx, tx, currentCustomerID)
	if err != nil {
		return customerportalapp.LoginResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.LoginResult{}, err
	}
	return customerportalapp.LoginResult{Token: token, MiniUserID: miniUserID, CurrentCustomerID: currentCustomerID, Bindings: bindings, Capabilities: capabilities}, nil
}

func (r Repository) CurrentContextByToken(ctx context.Context, token string) (customerportalapp.CurrentContext, error) {
	token = strings.TrimSpace(token)
	var out customerportalapp.CurrentContext
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.mini_user_id, COALESCE(s.current_customer_id,0), COALESCE(c.name,'')
		FROM %s.mini_sessions s
		LEFT JOIN %s.customers c ON c.id=s.current_customer_id
		WHERE s.token=$1 AND s.expire_at>now()
	`, r.schema, r.schema), token).Scan(&out.MiniUserID, &out.CurrentCustomerID, &out.CurrentCustomerName)
	if err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.CurrentContext{}, fmt.Errorf("mini session not found")
		}
		return customerportalapp.CurrentContext{}, err
	}
	bindings, err := r.ListBindings(ctx, out.MiniUserID)
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	out.Bindings = bindings
	out.Capabilities, err = r.CapabilitiesForCustomer(ctx, out.CurrentCustomerID)
	return out, err
}

func (r Repository) SwitchCurrentCustomer(ctx context.Context, token string, customerID int64) (customerportalapp.CurrentContext, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var miniUserID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT mini_user_id FROM %s.mini_sessions WHERE token=$1 AND expire_at>now() FOR UPDATE`, r.schema), strings.TrimSpace(token)).Scan(&miniUserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return customerportalapp.CurrentContext{}, fmt.Errorf("mini session not found")
		}
		return customerportalapp.CurrentContext{}, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.customer_portal_user_bindings
			WHERE mini_user_id=$1 AND customer_id=$2 AND status='approved'
		)
	`, r.schema), miniUserID, customerID).Scan(&exists); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if !exists {
		return customerportalapp.CurrentContext{}, customerportalapp.ErrCustomerBindingNotFound
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.mini_sessions SET current_customer_id=$2 WHERE token=$1`, r.schema), strings.TrimSpace(token), customerID); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return customerportalapp.CurrentContext{}, err
	}
	return r.CurrentContextByToken(ctx, token)
}

func (r Repository) ListBindings(ctx context.Context, miniUserID int64) ([]customerportalapp.CustomerBinding, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := r.listBindingsTx(ctx, tx, miniUserID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r Repository) CapabilitiesForCustomer(ctx context.Context, customerID int64) ([]customerportalapp.Capability, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := r.capabilitiesForCustomerTx(ctx, tx, customerID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}

type txQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (r Repository) listBindingsTx(ctx context.Context, tx txQuerier, miniUserID int64) ([]customerportalapp.CustomerBinding, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.customer_id, COALESCE(NULLIF(p.display_name,''), c.name, ''), b.role, b.status
		FROM %s.customer_portal_user_bindings b
		JOIN %s.customers c ON c.id=b.customer_id
		LEFT JOIN %s.customer_portal_profiles p ON p.customer_id=b.customer_id
		WHERE b.mini_user_id=$1 AND b.status='approved'
		ORDER BY b.id
	`, r.schema, r.schema, r.schema), miniUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []customerportalapp.CustomerBinding{}
	for rows.Next() {
		var row customerportalapp.CustomerBinding
		if err := rows.Scan(&row.CustomerID, &row.CustomerName, &row.Role, &row.Status); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) capabilitiesForCustomerTx(ctx context.Context, tx txQuerier, customerID int64) ([]customerportalapp.Capability, error) {
	if customerID <= 0 {
		return []customerportalapp.Capability{}, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT capability_code, enabled, config_json
		FROM %s.customer_service_capabilities
		WHERE customer_id=$1
		ORDER BY capability_code
	`, r.schema), customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []customerportalapp.Capability{}
	for rows.Next() {
		var row customerportalapp.Capability
		var raw []byte
		if err := rows.Scan(&row.Code, &row.Enabled, &raw); err != nil {
			return nil, err
		}
		row.Config = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &row.Config)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 5: Wire schema setup**

Modify `orderapp-remote/internal/appmain/schema_setup.go`:

```go
import (
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"
)
```

Add the schema step after `core` and before `support`:

```go
{Name: "customerportal", Run: func(ctx context.Context) error { return postgrescustomerportal.EnsureSchema(ctx, pool, schema) }},
```

- [ ] **Step 6: Verify schema tests pass**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/customerportal -count=1
go test ./internal/appmain -run Test -count=1
```

Expected: both commands pass.

- [ ] **Step 7: Commit**

```bash
git add orderapp-remote/internal/infrastructure/postgres/customerportal orderapp-remote/internal/appmain/schema_setup.go
git commit -m "feat: add customer portal postgres schema"
```

### Task 4: Mini API Routes

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/customerportal/module.go`
- Create: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Create: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/auth_middleware.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/auth_middleware_test.go`
- Modify: `orderapp-remote/internal/appmain/app_routes.go`

- [ ] **Step 1: Write API tests**

Create `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`:

```go
package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type fakeService struct {
	login customerportalapp.LoginResult
	me    customerportalapp.CurrentContext
	err   error
}

func (s fakeService) Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error) {
	if s.err != nil {
		return customerportalapp.LoginResult{}, s.err
	}
	return s.login, nil
}

func (s fakeService) Me(context.Context, string) (customerportalapp.CurrentContext, error) {
	if s.err != nil {
		return customerportalapp.CurrentContext{}, s.err
	}
	return s.me, nil
}

func (s fakeService) SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error) {
	if s.err != nil {
		return customerportalapp.CurrentContext{}, s.err
	}
	return s.me, nil
}

func TestMiniLoginAndMeAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login: customerportalapp.LoginResult{Token: "mini-token", MiniUserID: 3},
		me: customerportalapp.CurrentContext{
			MiniUserID: 3, CurrentCustomerID: 8, CurrentCustomerName: "客户A",
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		},
	}})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code","phone":"13800138000"}`))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK || !strings.Contains(loginRec.Body.String(), `"token":"mini-token"`) {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	meReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK || !strings.Contains(meRec.Body.String(), `"current_customer_name":"客户A"`) || !strings.Contains(meRec.Body.String(), customerportalapp.CapabilityDirectShip) {
		t.Fatalf("me status=%d body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestMiniMeRequiresToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{}})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/mini/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniCurrentCustomerPayload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: customerportalapp.CurrentContext{CurrentCustomerID: 9}}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/current-customer", strings.NewReader(`{"customer_id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got["current_customer_id"].(float64) != 9 {
		t.Fatalf("response=%v err=%v", got, err)
	}
}
```

- [ ] **Step 2: Run API tests and verify failure**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerportal -count=1
```

Expected: package does not compile because route files do not exist.

- [ ] **Step 3: Implement route module**

Create `orderapp-remote/internal/interfaces/http/customerportal/module.go`:

```go
package customerportal

import (
	"context"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type Service interface {
	Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error)
	Me(context.Context, string) (customerportalapp.CurrentContext, error)
	SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error)
}

type Dependencies struct {
	CustomerPortal Service
}

func RegisterRoutes(e *echo.Echo, deps Dependencies) {
	registerMiniAPI(e, deps.CustomerPortal)
}
```

- [ ] **Step 4: Implement mini API handlers**

Create `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`:

```go
package customerportal

import (
	"net/http"
	"strings"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniLoginRequest struct {
	Code     string `json:"code"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
}

type switchCustomerRequest struct {
	CustomerID int64 `json:"customer_id"`
}

func registerMiniAPI(e *echo.Echo, svc Service) {
	e.POST("/api/mini/login", func(c echo.Context) error {
		if svc == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "customer portal service required"})
		}
		var req miniLoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		result, err := svc.Login(c.Request().Context(), customerportalapp.LoginCommand{Code: req.Code, Phone: req.Phone, Nickname: req.Nickname})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.GET("/api/mini/me", func(c echo.Context) error {
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		result, err := svc.Me(c.Request().Context(), token)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})

	e.POST("/api/mini/current-customer", func(c echo.Context) error {
		token := miniTokenFromHeader(c.Request().Header.Get(echo.HeaderAuthorization))
		if token == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "mini token required"})
		}
		var req switchCustomerRequest
		if err := c.Bind(&req); err != nil || req.CustomerID <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "customer required"})
		}
		result, err := svc.SwitchCurrentCustomer(c.Request().Context(), token, req.CustomerID)
		if err != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	})
}

func miniTokenFromHeader(authz string) string {
	authz = strings.TrimSpace(authz)
	if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return strings.TrimSpace(authz[7:])
	}
	return ""
}
```

- [ ] **Step 5: Open `/api/mini/*` at BasicAuth boundary**

Modify `orderapp-remote/internal/interfaces/http/support/auth_middleware.go`, inside `isPublicUnauthenticatedPath`:

```go
return strings.HasPrefix(path, "/api/mini/") ||
	strings.HasPrefix(path, "/public/bean-list/") ||
	strings.HasPrefix(path, "/share/") ||
	strings.HasPrefix(path, "/assets/sales_order_assets/") ||
	path == "/vue-shell" ||
	strings.HasPrefix(path, "/vue-shell/")
```

Add this test case to `orderapp-remote/internal/interfaces/http/support/auth_middleware_test.go`:

```go
func TestMiniAPIBypassesBasicAuthForMiniTokenHandlers(t *testing.T) {
	if !isPublicUnauthenticatedPath("/api/mini/me") || !isPublicUnauthenticatedPath("/api/mini/login") {
		t.Fatal("/api/mini/* must bypass BasicAuth so mini handlers can enforce mini token auth")
	}
}
```

- [ ] **Step 6: Wire app routes**

Modify `orderapp-remote/internal/appmain/app_routes.go` imports:

```go
customerportalapp "orderapp/internal/application/customerportal"
postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"
customerportalhttp "orderapp/internal/interfaces/http/customerportal"
```

Add service construction in `registerAppRoutes`:

```go
customerPortalSvc := customerportalapp.NewService(
	postgrescustomerportal.NewRepository(pool, schema),
	customerportalhttp.StaticIdentityProvider{},
)
```

Register routes after support routes:

```go
customerportalhttp.RegisterRoutes(e, customerportalhttp.Dependencies{CustomerPortal: customerPortalSvc})
```

Add a temporary local identity provider in `orderapp-remote/internal/interfaces/http/customerportal/module.go` for P0 development:

```go
type StaticIdentityProvider struct{}

func (StaticIdentityProvider) Resolve(ctx context.Context, code string) (customerportalapp.MiniIdentity, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return customerportalapp.MiniIdentity{}, fmt.Errorf("code required")
	}
	return customerportalapp.MiniIdentity{OpenID: "dev-openid-" + code}, nil
}
```

Also add `fmt` and `strings` imports to `module.go`. Replace this provider with real WeChat `jscode2session` in a separate login hardening task after P0 shell works end-to-end.

- [ ] **Step 7: Verify API tests pass**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerportal -count=1
go test ./internal/interfaces/http/support -run 'TestMiniAPIBypassesBasicAuthForMiniTokenHandlers|TestAuthPublicPaths' -count=1
go test ./internal/appmain -run Test -count=1
```

Expected: all commands pass.

- [ ] **Step 8: Commit**

```bash
git add orderapp-remote/internal/interfaces/http/customerportal orderapp-remote/internal/interfaces/http/support/auth_middleware.go orderapp-remote/internal/interfaces/http/support/auth_middleware_test.go orderapp-remote/internal/appmain/app_routes.go
git commit -m "feat: add customer portal mini api"
```

### Task 5: Miniapp Shell

**Files:**
- Create: `miniapp/package.json`
- Create: `miniapp/vite.config.ts`
- Create: `miniapp/tsconfig.json`
- Create: `miniapp/src/main.ts`
- Create: `miniapp/src/App.vue`
- Create: `miniapp/src/pages.json`
- Create: `miniapp/src/manifest.json`
- Create: `miniapp/src/api/client.ts`
- Create: `miniapp/src/api/customerPortal.ts`
- Create: `miniapp/src/stores/session.ts`
- Create: `miniapp/src/utils/capabilities.ts`
- Create: `miniapp/src/utils/capabilities.test.ts`
- Create: `miniapp/src/pages/login/login.vue`
- Create: `miniapp/src/pages/home/home.vue`

- [ ] **Step 1: Create miniapp package and test**

Create `miniapp/package.json`:

```json
{
  "name": "kferp-miniapp",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev:mp-weixin": "uni -p mp-weixin",
    "build:mp-weixin": "uni build -p mp-weixin",
    "test": "vitest run"
  },
  "dependencies": {
    "@dcloudio/uni-app": "^3.0.0",
    "@dcloudio/uni-mp-weixin": "^3.0.0",
    "pinia": "^2.3.1",
    "vue": "^3.5.13"
  },
  "devDependencies": {
    "@dcloudio/vite-plugin-uni": "^3.0.0",
    "typescript": "^5.6.3",
    "vite": "^5.4.14",
    "vitest": "^2.1.8",
    "vue-tsc": "^2.2.0"
  }
}
```

Create `miniapp/src/utils/capabilities.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { visibleHomeEntries } from './capabilities'

describe('visibleHomeEntries', () => {
  it('shows only entries enabled by customer capabilities', () => {
    const entries = visibleHomeEntries([
      { code: 'direct_ship', enabled: true },
      { code: 'processing', enabled: false },
      { code: 'settlement', enabled: true },
    ])
    expect(entries.map((entry) => entry.key)).toEqual(['directShip', 'settlement'])
  })
})
```

- [ ] **Step 2: Run miniapp test and verify failure**

Run:

```bash
npm install --prefix miniapp
npm test --prefix miniapp
```

Expected: `FAIL` because `src/utils/capabilities.ts` does not exist.

- [ ] **Step 3: Add miniapp config files**

Create `miniapp/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

export default defineConfig({
  plugins: [uni()],
})
```

Create `miniapp/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "types": ["@dcloudio/types", "vitest/globals"]
  },
  "include": ["src/**/*.ts", "src/**/*.vue"]
}
```

Create `miniapp/src/pages.json`:

```json
{
  "pages": [
    {
      "path": "pages/login/login",
      "style": {
        "navigationBarTitleText": "客户登录"
      }
    },
    {
      "path": "pages/home/home",
      "style": {
        "navigationBarTitleText": "客户中心"
      }
    }
  ],
  "globalStyle": {
    "navigationBarTextStyle": "black",
    "navigationBarBackgroundColor": "#ffffff",
    "backgroundColor": "#f6f6f6"
  }
}
```

Create `miniapp/src/manifest.json`:

```json
{
  "name": "KFerp客户中心",
  "appid": "__UNI__KFERP",
  "description": "KFerp客户服务小程序",
  "versionName": "0.1.0",
  "versionCode": "100",
  "mp-weixin": {
    "appid": "",
    "setting": {
      "urlCheck": true
    },
    "usingComponents": true
  }
}
```

- [ ] **Step 4: Add app entry**

Create `miniapp/src/main.ts`:

```ts
import { createSSRApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'

export function createApp() {
  const app = createSSRApp(App)
  app.use(createPinia())
  return { app }
}
```

Create `miniapp/src/App.vue`:

```vue
<script setup lang="ts">
</script>

<style>
page {
  background: #f6f6f6;
  color: #171717;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

button {
  border-radius: 6px;
}
</style>
```

- [ ] **Step 5: Add capability helper**

Create `miniapp/src/utils/capabilities.ts`:

```ts
export type Capability = {
  code: string
  enabled: boolean
}

export type HomeEntry = {
  key: string
  label: string
  capability: string
}

const entries: HomeEntry[] = [
  { key: 'beanList', label: '我的豆单', capability: 'bean_list' },
  { key: 'productOrder', label: '现货下单', capability: 'product_order' },
  { key: 'directShip', label: '一件代发', capability: 'direct_ship' },
  { key: 'processing', label: '代加工', capability: 'processing' },
  { key: 'inventory', label: '我的库存', capability: 'inventory_custody' },
  { key: 'shipping', label: '物流查询', capability: 'shipping_query' },
  { key: 'settlement', label: '结算中心', capability: 'settlement' },
]

export function visibleHomeEntries(capabilities: Capability[] = []): HomeEntry[] {
  const enabled = new Set(capabilities.filter((item) => item.enabled).map((item) => item.code))
  return entries.filter((entry) => enabled.has(entry.capability))
}
```

- [ ] **Step 6: Add mini API client and store**

Create `miniapp/src/api/client.ts`:

```ts
const API_BASE = 'https://erp.qacoohee.com/app'

export type RequestOptions = {
  method?: 'GET' | 'POST'
  token?: string
  data?: unknown
}

export function miniRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    uni.request({
      url: `${API_BASE}${path}`,
      method: options.method || 'GET',
      data: options.data,
      header: {
        ...(options.token ? { Authorization: `Bearer ${options.token}` } : {}),
        'content-type': 'application/json',
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data as T)
          return
        }
        const body = res.data as { error?: string }
        reject(new Error(body?.error || `request failed: ${res.statusCode}`))
      },
      fail: (err) => reject(new Error(err.errMsg || 'network error')),
    })
  })
}
```

Create `miniapp/src/api/customerPortal.ts`:

```ts
import { miniRequest } from './client'
import type { Capability } from '../utils/capabilities'

export type CustomerBinding = {
  customer_id: number
  customer_name: string
  role: string
  status: string
}

export type LoginResponse = {
  token: string
  mini_user_id: number
  current_customer_id: number
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export type MeResponse = {
  mini_user_id: number
  current_customer_id: number
  current_customer_name: string
  bindings: CustomerBinding[]
  capabilities: Capability[]
}

export function loginWithCode(code: string): Promise<LoginResponse> {
  return miniRequest<LoginResponse>('/api/mini/login', { method: 'POST', data: { code } })
}

export function fetchMe(token: string): Promise<MeResponse> {
  return miniRequest<MeResponse>('/api/mini/me', { token })
}
```

Create `miniapp/src/stores/session.ts`:

```ts
import { defineStore } from 'pinia'
import type { Capability } from '../utils/capabilities'
import type { CustomerBinding } from '../api/customerPortal'

const tokenKey = 'kferp.mini.token'

export const useSessionStore = defineStore('session', {
  state: () => ({
    token: uni.getStorageSync(tokenKey) || '',
    miniUserID: 0,
    currentCustomerID: 0,
    currentCustomerName: '',
    bindings: [] as CustomerBinding[],
    capabilities: [] as Capability[],
  }),
  actions: {
    setToken(token: string) {
      this.token = token
      uni.setStorageSync(tokenKey, token)
    },
    applyContext(context: {
      mini_user_id: number
      current_customer_id: number
      current_customer_name?: string
      bindings: CustomerBinding[]
      capabilities: Capability[]
    }) {
      this.miniUserID = context.mini_user_id
      this.currentCustomerID = context.current_customer_id
      this.currentCustomerName = context.current_customer_name || ''
      this.bindings = context.bindings || []
      this.capabilities = context.capabilities || []
    },
  },
})
```

- [ ] **Step 7: Add login and home pages**

Create `miniapp/src/pages/login/login.vue`:

```vue
<template>
  <view class="page">
    <view class="panel">
      <text class="title">客户中心</text>
      <button type="primary" :loading="loading" @click="login">微信登录</button>
      <text v-if="error" class="error">{{ error }}</text>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { loginWithCode } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'

const loading = ref(false)
const error = ref('')
const session = useSessionStore()

function wxLoginCode(): Promise<string> {
  return new Promise((resolve, reject) => {
    uni.login({
      provider: 'weixin',
      success: (res) => resolve(res.code),
      fail: (err) => reject(new Error(err.errMsg || '微信登录失败')),
    })
  })
}

async function login() {
  loading.value = true
  error.value = ''
  try {
    const code = await wxLoginCode()
    const result = await loginWithCode(code)
    session.setToken(result.token)
    session.applyContext({
      mini_user_id: result.mini_user_id,
      current_customer_id: result.current_customer_id,
      bindings: result.bindings,
      capabilities: result.capabilities,
    })
    uni.redirectTo({ url: '/pages/home/home' })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.page { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 32rpx; }
.panel { width: 100%; display: grid; gap: 28rpx; padding: 32rpx; background: #fff; border: 1px solid #e5e5e5; border-radius: 8rpx; }
.title { font-size: 42rpx; font-weight: 700; }
.error { color: #b91c1c; font-size: 26rpx; }
</style>
```

Create `miniapp/src/pages/home/home.vue`:

```vue
<template>
  <view class="page">
    <view class="header">
      <text class="title">{{ session.currentCustomerName || '客户中心' }}</text>
      <text class="subtitle">已开通服务</text>
    </view>
    <view class="grid">
      <view v-for="entry in entries" :key="entry.key" class="entry">
        <text>{{ entry.label }}</text>
      </view>
      <view v-if="!entries.length" class="empty">当前账号还没有绑定客户或开通服务</view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { fetchMe } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import { visibleHomeEntries } from '../../utils/capabilities'

const session = useSessionStore()
const entries = computed(() => visibleHomeEntries(session.capabilities))

onMounted(async () => {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }
  const me = await fetchMe(session.token)
  session.applyContext(me)
})
</script>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; }
.header { display: grid; gap: 8rpx; margin-bottom: 24rpx; }
.title { font-size: 40rpx; font-weight: 800; }
.subtitle { color: #666; font-size: 26rpx; }
.grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16rpx; }
.entry { min-height: 132rpx; display: flex; align-items: center; padding: 22rpx; background: #fff; border: 1px solid #e5e5e5; border-radius: 8rpx; font-weight: 700; }
.empty { grid-column: 1 / -1; padding: 28rpx; color: #666; background: #fff; border: 1px solid #e5e5e5; border-radius: 8rpx; }
</style>
```

- [ ] **Step 8: Verify miniapp unit tests and build**

Run:

```bash
npm test --prefix miniapp
npm run build:mp-weixin --prefix miniapp
```

Expected: tests pass and `dist/build/mp-weixin` is generated.

- [ ] **Step 9: Commit**

```bash
git add miniapp
git commit -m "feat: scaffold customer miniapp shell"
```

### Task 6: Integration Verification

**Files:**
- No new files.

- [ ] **Step 1: Run focused Go tests**

Run:

```bash
cd orderapp-remote
go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal ./internal/interfaces/http/support ./internal/appmain -count=1
```

Expected: all selected packages pass.

- [ ] **Step 2: Run frontend tests and build**

Run:

```bash
npm test --prefix miniapp
npm run build:mp-weixin --prefix miniapp
```

Expected: miniapp test and mp-weixin build pass.

- [ ] **Step 3: Run full regression**

Run:

```bash
cd orderapp-remote
go test ./... -count=1
node --test frontend-vue-shell/src/lib/*.test.js frontend-vue-shell/src/api/*.test.js
npm run build --prefix frontend-vue-shell
git diff --check
```

Expected: all commands pass.

- [ ] **Step 4: Update PR/DEV/UT/API evidence**

In `orderapp-remote/internal/interfaces/http/support/req_store.go`, change P0 rows to:

```go
status: "done"
```

for `DEV-CUSTOMER-PORTAL-P0-*`, `UT-CUSTOMER-PORTAL-P0-01`, and `API-CUSTOMER-PORTAL-P0-01`, with evidence strings containing the exact commands from Steps 1-3.

Keep `PR-CUSTOMER-PORTAL-P0` as `review` and `REV-CUSTOMER-PORTAL-P0-01` as `todo` until Van accepts the feature.

- [ ] **Step 5: Commit verification evidence**

```bash
git add orderapp-remote/internal/interfaces/http/support/req_store.go
git commit -m "docs: record customer portal p0 verification"
```

## Execution Notes

- Work on branch `codex/customer-portal-p0-20260503`, branched from the latest `origin/develop`.
- Do not merge or deploy P0 until all Go tests, miniapp tests, miniapp build, Vue shell tests, Vue shell build, and `git diff --check` pass.
- Keep P1-P4 out of this branch. Add only the P0 shell and contracts needed for later stages.
- The `StaticIdentityProvider` is acceptable for P0 local integration only. Replace it with a real WeChat `jscode2session` provider in a follow-up login hardening plan before production mini-program release.

## Self-Review

- Spec coverage: P0 identity, binding, capabilities, `/api/mini/me`, miniapp shell, and PR/DEV/UT/API/REV are covered.
- Deliberately excluded: bean-list detail, order list, direct-ship import, processing job workflow, settlement generation, and real-time logistics. These are P1-P4.
- No fixed customer type enum is introduced; capabilities remain data-driven.
