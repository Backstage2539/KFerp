# Green Bean List Manual Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make green bean transaction prices come only from green bean-list versions, with green bean-list prices defaulting from BOM cost snapshots and remaining manually editable per template tier.

**Architecture:** Keep the existing products/BOM/bean-list/order tables. Add a BOM item material cost snapshot, compute green bean cost reference from the bound roasted BOM, expose green tier manual price overrides in the Vue bean-list editor, and make order pricing resolve the selected publication per product kind.

**Tech Stack:** Go, PostgreSQL, Vue 3, Vite, Node test runner.

---

### Task 1: BOM Cost Snapshot And Green Tier Defaults

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`
- Test: `orderapp-remote/internal/application/costing/service_test.go`

- [ ] **Step 1: Write failing tests**

Add tests that assert:

```go
func TestLoadProductInputsUsesBomMaterialCostSnapshotForGreenBeanCost(t *testing.T) {
	source := readFile(t, "repository.go")
	for _, want := range []string{
		"material_unit_cost_snapshot",
		"COALESCE(NULLIF(bi.material_unit_cost_snapshot,0), m.purchase_price, 0)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("green bean costing must use BOM material cost snapshot; missing %q", want)
		}
	}
	if strings.Contains(source, "mv.weighted_unit_cost, m.purchase_price") {
		t.Fatalf("green bean costing must not prefer inventory weighted cost over BOM snapshot")
	}
}

func TestGreenBeanTemplateSaleTiersDefaultToCostReferenceWithoutMargin(t *testing.T) {
	input := domain.ProductInput{
		ProductID: 909,
		Name: "兰卡拼配生豆",
		ProductKind: "green_bean",
		GreenBeanCostPerKg: 60,
		GradientTemplate: &domain.GradientTemplate{
			ID: 7,
			Name: "生豆模板",
			DisplayUnit: domain.GradientDisplayUnitLb,
			Tiers: []domain.GradientTemplateTier{{
				ID: 71, Label: "24-49lb", MinWeightG: 10896, MaxWeightG: floatPtr(22226), MarginRate: 0.5, Position: 1,
			}},
		},
	}
	resp, err := NewService(&fakeRepo{inputs: []domain.ProductInput{input}}).BeanList(context.Background())
	if err != nil {
		t.Fatalf("BeanList() error = %v", err)
	}
	tier := resp.Items[0].GreenBeanSaleTiers[0]
	if tier.PricePerKg != 60 || tier.PricePerUnit != 27.24 || tier.MarginRate != 0 {
		t.Fatalf("green tier should default to cost only, got %+v", tier)
	}
}
```

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/costing ./internal/application/costing -run 'TestLoadProductInputsUsesBomMaterialCostSnapshotForGreenBeanCost|TestGreenBeanTemplateSaleTiersDefaultToCostReferenceWithoutMargin' -count=1
```

Expected: FAIL because snapshot column and no-margin behavior are missing.

- [ ] **Step 2: Implement minimal code**

Add `material_unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0` to `product_bom_items`; backfill from `materials.purchase_price`. In `SaveItem`, when saving a material component, select the current material purchase price and write it to `material_unit_cost_snapshot`.

Change costing SQL so material BOM cost uses:

```sql
COALESCE(NULLIF(bi.material_unit_cost_snapshot,0), m.purchase_price, 0)
```

Change `buildGreenBeanTemplateSaleTiers` so green bean default sale price is the cost reference converted to the template display unit, with `MarginRate` set to `0`.

- [ ] **Step 3: Verify**

Run the same targeted Go tests. Expected: PASS.

### Task 2: Green Bean Manual Price Overrides In Bean-List Content

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/costing-bean-list-version-ui.test.js`

- [ ] **Step 1: Write failing tests**

Add tests that assert `buildBeanListPdfGroups` applies per-tier `greenPriceOverrides` for list type `green`, and that `CostingView.vue` contains green bean price input controls bound through `setGreenBeanTierPrice`.

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js
```

Expected: FAIL because the override controls and transformation are missing.

- [ ] **Step 2: Implement minimal code**

Store manual prices in `pdfCustomizers[productID].greenPriceOverrides[templateTierID or label]`. Add a compact green-only pricing editor in the product picker rows. `buildBeanListPdfGroups` must copy green tiers and replace `price_per_unit` when a manual price exists, leaving template tier ranges unchanged.

- [ ] **Step 3: Verify**

Run the same Node tests. Expected: PASS.

### Task 3: Order Form Bean-List Versions Per Product Kind

**Files:**
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] **Step 1: Write failing tests**

Add tests that assert order form version options include `list_type`, return defaults for `commercial`, `green`, and `drip`, and `buildOrderPayload` sends `commercial_bean_list_publication_id`, `green_bean_list_publication_id`, and `drip_bean_list_publication_id`.

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsBeanListVersionsByType' -count=1
cd frontend-vue-shell
node --test src/lib/order-entry.test.js
```

Expected: FAIL because the fields and selectors do not exist.

- [ ] **Step 2: Implement minimal code**

Add `ListType` to `BeanListVersionOption`. Update the order form query to partition latest customer/public fallback by `(customer_id, list_type)`. Add request/command fields for the three selected publication IDs. Update Vue to render compact selectors for 熟豆豆单、生豆豆单、挂耳豆单 and send the three IDs.

- [ ] **Step 3: Verify**

Run the same Go and Node tests. Expected: PASS.

### Task 4: Remove Bound Roasted Price Fallback And Enforce Green Bean-List Pricing

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/orderbeans/usage.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries_static_test.go`

- [ ] **Step 1: Write failing tests**

Replace the previous bound-roasted tests with:

```go
func TestOrderAPIFormDoesNotReturnBoundRoastedTiersForGreenBeanProduct(t *testing.T)
func TestOrderAPISavesGreenBeanOrderRequiresPublishedGreenBeanListPrice(t *testing.T)
func TestOrderAPISavesGreenBeanOrderUsingSelectedGreenBeanListVersion(t *testing.T)
```

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales -run 'GreenBeanOrder|BoundRoasted|SelectedGreenBeanList' -count=1
```

Expected: FAIL because the fallback still exists.

- [ ] **Step 2: Implement minimal code**

Remove the `green_bound_tiers` CTE and `greenBeanOrderPriceProductIDTx` / `greenBeanBoundRoastedTierPriceSourceJSON`. Add `ResolvePublishedUnitPriceFromPublication` so order saving uses the selected green publication when provided. If a green row has no published price, return a validation error such as `missing green bean list price`.

- [ ] **Step 3: Verify**

Run the same Go tests. Expected: PASS.

### Task 5: Documentation, PR/DEV, Acceptance, And Deployment

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_GREEN_BEAN_SALES.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify matching files under `orderapp-remote/docs/`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `docs/acceptance/2026-05-19-green-bean-list-manual-pricing.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-19-green-bean-list-manual-pricing.md`

- [ ] **Step 1: Update docs**

Remove all statements saying green bean orders fall back to bound roasted tiers. Add the new operating rule: green bean list price only, BOM snapshot cost reference, manual tier price edit, per-kind order bean-list selection.

- [ ] **Step 2: Run full verification**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js src/api/*.test.js
npm run build
cd ..
go test ./...
git diff --check
```

Expected: PASS.

- [ ] **Step 3: Integrate and deploy**

Push feature branch, merge latest `origin/develop` into the feature branch, rerun verification, merge into `develop`, push `develop`, deploy development stack, and smoke test:

```bash
ssh root@1.12.242.58 "cd /opt/stacks/erp && docker compose ps"
ssh root@1.12.242.58 "docker logs --tail=200 erp_orderapp"
```

Expected: development stack is up and authenticated order form / costing endpoints return 200.
