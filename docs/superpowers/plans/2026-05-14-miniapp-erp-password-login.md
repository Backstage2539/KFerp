# Miniapp ERP Password Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the miniapp user-facing login with ERP channel-customer username/password login and move account switching/logout into a personal center page.

**Architecture:** Add a miniapp-specific password login path that authenticates against ERP channel-customer accounts but still issues miniapp tokens and uses existing customer portal authorization. The repository maps an ERP employee account to a stable mini user shadow identity and synchronizes active customer ERP bindings into approved miniapp customer bindings. The miniapp removes the WeChat login button, adds an ERP password form, and centralizes switch/logout controls in `pages/profile/profile`.

**Tech Stack:** Go/Echo/pgx/PostgreSQL for backend APIs and repository; uni-app Vue 3 TypeScript Pinia for miniapp; Vitest, Go tests, miniapp build.

---

### Task 1: Backend Service Contract

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/application/customerportal/service_test.go`

- [ ] **Step 1: Write failing service tests**

Add tests proving `LoginWithPassword` trims credentials, rejects missing values, delegates to the repository, normalizes the login result, and propagates repository errors.

- [ ] **Step 2: Run service tests for red**

Run: `cd orderapp-remote && go test ./internal/application/customerportal -run 'TestServiceLoginWithPassword|TestServiceLoginWithPasswordRejectsMissingCredentials' -count=1`

Expected: FAIL because `LoginWithPassword` and `CreatePasswordLoginSession` do not exist.

- [ ] **Step 3: Implement minimal service contract**

Add `PasswordLoginCommand`, `CreatePasswordLoginSessionCommand`, extend `Repository`, and implement `Service.LoginWithPassword`.

- [ ] **Step 4: Run service tests for green**

Run: `cd orderapp-remote && go test ./internal/application/customerportal -run 'TestServiceLoginWithPassword|TestServiceLoginWithPasswordRejectsMissingCredentials' -count=1`

Expected: PASS.

### Task 2: Postgres Password Login

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Add integration tests for:
- active `channel_customer` with matching password and active customer ERP binding receives a mini token and `/me` context.
- internal employee with matching password is rejected.
- channel customer without active binding is rejected.
- disabled login is rejected.

- [ ] **Step 2: Run repository tests for red**

Run: `cd orderapp-remote && go test ./internal/infrastructure/postgres/customerportal -run 'TestCreatePasswordLoginSession' -count=1`

Expected: FAIL because repository password login does not exist.

- [ ] **Step 3: Implement repository login**

Add `CreatePasswordLoginSession`:
- resolve login by phone or employee name.
- verify `account_type='channel_customer'`, `active=true`, password hash, and `login_disabled=false`.
- require active `customer_erp_user_bindings` to active customers.
- create/update `mini_users.openid='erp-employee:<id>'`.
- upsert approved `customer_portal_user_bindings`.
- create `mini_sessions` and return the same `LoginResult` shape as `CreateLoginSession`.

- [ ] **Step 4: Run repository tests for green**

Run: `cd orderapp-remote && go test ./internal/infrastructure/postgres/customerportal -run 'TestCreatePasswordLoginSession' -count=1`

Expected: PASS.

### Task 3: HTTP Mini API

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/module.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests for `POST /api/mini/login/password` success, missing password 400, invalid login 401, internal employee/binding denial 403 via service errors.

- [ ] **Step 2: Run API tests for red**

Run: `cd orderapp-remote && go test ./internal/interfaces/http/customerportal -run 'TestMiniPasswordLogin' -count=1`

Expected: FAIL because route and service method do not exist.

- [ ] **Step 3: Implement API route and error mapping**

Add request struct `{login,password}`, extend the `Service` interface, register `POST /api/mini/login/password`, and map mini password login errors to 400/401/403.

- [ ] **Step 4: Run API tests for green**

Run: `cd orderapp-remote && go test ./internal/interfaces/http/customerportal -run 'TestMiniPasswordLogin' -count=1`

Expected: PASS.

### Task 4: Miniapp Login and Profile UI

**Files:**
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/api/customerPortal.test.ts`
- Modify: `miniapp/src/pages/login/login.vue`
- Modify: `miniapp/src/pages/home/home.vue`
- Modify: `miniapp/src/pages/mall/mall.vue`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/pages.json`
- Modify: `miniapp/src/utils/customerSwitch.test.ts`
- Create: `miniapp/src/pages/profile/profile.vue`

- [ ] **Step 1: Write failing miniapp tests**

Update Vitest source guards so they expect:
- `buildPasswordLoginPath()` returns `/api/mini/login/password`.
- login page calls `loginWithPassword`, contains username/password inputs, and does not contain `微信一键登录` or `uni.login`.
- `pages/profile/profile` is registered.
- `profile.vue` contains `切换用户`, `退出登录`, and customer switch handling.
- authenticated business pages contain `个人中心` and do not contain direct `退出登录`.

- [ ] **Step 2: Run miniapp tests for red**

Run: `npm test --prefix miniapp -- customerPortal.test.ts customerSwitch.test.ts`

Expected: FAIL because helpers and profile page do not exist and old login UI is still present.

- [ ] **Step 3: Implement miniapp UI**

Add password API helper, convert login page to ERP account form, add profile page, move customer switch/logout into profile, and make business pages navigate to profile.

- [ ] **Step 4: Run miniapp tests for green**

Run: `npm test --prefix miniapp -- customerPortal.test.ts customerSwitch.test.ts`

Expected: PASS.

### Task 5: Docs and Requirement Records

**Files:**
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/customer-portal-miniapp-test.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_273_miniapp_erp_password_login_test.go`

- [ ] **Step 1: Write failing requirement guard**

Add a source guard test that requires PR/DEV/UT/API/REV entries for miniapp ERP password login, operation manual references to ERP channel customer login, and acceptance evidence placeholders.

- [ ] **Step 2: Run support guard for red**

Run: `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestMiniappERPPasswordLoginRequirementRecords -count=1`

Expected: FAIL because requirement records and manual updates are missing.

- [ ] **Step 3: Update docs and requirement records**

Add PR-274 records and update miniapp operation/test manuals to describe ERP channel account login, personal center switch user, and no openid login UI.

- [ ] **Step 4: Run support guard for green**

Run: `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestMiniappERPPasswordLoginRequirementRecords -count=1`

Expected: PASS.

### Task 6: Full Verification, Merge, Deploy

**Files:**
- All changed files.

- [ ] **Step 1: Run focused backend tests**

Run: `cd orderapp-remote && go test ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal ./internal/interfaces/http/support -count=1`

- [ ] **Step 2: Run miniapp tests and build**

Run:
- `npm test --prefix miniapp`
- `npm run typecheck --prefix miniapp`
- `VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp`

- [ ] **Step 3: Update branch from latest develop**

Run:
- `git fetch origin`
- `git merge origin/develop`
- rerun focused tests if merge changes anything relevant.

- [ ] **Step 4: Commit, push feature branch, merge into develop**

Run:
- `git add <changed files>`
- `git commit -m "feat: add miniapp ERP password login"`
- `git push -u origin codex/miniapp-erp-password-login`
- `git switch develop`
- `git pull --ff-only origin develop`
- `git merge --no-ff codex/miniapp-erp-password-login`
- `git push origin develop`

- [ ] **Step 5: Deploy development stack**

Run:
- `git fetch origin`
- `git log --oneline -3 origin/develop`
- `git rev-parse origin/develop`
- `./deploy_orderapp.sh development`

- [ ] **Step 6: Postdeploy smoke**

Run read-only checks:
- unauthenticated `/app/` returns BasicAuth 401.
- unauthenticated `/app/api/mini/me` returns JSON 401.
- `POST /app/api/mini/login/password` with invalid credentials returns an expected non-500 JSON error.
