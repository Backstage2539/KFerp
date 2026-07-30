# Product Category Pagination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the product archive's global pager with independent pagination inside every product category.

**Architecture:** Keep the current client-side load and filter flow, but group the complete filtered parent-product list before slicing rows. A pure `skuGroupTableState` helper will normalize each group's page state and return the current rows independently; `ProductSettingsView.vue` will render one pager inside each expanded category and keep page state keyed by the stable business-group key.

**Tech Stack:** Vue 3 Composition API, Vite, Node.js built-in test runner, existing `PaginationControls` and pagination helpers.

---

## File map

- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`: own the pure per-category pagination calculation.
- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`: verify independent category state and the Vue source contract.
- `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`: change the product archive data flow, state, template, and category pager styling.
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`: document product-category pagination and session-state behavior.
- `ACCEPTANCE_TESTS.md`: add the user-visible acceptance criterion.

### Task 1: Pure category pagination state

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js:1-150`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js:70-105,2268-2300`

- [ ] **Step 1: Import the planned helper in the test file**

Add `skuGroupTableState` beside `skuTableState` in the named import from `./product-settings.js`.

```js
  skuGroupTableState,
  skuTableState,
```

- [ ] **Step 2: Write the failing independent-pagination tests**

Add these tests immediately after the existing `skuTableState keeps visible rows...` test:

```js
test('skuGroupTableState paginates every product category independently', () => {
  const coffeeRows = Array.from({ length: 12 }, (_, index) => ({
    id: `coffee-${index + 1}`,
    name: `咖啡豆 ${index + 1}`,
  }))
  const dripRows = Array.from({ length: 13 }, (_, index) => ({
    id: `drip-${index + 1}`,
    name: `挂耳咖啡 ${index + 1}`,
  }))

  const state = skuGroupTableState([
    { key: 'coffee', label: '咖啡豆', rows: coffeeRows },
    { key: 'drip', label: '挂耳咖啡', rows: dripRows },
  ], {
    coffee: { page: 2, pageSize: 10 },
    drip: { page: 1, pageSize: 10 },
  })

  assert.equal(state.groups[0].total, 12)
  assert.equal(state.groups[0].page, 2)
  assert.deepEqual(state.groups[0].rows.map((row) => row.id), ['coffee-11', 'coffee-12'])
  assert.equal(state.groups[1].total, 13)
  assert.equal(state.groups[1].page, 1)
  assert.deepEqual(state.groups[1].rows.map((row) => row.id), dripRows.slice(0, 10).map((row) => row.id))
  assert.deepEqual(state.pagination, {
    coffee: { page: 2, pageSize: 10 },
    drip: { page: 1, pageSize: 10 },
  })
  assert.equal(state.visibleRows.length, 12)
})

test('skuGroupTableState keeps full totals, clamps pages, and counts parent products only', () => {
  const parentRows = [{
    id: 1,
    name: '金色山脉',
    sku_rows: Array.from({ length: 6 }, (_, index) => ({ id: 100 + index })),
  }]
  const state = skuGroupTableState([
    { key: 'coffee', label: '咖啡豆', rows: parentRows },
    { key: 'empty', label: '空分类', rows: [] },
  ], {
    coffee: { page: 9, pageSize: 10 },
    empty: { page: 3, pageSize: 10 },
  })

  assert.equal(state.groups[0].total, 1)
  assert.equal(state.groups[0].page, 1)
  assert.equal(state.groups[0].rows.length, 1)
  assert.equal(state.groups[0].needsPagination, false)
  assert.equal(state.groups[1].total, 0)
  assert.equal(state.groups[1].page, 1)
  assert.equal(state.groups[1].needsPagination, false)
})
```

- [ ] **Step 3: Run the tests and capture RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: FAIL because `product-settings.js` does not export `skuGroupTableState`.

- [ ] **Step 4: Implement the pure helper**

Add this function after `skuTableState` in `product-settings.js`:

```js
export function skuGroupTableState(groups = [], paginationByGroup = {}, options = {}) {
  const sourceGroups = Array.isArray(groups) ? groups : []
  const sourcePagination = paginationByGroup && typeof paginationByGroup === 'object'
    ? paginationByGroup
    : {}
  const defaultPageSize = normalizePageSize(options.defaultPageSize)
  const pagination = {}
  const visibleRows = []

  const paginatedGroups = sourceGroups.map((group, index) => {
    const key = String(group?.key || `sku-group-${index}`)
    const sourceRows = Array.isArray(group?.rows) ? group.rows : []
    const requested = sourcePagination[key] || {}
    const pageSize = normalizePageSize(requested.pageSize || defaultPageSize)
    const page = clampPage(requested.page, sourceRows.length, pageSize)
    const rows = slicePageRows(sourceRows, { page, pageSize })
    pagination[key] = { page, pageSize }
    visibleRows.push(...rows)
    return {
      ...group,
      key,
      total: sourceRows.length,
      page,
      pageSize,
      needsPagination: sourceRows.length > pageSize,
      rows,
    }
  })

  return {
    groups: paginatedGroups,
    pagination,
    visibleRows,
    total: sourceGroups.reduce((sum, group) => sum + (Array.isArray(group?.rows) ? group.rows.length : 0), 0),
  }
}
```

- [ ] **Step 5: Run the targeted tests and capture GREEN**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: all `product-settings.test.js` tests PASS.

- [ ] **Step 6: Commit the pure helper**

```bash
git add orderapp-remote/frontend-vue-shell/src/lib/product-settings.js \
  orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js
git commit -m "fix: paginate product rows within categories"
```

### Task 2: Render independent pagers inside product categories

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue:210-305,1770-1840,1955-1975,2295-2340,2425-2445,2495-2520,3010-3070,5365-5385,7285-7400,7810-7830`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js:3290-3350`

- [ ] **Step 1: Rewrite the Vue source-contract assertions first**

In the compact product archive source-contract test, replace the assertions for the global `skuVisibleTableState`, `skuPaginationKey`, `skuPage`, and `skuPageSize` with the following assertions:

```js
  assert.match(template, /v-for="group in displaySkuGroups"/)
  assert.match(template, /\{\{ group\.total \}\} 款/)
  assert.match(template, /v-for="row in group\.rows"/)
  assert.match(template, /group\.needsPagination[\s\S]*<PaginationControls[\s\S]*handleSkuGroupPaginationChange\(group\.key, \$event\)/)
  assert.doesNotMatch(productArchiveWorkspace, /:key="skuPaginationKey"/)
  assert.doesNotMatch(productArchiveWorkspace, /:total="skuDisplayTotal"/)
  assert.match(script, /const filteredSkuRows = computed\(\(\) => filterSkuRows\(currentSkuSourceRows\.value, normalizedSkuFilters\.value\)\)/)
  assert.match(script, /const fullDisplaySkuGroups = computed\(\(\) => groupRowsByBusinessGroupTemplate\(filteredSkuRows\.value, \{/)
  assert.match(script, /const groupedSkuTableState = computed\(\(\) => skuGroupTableState\(fullDisplaySkuGroups\.value, skuGroupPagination\.value, \{/)
  assert.match(script, /const displaySkuGroups = computed\(\(\) => groupedSkuTableState\.value\.groups\)/)
  assert.match(script, /const displaySkuRows = computed\(\(\) => groupedSkuTableState\.value\.visibleRows\)/)
  assert.match(script, /function handleSkuGroupPaginationChange\(groupKey, \{ page, pageSize \}\)/)
  assert.match(script, /watch\(skuFilters, resetSkuGroupPages, \{ deep: true \}\)/)
```

- [ ] **Step 2: Run the source-contract test and capture RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: FAIL because `ProductSettingsView.vue` still renders one global pager and computes a global sliced page.

- [ ] **Step 3: Change imports and component state**

In `ProductSettingsView.vue`, import `filterSkuRows`, `primaryCategoryOptions`, `secondaryCategoryOptions`, and `skuGroupTableState` from `../lib/product-settings`. Keep `normalizePageSize` from `../lib/pagination`.

Replace:

```js
const skuPage = ref(1)
const skuPageSize = ref(10)
```

with:

```js
const DEFAULT_SKU_GROUP_PAGE_SIZE = 10
const skuGroupPagination = ref({})
```

- [ ] **Step 4: Replace global slicing with filter → group → per-group slice**

Replace the `skuVisibleTableState` through `editableDisplaySkuRows` computed block with:

```js
const normalizedSkuFilters = computed(() => normalizeVisibleSkuFilters(skuFilters.value, currentSkuSourceRows.value))
const filteredSkuRows = computed(() => filterSkuRows(currentSkuSourceRows.value, normalizedSkuFilters.value))
const skuPrimaryCategoryOptions = computed(() => primaryCategoryOptions(currentSkuSourceRows.value))
const skuSecondaryCategoryOptions = computed(() => secondaryCategoryOptions(currentSkuSourceRows.value, normalizedSkuFilters.value.primaryCategory))
const hasActiveSkuFilters = computed(() => Boolean(
  normalizedSkuFilters.value.query
    || normalizedSkuFilters.value.primaryCategory
    || normalizedSkuFilters.value.secondaryCategory,
))
const skuDisplayKey = computed(() => [
  skuContextCustomerID.value,
  filteredSkuRows.value.length,
  selectedProductGroupTemplateID.value,
  normalizedSkuFilters.value.query || '',
  normalizedSkuFilters.value.primaryCategory || '',
  normalizedSkuFilters.value.secondaryCategory || '',
].join(':'))
const skuTableKey = computed(() => `${skuDisplayKey.value}:table`)
const fullDisplaySkuGroups = computed(() => groupRowsByBusinessGroupTemplate(filteredSkuRows.value, {
  template: selectedProductGroupTemplate.value,
  assignments: businessGroupAssignments.value,
  usageKey: 'product_catalog',
  objectKey: 'product',
  objectIDForRow: (row) => Number(row.id || 0),
}))
const groupedSkuTableState = computed(() => skuGroupTableState(fullDisplaySkuGroups.value, skuGroupPagination.value, {
  defaultPageSize: DEFAULT_SKU_GROUP_PAGE_SIZE,
}))
const displaySkuGroups = computed(() => groupedSkuTableState.value.groups)
const displaySkuRows = computed(() => groupedSkuTableState.value.visibleRows)
const editableDisplaySkuRows = computed(() => displaySkuRows.value.filter(canEditSkuRow))
```

Delete the old later `displaySkuGroups` computed block so grouping happens only once, before pagination.

- [ ] **Step 5: Add page-state normalization and handlers**

Replace `syncVisibleSkuTableState` and `handleSkuPaginationChange` with:

```js
function syncVisibleSkuTableState() {
  const normalizedFilters = normalizeSkuFiltersForCurrentRows()
  if (JSON.stringify(normalizedFilters) !== JSON.stringify(skuFilters.value)) {
    skuFilters.value = normalizedFilters
  }
}

function syncSkuGroupPaginationState() {
  const normalized = groupedSkuTableState.value.pagination
  if (JSON.stringify(normalized) !== JSON.stringify(skuGroupPagination.value)) {
    skuGroupPagination.value = normalized
  }
}

function resetSkuGroupPages() {
  skuGroupPagination.value = Object.fromEntries(Object.entries(skuGroupPagination.value).map(([key, value]) => [key, {
    page: 1,
    pageSize: normalizePageSize(value?.pageSize || DEFAULT_SKU_GROUP_PAGE_SIZE),
  }]))
}

function handleSkuGroupPaginationChange(groupKey, { page, pageSize }) {
  skuGroupPagination.value = {
    ...skuGroupPagination.value,
    [String(groupKey || '')]: {
      page: Number(page || 1),
      pageSize: normalizePageSize(pageSize || DEFAULT_SKU_GROUP_PAGE_SIZE),
    },
  }
}
```

Add:

```js
watch(fullDisplaySkuGroups, syncSkuGroupPaginationState, { deep: true, immediate: true })
watch(skuFilters, resetSkuGroupPages, { deep: true })
```

Remove `skuPage` and `skuPageSize` from the existing table-state watcher. On customer or group-template changes, assign `skuGroupPagination.value = {}` instead of setting `skuPage.value = 1`.

- [ ] **Step 6: Persist the category page map in the session draft**

In `saveProductSettingsDraft`, replace `skuPage` and `skuPageSize` with:

```js
    skuGroupPagination: skuGroupPagination.value,
```

In `restoreProductSettingsDraft`, replace the two global assignments with:

```js
  skuGroupPagination.value = draft.skuGroupPagination && typeof draft.skuGroupPagination === 'object'
    ? draft.skuGroupPagination
    : {}
```

- [ ] **Step 7: Move the pager into every category**

In the category header, replace `{{ group.rows.length }} 款` with `{{ group.total }} 款`.

Inside `v-if="!isProductClassificationGroupCollapsed(group.key)"`, after the product-row loop, add:

```vue
<tr v-if="group.needsPagination" class="classification-pagination-row">
  <td :colspan="12">
    <PaginationControls
      :key="`${group.key}:${group.page}:${group.pageSize}:${group.total}`"
      :page="group.page"
      :page-size="group.pageSize"
      :total="group.total"
      :disabled="loading"
      @change="handleSkuGroupPaginationChange(group.key, $event)"
    />
  </td>
</tr>
```

Delete the global `PaginationControls` block below the table.

Add scoped styles beside `.classification-group-row`:

```css
.classification-pagination-row td {
  background: #fbf9f5;
  padding: 8px 16px 12px;
}
.classification-pagination-row :deep(.list-pagination-controls) { margin-top: 0; }
```

- [ ] **Step 8: Run the targeted frontend tests and capture GREEN**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
```

Expected: all tests PASS, including the source-contract assertions.

### Task 3: Documentation, acceptance, and integration verification

**Files:**
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md:64-75,110`
- Modify: `ACCEPTANCE_TESTS.md:256-285`

- [ ] **Step 1: Update the product operations manual**

After the product-list paragraph that starts with `` `商品档案` 是库存、销售、成本...``, add:

```markdown
- 商品档案先按当前搜索和状态筛选生成完整商品分组，再在每个分类内部独立分页，默认每类每页 10 款。分类标题数量是该分类筛选后的完整父商品数；咖啡豆等某一分类翻页不会改变挂耳咖啡、生豆、速溶咖啡或未分类的页码。只有父商品参与分页，销售规格模板派生的子 SKU 仍收在父商品的规格明细里。分类收起再展开保留当前页；搜索、状态、当前视图或分组模板变化后，各分类回到第 1 页。表头全选只选择当前实际显示的父商品行。
```

Update the PR-316 session-draft paragraph to say `各分类筛选页码` instead of the singular `筛选页码`.

- [ ] **Step 2: Add the acceptance criterion**

Under `## D. 商品档案（/app/products）`, add:

```markdown
- [ ] 商品档案按分组模板展示时，每个分类独立分页并显示该分类完整父商品总数；任一分类翻页或修改每页条数不影响其他分类，表格底部不再出现跨分类的全局分页；规格子 SKU 仍折叠在父商品内且不单独占分页条数。
```

- [ ] **Step 3: Run documentation and diff checks**

```bash
git diff --check
rg -n "每个分类独立分页|全局分页|各分类筛选页码" \
  orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md ACCEPTANCE_TESTS.md
```

Expected: `git diff --check` exits 0 and all three phrases are found.

- [ ] **Step 4: Run the frontend build**

```bash
cd orderapp-remote/frontend-vue-shell
npm ci
npm run build
```

Expected: build exits 0; the existing Vite chunk-size warning is acceptable.

- [ ] **Step 5: Run the final targeted verifier**

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
cd ../../..
scripts/verify_kferp.sh changed
git diff --check
```

Expected: targeted tests, changed verifier, and diff check all exit 0.

- [ ] **Step 6: Commit the Vue and documentation changes**

```bash
git add orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue \
  orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js \
  orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md \
  ACCEPTANCE_TESTS.md
git commit -m "fix: scope product pagination to categories"
```

- [ ] **Step 7: Rebase on current develop and re-run verification**

```bash
git fetch origin
git rebase origin/develop
cd orderapp-remote/frontend-vue-shell
node --test src/lib/product-settings.test.js
npm run build
cd ../../..
scripts/verify_kferp.sh changed
git diff --check
```

Expected: rebase succeeds and every verifier exits 0.

- [ ] **Step 8: Push the feature branch and merge without deployment**

```bash
git push -u origin codex/product-category-pagination-20260714
```

Use this guarded integration sequence to merge into `develop` without deployment:

```bash
integration_path=/private/tmp/kferp-product-category-pagination-integration-20260714
integration_branch=codex/integrate-product-category-pagination-20260714
git fetch origin
git worktree add -b "$integration_branch" "$integration_path" origin/develop
integration_base=$(git -C "$integration_path" rev-parse origin/develop)
git -C "$integration_path" merge --no-ff codex/product-category-pagination-20260714 \
  -m "merge: product category pagination"
cd "$integration_path/orderapp-remote/frontend-vue-shell"
npm ci
node --test src/lib/product-settings.test.js
npm run build
cd "$integration_path"
git fetch origin
test "$(git rev-parse origin/develop)" = "$integration_base"
git push origin HEAD:develop
```

Expected: the feature branch is merged into the same verified `origin/develop` base, the targeted test and build pass on the merge commit, and the push fast-forwards `develop`. If the base comparison fails or the push is rejected, remove the temporary integration worktree, update the feature branch from the new `origin/develop`, rerun verification, and repeat with a new integration branch. Do not run `deploy_orderapp.sh`.
