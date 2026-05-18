# Drip Product Kind Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add coffee drip bags as a real product kind with bag/box ordering, separate pricing, BOM based on roasted-bean inventory, dedicated bean lists, miniapp ordering, mall ordering, operation logs, manuals, and acceptance evidence.

**Architecture:** Add a lightweight product-kind and sales-unit foundation, then implement `drip_bag` as the first non-roasted-bean product kind. Keep formulas in typed Go domain code and persist published price snapshots for every transaction path. Extend BOM components to support materials and finished products, so drip production can consume roasted-bean inventory and request upstream roasting when short.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, Vue 3 + Vite, uni-app miniapp, Node test runner, Vitest, existing operation-log and manual infrastructure.

---

## File Structure

- Create: `orderapp-remote/internal/domain/catalog/product_kind.go`
  - Product-kind constants and validation helpers.
- Create: `orderapp-remote/internal/domain/sales/unit_pricing.go`
  - Unit-aware price matching and line-total helpers for `bag`, `box`, and legacy weight products.
- Modify: `orderapp-remote/internal/domain/sales/pricing_test.go`
  - Unit tests for bag and box pricing.
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
  - Add explicit drip wholesale tier results and Excel-matching formula nodes.
- Modify: `orderapp-remote/internal/domain/costing/engine_test.go`
  - Unit tests for drip formula and quantity tiers.
- Modify: `orderapp-remote/internal/application/catalog/service.go`
  - Product settings DTOs and commands for `product_kind`, drip unit config, and sales-channel flags.
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
  - Product columns, default product kind backfill, and price-tier metadata columns.
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
  - Product settings read/write support for drip fields.
- Modify: `orderapp-remote/internal/interfaces/http/catalog/products_json.go`
  - Product JSON includes product kind and sales-unit options for order forms.
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`
  - API tests for creating and reading drip products.
- Modify: `orderapp-remote/internal/application/bom/service.go`
  - BOM component DTOs for `material` and `finished_product`.
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/schema.go`
  - BOM component columns and indexes.
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/repository.go`
  - Save/read BOM components with material or finished-product source.
- Modify: `orderapp-remote/internal/interfaces/http/bom/bom_api.go`
  - BOM API request/response fields for component type and unit consumption.
- Modify: `orderapp-remote/frontend-vue-shell/src/views/BomView.vue`
  - UI for selecting material components or roasted-bean product components.
- Create: `orderapp-remote/frontend-vue-shell/src/lib/drip-product.js`
  - Frontend helpers for drip unit labels, unit conversions, and product form validation.
- Create: `orderapp-remote/frontend-vue-shell/src/lib/drip-product.test.js`
  - Node tests for drip helper behavior.
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
  - Drip price-template tables and default seed parameters.
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
  - Load, save, and publish drip prices to transaction price tiers.
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
  - Drip price-template and explanation endpoints.
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`
  - API tests for drip templates, publish, and explanations.
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/schema.go`
  - Order-item snapshot columns for product kind, sales unit, unit conversion, and price source.
- Modify: `orderapp-remote/internal/application/sales/service.go`
  - Save-order command carries sales-unit metadata.
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
  - Unit-aware price lookup, save, audit, and totals.
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
  - ERP order JSON accepts drip bag/box lines.
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_pricing_test.go`
  - Pricing tests for drip bag and box orders.
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
  - API-level order tests.
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
  - Fulfillment-customer order price matching for drip products.
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`
  - Customer fulfillment API exposes drip unit choices and accepts drip order lines.
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go`
  - API tests for fulfillment-customer drip ordering.
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
  - Miniapp service DTOs carry product kind, sales units, and unit specs.
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
  - Miniapp fulfillment and mall orders save drip unit snapshots.
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
  - Miniapp product and order endpoints support drip products.
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/bean_list_pdf.go`
  - Permit and render `drip` bean-list snapshots.
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`
  - API tests for miniapp drip service and mall orders.
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/plan_queries.go`
  - Split drip demand from roasted-bean demand and create upstream roast shortages.
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/material_consumption.go`
  - Consume finished-product components from finished inventory and material components from material inventory.
- Modify: `orderapp-remote/internal/interfaces/http/production/produce_plan_api_test.go`
  - API tests for drip production demand and roasted-bean shortage.
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
  - Product-kind selection and drip fields.
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
  - Drip price template UI, explanation, and drip bean-list publishing entry.
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
  - Bag/box order entry for drip products.
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue`
  - Product-kind and sales-unit display.
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
  - Unit-aware line payload and total helpers.
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`
  - Frontend order helper tests for drip lines.
- Modify: `miniapp/src/api/customerPortal.ts`
  - Miniapp API types for drip product metadata and order payloads.
- Modify: `miniapp/src/utils/servicePage.ts`
  - Fulfillment order helper support for bag/box products.
- Modify: `miniapp/src/utils/servicePage.test.ts`
  - Vitest cases for drip fulfillment payloads.
- Modify: `miniapp/src/utils/mall.ts`
  - Mall helper support for drip products.
- Modify: `miniapp/src/utils/mall.test.ts`
  - Vitest cases for drip mall products.
- Modify: `miniapp/src/pages/service/service.vue`
  - Drip unit selector in fulfillment customer ordering.
- Modify: `miniapp/src/pages/mall/mall.vue`
  - Drip product display and unit selection.
- Modify: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
  - Drip pricing template workflow and formula source.
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
  - BOM finished-product component workflow.
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
  - ERP bag/box drip order workflow.
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
  - Fulfillment customer drip order workflow.
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
  - Miniapp and mall drip ordering.
- Modify: `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
  - Drip production and upstream roast shortage workflow.
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js`
  - Manual entry coverage.
- Create: `orderapp-remote/docs/acceptance/2026-05-18-drip-product-kind.md`
  - Acceptance evidence checklist.

---

### Task 1: Product Kind And Unit Pricing Domain

**Files:**
- Create: `orderapp-remote/internal/domain/catalog/product_kind.go`
- Create: `orderapp-remote/internal/domain/sales/unit_pricing.go`
- Modify: `orderapp-remote/internal/domain/sales/pricing_test.go`
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Modify: `orderapp-remote/internal/domain/costing/engine_test.go`

- [ ] **Step 1: Add failing unit tests for drip unit pricing**

Add tests to `orderapp-remote/internal/domain/sales/pricing_test.go`:

```go
func TestDripBagLineUsesBagTier(t *testing.T) {
	tier := UnitPriceTier{
		ProductKind: "drip_bag",
		SalesUnit:   "bag",
		MinQty:      100,
		PricePerUnit: 2.15,
		UnitBagCount: 1,
	}
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind: "drip_bag",
		SalesUnit: "bag",
		Quantity: 120,
		UnitBagCount: 1,
		Tiers: []UnitPriceTier{tier},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.UnitPrice != 2.15 || got.LineTotal != 258 {
		t.Fatalf("got unit price %.2f total %.2f", got.UnitPrice, got.LineTotal)
	}
}

func TestDripBoxLineMatchesTierByConvertedBags(t *testing.T) {
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind: "drip_bag",
		SalesUnit: "box",
		Quantity: 12,
		UnitBagCount: 10,
		Tiers: []UnitPriceTier{{
			ProductKind: "drip_bag",
			SalesUnit: "box",
			MinQty: 10,
			PricePerUnit: 21.50,
			UnitBagCount: 10,
		}},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.MatchedQtyForTier != 120 || got.LineTotal != 258 {
		t.Fatalf("got matched %.0f total %.2f", got.MatchedQtyForTier, got.LineTotal)
	}
}
```

- [ ] **Step 2: Run the failing domain tests**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/sales -run 'TestDrip.*Line' -count=1
```

Expected: FAIL because `UnitPriceTier`, `UnitLineInput`, and `CalculateUnitLineTotal` do not exist.

- [ ] **Step 3: Add product-kind constants and unit-pricing implementation**

Create `orderapp-remote/internal/domain/catalog/product_kind.go` with:

```go
package catalog

const (
	ProductKindRoastedBean = "roasted_bean"
	ProductKindDripBag     = "drip_bag"
)

func NormalizeProductKind(kind string) string {
	if kind == ProductKindDripBag {
		return ProductKindDripBag
	}
	return ProductKindRoastedBean
}
```

Create `orderapp-remote/internal/domain/sales/unit_pricing.go` with typed structs for `UnitPriceTier`, `UnitLineInput`, and `UnitLineResult`. Implement:

```go
func CalculateUnitLineTotal(in UnitLineInput) (UnitLineResult, error)
```

Rules:

- `drip_bag + bag`: tier quantity is entered bag quantity.
- `drip_bag + box`: tier quantity is `quantity * unit_bag_count`.
- box price is per box; do not divide by 454g or 1000g.
- legacy products keep the existing weight-based behavior through a helper named `CalculateLegacyWeightLineTotal`.

- [ ] **Step 4: Add failing drip costing tests from the Excel formula**

Add tests to `orderapp-remote/internal/domain/costing/engine_test.go`:

```go
func TestDripWholesaleTiersMatchExcelFormula(t *testing.T) {
	params := DefaultParameters()
	params.RetailTaxRate = 0.03
	params.DripGreenRatioKgPerBag = 0.01
	params.DripProcessCostPerBag = 0.44
	params.DripExtraCostPerBag = 0.10
	params.DripPackingMaterialPerBag = 0.20
	params.WholesaleDripMultipliers = []float64{2.2, 1.8, 1.6, 1.35}

	out := CalculateProduct(params, ProductInput{
		Name: "挂耳测试",
		GreenBeanCostPerKg: 80,
		YieldRate: 0.8,
	})
	if len(out.DripWholesaleTiers) != 4 {
		t.Fatalf("tiers len=%d", len(out.DripWholesaleTiers))
	}
	base := (80/0.8 + params.SmallBatchProductionCostPerKg) * 0.01 + 0.44 + 0.10
	wantLoose := roundPrice(base*2.2 + base*(2.2-1)*0.03)
	wantPacked := roundPrice(wantLoose + 0.20)
	if out.DripWholesaleTiers[0].MinBags != 100 || out.DripWholesaleTiers[0].LoosePricePerBag != wantLoose || out.DripWholesaleTiers[0].PackedPricePerBag != wantPacked {
		t.Fatalf("first tier=%+v want loose %.2f packed %.2f", out.DripWholesaleTiers[0], wantLoose, wantPacked)
	}
}
```

- [ ] **Step 5: Implement explicit drip wholesale tier results**

Modify `orderapp-remote/internal/domain/costing/engine.go`:

- Add `DripWholesaleTier`.
- Add `ProductResult.DripWholesaleTiers`.
- Generate tiers at `100 / 1000 / 5000 / 10000` bags.
- Calculate tax as `base * (multiplier - 1) * RetailTaxRate`.
- Keep the existing `WholesaleDripBagPrices` arrays populated for backward UI compatibility until the Vue page is updated.

- [ ] **Step 6: Run and commit domain work**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/sales ./internal/domain/costing -count=1
git diff --check
git add internal/domain/catalog/product_kind.go internal/domain/sales/unit_pricing.go internal/domain/sales/pricing_test.go internal/domain/costing/engine.go internal/domain/costing/engine_test.go
git commit -m "feat: add drip product pricing domain"
```

Expected: tests PASS, commit created.

---

### Task 2: Catalog Product Kind Schema And Product Settings API

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/products_json.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_154_product_settings_create_product_test.go`

- [ ] **Step 1: Add failing API tests for drip product creation and readback**

In `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`, add a test that creates a product with:

```json
{
  "name": "耶加雪菲挂耳",
  "product_kind": "drip_bag",
  "drip_bag_grams": 10,
  "drip_box_bag_count": 10,
  "allow_fulfillment_order": true,
  "allow_mall_order": true
}
```

Assert the product settings response contains:

- `product_kind: "drip_bag"`
- `drip_bag_grams: 10`
- `drip_box_bag_count: 10`
- `sales_units: ["bag","box"]`

- [ ] **Step 2: Run the failing API test**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/catalog -run TestProductSettings.*Drip -count=1
```

Expected: FAIL because the fields are not persisted or returned.

- [ ] **Step 3: Add schema columns and defaults**

Modify `catalog.EnsureSchema`:

```sql
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_box_bag_count INT NOT NULL DEFAULT 10;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS allow_fulfillment_order BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS allow_mall_order BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS products_kind_active_idx ON %[1]s.products(product_kind, active);
UPDATE %[1]s.products SET product_kind='roasted_bean' WHERE COALESCE(product_kind,'')='';
```

- [ ] **Step 4: Extend application DTOs and repository scans**

Add these fields to `catalog.Product`, `catalog.ProductSettingsProduct`, and create/update commands:

```go
ProductKind string
DripBagGrams float64
DripBoxBagCount int
AllowFulfillmentOrder bool
AllowMallOrder bool
SalesUnits []string
```

Repository readback rule:

- `roasted_bean`: `SalesUnits` can be empty or legacy.
- `drip_bag`: `SalesUnits` must be `[]string{"bag", "box"}`.

- [ ] **Step 5: Extend HTTP product settings requests**

Modify `orderapp-remote/internal/interfaces/http/catalog/product_routes.go` to bind and validate the new fields:

- `product_kind` defaults to `roasted_bean`.
- `drip_bag_grams` defaults to `10` when product kind is `drip_bag`.
- `drip_box_bag_count` defaults to `10` when product kind is `drip_bag`.
- Reject `drip_bag_grams <= 0` or `drip_box_bag_count <= 0`.

- [ ] **Step 6: Preserve order-form product JSON compatibility**

Modify `products_json.go` so each product includes:

```json
"product_kind": "drip_bag",
"sales_units": ["bag", "box"],
"drip_bag_grams": 10,
"drip_box_bag_count": 10
```

Existing frontend consumers must continue to work when these fields are absent for old cached pages.

- [ ] **Step 7: Run and commit catalog work**

Run:

```bash
cd orderapp-remote
go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -count=1
git diff --check
git add internal/application/catalog internal/infrastructure/postgres/catalog internal/interfaces/http/catalog internal/interfaces/http/support/dev_154_product_settings_create_product_test.go
git commit -m "feat: add product kind settings"
```

Expected: tests PASS, commit created.

---

### Task 3: BOM Finished-Product Components

**Files:**
- Modify: `orderapp-remote/internal/application/bom/service.go`
- Modify: `orderapp-remote/internal/application/bom/service_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/bom/bom_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/bom/bom_sku_context_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/bom/bom_material_search_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/BomView.vue`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/drip-product.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/drip-product.test.js`

- [ ] **Step 1: Add failing service tests for BOM component validation**

Add tests to `orderapp-remote/internal/application/bom/service_test.go`:

```go
func TestSaveFinishedProductComponentRequiresComponentProduct(t *testing.T) {
	svc := NewService(fakeBomRepo{})
	err := svc.SaveItem(context.Background(), SaveItemCommand{
		ProductID: 1,
		ComponentType: "finished_product",
		ConsumeUnit: "g_per_bag",
		QtyPerUnit: 10,
	})
	if err == nil || !strings.Contains(err.Error(), "component_product_id required") {
		t.Fatalf("expected component_product_id error, got %v", err)
	}
}

func TestSaveMaterialComponentKeepsLegacyMaterialValidation(t *testing.T) {
	svc := NewService(fakeBomRepo{})
	err := svc.SaveItem(context.Background(), SaveItemCommand{
		ProductID: 1,
		ComponentType: "material",
		MaterialID: 2,
		ConsumeUnit: "unit_per_bag",
		QtyPerUnit: 1,
	})
	if err != nil {
		t.Fatalf("SaveItem: %v", err)
	}
}
```

- [ ] **Step 2: Run the failing BOM service tests**

Run:

```bash
cd orderapp-remote
go test ./internal/application/bom -run TestSave.*Component -count=1
```

Expected: FAIL because component fields do not exist.

- [ ] **Step 3: Extend BOM item schema**

Modify `orderapp-remote/internal/infrastructure/postgres/bom/schema.go`:

```sql
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_type TEXT NOT NULL DEFAULT 'material';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS component_spec_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS consume_unit TEXT NOT NULL DEFAULT 'ratio_pct';
ALTER TABLE %[1]s.product_bom_items ADD COLUMN IF NOT EXISTS qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0;
UPDATE %[1]s.product_bom_items SET component_type='material' WHERE COALESCE(component_type,'')='';
CREATE INDEX IF NOT EXISTS product_bom_items_component_product_idx ON %[1]s.product_bom_items(component_type, component_product_id);
```

- [ ] **Step 4: Extend BOM DTOs and validation**

Update `bom.Item` and `SaveItemCommand`:

```go
ComponentType string `json:"component_type"`
ComponentProductID int64 `json:"component_product_id"`
ComponentSpecG int64 `json:"component_spec_g"`
ConsumeUnit string `json:"consume_unit"`
QtyPerUnit float64 `json:"qty_per_unit"`
```

Validation:

- `material` requires `material_id > 0`.
- `finished_product` requires `component_product_id > 0`.
- `consume_unit` allowed values: `ratio_pct`, `g_per_bag`, `unit_per_bag`, `unit_per_box`.
- `qty_per_unit > 0` for non-legacy consume units.
- legacy `ratio_pct` keeps the existing `(0,100]` rule.

- [ ] **Step 5: Update BOM repository and API**

Modify repository `Detail`, `SaveItem`, version copy, and delete logic so new component columns round-trip. Modify `bom_api.go` request structs and responses to expose the same fields.

- [ ] **Step 6: Add frontend helper tests**

Create `orderapp-remote/frontend-vue-shell/src/lib/drip-product.test.js`:

```js
import test from 'node:test'
import assert from 'node:assert/strict'
import { dripUnitOptions, validateDripProduct } from './drip-product.js'

test('drip unit options expose bag and box', () => {
  assert.deepEqual(dripUnitOptions({ drip_box_bag_count: 10 }), [
    { value: 'bag', label: '袋', spec: '10g/袋' },
    { value: 'box', label: '盒', spec: '10袋/盒' }
  ])
})

test('validate drip product requires positive bag grams and box count', () => {
  assert.deepEqual(validateDripProduct({ product_kind: 'drip_bag', drip_bag_grams: 0, drip_box_bag_count: 10 }), ['每袋熟豆克重必须大于 0'])
})
```

- [ ] **Step 7: Implement frontend helpers and update `BomView.vue`**

Create `drip-product.js` with named exports:

- `isDripProduct(product)`
- `dripUnitOptions(product)`
- `validateDripProduct(product)`
- `componentTypeLabel(type)`

Update `BomView.vue` so component rows can choose:

- Material component: material picker and material consumption unit.
- Finished product component: roasted-bean product picker and `g_per_bag` consumption.

- [ ] **Step 8: Run and commit BOM work**

Run:

```bash
cd orderapp-remote
go test ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom -count=1
cd frontend-vue-shell
node --test src/lib/drip-product.test.js
npm run build
cd ../..
git diff --check
git add internal/application/bom internal/infrastructure/postgres/bom internal/interfaces/http/bom frontend-vue-shell/src/views/BomView.vue frontend-vue-shell/src/lib/drip-product.js frontend-vue-shell/src/lib/drip-product.test.js
git commit -m "feat: support finished product bom components"
```

Expected: tests and build PASS, commit created.

---

### Task 4: Drip Pricing Templates And Published Price Snapshots

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Modify: `orderapp-remote/internal/domain/costing/engine_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_pdf_source_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_vue_source_test.go`

- [ ] **Step 1: Add failing repository tests for published drip price tiers**

Add a test to `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go` that publishes a run containing one `drip_bag` product and asserts `product_price_tiers` contains:

- `product_kind = 'drip_bag'`
- `sales_unit = 'bag'`
- `sales_unit = 'box'`
- `price_basis = 'unit'`
- `unit_bag_count = 10`
- bag tier min quantities `100 / 1000 / 5000 / 10000`
- box tier min quantities `10 / 100 / 500 / 1000`

- [ ] **Step 2: Run the failing costing repository test**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/costing -run TestPublishRun.*Drip -count=1
```

Expected: FAIL because drip prices are not published as transaction tiers.

- [ ] **Step 3: Add drip template tables**

Modify `costing.EnsureSchema`:

```sql
CREATE TABLE IF NOT EXISTS %[1]s.drip_price_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10,
	box_bag_count INT NOT NULL DEFAULT 10,
	include_packaging BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.drip_price_template_tiers (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL REFERENCES %[1]s.drip_price_templates(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	min_bags NUMERIC(14,3) NOT NULL,
	max_bags NUMERIC(14,3) NULL,
	multiplier NUMERIC(14,6) NOT NULL,
	position INT NOT NULL DEFAULT 1,
	active BOOLEAN NOT NULL DEFAULT true
);
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS price_basis TEXT NOT NULL DEFAULT 'weight';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS sales_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS unit_bag_count INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb;
```

Seed a default template named `默认挂耳供应价` with four tiers:

- `100袋`, min `100`, multiplier `2.2`
- `1000袋`, min `1000`, multiplier `1.8`
- `5000袋`, min `5000`, multiplier `1.6`
- `10000袋`, min `10000`, multiplier `1.35`

- [ ] **Step 4: Publish drip tiers from costing results**

Modify `PublishRun` so `drip_bag` products publish:

- Bag tiers with `sales_unit='bag'`, `unit_bag_count=1`, `price_per_unit=packed_price_per_bag`.
- Box tiers with `sales_unit='box'`, `unit_bag_count=product.drip_box_bag_count`, `price_per_unit=packed_price_per_bag * box_bag_count`.
- `min_qty_units` for box tiers is `ceil(min_bags / box_bag_count)`.
- `price_source_json` stores template id, tier id, bag grams, box bag count, loose price, packed price, multiplier, and tax rate.

Do not delete legacy roasted-bean price behavior.

- [ ] **Step 5: Add API endpoints**

Add endpoints in `costing_api.go`:

- `GET /api/drip-price-templates`
- `POST /api/drip-price-templates`
- `PUT /api/drip-price-templates/:id`
- `POST /api/drip-price-templates/:id/deactivate`
- `POST /api/costing/drip-price-explanation`

Every write endpoint records an audit log action through the existing Postgres audit helper.

- [ ] **Step 6: Add API tests and source guards**

Tests must prove:

- Template save validates positive bag grams and box bag count.
- Deactivate preserves published price tiers.
- Explanation response includes roasted-bean cost/kg, bag grams, process cost, extra cost, multiplier, tax rate, packed price, and box conversion.
- Vue and PDF source guards include `drip` bean-list support.

- [ ] **Step 7: Run and commit costing work**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing -count=1
git diff --check
git add internal/domain/costing internal/infrastructure/postgres/costing internal/interfaces/http/costing
git commit -m "feat: publish drip pricing templates"
```

Expected: tests PASS, commit created.

---

### Task 5: ERP Sales Orders With Bag And Box Units

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/schema.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_dto.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_pricing_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_queries_test.go`

- [ ] **Step 1: Add failing order-pricing tests**

Add tests to `orderapp-remote/internal/interfaces/http/sales/order_pricing_test.go`:

- Bag order: 120 bags at 2.15 totals 258.00.
- Box order: 12 boxes, 10 bags per box, at 21.50 totals 258.00.
- Legacy 454g order still uses the existing per-lb behavior.

- [ ] **Step 2: Run failing sales pricing tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/sales -run Test.*Drip.*Pricing -count=1
```

Expected: FAIL because order save still assumes weight display units.

- [ ] **Step 3: Extend order-item schema**

Modify `sales.EnsureSchema`:

```sql
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS sales_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS unit_bag_count INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS unit_bean_g NUMERIC(12,3) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS matched_price_qty NUMERIC(14,3) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb;
```

- [ ] **Step 4: Extend save-order command and HTTP binding**

Add to `salesapp.OrderItemCommand`:

```go
ProductKind string
SalesUnit string
UnitBagCount int
UnitBeanG float64
```

JSON payload for a drip box line:

```json
{
  "product_id": 12,
  "product_kind": "drip_bag",
  "sales_unit": "box",
  "unit_bag_count": 10,
  "unit_bean_g": 10,
  "qty": 12
}
```

- [ ] **Step 5: Replace drip line-total calculation path**

In `sales.Repository.SaveOrder`:

- If product kind is `drip_bag`, select from `product_price_tiers` by `product_id`, `sales_unit`, and `min_qty_units/max_qty_units`.
- Match box quantity using converted bag count for tier selection when the tier source is bag-based.
- Do not call `wholesaleLineTotalFromDisplayUnit` for drip products.
- Save product-kind, sales-unit, conversion, matched quantity, and price-source snapshot on each order item.
- Keep manual price override working for drip lines.
- Insert audit log for create/edit order with drip metadata included.

- [ ] **Step 6: Update read models**

Update order-list and order-detail queries to include:

- product kind
- sales unit
- unit conversion label
- price source version

Order list display label rules:

- `drip_bag + bag`: `10g/袋`
- `drip_bag + box`: `10袋/盒`
- legacy products: existing spec string

- [ ] **Step 7: Run and commit sales work**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/sales ./internal/application/sales ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales -count=1
git diff --check
git add internal/domain/sales internal/application/sales internal/infrastructure/postgres/sales internal/interfaces/http/sales
git commit -m "feat: support drip sales order units"
```

Expected: tests PASS, commit created.

---

### Task 6: Fulfillment Customer And Mall Backend APIs

**Files:**
- Modify: `orderapp-remote/internal/application/customerfulfillment/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go`
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] **Step 1: Add failing fulfillment API tests**

Add tests proving:

- Customer fulfillment product list returns a drip product with `sales_units`.
- Customer fulfillment order submit accepts `sales_unit='bag'`.
- Customer fulfillment order submit accepts `sales_unit='box'`.
- Product without published drip price returns 400.
- Product without valid BOM returns 400.

- [ ] **Step 2: Add failing miniapp mall API tests**

Add tests proving:

- Miniapp service page product payload includes product kind, bag spec, box spec, and price tiers.
- Miniapp fulfillment submit saves drip unit snapshots.
- Mall public catalog returns public drip products without exposing supply price when no mall price is configured.
- Mall order saves drip unit snapshots when mall price exists.

- [ ] **Step 3: Run failing API tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/customerportal -run 'Test.*Drip' -count=1
```

Expected: FAIL because APIs do not understand drip units yet.

- [ ] **Step 4: Extend backend DTOs and queries**

Add product fields in fulfillment and customerportal DTOs:

```go
ProductKind string `json:"product_kind"`
SalesUnits []string `json:"sales_units"`
DripBagGrams float64 `json:"drip_bag_grams"`
DripBoxBagCount int `json:"drip_box_bag_count"`
```

Order commands include:

```go
SalesUnit string
UnitBagCount int
UnitBeanG float64
```

- [ ] **Step 5: Reuse sales unit-pricing helper**

Use `salesdomain.CalculateUnitLineTotal` or the same Postgres tier-selection rules for:

- fulfillment customer order submit
- miniapp fulfillment order submit
- mall order submit

Do not copy the old `spec_g / 454g` logic into drip paths.

- [ ] **Step 6: Audit every write path**

Add audit entries for:

- fulfillment customer drip submit
- miniapp fulfillment drip submit
- mall drip submit

Audit metadata includes product ID, sales unit, quantity, unit bag count, unit bean grams, price source, and total.

- [ ] **Step 7: Run and commit customer backend work**

Run:

```bash
cd orderapp-remote
go test ./internal/application/customerfulfillment ./internal/infrastructure/postgres/customerfulfillment ./internal/interfaces/http/customerfulfillment ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal -count=1
git diff --check
git add internal/application/customerfulfillment internal/infrastructure/postgres/customerfulfillment internal/interfaces/http/customerfulfillment internal/application/customerportal internal/infrastructure/postgres/customerportal internal/interfaces/http/customerportal
git commit -m "feat: support drip customer orders"
```

Expected: tests PASS, commit created.

---

### Task 7: Drip Bean Lists

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/bean_list_metadata.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/public_bean_list.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/bean_list_pdf.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/bean_list_pdf_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`

- [ ] **Step 1: Add failing tests for `drip` bean-list type**

Tests must prove:

- `drip` is an allowed list type.
- `drip` PDF does not sanitize to `commercial`.
- published `drip` snapshots include only `product_kind='drip_bag'`.
- published snapshot preserves bag/box prices when templates change.

- [ ] **Step 2: Run failing bean-list tests**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing ./internal/interfaces/http/costing ./internal/interfaces/http/customerportal ./internal/infrastructure/pdf -run 'Test.*Drip.*Bean|Test.*Bean.*Drip' -count=1
cd frontend-vue-shell
node --test src/lib/bean-list-pdf.test.js
```

Expected: FAIL because `drip` is not supported end to end.

- [ ] **Step 3: Add `drip` list metadata**

Add metadata:

```go
Code: "drip"
DisplayName: "挂耳豆单"
ProductKind: "drip_bag"
```

Filter published content by product kind and published drip price availability.

- [ ] **Step 4: Update PDF and Vue source guards**

Allow `drip` in `bean_list_pdf.go` and update frontend helper tests so multiple links and download labels include “挂耳豆单”.

- [ ] **Step 5: Run and commit bean-list work**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing ./internal/interfaces/http/costing ./internal/interfaces/http/customerportal ./internal/infrastructure/pdf -count=1
cd frontend-vue-shell
node --test src/lib/bean-list-pdf.test.js
npm run build
cd ../..
git diff --check
git add internal/domain/costing internal/infrastructure/postgres/costing internal/interfaces/http/costing internal/interfaces/http/customerportal internal/infrastructure/pdf frontend-vue-shell/src/lib/bean-list-pdf.test.js frontend-vue-shell/src/views/CostingView.vue
git commit -m "feat: add drip bean list"
```

Expected: tests and build PASS, commit created.

---

### Task 8: Production Planning And Inventory Consumption

**Files:**
- Modify: `orderapp-remote/internal/application/production/service.go`
- Modify: `orderapp-remote/internal/application/production/service_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/plan_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/material_consumption.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/material_consumption_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/production/produce_plan_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/production/manufacturing_gap_api_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/produce-plan.test.js`

- [ ] **Step 1: Add failing production tests**

Add tests proving:

- A drip order creates a drip production demand, not a roasted-bean demand directly.
- Drip production consumes finished roasted-bean inventory using the `finished_product` BOM component.
- When roasted-bean finished inventory is short, the plan shows an upstream roast shortage for the linked roasted-bean SKU.
- Material components such as filter bag and box still consume material inventory.

- [ ] **Step 2: Run failing production tests**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -run 'Test.*Drip|Test.*FinishedProductComponent' -count=1
```

Expected: FAIL because production currently assumes product BOM material components only.

- [ ] **Step 3: Extend production demand model**

Add fields to production demand/read model:

```go
ProductKind string `json:"product_kind"`
SalesUnit string `json:"sales_unit"`
NeedUnits int64 `json:"need_units"`
NeedBags int64 `json:"need_bags"`
UpstreamProductID int64 `json:"upstream_product_id"`
UpstreamShortageG int64 `json:"upstream_shortage_g"`
```

- [ ] **Step 4: Split drip demand and upstream roast shortage**

In `plan_queries.go`:

- For `roasted_bean`, keep existing `need_g = qty * spec_g`.
- For `drip_bag + bag`, calculate `need_bags = qty`.
- For `drip_bag + box`, calculate `need_bags = qty * unit_bag_count`.
- Check drip finished inventory first.
- Use BOM `finished_product` component to calculate roasted-bean grams required.
- Compare with finished roasted-bean inventory.
- Produce upstream shortage rows when roasted-bean inventory is insufficient.

- [ ] **Step 5: Extend consumption code**

In `material_consumption.go`:

- `component_type='material'` keeps current material deduction behavior.
- `component_type='finished_product'` deducts from finished inventory or creates a shortage record; it must not deduct from raw material batches.
- Component consumption supports `g_per_bag`, `unit_per_bag`, and `unit_per_box`.

- [ ] **Step 6: Add audit entries**

Audit:

- generated drip production demand
- generated upstream roast demand
- finished-product component consumption

- [ ] **Step 7: Run and commit production work**

Run:

```bash
cd orderapp-remote
go test ./internal/application/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -count=1
cd frontend-vue-shell
node --test src/lib/produce-plan.test.js
npm run build
cd ../..
git diff --check
git add internal/application/production internal/infrastructure/postgres/production internal/interfaces/http/production frontend-vue-shell/src/lib/produce-plan.test.js
git commit -m "feat: plan drip production from roasted inventory"
```

Expected: tests and build PASS, commit created.

---

### Task 9: ERP Vue/Vite User Interfaces

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/gradient-templates.test.js`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_169_product_settings_sku_list_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_271_product_formula_menu_click_matrix_test.go`

- [ ] **Step 1: Add failing frontend helper tests**

Extend `order-entry.test.js`:

```js
test('drip box order payload includes unit snapshot', () => {
  const product = { id: 9, name: '耶加挂耳', product_kind: 'drip_bag', drip_bag_grams: 10, drip_box_bag_count: 10 }
  const line = buildOrderLinePayload({ product, salesUnit: 'box', qty: 12 })
  assert.equal(line.product_kind, 'drip_bag')
  assert.equal(line.sales_unit, 'box')
  assert.equal(line.unit_bag_count, 10)
  assert.equal(line.unit_bean_g, 10)
})
```

- [ ] **Step 2: Run failing frontend tests**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/order-entry.test.js src/lib/drip-product.test.js src/lib/gradient-templates.test.js
```

Expected: FAIL until UI helpers are updated.

- [ ] **Step 3: Product settings UI**

Update `ProductSettingsView.vue`:

- product-kind segmented control: 熟豆 / 挂耳
- drip fields: 每袋熟豆克重, 每盒袋数
- channel toggles: 履约客户可下单, 商城可售
- product-kind filter in the product table
- do not add or edit legacy HTML templates

- [ ] **Step 4: Costing UI**

Update `CostingView.vue`:

- show drip template section
- show bag tiers `100 / 1000 / 5000 / 10000`
- show loose and packed prices
- show box conversion preview
- add drip price explanation drawer
- add drip bean-list publish action

- [ ] **Step 5: Order UI and order list**

Update `OrderEntryView.vue`:

- when selected product is `drip_bag`, show sales-unit selector with bag and box
- hide weight-spec controls for drip products
- show unit spec label `10g/袋` or `10袋/盒`
- send unit snapshot fields in payload

Update `OrdersView.vue`:

- show product kind
- show sales unit and unit spec
- show price source version when available

- [ ] **Step 6: Production UI**

Update production plan/log views so drip demand rows show:

- 挂耳生产
- 需求袋数
- 熟豆组件缺口
- 上游烘焙需求

- [ ] **Step 7: Run and commit ERP frontend work**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js src/api/*.test.js
npm run build
cd ..
go test ./internal/interfaces/http/support -run 'Test.*Product|Test.*Formula|Test.*Manual|Test.*Vue' -count=1
cd ..
git diff --check
git add orderapp-remote/frontend-vue-shell orderapp-remote/internal/interfaces/http/support
git commit -m "feat: add drip product erp ui"
```

Expected: tests and build PASS, commit created.

---

### Task 10: Miniapp Fulfillment And Mall UI

**Files:**
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/api/customerPortal.test.ts`
- Modify: `miniapp/src/utils/servicePage.ts`
- Modify: `miniapp/src/utils/servicePage.test.ts`
- Modify: `miniapp/src/utils/mall.ts`
- Modify: `miniapp/src/utils/mall.test.ts`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/pages/mall/mall.vue`

- [ ] **Step 1: Add failing miniapp helper tests**

Add tests:

- drip fulfillment product maps to bag and box options.
- drip fulfillment submit payload includes sales-unit snapshot.
- mall drip product without mall price is hidden or disabled with no supply price shown.
- mall drip product with mall price submits sales unit and quantity.

- [ ] **Step 2: Run failing miniapp tests**

Run:

```bash
cd miniapp
npm run test -- --run src/utils/servicePage.test.ts src/utils/mall.test.ts src/api/customerPortal.test.ts
```

Expected: FAIL until API types and helpers support drip products.

- [ ] **Step 3: Update miniapp API types**

Update product and order types:

```ts
product_kind: 'roasted_bean' | 'drip_bag'
sales_units: Array<'bag' | 'box'>
drip_bag_grams: number
drip_box_bag_count: number
sales_unit?: 'bag' | 'box'
unit_bag_count?: number
unit_bean_g?: number
```

- [ ] **Step 4: Update service and mall pages**

`service.vue`:

- show bag/box selector for drip products
- quantity input uses selected unit label
- submit unit snapshot

`mall.vue`:

- show drip unit labels
- do not show wholesale supply price
- use mall price only

- [ ] **Step 5: Run and commit miniapp work**

Run:

```bash
cd miniapp
npm run test -- --run
npm run typecheck
npm run build:mp-weixin
cd ..
git diff --check
git add miniapp/src
git commit -m "feat: add drip miniapp ordering"
```

Expected: tests, typecheck, and build PASS; commit created.

---

### Task 11: Manuals, Operation Logs, And Acceptance Evidence

**Files:**
- Modify: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- Modify: `orderapp-remote/docs/OPERATION_MANUALS.md`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js`
- Modify: `orderapp-remote/internal/interfaces/http/support/operation_log_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_operation_manuals_test.go`
- Create: `orderapp-remote/docs/acceptance/2026-05-18-drip-product-kind.md`

- [ ] **Step 1: Add operation-log tests for all drip write paths**

Tests must cover:

- create/edit drip product
- create/edit drip BOM
- save/deactivate drip price template
- publish drip price
- publish drip bean list
- ERP drip order create/edit
- fulfillment customer drip order submit
- mall drip order submit
- drip production demand generation
- upstream roast shortage generation

- [ ] **Step 2: Update manuals**

Manuals must include:

- how to create a drip product
- how to configure drip BOM with roasted-bean component
- how bag and box units work
- how drip price template maps to the Excel formula
- how ERP order entry handles bags and boxes
- how fulfillment customers order drip products
- how mall drip products differ from supply prices
- how production checks roasted-bean inventory and creates upstream roasting demand
- common failure handling for missing BOM, missing price, insufficient roasted inventory, and missing packaging material

- [ ] **Step 3: Add acceptance evidence file**

Create `orderapp-remote/docs/acceptance/2026-05-18-drip-product-kind.md` with:

```markdown
# 挂耳产品形态验收记录

## 需求覆盖

- 产品设置：已验证
- 挂耳 BOM：已验证
- 挂耳价格模板：已验证
- 挂耳豆单：已验证
- ERP 按袋录单：已验证
- ERP 按盒录单：已验证
- 履约客户下单：已验证
- 商城下单：已验证
- 挂耳生产需求：已验证
- 熟豆不足上游烘焙需求：已验证
- 操作日志：已验证
- 操作手册：已验证

## 验证命令

记录实际运行命令和结果。

## 关键截图或接口响应

记录本次验收中使用的代表性接口响应摘要。
```

- [ ] **Step 4: Run and commit manuals and acceptance**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'Test.*Operation|Test.*Manual|Test.*Audit' -count=1
cd frontend-vue-shell
node --test src/lib/operation-manuals.test.js
npm run build
cd ../..
git diff --check
git add orderapp-remote/docs orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js orderapp-remote/internal/interfaces/http/support
git commit -m "docs: update drip operation manuals"
```

Expected: tests and build PASS, commit created.

---

### Task 12: Full Verification, Integration, And Develop Deployment

**Files:**
- No feature files should be edited in this task unless verification exposes a defect.

- [ ] **Step 1: Run full backend verification**

Run:

```bash
cd orderapp-remote
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full ERP frontend verification**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js src/api/*.test.js
npm run build
```

Expected: PASS.

- [ ] **Step 3: Run full miniapp verification**

Run:

```bash
cd miniapp
npm run test -- --run
npm run typecheck
npm run build:mp-weixin
```

Expected: PASS.

- [ ] **Step 4: Run diff and status checks**

Run:

```bash
git diff --check
git status --short
git log --oneline -5
```

Expected: no whitespace errors; only intentional tracked changes are present.

- [ ] **Step 5: Push feature branch**

Run:

```bash
git push -u origin codex/drip-product-design
```

Expected: branch pushed successfully.

- [ ] **Step 6: Integrate latest develop into the feature branch**

Run:

```bash
git fetch origin
git merge origin/develop
```

Expected: clean merge or conflicts resolved on the feature branch.

After resolving any conflicts, rerun:

```bash
cd orderapp-remote
go test ./... -count=1
cd frontend-vue-shell
node --test src/lib/*.test.js src/api/*.test.js
npm run build
cd ../../miniapp
npm run test -- --run
npm run typecheck
npm run build:mp-weixin
```

Expected: PASS.

- [ ] **Step 7: Merge to develop without force-push**

Run:

```bash
git switch develop
git pull --ff-only origin develop
git merge --no-ff codex/drip-product-design
git push origin develop
```

Expected: `develop` contains the verified feature branch.

- [ ] **Step 8: Record deployment target commit**

Run:

```bash
git fetch origin
git log --oneline -3 origin/develop
git rev-parse origin/develop
```

Record the exact SHA in the final deployment notes and in `orderapp-remote/docs/acceptance/2026-05-18-drip-product-kind.md`.

- [ ] **Step 9: Deploy development stack**

Use the existing KFerp deployment workflow for the agreed development environment. Verify:

- ERP product settings loads.
- Costing page loads drip template section.
- ERP order entry can submit bag and box drip orders.
- Miniapp API smoke test returns drip product metadata.
- Operation log page shows drip write entries.

- [ ] **Step 10: Final acceptance summary**

Final response must include:

- branch name
- merge commit or develop SHA
- deployment target
- verification commands and PASS/FAIL summary
- acceptance evidence file path
- any residual risks
