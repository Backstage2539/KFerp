# Miniapp Main Tabs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the miniapp blank startup route and replace top account links with four bottom main entries: 首页, 订单, 账单, 我的.

**Architecture:** Add a first-route startup page that normalizes login routing with `uni.reLaunch`. Add a reusable `MainTabBar` component and mount it on authenticated top-level pages. Reuse the existing service page and backend APIs for order and settlement data.

**Tech Stack:** uni-app + Vue 3 + Pinia miniapp, Vitest source/unit tests, Go support evidence tests.

---

### Task 1: Startup Route

**Files:**
- Create: `miniapp/src/pages/index/index.vue`
- Modify: `miniapp/src/pages.json`
- Test: `miniapp/src/utils/mainTabs.test.ts`

- [ ] **Step 1: Write the failing test**

Add a Vitest source test asserting that `pages/index/index` is first in `pages.json`, the startup page calls `uni.reLaunch`, routes no-token users to login, and routes token users to home.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test --prefix miniapp -- src/utils/mainTabs.test.ts`

Expected: FAIL because `mainTabs.test.ts` and `pages/index/index.vue` do not exist yet.

- [ ] **Step 3: Implement startup page**

Create `pages/index/index.vue` with a minimal loading state and `onShow` redirect logic. Put it first in `pages.json`.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test --prefix miniapp -- src/utils/mainTabs.test.ts`

Expected: PASS for startup route checks.

### Task 2: Bottom Four Entries

**Files:**
- Create: `miniapp/src/components/MainTabBar.vue`
- Modify: `miniapp/src/pages/home/home.vue`
- Modify: `miniapp/src/pages/mall/mall.vue`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/pages/profile/profile.vue`
- Modify: `miniapp/src/utils/capabilities.ts`
- Test: `miniapp/src/utils/mainTabs.test.ts`
- Test: `miniapp/src/utils/capabilities.test.ts`
- Test: `miniapp/src/utils/customerSwitch.test.ts`

- [ ] **Step 1: Extend failing tests**

Assert that authenticated pages import/use `MainTabBar`, do not show the top `个人中心` button, and that `visibleHomeEntries` excludes `orders` and `settlement`.

- [ ] **Step 2: Run tests to verify failure**

Run: `npm test --prefix miniapp -- src/utils/mainTabs.test.ts src/utils/capabilities.test.ts src/utils/customerSwitch.test.ts`

Expected: FAIL because the bottom component is missing and home still includes order/settlement entries.

- [ ] **Step 3: Implement bottom nav**

Add `MainTabBar.vue` with four entries:

- 首页 -> `/pages/home/home`
- 订单 -> `/pages/service/service?key=orders`
- 账单 -> `/pages/service/service?key=settlement`
- 我的 -> `/pages/profile/profile`

Use `uni.reLaunch` and a `current` prop. Mount it on home, mall, service, and profile pages. Remove top profile buttons. Add bottom padding to page containers.

- [ ] **Step 4: Keep home focused**

Filter `orders` and `settlement` out of `visibleHomeEntries`. Keep product ordering, bean list, mall, direct ship, processing, and inventory entries as current service cards.

- [ ] **Step 5: Run tests to verify pass**

Run: `npm test --prefix miniapp -- src/utils/mainTabs.test.ts src/utils/capabilities.test.ts src/utils/customerSwitch.test.ts`

Expected: PASS.

### Task 3: Docs And Support Evidence

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_278_miniapp_main_tabs_test.go`

- [ ] **Step 1: Write support test**

Add a Go source evidence test checking `PR-278-MINIAPP-MAIN-TABS`, miniapp files, and manual docs mention the four bottom entries and startup route.

- [ ] **Step 2: Run test to verify failure**

Run: `go test ./internal/interfaces/http/support -run TestDev278MiniappMainTabs -count=1`

Expected: FAIL before docs and req store records are added.

- [ ] **Step 3: Update docs and records**

Add PR/DEV/UT/API/REV rows and update manuals/acceptance docs with the new miniapp operation flow.

- [ ] **Step 4: Run support test**

Run: `go test ./internal/interfaces/http/support -run TestDev278MiniappMainTabs -count=1`

Expected: PASS.

### Task 4: Verification, Merge, Deploy

**Files:**
- All changed files from Tasks 1-3.

- [ ] **Step 1: Run targeted checks**

Run:

```bash
npm test --prefix miniapp -- src/utils/mainTabs.test.ts src/utils/capabilities.test.ts src/utils/customerSwitch.test.ts
go test ./internal/interfaces/http/support -run TestDev278MiniappMainTabs -count=1
```

- [ ] **Step 2: Run full checks**

Run:

```bash
npm test --prefix miniapp
npm run typecheck --prefix miniapp
VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp
cd orderapp-remote && go test ./... -count=1
git diff --check
```

- [ ] **Step 3: Commit and push feature branch**

Commit with `fix: add miniapp startup and main tabs`, push `codex/miniapp-main-tabs-20260515`.

- [ ] **Step 4: Merge and deploy**

Fetch latest `origin/develop`, fast-forward/merge cleanly into `develop`, push `develop`, run `./deploy_orderapp.sh`, and smoke test the deployed development stack.
