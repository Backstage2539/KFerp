# Green Bean Sales Fusion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full green bean sales flow inside the existing product, BOM, pricing, bean list, order, customer fulfillment, mall, material receipt, and quality inspection systems.

**Architecture:** Keep one product backbone. Green bean products carry only sales-specific attributes and bind to a roasted BOM product; green bean pricing uses the bound BOM raw cost plus gradient template tiers, and green bean bean-list quality uses the bound roasted product's latest passed production inspection. Material inbound quality remains a stock workflow and does not feed bean-list quality.

**Tech Stack:** Go, PostgreSQL via pgx, Echo HTTP handlers, Vue 3 + Vite, miniapp TypeScript/Vue, Node test runner, Go tests.

---

## File Map

- `orderapp-remote/internal/application/catalog/service.go`: product commands, product settings payload, validation for green bean type and bound BOM product.
- `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`: `products.green_bean_type` and `products.green_bean_bom_product_id`.
- `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`: create/update/read product green bean fields and audit metadata.
- `orderapp-remote/internal/infrastructure/postgres/catalog_queries.go`: shared product option query fields.
- `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`: product create/update request and response JSON.
- `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`: API-level tests for product settings behavior.
- `orderapp-remote/internal/domain/costing/engine.go`: green bean price calculation from gradient templates, not direct price tiers.
- `orderapp-remote/internal/domain/costing/engine_test.go`: unit tests for green bean pricing and preserved roasted pricing.
- `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`: load green bean inputs through bound BOM product; load latest passed production QC for bean lists.
- `orderapp-remote/internal/interfaces/http/costing/costing_api.go`: bean-list payload fields and publication checks.
- `orderapp-remote/internal/interfaces/http/costing/public_bean_list.go`: public green bean title, labels, and QC rendering.
- `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`: API tests for green bean bean list and QC source.
- `orderapp-remote/internal/infrastructure/postgres/stock/schema.go`: material receipt and batch crop/origin/producer flavor fields.
- `orderapp-remote/internal/application/stock/service.go`: material receipt command/result fields.
- `orderapp-remote/internal/infrastructure/postgres/stock/repository.go`: persist receipt batch metadata and expose it in batch lists.
- `orderapp-remote/internal/interfaces/http/stock/stock_api.go`: material receipt request/response fields.
- `orderapp-remote/internal/interfaces/http/stock/stock_api_test.go`: API tests for inbound fields and batch quality status.
- `orderapp-remote/internal/application/production/service.go`: structured quality metrics normalization for inbound QC.
- `orderapp-remote/internal/infrastructure/postgres/production/quality.go`: existing status update remains the persistence backbone.
- `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`: wholesale custom spec option.
- `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`: unit tests for direct gram input in wholesale/green bean order entry.
- `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`: direct grams UI for wholesale and product-kind badges.
- `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`: filters, green bean type, bound BOM product, remove default/sale price UI.
- `orderapp-remote/frontend-vue-shell/src/views/MaterialReceiptsView.vue`: crop season, origin, producer flavor fields.
- `orderapp-remote/frontend-vue-shell/src/views/QualityInspectionsView.vue`: inbound QC structured fields for factory flavor, moisture, density.
- `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`: public SKU filters and no manual sale price column.
- `orderapp-remote/frontend-vue-shell/src/views/MallSettingsView.vue`: remove authoritative unit price editing and show server-computed product pricing.
- `orderapp-remote/frontend-vue-shell/src/lib/*.test.js`: frontend behavior/source tests for changed views.
- `orderapp-remote/internal/application/customerportal/service.go`: service DTOs keep product kind and computed price semantics.
- `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`: customer fulfillment and mall order price lookup from product tiers.
- `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`: miniapp service/mall responses expose green bean kind and computed prices.
- `miniapp/src/utils/mall.ts`, `miniapp/src/utils/beanListDisplay.ts`, `miniapp/src/pages/service/service.vue`, `miniapp/src/pages/mall/mall.vue`: product-kind labels and green bean display.
- `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/docs/REQUIREMENTS.md`, `orderapp-remote/docs/ACCEPTANCE_TESTS.md`: requirement and acceptance evidence.
- `OP_MANUAL_GREEN_BEAN_SALES.md`, `orderapp-remote/docs/OP_MANUAL_GREEN_BEAN_SALES.md`, other touched manuals, and `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js`: user workflow documentation.
- `orderapp-remote/internal/interfaces/http/support/req_store.go`: PR/DEV/UT/API/REV seed rows for this feature.

## Task 1: Product Data Model And Product Settings API

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog_queries.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Test: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`

- [ ] **Step 1: Write failing API tests**

Add tests that post and update a green bean product with:

```json
{
  "name": "埃塞瑰夏生豆",
  "product_kind": "green_bean",
  "green_bean_type": "single_origin",
  "green_bean_bom_product_id": 7
}
```

Expected assertions:
- request succeeds without `default_price` or `tiers`;
- command records `GreenBeanType == "single_origin"`;
- command records `GreenBeanBomProductID == 7`;
- response contains `"green_bean_type":"single_origin"` and `"green_bean_bom_product_id":7`;
- update request preserves the same fields;
- update request with `green_bean_bom_product_id` pointing to a green bean product returns HTTP 400 once repository validation is wired.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPICreatesGreenBeanProductWithBomBinding|TestProductSettingsAPIUpdatesGreenBeanBomBinding' -count=1
```

Expected: FAIL because product commands and responses do not yet contain `green_bean_type` or `green_bean_bom_product_id`.

- [ ] **Step 3: Add catalog app fields and validation**

Add `GreenBeanType string` and `GreenBeanBomProductID int64` to `Product`, `ProductSettingsProduct`, `CreateProductCommand`, `UpdateProductBasicsCommand`, and `ReplacePriceTiersCommand`.

Validation:
- normalize `green_bean_type` to `single_origin` or `blend`;
- default empty green bean type to `single_origin`;
- green bean products require `GreenBeanBomProductID > 0`;
- roasted products clear both green bean fields;
- green bean create/update ignores `DefaultPrice` and retail prices by setting them to zero in service commands.

- [ ] **Step 4: Add schema and repository persistence**

Add schema columns:

```sql
ALTER TABLE <schema>.products ADD COLUMN IF NOT EXISTS green_bean_type TEXT NOT NULL DEFAULT '';
ALTER TABLE <schema>.products ADD COLUMN IF NOT EXISTS green_bean_bom_product_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS products_green_bean_bom_product_idx ON <schema>.products(green_bean_bom_product_id);
```

Update product insert, update, fetch, and JSON mapping to read/write those fields.

- [ ] **Step 5: Run GREEN**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPICreatesGreenBeanProductWithBomBinding|TestProductSettingsAPIUpdatesGreenBeanBomBinding|TestProductSettingsAPICreatesPublicProduct' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Commit:

```bash
git add orderapp-remote/internal/application/catalog/service.go orderapp-remote/internal/infrastructure/postgres/catalog/schema.go orderapp-remote/internal/infrastructure/postgres/catalog/repository.go orderapp-remote/internal/infrastructure/postgres/catalog_queries.go orderapp-remote/internal/interfaces/http/catalog/product_routes.go orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go
git commit -m "feat: add green bean product bom binding"
```

## Task 2: Green Bean Pricing From Bound BOM And Gradient Template

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Test: `orderapp-remote/internal/domain/costing/engine_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`

- [ ] **Step 1: Write failing domain tests**

Add a test named `TestGreenBeanProductUsesGradientTemplateAndSkipsRoastCosting`. Input:
- `ProductKind: "green_bean"`;
- `GreenBeanCostPerKg: 50`;
- `YieldRate: 0.7`;
- gradient template display unit `kg`, one tier `MinWeightG:1000`, `MarginRate:0.2`.

Expected:
- one `GreenBeanSaleTiers` row;
- `PricePerUnit == 60`;
- `RoastedBeanCostPerKg == 0`;
- no `CommercialWholesaleTiers`;
- `BomStatus == "bom_cost_template_price"`;
- no direct tier input is needed.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing -run TestGreenBeanProductUsesGradientTemplateAndSkipsRoastCosting -count=1
```

Expected: FAIL because green bean calculation currently only copies direct sale tiers.

- [ ] **Step 3: Implement green bean template calculation**

Change `calculateGreenBeanProduct` so it:
- normalizes `GradientTemplate`;
- computes each green bean sale tier from `GreenBeanCostPerKg * (1 + margin_rate)`;
- applies display unit conversion through the existing gradient unit rules;
- stores `TemplateID`, `TemplateTierID`, `DisplayUnit`, `MinWeightG`, `MaxWeightG`, and `MarginRate`;
- does not use yield rate, roasted cost, small/large batch cost, package cost, product loss, or retail tax.

- [ ] **Step 4: Change costing repository BOM source**

Update `LoadProductInputs` so green bean products join BOM and BOM items through `COALESCE(NULLIF(p.green_bean_bom_product_id,0), p.id)` as the costing BOM product. Keep product category and visibility from the green bean product itself.

Remove `loadGreenBeanSaleTiers` as the authoritative green bean price source; published prices still come from `PublishRun` snapshots after calculation.

- [ ] **Step 5: Run GREEN**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing ./internal/infrastructure/postgres/costing -run 'GreenBean|GradientTemplate' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

Commit:

```bash
git add orderapp-remote/internal/domain/costing/engine.go orderapp-remote/internal/domain/costing/engine_test.go orderapp-remote/internal/infrastructure/postgres/costing/repository.go orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go
git commit -m "feat: price green beans from bound bom templates"
```

## Task 3: Bean List Latest Passed Production QC

**Files:**
- Modify: `orderapp-remote/internal/application/costing/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/public_bean_list.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`
- Modify: `miniapp/src/utils/beanListDisplay.ts`
- Test: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js`
- Test: `miniapp/src/utils/beanListDisplay.test.ts`

- [ ] **Step 1: Write failing tests**

Add tests proving:
- roasted bean list uses latest passed production QC for the product;
- green bean list uses latest passed production QC for `green_bean_bom_product_id`;
- rejected or held inspections are ignored;
- newer failed records do not hide an older passed record;
- public green bean list title is `生豆豆单`;
- PDF/native display includes factory flavor, moisture, density, and inspection time.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/costing -run 'GreenBean.*Quality|PublicBeanList.*Green' -count=1
cd frontend-vue-shell && npm test -- bean-list-pdf.test.js
cd ../../miniapp && npm test -- beanListDisplay.test.ts
```

Expected: FAIL because QC fields are not returned or rendered consistently.

- [ ] **Step 3: Add QC DTO and repository query**

Add a bean-list QC struct with:
- `factory_flavor_description`;
- `moisture`;
- `density`;
- `inspection_created_at`;
- `inspection_reference_no`.

Repository query:
- resolve target product ID as product ID for roasted, bound BOM product ID for green bean;
- join `quality_inspections` through `work_orders` and finished batches;
- filter `result='pass'`;
- order by `created_at DESC, id DESC`;
- parse `metrics_json` keys `factory_flavor_description`, `factory_flavor`, `工厂风味描述`, `moisture`, `水分`, `density`, `密度`.

- [ ] **Step 4: Render QC consistently**

Add QC fields to ERP bean-list payload, public page/PDF payload, frontend PDF renderer, and miniapp display utilities.

- [ ] **Step 5: Run GREEN**

Run the RED commands again. Expected: PASS.

- [ ] **Step 6: Commit**

Commit:

```bash
git add orderapp-remote/internal/application/costing/service.go orderapp-remote/internal/infrastructure/postgres/costing/repository.go orderapp-remote/internal/interfaces/http/costing/costing_api.go orderapp-remote/internal/interfaces/http/costing/public_bean_list.go orderapp-remote/internal/interfaces/http/costing/costing_api_test.go orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js miniapp/src/utils/beanListDisplay.ts miniapp/src/utils/beanListDisplay.test.ts
git commit -m "feat: show latest passed qc on green bean lists"
```

## Task 4: Material Receipt Fields And Inbound QC

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/schema.go`
- Modify: `orderapp-remote/internal/application/stock/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/stock/stock_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/MaterialReceiptsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/QualityInspectionsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/material-receipts.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/quality-inspections.js`
- Test: `orderapp-remote/internal/interfaces/http/stock/stock_api_test.go`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/material-receipts.test.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/quality-inspections.test.js`

- [ ] **Step 1: Write failing tests**

Add tests proving material receipt POST persists and returns:
- `crop_season`;
- `origin`;
- `producer_flavor_description`.

Add frontend tests proving the material receipt request body carries those fields.

Add quality inspection tests proving raw material scope can build:

```json
{
  "factory_flavor_description": "茉莉花、柑橘",
  "moisture": "10.8%",
  "density": "780g/L"
}
```

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/stock -run MaterialReceipt -count=1
cd frontend-vue-shell && npm test -- material-receipts.test.js quality-inspections.test.js
```

Expected: FAIL because fields and helpers are missing.

- [ ] **Step 3: Persist receipt fields**

Add columns to both `material_receipts` and `material_batches`, wire command structs and SQL insert/selects, and return the fields in material batch list rows.

- [ ] **Step 4: Add Vue forms**

Material receipt form includes compact inputs for 产季、产地、产家风味描述. Inbound QC view shows structured fields when scope is raw material and serializes them into `metrics_json`.

- [ ] **Step 5: Run GREEN**

Run the RED commands again. Expected: PASS.

- [ ] **Step 6: Commit**

Commit:

```bash
git add orderapp-remote/internal/infrastructure/postgres/stock/schema.go orderapp-remote/internal/application/stock/service.go orderapp-remote/internal/infrastructure/postgres/stock/repository.go orderapp-remote/internal/interfaces/http/stock/stock_api.go orderapp-remote/internal/interfaces/http/stock/stock_api_test.go orderapp-remote/frontend-vue-shell/src/views/MaterialReceiptsView.vue orderapp-remote/frontend-vue-shell/src/views/QualityInspectionsView.vue orderapp-remote/frontend-vue-shell/src/lib/material-receipts.js orderapp-remote/frontend-vue-shell/src/lib/material-receipts.test.js orderapp-remote/frontend-vue-shell/src/lib/quality-inspections.js orderapp-remote/frontend-vue-shell/src/lib/quality-inspections.test.js
git commit -m "feat: add material receipt profile and inbound qc fields"
```

## Task 5: Order Entry Green Beans And Direct Gram Specs

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_pricing_test.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`

- [ ] **Step 1: Write failing tests**

Add tests proving:
- wholesale spec options include custom direct gram entry;
- a custom `spec_g` of `1500` can match product tiers by total grams;
- green bean order items preserve `product_kind='green_bean'`;
- order list and detail include product-kind summary.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell && npm test -- order-entry.test.js
cd .. && go test ./internal/interfaces/http/sales -run 'Order.*GreenBean|Order.*CustomSpec|Pricing' -count=1
```

Expected: FAIL on missing custom wholesale spec and any missing green bean payload fields.

- [ ] **Step 3: Implement custom grams and green bean badges**

Add custom sentinel to wholesale options, reuse existing retail custom input behavior, and keep server-side price calculation authoritative. Ensure order saves and queries continue snapshotting `product_kind`.

- [ ] **Step 4: Run GREEN**

Run the RED commands again. Expected: PASS.

- [ ] **Step 5: Commit**

Commit:

```bash
git add orderapp-remote/frontend-vue-shell/src/lib/order-entry.js orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue orderapp-remote/internal/interfaces/http/sales/order_api.go orderapp-remote/internal/infrastructure/postgres/sales/repository.go orderapp-remote/internal/interfaces/http/sales/order_pricing_test.go orderapp-remote/internal/interfaces/http/sales/order_api_test.go
git commit -m "feat: support green bean order entry grams"
```

## Task 6: Public SKU Filters, Customer Fulfillment, Mall, And Miniapp

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/MallSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/customer-mall.js`
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/admin_api.go`
- Modify: `miniapp/src/utils/mall.ts`
- Modify: `miniapp/src/utils/servicePage.ts`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/pages/mall/mall.vue`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.test.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/customer-mall.test.js`
- Test: `orderapp-remote/internal/application/customerportal/service_test.go`
- Test: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`
- Test: `miniapp/src/utils/mall.test.ts`
- Test: `miniapp/src/utils/servicePage.test.ts`

- [ ] **Step 1: Write failing tests**

Tests must prove:
- public SKU list filters by product kind, name, first category, second category;
- public SKU list no longer exposes editable sales price;
- customer fulfillment can choose and submit a green bean product;
- mall settings no longer treats `unit_price` as authoritative;
- miniapp mall and service pages label green bean products and submit them.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell && npm test -- customer-fulfillment.test.js customer-mall.test.js
cd .. && go test ./internal/application/customerportal ./internal/interfaces/http/customerportal -run 'GreenBean|Mall|SKU' -count=1
cd ../miniapp && npm test -- mall.test.ts servicePage.test.ts
```

Expected: FAIL where price columns and green bean support are not updated.

- [ ] **Step 3: Implement filters and server price authority**

Update frontend selectors and server DTOs. For mall orders, server recomputes line price from product tiers using product ID, spec grams, and quantity, while keeping `mall_products.unit_price` only for backward-compatible reads.

- [ ] **Step 4: Run GREEN**

Run the RED commands again. Expected: PASS.

- [ ] **Step 5: Commit**

Commit:

```bash
git add orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue orderapp-remote/frontend-vue-shell/src/views/MallSettingsView.vue orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.js orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.test.js orderapp-remote/frontend-vue-shell/src/lib/customer-mall.js orderapp-remote/frontend-vue-shell/src/lib/customer-mall.test.js orderapp-remote/internal/application/customerportal/service.go orderapp-remote/internal/application/customerportal/service_test.go orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go orderapp-remote/internal/interfaces/http/customerportal/mini_api.go orderapp-remote/internal/interfaces/http/customerportal/admin_api.go orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go miniapp/src/utils/mall.ts miniapp/src/utils/mall.test.ts miniapp/src/utils/servicePage.ts miniapp/src/utils/servicePage.test.ts miniapp/src/pages/service/service.vue miniapp/src/pages/mall/mall.vue
git commit -m "feat: enable green beans in customer portal and mall"
```

## Task 7: Manuals, Requirement Tables, And Acceptance Evidence

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_GREEN_BEAN_SALES.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `OP_MANUAL_PRODUCTION.md`
- Modify: `OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_GREEN_BEAN_SALES.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_289_green_bean_sales_test.go`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js`

- [ ] **Step 1: Write failing documentation guard tests**

Update support test expectations so the old direct-price text fails and the new text must include:
- `绑定熟豆 BOM`;
- `阶梯模板`;
- `最新通过生产质检`;
- `入库质检`;
- `产季`;
- `产地`;
- `产家风味描述`.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run GreenBeanSales -count=1
cd frontend-vue-shell && npm test -- operation-manuals.test.js
```

Expected: FAIL because manuals still describe direct price maintenance.

- [ ] **Step 3: Update manuals and requirement seeds**

Rewrite the green bean manual and patch related manuals for changed entries, fields, publication rules, inbound QC, fulfillment, and mall. Add PR/DEV/UT/API/REV rows that reference the tests and acceptance evidence.

- [ ] **Step 4: Run GREEN**

Run the RED commands again. Expected: PASS.

- [ ] **Step 5: Commit**

Commit:

```bash
git add REQUIREMENTS.md ACCEPTANCE_TESTS.md OP_MANUAL_GREEN_BEAN_SALES.md OP_MANUAL_COSTING.md OP_MANUAL_ORDER_SALES.md OP_MANUAL_INVENTORY_MATERIALS.md OP_MANUAL_PRODUCTION.md OP_MANUAL_CUSTOMER_FULFILLMENT.md OP_MANUAL_CUSTOMER_PORTAL.md orderapp-remote/docs/REQUIREMENTS.md orderapp-remote/docs/ACCEPTANCE_TESTS.md orderapp-remote/docs/OP_MANUAL_GREEN_BEAN_SALES.md orderapp-remote/docs/OP_MANUAL_COSTING.md orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md orderapp-remote/docs/OP_MANUAL_PRODUCTION.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.js orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js orderapp-remote/internal/interfaces/http/support/req_store.go orderapp-remote/internal/interfaces/http/support/dev_289_green_bean_sales_test.go
git commit -m "docs: update green bean sales workflow"
```

## Task 8: Full Verification, Merge, And Deployment

**Files:**
- No source edits unless verification finds a defect.

- [ ] **Step 1: Run targeted test suites**

Run:

```bash
cd orderapp-remote
go test ./internal/domain/costing ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/costing ./internal/interfaces/http/stock ./internal/interfaces/http/sales ./internal/application/customerportal ./internal/interfaces/http/customerportal ./internal/interfaces/http/support -count=1
cd frontend-vue-shell && npm test
cd ../../miniapp && npm test
```

Expected: PASS.

- [ ] **Step 2: Run build**

Run:

```bash
cd orderapp-remote/frontend-vue-shell && npm run build
cd ../../miniapp && npm run test -- --runInBand
```

Expected: frontend build succeeds; miniapp command succeeds or reports the repo's supported test mode.

- [ ] **Step 3: Push feature branch**

Run:

```bash
git status --short --branch
git push origin codex/green-bean-sales-fusion-20260518
```

Expected: feature branch pushed.

- [ ] **Step 4: Merge latest develop safely**

Run:

```bash
git fetch origin
git merge origin/develop
```

Expected: clean merge or conflicts resolved on the feature branch, followed by rerunning Step 1.

- [ ] **Step 5: Merge into develop**

Run:

```bash
git checkout develop
git pull --ff-only origin develop
git merge --no-ff codex/green-bean-sales-fusion-20260518
git push origin develop
git rev-parse origin/develop
git log --oneline -3 origin/develop
```

Expected: `origin/develop` points at the intended merge commit.

- [ ] **Step 6: Deploy development**

Run from the develop checkout:

```bash
./deploy_orderapp.sh development
```

Expected: deployment script succeeds.

- [ ] **Step 7: Smoke test deployed site**

Run curl or browser smoke checks for:
- product settings page;
- price and bean list API;
- material receipt API;
- quality inspection API;
- order entry shell;
- customer portal miniapp APIs.

Expected: HTTP 200 responses and green bean fields present where applicable.
