# Product Industry Fields Template-Only Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a selected industry field template the only source of product industry fields, remove legacy unbound fields, and prevent them from being recreated.

**Architecture:** The Vue layer projects fields from the selected template for list display, drawer editing, template switching, and save payloads. The catalog application and PostgreSQL repository enforce the same rule, while an idempotent schema cleanup removes orphaned, untemplated, and template-external rows after stopping the old `roast_level` / `special_attrs_json` field backfill.

**Tech Stack:** Vue 3, Vite, Node test runner, Go, Echo, pgx/PostgreSQL, Markdown requirement and acceptance contracts.

---

## File map

- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`: pure template projection for product industry fields.
- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`: pure behavior and Vue source-contract regression tests.
- `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`: list display, drawer loading, template switching, and save payload wiring.
- `orderapp-remote/internal/application/catalog/service.go`: service-level clearing for template ID 0.
- `orderapp-remote/internal/application/catalog/service_test.go`: application-service regression test.
- `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`: repository defense and product-copy behavior.
- `orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go`: repository and copy source contracts.
- `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`: stop legacy field creation and run idempotent cleanup.
- `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`: API regression for old clients sending fields without a template.
- `orderapp-remote/internal/interfaces/http/support/req_store.go`: PR/DEV progress rows.
- `orderapp-remote/internal/interfaces/http/support/dev_536_product_industry_template_only_test.go`: requirement/document/source contract.
- `orderapp-remote/docs/REQUIREMENTS.md`: replace the PR-409 compatibility rule with PR-536.
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`: add template-only acceptance and adjust copy semantics.
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`: document selection, clearing, and copy behavior.
- `orderapp-remote/docs/acceptance/2026-07-14-product-industry-template-only.md`: RED/GREEN and delivery evidence.
- `ACTIVE_REQUIREMENTS.md`: short-lived PR-536 state and verifier evidence.

All command blocks start from the KFerp worktree root unless the block begins with an explicit `cd`; each block is independent of the previous block's shell directory.

### Task 1: Make the pure frontend projection reject untemplated and aliased legacy fields

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js:1-70,1265-1340`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js:2388-2470`

- [ ] **Step 1: Add failing pure regression tests**

Import `productProductionConfigFieldsFromTemplate`, add an explicit no-template case, and change the legacy-alias expectation:

```js
import {
  // existing imports
  productProductionConfigFieldsFromTemplate,
} from './product-settings.js'

test('product production config has no industry fields without a selected template', () => {
  const legacyFields = [{
    field_key: 'roast_level',
    template_field_key: '',
    label: 'roast_level',
    field_type: 'text',
    value_text: '深烘',
    sort_order: 1,
  }]

  assert.deepEqual(productProductionConfigFieldsFromTemplate(legacyFields, null), [])

  const form = buildProductProductionConfigForm({
    product_id: 556,
    industry_field_template_id: 0,
    fields: legacyFields,
  }, { id: 556, name: '无模板旧商品' })

  assert.deepEqual(form.fields, [])
})
```

In `product production config form keeps only current template industry fields`, keep the exact-key case and change the alias-only assertions to:

```js
assert.equal(legacyOnly.fields.length, 1)
assert.equal(legacyOnly.fields[0].field_key, '烘焙度')
assert.equal(legacyOnly.fields[0].template_field_key, '烘焙度')
assert.equal(legacyOnly.fields[0].field_type, 'select')
assert.equal(legacyOnly.fields[0].value_text, '')
```

- [ ] **Step 2: Run the frontend test and capture RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: the no-template test receives the legacy row instead of `[]`, and the alias-only case receives `中烘` instead of an empty value.

- [ ] **Step 3: Implement strict template projection**

Replace the legacy alias/index/match implementation and the empty-template branch with:

```js
function indexProductProductionConfigFields(fields = []) {
  const byKey = new Map()
  for (const field of Array.isArray(fields) ? fields : []) {
    const keys = [field?.template_field_key, field?.field_key]
      .map((value) => String(value || '').trim())
      .filter(Boolean)
    for (const key of keys) {
      if (!byKey.has(key)) byKey.set(key, field)
    }
  }
  return { byKey }
}

function productProductionConfigTemplateFieldMatch(field = {}, index = {}) {
  const key = String(field.field_key || '').trim()
  return index.byKey?.get(key) || {}
}

export function productProductionConfigFieldsFromTemplate(fields = [], template = {}) {
  const templateFields = Array.isArray(template?.fields) ? template.fields : []
  if (!templateFields.length) return []
  const existingIndex = indexProductProductionConfigFields(fields)
  return templateFields
    .slice()
    .sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0))
    .map((field, index) => {
      const key = String(field.field_key || '').trim()
      const existing = productProductionConfigTemplateFieldMatch(field, existingIndex)
      return buildProductProductionConfigField({
        ...existing,
        field_key: key,
        template_field_key: key,
        label: field.label || key,
        field_type: field.field_type || existing.field_type || 'text',
        unit: field.unit || '',
        value_text: existing.value_text || templateFieldDefaultText(field),
        required: Boolean(field.required),
        options_json: field.options_json || '[]',
        show_in_price_list: existing.show_in_price_list !== false,
        sort_order: Number(field.sort_order || index + 1),
      }, index)
    })
}
```

Delete `legacyIndustryFieldAliases` and all label/alias matching.

- [ ] **Step 4: Run the frontend test and capture GREEN**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: all product settings tests pass with zero failures.

- [ ] **Step 5: Commit the pure frontend contract**

```bash
git add orderapp-remote/frontend-vue-shell/src/lib/product-settings.js orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js
git commit -m "fix: require product industry field templates"
```

### Task 2: Enforce template projection in the product list, drawer, switch, and save flow

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue:2288-2300,5850-5910,6229-6268`

- [ ] **Step 1: Add a failing Vue source-contract test**

```js
test('product archive displays and saves industry fields only through the selected template', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  const script = source.split('<script setup>')[1]?.split('</script>')[0] || ''

  assert.match(script, /function productionConfigPriceListFields[\s\S]*industryFieldTemplateForConfig\(config\)[\s\S]*productProductionConfigFieldsFromTemplate/)
  assert.match(script, /function applyIndustryFieldTemplateToProductionConfig[\s\S]*productProductionConfigForm\.value\.fields\s*=\s*productProductionConfigFieldsFromTemplate/)
  assert.match(script, /async function saveProductProductionConfig[\s\S]*industryFieldTemplateForConfig\(productProductionConfigForm\.value\)[\s\S]*productProductionConfigFieldsFromTemplate/)
  assert.doesNotMatch(script, /function applyIndustryFieldTemplateToProductionConfig\(\) \{\s*const template = selectedIndustryFieldTemplate\(\)\s*if \(!template\) return/)
})
```

- [ ] **Step 2: Run the frontend test and capture RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: the list, clear-template, and save projection source assertions fail.

- [ ] **Step 3: Project list fields through the referenced template**

```js
function productionConfigPriceListFields(product) {
  const config = productionConfigForProduct(product)
  const template = industryFieldTemplateForConfig(config)
  return productProductionConfigFieldsFromTemplate(config.fields || [], template)
    .filter((field) => field.show_in_price_list)
}
```

- [ ] **Step 4: Re-project fields after the drawer finishes loading templates**

At the start of `openProductProductionConfig`, retain the raw config:

```js
const config = productProductionConfigByProductID(row?.id)
productProductionConfigProduct.value = row || null
productProductionConfigForm.value = defaultProductProductionConfigForm(config, row)
```

Immediately after the `Promise.all` that loads BOMs, routes, and industry templates, add:

```js
productProductionConfigForm.value.fields = productProductionConfigFieldsFromTemplate(
  config?.fields || [],
  industryFieldTemplateForConfig(config),
)
```

- [ ] **Step 5: Make template clearing and save payloads strict**

Replace the apply function with:

```js
function applyIndustryFieldTemplateToProductionConfig() {
  const template = industryFieldTemplateForConfig(productProductionConfigForm.value)
  productProductionConfigForm.value.fields = productProductionConfigFieldsFromTemplate(
    productProductionConfigForm.value.fields || [],
    template,
  )
}
```

In `saveProductProductionConfig`, replace the field construction with:

```js
const industryFieldTemplate = industryFieldTemplateForConfig(productProductionConfigForm.value)
const fields = productProductionConfigFieldsFromTemplate(
  productProductionConfigForm.value.fields || [],
  industryFieldTemplate,
)
  .map((field, index) => normalizeProductProductionConfigFieldForSave(field, index))
  .filter((field) => field.label || field.value_text || field.value_number !== null || field.value_bool !== null)
```

Remove `selectedIndustryFieldTemplate` if it has no remaining callers.

- [ ] **Step 6: Run the frontend test and capture GREEN**

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: all tests pass, including the list/drawer/switch/save source contract.

- [ ] **Step 7: Commit the Vue wiring**

```bash
git add orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js
git commit -m "fix: project product fields from selected template"
```

### Task 3: Clear untemplated fields in the application service and API

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service_test.go:8-55,120-145`
- Modify: `orderapp-remote/internal/application/catalog/service.go:1334-1368`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go:286-320,2400-2460`

- [ ] **Step 1: Add a failing application-service test**

Add `productionConfig SaveProductProductionConfigCommand` to `fakeRepo`, assign it in the fake method, and add:

```go
func TestSaveProductProductionConfigClearsFieldsWithoutIndustryTemplate(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo)

	result, err := service.SaveProductProductionConfig(context.Background(), SaveProductProductionConfigCommand{
		ProductID:               91,
		IndustryFieldTemplateID: 0,
		Fields: []ProductProductionConfigField{{
			FieldKey: "roast_level",
			Label:    "roast_level",
			ValueText: "深烘",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.productionConfig.Fields) != 0 {
		t.Fatalf("saved fields=%+v, want none without industry template", repo.productionConfig.Fields)
	}
	if len(result.Fields) != 0 {
		t.Fatalf("result fields=%+v, want none without industry template", result.Fields)
	}
}
```

Update the fake repository method:

```go
func (r *fakeRepo) SaveProductProductionConfig(ctx context.Context, cmd SaveProductProductionConfigCommand) (ProductProductionConfig, error) {
	r.productionConfig = cmd
	return ProductProductionConfig{
		ProductID:               cmd.ProductID,
		ProductionBomID:         cmd.ProductionBomID,
		ProductionBomVersionID:  cmd.ProductionBomVersionID,
		ProcessRouteID:          cmd.ProcessRouteID,
		ExpectedLossRate:        cmd.ExpectedLossRate,
		IndustryFieldTemplateID: cmd.IndustryFieldTemplateID,
		Note:                    cmd.Note,
		Fields:                  cmd.Fields,
	}, nil
}
```

- [ ] **Step 2: Add a failing HTTP API regression test**

```go
func TestProductSettingsAPIClearsIndustryFieldsWithoutTemplate(t *testing.T) {
	repo := &productSettingsRepo{}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPut, "/api/product-production-configs/91", bytes.NewBufferString(`{
		"industry_field_template_id":0,
		"expected_loss_rate":0.2,
		"fields":[{"field_key":"roast_level","label":"roast_level","value_text":"深烘"}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT product production config status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.savedProductionConfig.Fields) != 0 {
		t.Fatalf("saved fields=%+v, want none without industry template", repo.savedProductionConfig.Fields)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"fields":[]`)) {
		t.Fatalf("response should expose empty fields: %s", rec.Body.String())
	}
}
```

- [ ] **Step 3: Run application and API tests and capture RED**

```bash
cd orderapp-remote
go test ./internal/application/catalog -run TestSaveProductProductionConfigClearsFieldsWithoutIndustryTemplate -count=1
go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIClearsIndustryFieldsWithoutTemplate -count=1
```

Expected: both tests show the legacy field is still passed through.

- [ ] **Step 4: Clear fields before service validation and repository dispatch**

In `SaveProductProductionConfig`, after validating the template ID and before the field loop, add:

```go
if cmd.IndustryFieldTemplateID == 0 {
	cmd.Fields = []ProductProductionConfigField{}
}
```

- [ ] **Step 5: Run application and API tests and capture GREEN**

```bash
cd orderapp-remote
go test ./internal/application/catalog -run TestSaveProductProductionConfigClearsFieldsWithoutIndustryTemplate -count=1
go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIClearsIndustryFieldsWithoutTemplate -count=1
```

Expected: both tests pass and the API returns `"fields":[]`.

- [ ] **Step 6: Commit the service/API contract**

```bash
git add orderapp-remote/internal/application/catalog/service.go orderapp-remote/internal/application/catalog/service_test.go orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go
git commit -m "fix: clear untemplated product fields in catalog API"
```

### Task 4: Add repository defense and stop copying orphan industry fields

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go:713-727,933-963`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go:520-575,1010-1045,1158-1163`

- [ ] **Step 1: Reverse the old compatibility test and copy contract**

Replace the old no-template test with:

```go
func TestProductProductionConfigFieldsRequireIndustryTemplate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func normalizeProductProductionConfigFieldsAgainstTemplateTx", "func (r Repository) ListProductClassificationTemplates")
	if !strings.Contains(fn, "if templateID <= 0 {\n\t\treturn []catalogapp.ProductProductionConfigField{}, nil\n\t}") {
		t.Fatalf("product production fields must be empty without an industry template")
	}
}
```

Change `TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates` so `product_production_config_fields` is removed from required markers and added to forbidden markers. Add a list-response assertion:

```go
func TestProductProductionConfigListInitializesEmptyFields(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	fn := catalogRepositoryFunctionForTest(t, string(repository), "func (r Repository) ListProductProductionConfigs", "func (r Repository) GetProductProductionConfig")
	if !strings.Contains(fn, "row.Fields = []catalogapp.ProductProductionConfigField{}") {
		t.Fatalf("product production config responses must encode empty fields as []")
	}
}
```

- [ ] **Step 2: Run repository tests and capture RED**

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/catalog -run 'TestProductProductionConfigFieldsRequireIndustryTemplate|TestProductProductionConfigListInitializesEmptyFields|TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates' -count=1
```

Expected: tests fail because fields pass through, list rows start nil, and copy still inserts field rows.

- [ ] **Step 3: Implement repository defense and list shape**

Change the no-template branch to:

```go
if templateID <= 0 {
	return []catalogapp.ProductProductionConfigField{}, nil
}
```

After scanning each production config row and before appending it, initialize:

```go
row.Fields = []catalogapp.ProductProductionConfigField{}
out = append(out, row)
```

- [ ] **Step 4: Remove orphan-field copying**

Delete the entire `INSERT INTO ... product_production_config_fields ... SELECT` block from `CopyProduct`. Keep product master copying, audit insertion, derived SKU synchronization, and transaction handling unchanged.

- [ ] **Step 5: Run repository tests and capture GREEN**

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/catalog -run 'TestProductProductionConfigFieldsRequireIndustryTemplate|TestProductProductionConfigListInitializesEmptyFields|TestCopyProductArchiveCopiesOnlyMasterDataNotPriceOrBomTemplates' -count=1
```

Expected: all three tests pass.

- [ ] **Step 6: Commit repository enforcement**

```bash
git add orderapp-remote/internal/infrastructure/postgres/catalog/repository.go orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go
git commit -m "fix: prevent orphan product industry fields"
```

### Task 5: Stop legacy field backfill and add idempotent cleanup

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go:675-712`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go:1172-1235`

- [ ] **Step 1: Add a failing schema contract test**

Rename the existing backfill test to `TestProductProductionConfigSchemaBackfillsLegacyBOMAndCleansIndustryFields` and add:

```go
schemaSource := string(schema)
start := strings.Index(schemaSource, "func backfillProductProductionConfigs")
if start < 0 {
	t.Fatal("backfillProductProductionConfigs missing")
}
backfill := schemaSource[start:]
if strings.Contains(backfill, "jsonb_each_text") {
	t.Fatalf("legacy special_attrs_json must not create product industry fields")
}
for _, want := range []string{
	"cleanupProductProductionConfigIndustryFields",
	"DELETE FROM %[1]s.product_production_config_fields",
	"industry_field_template_id",
	"industry_field_templates",
	"industry_field_definitions",
	"to_regclass",
} {
	if !strings.Contains(backfill, want) {
		t.Fatalf("product industry field cleanup missing %q", want)
	}
}
```

Remove `jsonb_each_text` from the old required-marker list.

- [ ] **Step 2: Run the schema test and capture RED**

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/catalog -run TestProductProductionConfigSchemaBackfillsLegacyBOMAndCleansIndustryFields -count=1
```

Expected: the test finds `jsonb_each_text` and does not find cleanup SQL.

- [ ] **Step 3: Keep only production-config/BOM backfill**

Remove the `source_attrs`, `attr_rows`, and `INSERT INTO product_production_config_fields` SQL from `backfillProductProductionConfigs`. After executing the remaining config backfill, call:

```go
if _, err := pool.Exec(ctx, q); err != nil {
	return err
}
return cleanupProductProductionConfigIndustryFields(ctx, pool, schema)
```

- [ ] **Step 4: Implement idempotent cleanup with first-boot safety**

Add:

```go
func cleanupProductProductionConfigIndustryFields(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
DELETE FROM %[1]s.product_production_config_fields f
WHERE NOT EXISTS (
	SELECT 1 FROM %[1]s.product_production_configs c WHERE c.product_id=f.product_id
)
OR EXISTS (
	SELECT 1
	FROM %[1]s.product_production_configs c
	WHERE c.product_id=f.product_id
	  AND COALESCE(c.industry_field_template_id,0) <= 0
);
`, schema)); err != nil {
	return err
}

	var hasIndustryTemplates, hasIndustryDefinitions bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL`, schema+".industry_field_templates", schema+".industry_field_definitions").Scan(&hasIndustryTemplates, &hasIndustryDefinitions); err != nil {
		return err
	}
	if !hasIndustryTemplates || !hasIndustryDefinitions {
		return nil
	}

	_, err := pool.Exec(ctx, fmt.Sprintf(`
DELETE FROM %[1]s.product_production_config_fields f
WHERE EXISTS (
	SELECT 1
	FROM %[1]s.product_production_configs c
	WHERE c.product_id=f.product_id
	  AND COALESCE(c.industry_field_template_id,0) > 0
	  AND NOT EXISTS (
		SELECT 1
		FROM %[1]s.industry_field_templates t
		JOIN %[1]s.industry_field_definitions d ON d.template_id=t.id
		WHERE t.id=c.industry_field_template_id
		  AND lower(btrim(d.field_key))=lower(btrim(COALESCE(NULLIF(f.template_field_key,''),f.field_key)))
	  )
);
`, schema))
	return err
}
```

The `to_regclass` guard is required because catalog schema setup runs before manufacturing creates `industry_field_definitions` on a brand-new database. Existing environments already have the table and receive full mismatch cleanup; new databases cannot have legacy field rows because the legacy insert has been removed.

- [ ] **Step 5: Run the schema test and capture GREEN**

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres/catalog -run TestProductProductionConfigSchemaBackfillsLegacyBOMAndCleansIndustryFields -count=1
```

Expected: the test passes with no legacy field-generation marker.

- [ ] **Step 6: Commit the migration behavior**

```bash
git add orderapp-remote/internal/infrastructure/postgres/catalog/schema.go orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go
git commit -m "fix: clean legacy product industry fields"
```

### Task 6: Update PR/DEV contracts, requirements, manual, and acceptance evidence

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/support/dev_536_product_industry_template_only_test.go`
- Create: `orderapp-remote/docs/acceptance/2026-07-14-product-industry-template-only.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go:565-580`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md:697-703,848-860`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md:241-245,1160-1170,1458-end`
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md:65-95,154-172`
- Modify: `ACTIVE_REQUIREMENTS.md:7-35`

- [ ] **Step 1: Add a failing PR-536 support contract**

```go
package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev536ProductIndustryTemplateOnlyContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY",
			"DEV-536-FRONTEND-TEMPLATE-PROJECTION",
			"DEV-536-BACKEND-TEMPLATE-CONSTRAINT",
			"DEV-536-LEGACY-FIELD-CLEANUP",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", "无行业字段模板时字段必须为空"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", "模板外历史字段"},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {"取消行业字段模板会清空商品行业字段"},
		filepath.Join("docs", "acceptance", "2026-07-14-product-industry-template-only.md"): {"PR-536 商品行业字段仅来源于模板验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-536 marker %q", rel, want)
			}
		}
	}
}
```

- [ ] **Step 2: Run the support test and capture RED**

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1
```

Expected: PR-536 markers and the acceptance file are missing.

- [ ] **Step 3: Add PR/DEV rows to `req_store.go`**

```go
{table: "req_product", code: "PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", title: "商品行业字段只允许来自明确引用的行业字段模板", status: "review", assignee: "VA", evidence: "ACTIVE_REQUIREMENTS.md; ProductSettingsView.vue; docs/acceptance/2026-07-14-product-industry-template-only.md"},
{table: "req_dev", code: "DEV-536-FRONTEND-TEMPLATE-PROJECTION", title: "商品列表抽屉模板切换和保存只投影当前行业字段模板", status: "done", assignee: "Codex", evidence: "product-settings.js; ProductSettingsView.vue; product-settings.test.js"},
{table: "req_dev", code: "DEV-536-BACKEND-TEMPLATE-CONSTRAINT", title: "应用服务和仓储清空无模板字段且复制商品不制造孤立字段", status: "done", assignee: "Codex", evidence: "catalog/service.go; catalog/repository.go; product_settings_api_test.go"},
{table: "req_dev", code: "DEV-536-LEGACY-FIELD-CLEANUP", title: "停止旧属性字段回填并幂等清理无模板孤儿和模板外字段", status: "done", assignee: "Codex", evidence: "catalog/schema.go; catalog/repository_test.go"},
{table: "req_dev", code: "DEV-536-DOCS-ACCEPTANCE", title: "同步商品行业字段需求验收操作手册和交付证据", status: "done", assignee: "Codex", evidence: "OP_MANUAL_INVENTORY_MATERIALS.md; docs/ACCEPTANCE_TESTS.md; docs/acceptance/2026-07-14-product-industry-template-only.md"},
{table: "req_review", code: "REV-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", prCode: "PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", title: "验收：无模板商品没有行业字段且模板商品只保留模板字段", status: "todo", assignee: "VA", evidence: "docs/ACCEPTANCE_TESTS.md; docs/acceptance/2026-07-14-product-industry-template-only.md"},
```

- [ ] **Step 4: Replace the old requirement and copy semantics**

In PR-409 compatibility text, state:

```markdown
- PR-536 后，PR-409 的“无模板旧字段原样保存”兼容规则结束。商品未引用行业字段模板时字段必须为空；引用模板时只允许保存该模板定义的字段。旧 `roast_level`、`special_attrs_json` 继续保留为历史数据，但不得再回填成商品行业字段。
```

Add a PR-536 requirement section that specifies frontend projection, backend enforcement, idempotent cleanup, preserved history snapshots, and no template deletion. Update product-copy requirements to say industry field values are not copied because the current copy flow does not copy the template reference.

- [ ] **Step 5: Update the manual and acceptance checklist**

Use these exact user-facing rules:

```markdown
- 商品行业字段只有在商品档案配置中选择行业字段模板后才出现，字段定义完全来自该模板。未选择模板时不显示也不保存行业字段；取消行业字段模板会清空商品行业字段。
- 复制为商品档案不复制行业字段模板或行业字段值。需要行业字段时，在新商品档案配置中选择模板后重新填写。
```

Add `K78. 商品行业字段仅来源于模板（PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY）` with checks for no-template list/drawer/save behavior, exact template projection, legacy cleanup, copy behavior, preserved template definitions, preserved historical snapshots, and operation-log continuity.

- [ ] **Step 6: Write the acceptance record with observed evidence**

Create:

```markdown
# PR-536 商品行业字段仅来源于模板验收

日期：2026-07-14

## 根因

- 旧 schema 回填把 `roast_level` / `special_attrs_json` 写入 `product_production_config_fields`。
- 前后端允许 `industry_field_template_id=0` 时透传字段，复制商品还会复制没有模板引用的字段值。

## RED

- 前端回归证明无模板配置仍返回历史字段，并通过 `烘焙度 ↔ roast_level` 继承旧值。
- 应用/API 回归证明模板 ID 为 0 时字段仍进入仓储命令和响应。
- 仓储/schema 回归证明无模板字段仍被接受、复制和启动回填。

## GREEN

- 商品设置前端测试通过：无模板为空，有模板只保留精确模板键，列表/抽屉/保存使用同一投影。
- Catalog application、PostgreSQL repository 和 HTTP API 定向测试通过。
- Schema 合同确认旧属性不再生成商品行业字段，并包含幂等清理。
- 前端生产构建与 `scripts/verify_kferp.sh changed` 通过。

## 数据边界

- 清理目标仅为无模板、孤儿和模板外的 `product_production_config_fields`。
- 行业字段模板、`products.roast_level`、`products.special_attrs_json`、已发布价格表、历史订单和历史工单快照不删除。

## 部署

- 本次未部署；代码合并到 `develop` 后等待明确部署任务。实际部署前必须备份目标数据库。
```

- [ ] **Step 7: Complete ACTIVE_REQUIREMENTS evidence and run the support test GREEN**

Update PR-536 status to `verifying`, replace planned verifier lines with captured RED/GREEN commands, and keep `Deployment: not requested; do not deploy in this task`.

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestDev536ProductIndustryTemplateOnlyContracts -count=1
```

Expected: the support contract passes.

- [ ] **Step 8: Commit requirements and evidence**

```bash
git add ACTIVE_REQUIREMENTS.md orderapp-remote/internal/interfaces/http/support/req_store.go orderapp-remote/internal/interfaces/http/support/dev_536_product_industry_template_only_test.go orderapp-remote/docs/REQUIREMENTS.md orderapp-remote/docs/ACCEPTANCE_TESTS.md orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md orderapp-remote/docs/acceptance/2026-07-14-product-industry-template-only.md
git commit -m "docs: record template-only industry field workflow"
```

### Task 7: Run full verification, review, push, and merge to develop without deployment

**Files:**
- Modify if review finds a defect: only files already listed in Tasks 1-6

- [ ] **Step 1: Run targeted frontend and backend suites**

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js

cd ..
go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1
```

Expected: all packages and frontend tests pass with zero failures.

- [ ] **Step 2: Run frontend build and repository verifier**

```bash
cd orderapp-remote/frontend-vue-shell
npm run build

cd ../..
scripts/verify_kferp.sh changed
git diff --check
git status --short --branch
```

Expected: build and verifier exit 0; only the known Vite chunk-size and existing npm audit notices may remain; no uncommitted changes remain.

- [ ] **Step 3: Review the implementation against the written spec**

Check every requirement in `docs/superpowers/specs/2026-07-14-product-industry-fields-template-only-design.md`. Verify specifically that templates and historical snapshots are not deleted, `roast_level` / `special_attrs_json` are preserved, no deployment command appears, and every user-triggered save still reaches the existing product-production-config audit insertion.

Dispatch an independent code-review agent and resolve every blocking finding with an additional RED/GREEN cycle before integration.

- [ ] **Step 4: Bring the feature branch up to date and rerun focused checks**

```bash
git fetch origin
git merge --no-edit origin/develop

cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js

cd ..
go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1

cd ..
git diff --check
```

Expected: merge succeeds without unresolved conflicts and all focused checks pass.

- [ ] **Step 5: Push the verified feature branch**

```bash
git push -u origin codex/product-industry-template-only-20260714
```

Expected: the remote feature branch points to the verified local HEAD.

- [ ] **Step 6: Merge through a clean develop integration clone**

```bash
repo_url=$(git remote get-url origin)
merge_root=$(mktemp -d /private/tmp/kferp-product-industry-template-only-merge-20260714.XXXXXX)
git clone "$repo_url" "$merge_root/KFerp"
git -C "$merge_root/KFerp" switch develop
git -C "$merge_root/KFerp" pull --ff-only origin develop
git -C "$merge_root/KFerp" merge --no-ff origin/codex/product-industry-template-only-20260714 -m "merge: enforce template-only product industry fields"
git -C "$merge_root/KFerp" push origin develop
git -C "$merge_root/KFerp" rev-parse HEAD
git ls-remote origin refs/heads/develop
```

Expected: `origin/develop` advances to the clean merge commit and both final hashes match.

- [ ] **Step 7: Hand off without deployment**

Report the root cause, physical cleanup boundary, RED/GREEN commands, feature and develop commits, changed manuals, and `Deployment: not performed`. Retain the integration clone path until handoff so it can be inspected if needed.
