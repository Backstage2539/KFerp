# Miniapp Password SKU Alias Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mask the miniapp ERP password field and make customer-only SKU aliases replace their base public SKU in customer service product selectors.

**Architecture:** Keep login masking in `miniapp/src/pages/login/login.vue`. Implement SKU replacement inside the existing PostgreSQL visibility SQL used by `Repository.listProducts`, so every miniapp service product selector gets the same behavior without page-specific logic.

**Tech Stack:** Go, pgx/PostgreSQL repository tests, Vue/uni-app miniapp, Vitest source guards.

---

### Task 1: Password Masking Source Guard

**Files:**
- Modify: `miniapp/src/utils/customerSwitch.test.ts`
- Modify: `miniapp/src/pages/login/login.vue`

- [ ] **Step 1: Write the failing test**

Update the login source guard to expect the WeChat-compatible boolean password prop:

```ts
expect(login).toContain('password placeholder="密码"')
expect(login).not.toContain('type="text" placeholder="密码"')
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test --prefix miniapp -- src/utils/customerSwitch.test.ts`

Expected: FAIL before implementation if the page only uses `type="password"`.

- [ ] **Step 3: Implement minimal page change**

Change the password input to:

```vue
<input v-model="loginForm.password" class="input" password placeholder="密码" />
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test --prefix miniapp -- src/utils/customerSwitch.test.ts`

Expected: PASS.

### Task 2: Customer SKU Alias Replacement

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`

- [ ] **Step 1: Write the failing repository test**

Add a repository test that seeds:

- customer A
- public base product `基础款曲奇`
- unrelated public product `公共保留款`
- customer A alias product `岩师傅兰卡`, `customer_id=A`, `visibility='customer_only'`, `base_product_id=基础款曲奇`

Then load the product order service page for customer A and assert:

```go
if strings.Contains(got, "基础款曲奇") {
    t.Fatalf("alias base product should be replaced for customer A: %q", got)
}
if !strings.Contains(got, "岩师傅兰卡") || !strings.Contains(got, "公共保留款") {
    t.Fatalf("products=%q missing alias or unrelated public product", got)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infrastructure/postgres/customerportal -run TestLoadProductServicePageReplacesBaseProductWithCustomerAlias -count=1`

Expected: FAIL because existing visibility SQL returns both public base and customer alias.

- [ ] **Step 3: Implement minimal SQL exclusion**

Update the product visibility SQL so a public product is hidden when the current customer has an active customer-only product whose `base_product_id` equals the public product id:

```sql
AND NOT (
  COALESCE(product.customer_id,0)=0
  AND EXISTS (
    SELECT 1 FROM products alias
    WHERE alias.active=true
      AND COALESCE(alias.customer_id,0)=customer_id
      AND COALESCE(alias.base_product_id,0)=product.id
      AND COALESCE(NULLIF(alias.visibility,''),'customer_only')='customer_only'
  )
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/infrastructure/postgres/customerportal -run TestLoadProductServicePageReplacesBaseProductWithCustomerAlias -count=1`

Expected: PASS.

### Task 3: Manual And Requirement Evidence

**Files:**
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_277_miniapp_password_sku_alias_test.go`

- [ ] **Step 1: Add PR-277 records**

Add PR/DEV/UT/API/REV rows for `PR-277-MINIAPP-PASSWORD-SKU-ALIAS`.

- [ ] **Step 2: Add support guard**

Add a Go support test asserting the records and manuals mention password masking and customer SKU replacement.

- [ ] **Step 3: Run support tests**

Run: `go test ./internal/interfaces/http/support -run TestDEV277MiniappPasswordSkuAliasRecords -count=1`

Expected: PASS after records and manuals are updated.

### Task 4: Full Verification, Merge, Deploy

- [ ] Run targeted Go tests:

```bash
go test ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support -count=1
```

- [ ] Run miniapp tests and typecheck:

```bash
npm test --prefix miniapp
npm run typecheck --prefix miniapp
VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp
```

- [ ] Run full backend tests:

```bash
cd orderapp-remote && go test ./... -count=1
```

- [ ] Push feature branch, merge latest `origin/develop`, push to `develop`, deploy development stack, and smoke test `/app/`, `/app/api/mini/me`, and product service behavior for 岩师傅.
