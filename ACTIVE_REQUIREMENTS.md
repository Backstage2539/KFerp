# ACTIVE_REQUIREMENTS

Purpose: short-lived coordination for Codex workflows. Keep active requirement ids, branches, verifier commands, deployment ownership, and unresolved blockers here so future sessions do not have to recover this from chat history.

This is not long-term memory. Move durable product/deployment decisions to `MEMORY.md` or source docs, then remove stale entries from this file.

## Active

### PR-414-MATERIALS-CLASSIFICATION-INDUSTRY-SETTINGS
- Branch: codex/materials-classification-industry-settings-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 商品价格表候选不再因旧豆单 metadata code 缺失隐藏挂耳/速溶等商品；普通仓库也可打开仓库设置并显示空状态；物料档案支持分类大类/小类、新建、编辑、失效、全局单位字典、单数量库存补录；咖啡生豆硬编码属性改由行业字段模板承接，行业字段模板移动到 设置 / 行业设置。
- DEV:
  - DEV-414-PRICE-LIST-METADATA-CANDIDATES：商品价格表候选、预览和 PDF 选择不再用旧 bean-list code 作为可见门槛，无计价方式只提示不隐藏。
  - DEV-414-WAREHOUSE-SETTINGS-EMPTY：普通仓库可打开仓库设置抽屉，无配置项时展示空状态；客户上下文仍隐藏内部设置入口。
  - DEV-414-MATERIALS-CLASSIFICATION-CRUD：物料分类大类/组内分类 schema/API/Vue 交互，物料支持新建、编辑、失效和操作日志。
  - DEV-414-MATERIALS-UNIT-STOCK-QTY：物料单位来自全局单位字典，物料补录和库存调整新前端只写 `target_qty/unit_code`，旧 `target_g/target_units` 兼容。
  - DEV-414-MATERIALS-INDUSTRY-FIELDS：物料绑定行业字段模板并保存字段值，旧咖啡生豆属性保留兼容 fallback。
- Verifier:
  - RED: targeted frontend/materials and Go materials tests initially failed on missing menu move, material classification schema/API and single quantity support.
- GREEN: `node --test src/lib/materials-ui.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` passed 33/33; targeted Go materials/stock/costing/support packages passed; `go test ./...` passed; `npm run build` passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-materials-classification-industry-settings.md`; requirement seed updated in `orderapp-remote/internal/interfaces/http/support/req_store.go`.
- Integration: feature commit `0649aa30` pushed to `origin/codex/materials-classification-industry-settings-20260604` and fast-forward merged/pushed to `origin/develop=0649aa30507ae4fed33b56b282e116bb86b53c4f`.
- Deployment: development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604232739`.
- Smoke: containers running (`erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert`); unauthenticated GET `/app/` returned 303 to `/app/orders`; GET `/app/vue-shell` returned 200; protected `/app/api/materials` and `/app/api/req/product` returned 401 without login; deployed docs/source contain `PR-414-MATERIALS-CLASSIFICATION-INDUSTRY-SETTINGS`, `行业设置`, `当前仓库暂无可配置项`, `target_qty`; development DB has `material_classification_groups`, `material_classification_assignments`, `material_industry_field_values`, and `materials.industry_field_template_id`.
- Last update: 2026-06-04 Asia/Shanghai

### FIX-20260604-BEAN-LIST-UNCATEGORIZED-DISPLAY
- Branch: codex/bom-group-tabs-industry-layout-20260603
- Owner/session: Codex / 2026-06-04
- Status: fixing (not yet merged/deployed)
- Scope: 商品价格表对未匹配硬编码豆单元数据且无产品分类的商品（如速溶咖啡、挂耳咖啡）也生成默认豆单展示（未分类 + product_id 编号），避免 Vue 前端因 `beanMeta.code` 为空将其过滤掉。
- Root cause: `CalculateProduct` 中的 `hasSkuCategoryBeanListMetadata` 门控导致没有产品分类的商品无法回退到 `customerCategoryBeanListDisplay` 生成默认展示码。
- Fix: 将 `else if hasSkuCategoryBeanListMetadata(in)` 改为 `else`，使所有无硬编码元数据的商品都通过 `customerCategoryBeanListDisplay` 生成回退展示（该函数本身就支持无分类场景：`categoryName == ""` 时生成"未分类"分组和 `product_category_position.product_id` 格式编号）。
- DEV:
  - DEV-FIX-BEAN-LIST-UNCATEGORIZED：`CalculateProduct` 中移除 `hasSkuCategoryBeanListMetadata` 门控，始终为无硬编码元数据的商品回退到 `customerCategoryBeanListDisplay`。
- Verifier:
  - RED: `go test ./internal/domain/costing -run TestCalculateProductGeneratesDefaultBeanListDisplayForUncategorizedProducts -count=1` — 修复前 `CommercialBeanList.Code` 和 `DripBeanList.Code` 为空。
  - GREEN: `go test ./internal/domain/costing -count=1` passed (incl. new test); `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` passed.
  - Frontend: unaffected (Vue 端已有基于 `beanMeta.code` 的过滤逻辑，后端提供码后即可展示)。
  - Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: 无用户流程变化；无需更新操作手册。
- Last update: 2026-06-04 Asia/Shanghai

### PR-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG
- Branch: codex/product-create-null-production-config-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 修复新建商品档案成功后自动打开“商品档案配置”抽屉时，商品尚无 `product_production_configs` 行导致前端读取 `expected_loss_rate` 空值崩溃的问题。
- DEV:
  - DEV-413-PRODUCT-CREATE-NULL-CONFIG-GUARD：把商品生产配置表单构造逻辑抽为可测试 helper，并兼容 `null`/缺失生产配置行；新商品仍使用商品档案行默认值打开配置抽屉。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed because `buildProductProductionConfigForm` was not exported and null config was not guarded.
  - Frontend: `node --test src/lib/product-settings.test.js` passed 109/109.
  - API/backend: `go test ./internal/interfaces/http/support -run 'TestDev413|TestDev408' -count=1` passed.
  - Frontend/build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
  - Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: no workflow change; update requirements, acceptance record, and requirement seed only.
- Integration: feature commit `6a0acc49` pushed to `origin/codex/product-create-null-production-config-20260604` and fast-forward merged/pushed to `origin/develop=6a0acc4979f787f6b95de5524fac69a7ae0b7165`.
- Deployment: development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604212529`.
- Smoke: containers running (`erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert`); unauthenticated GET `/app/` returned 303 to `/app/orders`; GET `/app/vue-shell/` returned 200; authenticated requirement API returned 200 and exposes `PR-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG`; remote source/docs contain `buildProductProductionConfigForm` and the PR-413 acceptance record.
- Last update: 2026-06-04 Asia/Shanghai

### PR-412-CLASSIFICATION-CONFIG-TEMPLATE-INHERITANCE
- Branch: codex/classification-config-template-inheritance-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 分类模板和分类项不再直接引用阶梯价模板和单位模板，改为引用商品配置模板；商品价格表/成本输入按 `商品引用模板 > 子类引用模板 > 大类引用模板 > 旧兼容字段` 读取计价、单位和价格表规则。
- DEV:
  - DEV-412-CLASSIFICATION-TEMPLATE-CONFIG-REFERENCE：分类模板和分类项 schema/API/repository/Vue 表单支持 `product_config_template_id`，新 UI 下线分类直接阶梯价/单位模板引用，并提示商品配置模板可以被商品覆盖。
  - DEV-412-PRICE-LIST-CONFIG-INHERITANCE：商品价格表和成本输入读取分类项/分类模板引用的商品配置模板，放在商品/客户商品配置模板之后、旧直接字段之前。
  - DEV-412-MANUAL-DOCS：更新商品配置和分类模板、商品价格表手册、需求、验收和 acceptance 记录。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed on old classification gradient/unit UI and old warning helper; targeted Go tests failed on missing `ProductConfigTemplateID`, missing classification config SQL join, and missing PR-412 seed.
  - Frontend: `node --test src/lib/product-settings.test.js` passed 108/108; broader target `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/view-routing.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` passed 178/178.
  - API/backend: targeted catalog/costing/support tests passed; `go test ./...` passed.
  - Frontend/build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
  - Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
  - Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-classification-config-template-inheritance.md`
- Integration: feature commit `f4d4ef4c` pushed to `origin/codex/classification-config-template-inheritance-20260604` and fast-forward merged/pushed to `origin/develop=f4d4ef4c26e899fc1112a24b69c353c6b8361a4a`.
- Deployment: development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604210721`.
- Smoke: containers running (`erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert`); unauthenticated GET `/app/` returned 303 to `/app/orders`; GET `/app/vue-shell/` returned 200; protected `/app/api/req/product` and `/app/api/product-settings` returned 401 without login; deployed docs contain `PR-412-CLASSIFICATION-CONFIG-TEMPLATE-INHERITANCE`; deployed Vue bundle contains `模板默认商品配置模板`, `分类项商品配置模板`, and `product_config_template_id`.
- Last update: 2026-06-04 Asia/Shanghai

### PR-411-CUSTOMER-PRODUCT-CONFIG-TEMPLATE-PRICING
- Branch: codex/customer-product-config-template-pricing-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 客户商品名统一更名为客户商品；客户商品配置不再直接维护阶梯价模板和单位模板，只选择商品配置模板，默认继承绑定商品档案的商品配置模板；商品配置模板中删除独立阶梯价模板配置，只在价格表生成规则选择“按阶梯价模板”时展示和保存阶梯价模板。
- DEV:
  - DEV-411-CUSTOMER-PRODUCT-RENAME：商品与配方菜单、客户商品列表、抽屉、提示和手册统一使用“客户商品”。
  - DEV-411-ALIAS-CONFIG-TEMPLATE：客户商品 API/仓储/前端新增 `product_config_template_id`，新 UI 停止写客户商品直接 `gradient_template_id` / `unit_template_id`。
  - DEV-411-PRICE-LIST-CONFIG-TEMPLATE-SOURCE：客户范围商品价格表取价顺序改为客户商品配置模板 → 商品档案配置模板 → 旧直接字段/旧规则 fallback。
  - DEV-411-PRODUCT-CONFIG-PRICING-RULE：商品配置模板只在计价方式为“按阶梯价模板”时选择阶梯价模板，固定单价/成本加成保存时清空阶梯价模板。
  - DEV-411-MANUAL-DOCS：更新商品、客户商品和商品价格表手册、需求、验收和 acceptance 记录。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed because `productConfigTemplateNeedsGradientTemplate` was missing; targeted Go tests failed because customer product aliases lacked `product_config_template_id`, costing SQL still prioritized direct alias templates, and PR-411 seed was absent.
- GREEN: `node --test src/lib/product-settings.test.js` passed 108/108; broader frontend target `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/view-routing.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` passed 178/178.
- API/backend: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1` passed.
- Full backend: `go test ./...` in `orderapp-remote` passed.
- Frontend/build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
- Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-customer-product-config-template-pricing.md`
- Integration: feature commit `e357adeb` pushed to `origin/codex/customer-product-config-template-pricing-20260604` and fast-forward merged/pushed to `origin/develop=e357adebaa31d6db790dd931925cabe2edf1b8d5`.
- Deployment: first attempts were blocked by SSH closing connections before key exchange; retried successfully and deployed development stack with `./deploy_orderapp.sh` at `origin/develop=06dd6c9f627fc11705b8a94b1790f8d196cae69e`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604202755`.
- Smoke: containers running (`erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert`); unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell/` returned 200; requirement API exposes `PR-411-CUSTOMER-PRODUCT-CONFIG-TEMPLATE-PRICING`; authenticated `/app/api/product-settings` and `/app/api/customer-product-aliases?active=all&q=` returned 200; remote source contains `product_config_template_id`, `客户商品`, and `productConfigTemplateNeedsGradientTemplate`.
- Last update: 2026-06-04 Asia/Shanghai

### PR-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY
- Branch: codex/customer-alias-rename-price-display-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 客户商品名配置中把旧“品牌名”改为“重命名”；客户商品名列表和客户商品价格表都优先展示重命名后的名称；客户商品名列表删除品牌名列；商品档案列表展示稳定商品编号而不是列表序号。
- DEV:
  - DEV-410-ALIAS-RENAME-UI：客户商品名列表删除品牌名列，编辑抽屉字段改为“重命名”，列表名称显示 `重命名 > 客户商品名`。
  - DEV-410-PRICE-LIST-RENAME-SOURCE：客户范围商品价格表候选和 PDF/发布内容优先使用客户商品名重命名值。
  - DEV-410-PRODUCT-CODE-DISPLAY：商品档案列表显示 `product_code/SKU-000xxx` 稳定商品编号，不再显示分类内序号。
  - DEV-410-MANUAL-DOCS：更新商品档案、客户商品名和商品价格表手册、需求、验收记录。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed before `customerAliasEffectiveDisplayName/productCodeLabel` existed; `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsUsesCustomerAliasRenameAsCustomerDisplayName -count=1` failed before costing SQL used rename first.
- GREEN: `node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js` passed 128/128; broader frontend target `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/view-routing.test.js` passed 147/147.
- API/backend: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1` passed.
- Frontend/build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
- Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-customer-alias-rename-price-display.md`
- Deployment: feature commit `79f53f17` pushed to `origin/codex/customer-alias-rename-price-display-20260604` and fast-forward merged to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604192223`.
- Smoke: containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` 200; requirement API exposes `PR-410-CUSTOMER-ALIAS-RENAME-PRICE-DISPLAY`; authenticated `/app/api/product-settings` and `/app/api/customer-product-aliases?active=all&q=` returned 200; remote Vue bundle contains `重命名` and `customerAliasEffectiveDisplayName`.
- Last update: 2026-06-04 Asia/Shanghai

### PR-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG
- Branch: codex/customer-alias-pricing-bom-config-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 客户商品名支持客户侧阶梯价模板和单位模板覆盖；商品档案配置绑定生产 BOM 时只显示有效 BOM、可按 BOM 编号/名称模糊搜索并显示最新版本号；修复商品档案修改 BOM 时旧行业字段触发 `industry_field_template_id required for product information fields` 的保存错误。
- DEV:
  - DEV-409-CUSTOMER-ALIAS-PRICING-UNIT：客户商品名 API、仓储、前端抽屉和列表增加阶梯价模板/单位模板覆盖字段，操作日志记录模板选择。
  - DEV-409-PRICE-LIST-ALIAS-OVERRIDE：商品价格表/成本输入在客户范围优先读取客户商品名覆盖的阶梯价模板和单位模板，再回退商品档案配置。
  - DEV-409-PRODUCT-BOM-SELECTOR：商品档案配置抽屉的生产 BOM 选择器改为可搜索有效 BOM，并在选项中显示 BOM 编号、名称和版本号。
  - DEV-409-INDUSTRY-FIELD-LEGACY-SAVE：无行业字段模板时允许旧商品生产配置字段原样保存，避免仅修改 BOM 绑定时报错。
  - DEV-409-MANUAL-DOCS：更新商品档案、客户商品名、商品价格表相关手册、需求、验收和 acceptance 记录。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed before active BOM SearchableSelect/version markers existed; `go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing -count=1` failed before alias pricing/unit persistence and price-list source markers existed.
  - GREEN: `node --test src/lib/product-settings.test.js` passed 104/104; `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 123/123.
  - API/backend: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1` passed.
  - Frontend/build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
  - Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-customer-alias-pricing-bom-config.md`
- Deployment: feature commit `47bbcdac` pushed to `origin/codex/customer-alias-pricing-bom-config-20260604` and fast-forward merged to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development`. Initial backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604183505`.
- Smoke: containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; requirement API exposes `PR-409-CUSTOMER-ALIAS-PRICING-BOM-CONFIG`; authenticated `/app/api/product-settings`、`/app/api/customer-product-aliases?active=all&q=`、`/app/api/production-boms?status=all` returned 200.
- Last update: 2026-06-04 Asia/Shanghai

### PR-408-PRODUCT-CREATE-CONFIG-DRAWER
- Branch: codex/product-create-config-drawer-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 修复新增商品档案后配置入口断掉的问题。创建商品档案成功后，前端使用 `/api/product-settings/skus` 返回值和重载后的商品列表定位新商品，并自动打开“商品档案配置”抽屉；后续点击商品名仍进入同一配置入口。
- DEV:
  - DEV-408-PRODUCT-CREATE-OPEN-CONFIG：新增 `resolveCreatedProductForConfig` helper，`createSku` 成功后重载列表并打开新商品配置抽屉。
  - DEV-408-MANUAL-DOCS：更新商品档案手册、需求、验收和 acceptance 记录。
- Verifier:
  - RED: `node --test src/lib/product-settings.test.js` failed because `resolveCreatedProductForConfig` was not exported and `createSku` did not use the create response to open the config drawer.
  - GREEN so far: `node --test src/lib/product-settings.test.js` passed 102/102.
  - Broader GREEN: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 121/121; `go test ./internal/interfaces/http/catalog -run TestProductSettingsAPICreatesUnifiedSKUWithoutLegacyFields -count=1` passed; `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0.
  - Seed follow-up GREEN: `go test ./internal/interfaces/http/support -run TestDev408 -count=1` passed; `go test ./internal/interfaces/http/support -count=1` passed; `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-product-create-config-drawer.md`
- Deployment: feature commit `471a2df3` and seed follow-up `57f660d9` pushed to `origin/codex/product-create-config-drawer-20260604`; fast-forward merged to `origin/develop=57f660d94ff9c905d3c2beb2d3f2d6eee349b27e`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604175632`.
- Smoke: containers running; unauthenticated GET `/app/` returned 303; authenticated `/app/vue-shell` returned 200; authenticated `/app/api/product-settings` returned 200; requirement API exposes `PR-408-PRODUCT-CREATE-CONFIG-DRAWER`; remote source contains `resolveCreatedProductForConfig`.
- Last update: 2026-06-04 Asia/Shanghai

### PR-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT
- Branch: codex/production-bom-categories-version-edit-20260604
- Owner/session: Codex / 2026-06-04
- Status: merged and deployed to development
- Scope: 生产 BOM 分组 Tab 作为大组，自定义大组内增加组内分类；BOM 同时只能属于一个大组和一个组内分类，跨大组移动清空小分类；配方比例和物料编辑归属 BOM 版本，新建 BOM 默认生成 V001 草稿，已发布版本只读，复制为新版草稿后编辑。
- DEV:
  - DEV-407-BOM-GROUP-CATEGORIES-DATA-API：新增 `production_bom_group_categories` 和 `production_boms.group_category_id`，补组内分类 CRUD、删除分类回组内未分类、跨大组清空小分类和操作日志。
  - DEV-407-BOM-VERSION-DRAFT-RECIPE：新建生产 BOM 初始版本改为 `V001 draft`，空初始已发布版本安全修复为草稿；`GET /api/production-boms/:id?version_id=...` 按选中版本返回配方明细。
  - DEV-407-BOM-VUE-GROUP-CATEGORY-UX：生产 BOM 自定义大组 Tab 下按组内分类分组展示，支持新增/改名/删除小分类，勾选 BOM 移动到小分类；BOM 抽屉显示大组和组内分类。
  - DEV-407-BOM-VUE-VERSION-RECIPE-UX：BOM 版本区下方展示配方明细、合计比例和保存组件；草稿可编辑，已发布版本显示只读提示，复制新版草稿后自动选中。
  - DEV-407-MANUAL-DOCS：更新生产 BOM 手册、需求、验收和 acceptance 证据。
- Verifier:
  - RED: `node --test src/lib/bom.test.js` failed on missing group category/version recipe markers; `go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1` failed before new category types/schema existed.
  - GREEN so far: `node --test src/lib/bom.test.js` passed 10/10; `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1` passed; `npm run build` passed with existing chunk-size warning.
  - Broader GREEN: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 120/120; `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1` passed; `go test ./...` in `orderapp-remote` passed; `npm run build` in `frontend-vue-shell` passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0.
  - Seed follow-up GREEN: `go test ./internal/interfaces/http/support -run 'TestDev407|TestDev271|TestDev389' -count=1` passed; `go test ./internal/interfaces/http/support -count=1` passed; `scripts/verify_kferp.sh changed` exited 0.
- Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-04-production-bom-group-categories-version-edit.md`
- Deployment: feature commit `a56fe8f5` pushed to `origin/codex/production-bom-categories-version-edit-20260604` and fast-forward merged to `develop`; seed/evidence follow-up `912fa6d3` pushed to `origin/develop=912fa6d31bd1092408200142546486d7066f7270`; development stack deployed with `./deploy_orderapp.sh development`. Runtime backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604150219`.
- Smoke: containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` 200; authenticated `/app/api/production-boms?status=all` 200; authenticated `/app/api/production-bom-groups` 200; requirement API exposes `PR-407-PRODUCTION-BOM-GROUP-CATEGORIES-VERSION-EDIT`; remote source/docs contain `production_bom_group_categories`, `groupProductionBomRowsByInnerCategory`, and `V001 草稿`.
- Last update: 2026-06-04 Asia/Shanghai

### PR-406-BOM-PRODUCT-ALIAS-LAYOUT
- Branch: codex/bom-product-alias-layout-20260603
- Follow-up branch: codex/unbound-production-bom-recipe-detail-20260604
- Current follow-up branch: codex/production-bom-independent-list-20260604
- Detail-return follow-up branch: codex/bom-detail-product-return-20260604
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 生产 BOM 删除顶部 SKU归属/商品选择并统一为生产 BOM 独立配方库；商品档案和客户商品名删除旧 SKU归属/旧客户 SKU 收敛检查，分类操作收敛为 Tab 行右侧“增加分类 / 移动到分类”可搜索下拉；客户商品名新建改为抽屉并对绑定商品失效标红。本轮 follow-up 修复：生产 BOM 列表不能再混入商品档案行，列表只展示生产 BOM，商品引用只在 BOM 详情展示；点击任意 BOM 名称都能进入右侧配方明细。
- Current follow-up scope: 撤销上一轮“缺 BOM 商品行在生产 BOM 列表创建BOM”的列表逻辑。生产 BOM 页面只读 `/api/production-boms?status=all`；去掉商品列、商品过滤、`无生产 BOM / 未维护` 商品行和 `创建BOM` 操作；商品档案配置跳转改传 `production_bom_id`。
- Detail-return follow-up scope: BOM 详情的“引用商品”显示商品档案商品名并可跳转到商品档案配置，商品档案左上角可返回 BOM 编辑；`BOM版本` 与 `全局规格袋材映射` 移入 BOM 编辑详情，不再作为列表行抽屉入口；Vue 开发规范新增跨页面跳转必须携带临时 `returnNavigation` 的规则。
- DEV:
  - DEV-406-BOM-INDEPENDENT-LIST：BOM 页面只使用 `/api/production-boms?status=all` 展示独立生产 BOM 档案，行 key 使用 `bom:{production_bom_id}`，不再合并 `/api/bom/list` 商品行。
  - DEV-406-BOM-DETAIL-REFERENCED-PRODUCTS：`/api/production-boms/:id` 返回 `referenced_products`，右侧配方明细展示引用商品；商品引用不参与列表行。
  - DEV-406-PRODUCT-BOM-NAV-ID：商品档案配置的“维护当前 BOM 明细”通过 `production_bom_id` 跳转生产 BOM，不再传商品筛选参数。
  - DEV-406-BOM-DETAIL-INLINE-VERSIONS-MAPPINGS：BOM 编辑详情内展示和维护 BOM 版本、复制新版草稿、发布草稿和全局规格袋材映射；列表行删除 BOM版本/规格袋材映射抽屉入口。
  - DEV-406-BOM-REFERENCED-PRODUCT-RETURN：BOM 详情引用商品按钮跳转商品档案配置，并通过 `returnNavigation` 提供左上角返回 BOM 编辑；`.agents/skills/kferp-vue-change` 固化跨页面跳转返回规则。
  - DEV-406-PRODUCT-ARCHIVE-LAYOUT：商品档案页压缩顶部说明、删除 `SKU归属`，过滤行右侧放创建/失效，反馈走 `kferp:notify`。
  - DEV-406-ALIAS-DRAWER-BATCH-DISABLE：客户商品名页删除旧收敛检查，新建客户商品抽屉包含单个/批量模式，过滤行右侧放新建/批量失效，绑定商品失效标红。
  - DEV-406-CLASSIFICATION-DROPDOWNS：商品档案和客户商品名分类操作改为 Tab 行右侧两个可搜索下拉，选择后确认执行并允许覆盖旧归类。
  - DEV-406-MANUAL-DOCS：更新商品/BOM/客户履约手册、需求、验收和 acceptance 证据。
- Verifier:
  - RED: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - Follow-up RED: `node --test src/lib/bom.test.js` failed because `productionBomDetailAsRecipeDetail` was not exported and unbound BOM selection still cleared detail.
  - Current follow-up RED: `node --test src/lib/bom.test.js` failed because `defaultProductionBomNameForProduct` / missing-row helper were not exported and the Vue page lacked the create-and-bind path.
  - Independent-list RED: `node --test src/lib/bom.test.js` failed because production BOM label did not read independent `code/name/latest_version_no` and BomView still contained `商品 BOM列表` / `商品过滤` / `mergeProductionBomRows`; targeted Go test failed because `ProductionBomDetail` did not expose `referenced_products`.
  - Follow-up frontend: `node --test src/lib/bom.test.js` passed 9/9; `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 118/118; `npm run build` passed.
  - Current follow-up frontend: `node --test src/lib/bom.test.js` passed 10/10; `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 119/119; `npm run build` passed; `scripts/verify_kferp.sh changed` passed.
  - Independent-list frontend/API: `node --test src/lib/bom.test.js` passed 8/8; `node --test src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 109/109; `go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1` passed.
  - Detail-return RED: `node --test src/lib/bom.test.js` failed because BOM 详情还没有引用商品跳转和详情内版本/袋材映射；`node --test src/lib/view-routing.test.js` failed because Vue 开发规范还没有 `returnNavigation` 规则。
  - Detail-return GREEN: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 119/119; `go test ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` passed; `npm run build` passed; `scripts/verify_kferp.sh changed` passed.
  - Follow-up changed verifier: `scripts/verify_kferp.sh changed` passed.
  - Frontend: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-context-pages.test.js` passed 139/139; `npm run build` passed in `orderapp-remote/frontend-vue-shell`
  - API/backend: `go test ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` passed
  - Changed verifier: `scripts/verify_kferp.sh changed` passed
  - Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-bom-product-alias-layout.md`
- Deployment: follow-up commit `67f09bfc` pushed to feature branch and fast-forward merged to `develop`; `origin/develop=67f09bfc9fd412fd316852178d69e2c66b0b91ad` deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604104002`.
- Last update: 2026-06-04 Asia/Shanghai
- Follow-up deployment: `e5cbda1d` pushed to `origin/develop=e5cbda1d580a1b3edaf53bf8660082f7836038d6` and deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604114317`.
- Last update: 2026-06-04 Asia/Shanghai
- Current follow-up deployment: `22f674af` pushed to `origin/develop=22f674afb730804b6834236fcdbf80ff51b835e9` and deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604122816`.
- Last update: 2026-06-04 Asia/Shanghai
- Independent-list deployment: feature commit `4011bbfb` pushed to `origin/codex/production-bom-independent-list-20260604`; merged to `develop` with `origin/develop=757decf7ccfcd397d66c8726921986ae47e66cf7`; deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604130904`.
- Last update: 2026-06-04 Asia/Shanghai
- Detail-return deployment: feature commit `c04916b8` pushed to `origin/codex/bom-detail-product-return-20260604`; merged to `develop` with `origin/develop=189fed6972c2953e913b0c6dcdab2bb619b59d34`; deployed to development with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260604133129`.
- Last update: 2026-06-04 Asia/Shanghai
- Notes: Follow-up RED evidence added for BOM name click restoring recipe detail, customer alias move-to-unclassified sentinel handling, hidden price-list switches, batch search placement, and unbound production BOM detail projection. Follow-up GREEN evidence: frontend target/source-marker tests passed 139/139 before earlier merge; develop branch spot checks passed 109/109 for `bom.test.js` + `product-settings.test.js`; catalog/support Go tests passed; Vue build passed; changed verifier passed. Unbound BOM follow-up evidence: `node --test src/lib/bom.test.js` passed 9/9; `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 118/118; Vue build passed; changed verifier passed. Deploy evidence: Docker build ran `go test ./...`; containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated GET `/app/vue-shell` returned 200; requirement API exposes `PR-406-BOM-PRODUCT-ALIAS-LAYOUT`; authenticated `/app/api/bom/list`, `/app/api/customer-product-aliases?active=all&q=`, and `/app/api/production-boms?status=all` returned 200; remote source includes `openBomRowPrimary`, `alias-batch-list-filters`, and `productionBomDetailAsRecipeDetail`. Browser/manual验收 per current convention not executed.
- Superseded follow-up notes: 上一轮曾把 `/api/bom/list` 返回的缺 BOM 商品档案行改为 `创建BOM`，但 Van 进一步确认生产 BOM 列表不应包含商品列表。本轮撤销该列表逻辑：生产 BOM 页面只展示 `/api/production-boms?status=all` 的独立配方档案；商品引用只在详情展示，商品档案侧通过 `production_bom_id` 跳转。
- Independent-list GREEN evidence: `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 117/117; `go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -count=1` passed; `go test ./...` passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` passed. Deploy smoke: containers running; unauthenticated `/app/` returned 303 to `/app/orders`; `/app/vue-shell` returned 200; `/app/api/production-boms?status=all` returned 200; orderapp logs show normal startup on `:8080`; remote source contains `生产 BOM列表` and no longer contains `商品过滤`.

### PR-405-PRODUCTION-BOM-BATCH-DEACTIVATE
- Branch: codex/bom-batch-deactivate-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 生产 BOM 页面去掉单独“失效当前 BOM”入口，商品 BOM列表新增勾选后的“批量失效”；行内失效和批量失效都直接执行，不弹确认框；生产 BOM 失效不再因启用商品引用被后端拒绝，商品侧继续提示 BOM 已失效。
- DEV:
  - DEV-405-BOM-BATCH-DEACTIVATE-UI：商品 BOM列表保留单行失效并新增批量失效卡片，删除单独“失效当前 BOM”按钮和确认弹窗。
  - DEV-405-BOM-DEACTIVATE-API：生产 BOM 更新为已失效时不再检查启用商品引用，继续写操作日志。
  - DEV-405-MANUAL-DOCS：更新需求、验收和库存物料手册。
- Verifier:
  - RED: `node --test src/lib/bom.test.js`; `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/support -run 'TestProductionBomCanDeactivateWhenActiveProductsReferenceIt|TestDev167VueShowsProductMultiDeactivateAndBomInactiveWarnings' -count=1`
  - Frontend: `node --test src/lib/bom.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`
  - API/backend: `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual/docs: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-production-bom-batch-deactivate.md`
- Deployment: feature branch pushed with `42fc561c`; fast-forward merged to `develop`, then deployment evidence commit pushed to `origin/develop=5a96e1cff12e44a1e4205c8bfb482c39d4c96cf8`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603220704`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence captured: frontend BOM test failed because page still had `失效当前 BOM` and confirm; backend repository test failed because `UpdateProductionBom` still blocked active product references. GREEN evidence: `node --test src/lib/bom.test.js` passed; `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1` passed; `go test ./...` passed; Vue build passed with existing chunk-size/plugin timing warnings; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...`; containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated GET `/app/vue-shell` returned 200; requirement API exposes `PR-405-PRODUCTION-BOM-BATCH-DEACTIVATE`; authenticated `/app/api/production-boms?status=all` returned 200 with BOM data.

### PR-404-PRICE-WARNING-BOM-DRAWERS
- Branch: codex/price-warning-bom-drawers-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 商品价格表 warning 改为短 `未设置计价方式` + 感叹号 hover/focus tooltip；缺计价方式 warning 不再按 `green_bean/drip_bag` 商品形态豁免；生产 BOM 页面重排列表工具区，并把 BOM 版本和全局规格袋材映射改为列表行按钮打开抽屉。
- DEV:
  - DEV-404-PRICE-LIST-WARNING-ICON：成本引擎使用 `MissingPricingMethodWarning`，固定单价、成本加成和有效阶梯价模板都视为有效计价方式；Vue 商品价格表用感叹号图标和 tooltip 展示 warning。
  - DEV-404-BOM-LAYOUT-DRAWERS：生产 BOM 新建按钮右上角展示，列表标题下方放状态过滤/搜索，移动分组卡片位于分组 Tab 上方；BOM 行提供 `BOM版本` 与 `规格袋材映射` 抽屉入口。
  - DEV-404-MANUAL-DOCS：更新需求、验收、成本手册和库存物料手册。
- Verifier:
  - RED: `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing -run 'Test(ProductWithoutPricingMethodDoesNotPublishCommercialTiers|ProductWithGradientTemplateDoesNotWarnMissingPricingMethod|PricingMethodWarningDoesNotExemptProductKind|ConfiguredFixedPriceAndCostPlusDoNotWarnMissingPricingMethod|BeanListRequiresExplicitGradientTemplateForCommercialTiers|CostingCalculateAPIRequiresGradientTemplateForCommercialTiers)' -count=1`; `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - Frontend: `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`
  - API/backend: `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing -count=1`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual/docs: `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-price-warning-bom-drawers.md`
- Deployment: feature branch pushed with `567df568`; fast-forward merged to `develop` and pushed to `origin/develop=567df568c22d8c5b7d7c86d7a1183885185d0fc1`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603214445`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence captured: Go tests failed because new `MissingPricingMethodWarning` did not exist and old生豆/挂耳分支仍豁免；frontend tests failed because warning 仍是文本 chip，BOM 版本/规格袋材映射仍是底部 panel。GREEN evidence: `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` passed; `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js` passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...`; containers running; unauthenticated GET `/app/` returned 303 to `/app/orders`; authenticated GET `/app/vue-shell` returned 200; requirement API exposes `PR-404-PRICE-WARNING-BOM-DRAWERS`.

### PR-403-BOM-PRICE-INDUSTRY-POLISH
- Branch: codex/bom-price-industry-polish-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 优化商品价格表缺少计价方式提示；行业字段模板改为左侧列表搜索/状态过滤、右侧编辑；生产 BOM 操作区移到列表和详情共同顶部，列表独立滚动；修复 `Codex测试豆` 这类旧 BOM 有明细但无生产 BOM 绑定导致不能勾选移动分组的问题。
- DEV:
  - DEV-403-PRICE-LIST-PRICING-METHOD-WARNING：把“未配置阶梯价模板”改为“未设置计价方式”，并在提示中写清主菜单路径 `商品与配方 → 商品配置和分类模板 → 商品配置模板 → 计价方式`。
  - DEV-403-INDUSTRY-TEMPLATE-LIST-EDITOR：行业字段模板左侧列表支持搜索模板名和启用/停用/全部过滤，点击模板后在右侧编辑，新建模板也在右侧编辑。
  - DEV-403-BOM-GROUP-ACTION-LAYOUT：生产 BOM 的新建、状态过滤、搜索、分组 Tab 和移动分组卡片放在 BOM 列表与编辑详情共同顶部；BOM 列表保持独立滚动窗口。
  - DEV-403-LEGACY-BOM-BINDING-REPAIR：BOM 列表/详情加载时幂等修复旧 `product_bom` / `product_bom_items` 有数据但缺少 `production_boms` 和商品绑定的记录，使旧 BOM 行可被勾选移动分组。
- Verifier:
  - RED: `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`; `go test ./internal/infrastructure/postgres/bom -run TestProductionBomBackfillRepairsLegacyItemsWithoutBindings -count=1`; `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - Frontend: `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`
  - API/backend: `go test ./internal/domain/costing ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`
  - Full backend: `go test ./...` in `orderapp-remote`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual/docs: `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-bom-price-industry-polish.md`
- Deployment: feature branch pushed with `6ef54131`; fast-forward merged to `develop` and pushed to `origin/develop=6ef5413121796d4b5a732dffa5193ac0c7b2ba23`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603210236`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: `Codex测试豆` 的根因是旧 BOM 行有 `item_count=1` 和 legacy `product_bom_items`，但 `production_bom_id=0`，前端按新模型禁止移动未绑定生产 BOM 的行。PR-403 不把前端限制放宽，而是在 BOM 读路径修复 legacy 绑定，保证旧 BOM 也进入生产 BOM 配方库后再移动分组。GREEN evidence: frontend target tests passed 22/22; targeted Go packages passed; `go test ./...` passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...`; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; requirement API exposes `PR-403-BOM-PRICE-INDUSTRY-POLISH`; authenticated BOM list API returned `Codex测试豆 production_bom_id=604 version=V001 group=0`.

### PR-402-PRODUCTION-BOM-GROUP-TABS
- Branch: codex/production-bom-group-tabs-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 生产 BOM 分组去掉默认分组；生产 BOM 页面只保留商品 BOM列表作为主列表，在列表上方展示全部分组、未分类和用户新增分组 Tab，并支持勾选 BOM 批量移动到分组。
- DEV:
  - DEV-402-BOM-GROUP-TABS-UI：移除独立“生产 BOM 档案”列表，把新建、状态过滤、搜索、分组 Tab、批量移动分组和 BOM 名称编辑入口集中到商品 BOM列表。
  - DEV-402-BOM-GROUP-UNCLASSIFIED：后端不再创建默认分组；旧默认分组迁回 `group_id=0` 未分类；删除分组时 BOM 回到未分类。
  - DEV-402-MANUAL-DOCS：更新需求、验收和 BOM 操作手册。
- Verifier:
  - RED: `node --test src/lib/bom.test.js`; `go test ./internal/infrastructure/postgres/bom -run TestProductionBomGroupsArePureUIFoldersWithDeleteAndSort -count=1`
  - Frontend: `node --test src/lib/bom.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`
  - API/backend: `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/application/bom -count=1`
  - Full backend: `go test ./...` in `orderapp-remote`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-production-bom-group-tabs.md`
- Deployment: feature branch pushed with `af9015f4`; merged to `develop` with `88dc3043ba9c0839d3bd7d01ff1c75c4d8c72f4b`; evidence commit `68a00a6ebe997d88c4b5be8e0e899104e8c40dcb`; development stack final-deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603204052`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence captured: frontend BOM test failed because page still rendered 独立“生产 BOM 档案”、`group-tree` and default group wording; backend repository marker test failed because delete group still moved BOM to default group. GREEN evidence: `node --test src/lib/bom.test.js`, targeted BOM/support Go packages, Vue build, `go test ./...`, and `scripts/verify_kferp.sh changed` passed. Deploy evidence: Vue shell build passed with existing chunk-size warning; Docker build ran `go test ./...` successfully; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; requirement API exposes `PR-402-PRODUCTION-BOM-GROUP-TABS`; authenticated production BOM groups API returned 200 with default group count 0; authenticated BOM list API returned 200.

### PR-401-PRICE-LIST-MISSING-GRADIENT-WARNING
- Branch: codex/price-list-missing-gradient-warning-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 商品价格表中，像 `初晓2.5kg装` 这类没有配置阶梯价模板且不会生成商业阶梯价的商品，要显示“未配置阶梯价模板”提示，避免报价卡片静默空白。
- DEV:
  - DEV-401-MISSING-GRADIENT-WARNING：成本引擎为需要商业报价但没有有效阶梯价模板的商品追加 warning；绑定模板商品、生豆直接销售和挂耳商品不误提示。
  - DEV-401-MANUAL-DOCS：更新成本手册、需求、验收和需求管理种子。
- Verifier:
  - RED: `go test ./internal/domain/costing -run 'TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers|TestProductWithGradientTemplateDoesNotWarnMissingGradientTemplate' -count=1`; `go test ./internal/application/costing -run TestBeanListRequiresExplicitGradientTemplateForCommercialTiers -count=1`
  - API/backend: `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - Full backend: `go test ./...` in `orderapp-remote`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-price-list-missing-gradient-warning.md`
- Deployment: feature branch pushed with `61b96cfa`; merged to `develop` with `577e821444d08f4f72878ec9a010d490514be44a`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603201536`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence captured: domain and BeanList service tests failed because no-template products returned empty `warnings`. GREEN evidence: targeted costing/API/support packages passed, `go test ./...` passed, and `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Vue shell build passed with existing chunk-size warning; Docker build ran `go test ./...` successfully; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; requirement API exposes `PR-401-PRICE-LIST-MISSING-GRADIENT-WARNING`; live Karen price-list API returned `初晓2.5kg装 tiers 0` and warning `未配置阶梯价模板`.

### PR-400-PRICE-LIST-EXPLICIT-GRADIENT-TIERS
- Branch: codex/price-list-explicit-gradient-tiers-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 修复商品价格表在商品没有任何明确阶梯价模板时仍由成本引擎或发布保存逻辑自动生成默认 4 档阶梯价的问题；例如“初晓2.5kg装”未绑定阶梯价模板时，商品价格表不应出现阶梯价。
- DEV:
  - DEV-400-COSTING-ENGINE-EXPLICIT-GRADIENT：成本引擎只在 `ProductInput.GradientTemplate` 有效时输出 `CommercialWholesaleTiers`，无模板时保留基础 kg/lb 成本价但不发布商业阶梯。
  - DEV-400-PUBLISH-NO-DEFAULT-TIERS：发布保存 `product_price_tiers` 时只保存结果中的显式商业阶梯价，不再根据 `WholesaleKgPrices/WholesaleLbPrices` 补默认 4 档。
  - DEV-400-MANUAL-DOCS：更新成本手册、需求和验收记录，明确“无明确阶梯价模板 = 不展示/不发布阶梯价”。
- Verifier:
  - RED: `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`; `go test ./internal/infrastructure/postgres/costing -run TestCommercialTiersForPublishDoesNotInventDefaultTiers -count=1`
  - API/backend: `go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing -count=1`
  - Full backend: `go test ./...` in `orderapp-remote`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-price-list-explicit-gradient-tiers.md`
- Deployment: feature branch pushed with `5e00e073`; merged to `develop` with `9d7d3e5dfcdbd84b574c72ef0d493e291924b432`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603190857`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence captured: engine test failed because `初晓2.5kg装` with no `GradientTemplate` still produced `2包-13包` through `48包+`; repository publish test failed because `commercialTiersForPublish` re-created the same defaults from kg/lb base prices. GREEN evidence: targeted costing/support packages passed, `go test ./...` passed, and `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Vue build passed with existing chunk-size warning; Docker build ran `go test ./...` successfully; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; requirement API exposes `PR-400-PRICE-LIST-EXPLICIT-GRADIENT-TIERS`; live read-only price list API for Karen returned `初晓2.5kg装 tiers 0 gradient_template False`.

### PR-399-PRICE-LIST-GRADIENT-SOURCE-FIX
- Branch: codex/price-list-unbound-gradient-fix-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 修复商品价格表候选把旧分类/父分类阶梯价模板当作商品实际阶梯价来源的问题；商品未绑定商品配置模板/阶梯价模板时，不应仅因归类或分类模板引用而显示阶梯价。
- DEV:
  - DEV-399-PRICE-LIST-GRADIENT-SOURCE：`LoadProductInputs` 只从客户规则、客户规则模板、商品级覆盖和商品配置模板解析实际阶梯价模板，不再从旧产品分类或父分类兜底。
  - DEV-399-MANUAL-DOCS：更新成本手册、需求和验收记录，明确分类模板/分类项引用阶梯价只用于归类口径、默认检查和不一致提示，不参与实际价格计算。
- Verifier:
  - API/backend: `go test ./internal/infrastructure/postgres/costing -run 'TestLoadProductInputsDoesNotFallbackToCategoryGradientTemplates|TestLoadProductInputsResolvesCustomerProductRuleTemplates' -count=1`; `go test ./internal/application/costing ./internal/interfaces/http/costing -count=1`
  - Full backend: `go test ./...` in `orderapp-remote`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-price-list-gradient-source-fix.md`
- Deployment: feature branch pushed; merged to `develop` with `c48919181250f266d842b962f39f9e75473f5c17`; development stack deployed with `./deploy_orderapp.sh development` at `c48919181250f266d842b962f39f9e75473f5c17`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603143627`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: RED evidence: `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsDoesNotFallbackToCategoryGradientTemplates -count=1` failed because `effective_gradient_template_id` still contained `NULLIF(pc.gradient_template_id,0)`. GREEN evidence on feature branch and merged develop: targeted costing repository/application/http/support tests passed after removing category fallback from actual gradient source; `go test ./...` in `orderapp-remote` passed; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Vue build passed with existing chunk-size warning; Docker build ran `go test ./...` successfully; containers running; unauthenticated `/app/` returned 303; authenticated `/vue-shell` and `/app/vue-shell` returned 200; requirement API exposes `PR-399-PRICE-LIST-GRADIENT-SOURCE-FIX`; deployed docs contain `2026-06-03-price-list-gradient-source-fix.md`.

### PR-398-BOM-INDUSTRY-TEMPLATE-REFINE
- Branch: codex/bom-industry-template-refine-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 生产 BOM 档案支持新建、编辑、失效、复制、启用/失效/全部过滤和名称/编号搜索；行业字段模板去掉行业键、显示名、单位和必填入口，字段键即显示名，文本/下拉用类型右侧输入框维护。
- DEV:
  - DEV-398-PRODUCTION-BOM-CATALOG-CRUD：生产 BOM 页面新增“生产 BOM 档案”管理区，支持新建、编辑、失效、复制失效 BOM、状态过滤和名称/编号搜索。
  - DEV-398-INDUSTRY-TEMPLATE-KEY-ONLY：行业字段模板改为字段键 key-only，文本默认值和下拉选项在类型右侧输入，未传行业键默认 `general`。
  - DEV-398-MANUAL-DOCS：更新需求、验收、商品/生产/成本手册和 acceptance 证据。
- Verifier:
  - Frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js`
  - API/backend: `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`; `go test ./...` in `orderapp-remote`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OPERATION_MANUALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-bom-industry-template-refine.md`
- Deployment: feature branch pushed; merged to `develop` with `fa0e83789e9eca81ea1034fbce9ff3ebfa531b59`; deployment status commit `a684b41365073d11c09de0d3c0e57582b09daa87` pushed to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development` at `a684b41365073d11c09de0d3c0e57582b09daa87`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603134559`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: Van requested no browser/manual验收 for current workflow; use code/docs/unit/API/build verification. RED evidence: frontend target tests failed on missing `filterProductionBomCatalog` and old industry field UI markers; manufacturing API test failed because `label` was required; repository RED guard failed until production BOM deactivation checked active product bindings. GREEN evidence: frontend target tests passed 124/124; targeted BOM/manufacturing/support Go API tests passed; repository guard package passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0; `go test ./...` passed. Merge-gate evidence on `develop`: frontend target tests passed 124/124; targeted Go packages passed; `go test ./...` passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...` successfully; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell`, `/app/api/product-settings`, `/app/api/production-boms`, and `/app/api/industry-field-templates` returned 200; requirement API exposes `PR-398-BOM-INDUSTRY-TEMPLATE-REFINE`.

### PR-397-PRODUCT-CLASSIFICATION-INDUSTRY-FIELDS
- Branch: codex/product-classification-industry-fields-20260603
- Owner/session: Codex / 2026-06-03
- Status: merged and deployed to development
- Scope: 分类交互拆为增加分类和移动分类两张卡片；客户商品名补齐搜索、启停过滤、批量停用和客户行业字段覆盖；商品价格表 SQL 修复 `classification_template_id` 歧义；分类模板编辑优化；生产 BOM 移到生产管理；行业字段模板改成左列表右编辑、文本/下拉字段。
- DEV:
  - DEV-397-CLASSIFICATION-ACTION-CARDS：商品档案和客户商品名分类工具区拆成两张 action card，分类模板保存/删除在底部，分类项阶梯价/单位模板并排。
  - DEV-397-CUSTOMER-ALIAS-FILTER-BATCH-DISABLE：`GET /api/customer-product-aliases` 支持 `active/q`，新增 `/api/customer-product-aliases/batch-disable`，客户商品名列表共享过滤和批量停用。
  - DEV-397-PRICE-LIST-SQL-CLASSIFICATION：商品价格表候选查询使用 current classification 字段，客户范围优先读取客户商品行业字段覆盖值。
  - DEV-397-INDUSTRY-FIELD-TEMPLATE-SIMPLE-UI：行业字段模板改为左列表右编辑，新增字段只用文本/下拉，下拉预设用逗号；客户商品行业字段保存到覆盖表。
  - DEV-397-PRODUCTION-BOM-MENU：生产 BOM 菜单移到生产管理，route key 保持 `bom`。
  - DEV-397-MANUAL-DOCS：更新需求、验收、商品/成本/生产/履约手册和 acceptance 证据。
- Verifier:
  - Frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js orderapp-remote/frontend-vue-shell/src/lib/menu-ia.test.js`
  - API/backend: `go test ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`; `go test ./...` in `orderapp-remote`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`; `orderapp-remote/docs/OPERATION_MANUALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-03-product-classification-industry-fields.md`
- Deployment: feature branch pushed; merged to `develop` with `fa0b3241d3edda505fe77e0e1602513fd4d58701`; full Go test fake fix merged with `ba0da7e4011170efd67e5d9651060a93d1cfa13a`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603012742`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: Van requested no browser/manual验收; use code/docs/unit/API/build verification only. Evidence: frontend target tests passed 130/130; targeted catalog/costing/manufacturing/support Go tests passed; `go test ./...` passed locally and inside Docker build; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Smoke: containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell`, `/app/api/product-settings`, and `/app/api/customer-product-aliases?active=all&q=` returned 200; requirement API exposes `PR-397-PRODUCT-CLASSIFICATION-INDUSTRY-FIELDS`.

### PR-396-PRODUCT-CLASSIFICATION-COPY-FIXES
- Branch: codex/product-classification-copy-fixes-20260602
- Owner/session: Codex / 2026-06-02
- Status: merged and deployed to development
- Scope: 商品档案复制改为真正复制商品档案配置；下线历史 SKU 复制入口/API；商品档案和客户商品名分类交互改为“增加分类 / 移动到分类 / 移动到子类”；客户商品编号由系统生成；商品价格表候选按当前分类 assignment 生成。
- DEV:
  - DEV-396-PRODUCT-COPY：新增 `POST /api/product-settings/products/:id/copy`，复制商品基础信息、商品配置模板、生产配置、生产 BOM 绑定、生产配置字段和价格阶梯，并写操作日志。
  - DEV-396-LEGACY-SKU-COPY-REMOVAL：删除历史 SKU 复制前端入口、服务层/仓储旧路径和旧路由，旧 HTTP 路由不可用。
  - DEV-396-CLASSIFICATION-UX：商品档案和客户商品名固定未分类 Tab，`增加分类` 启用模板，移动归类直接覆盖旧归类，当前子类重复移动置灰。
  - DEV-396-CUSTOMER-ALIAS-CODE：客户商品名单个/批量新增不提交客户商品编号，后端自动生成编号。
  - DEV-396-PRICE-LIST-CLASSIFICATION：商品价格表候选从商品/客户商品当前分类 assignment 读取，未归类为“其他”。
  - DEV-396-MANUAL-DOCS：更新需求、验收、商品/成本/履约手册和 acceptance 证据。
- Verifier:
  - Frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/product-bean-list-split.test.js`
  - API/backend: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/domain/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-02-product-classification-copy-fixes.md`
- Deployment: feature branch pushed; merged to `develop` with `679bdfffb30a5925152814221401f2d07c105f82`; PR/DEV req_store evidence commit `27b281f0a0f499984edd71bcc867d45051c8bf83` pushed to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603001306`.
- Last update: 2026-06-03 Asia/Shanghai
- Notes: Van requested no browser/manual验收; use code/docs/unit/API/build verification. Feature-branch and merge-gate evidence: frontend target tests passed 110/110; targeted catalog/bom/costing/support Go tests passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...` and succeeded; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; authenticated `/app/api/product-settings` returned 200; requirement API exposes `PR-396-PRODUCT-CLASSIFICATION-COPY-FIXES`.

### PR-395-PRODUCT-PRICE-CLASSIFICATION
- Branch: codex/product-price-classification-20260602
- Owner/session: Codex / 2026-06-02
- Status: merged and deployed to development
- Scope: 商品配置模板更名为“商品配置和分类模板”；阶梯价模板、单位模板拆成商品与配方独立菜单；产品价格表更名为商品价格表；商品档案和客户商品名单归类；分类模板/分类项可引用阶梯价模板和单位模板；商品价格表类型来自当前归类分类模板，未归类为“其他”。
- DEV:
  - DEV-395-MENU-PAGES：调整商品与配方菜单、App sectionMode 和 ProductSettingsView 页面呈现。
  - DEV-395-CLASSIFICATION-TEMPLATE-REFS：分类模板/分类项新增阶梯价模板、单位模板引用字段/API/schema/repository。
  - DEV-395-SINGLE-CLASSIFICATION：商品档案和客户商品名保存归类时前后端拒绝重复归类，全部 Tab 展示当前归类。
  - DEV-395-CUSTOMER-ALIAS-CLEANUP：客户商品名列表删除无效编辑和生产/BOM 操作，保留停用。
  - DEV-395-PRODUCT-PRICE-LIST：商品价格表使用 classification 字段优先，发布快照保存分类模板/分类项字段，旧 product_type 字段兼容。
  - DEV-395-MANUAL-DOCS：更新商品、成本、履约手册、需求和验收文档。
- Verifier:
  - Frontend: `node --test src/lib/product-settings.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/menu-ia.test.js`
  - API/backend: `go test ./internal/interfaces/http/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/costing ./internal/application/costing ./internal/infrastructure/postgres/costing -count=1`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`; `orderapp-remote/docs/OPERATION_MANUALS.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-02-product-price-classification.md`
- Deployment: feature branch pushed; merged to `develop` with `e70131af2fcb4b8f578d7ba0706618576939d165`; support marker fix `cff0021b957ce6df1cbf1d20db6639dca57d067b` pushed to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260602224205`.
- Last update: 2026-06-02 22:45 Asia/Shanghai
- Notes: Van requested no browser/manual验收 for this round; use code/docs/unit/API/build verification only. Local and merge-gate evidence: frontend target tests passed 124/124; targeted catalog/costing Go tests passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...` and succeeded after support marker tests were updated to the PR-395 terminology; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; authenticated `/app/api/product-settings` returned 200; deployed docs contain `PR-395-PRODUCT-PRICE-CLASSIFICATION`.

### PR-394-PRODUCT-CLASSIFICATION-VIEW-TABS
- Branch: codex/product-classification-view-tabs-20260602
- Owner/session: Codex / 2026-06-02
- Status: merged and deployed to development
- Scope: 商品档案和客户商品名不再直接引用分类模板；分类模板作为列表页面启用的分类视图，一个模板一个 Tab。分类模板页面只维护分类结构；商品档案页和客户商品名页分别启用模板、按分类分组展示、用列表勾选做对象归类。客户商品名层删除生产/BOM 操作入口；生产 BOM 返回商品档案配置改为公共临时返回导航。
- DEV:
  - DEV-394-CLASSIFICATION-USAGE-API：新增商品档案/客户商品名分类模板启用 API，保留旧字段兼容，新写入不再把模板挂到对象本身。
  - DEV-394-PRODUCT-CLASSIFICATION-TABS：商品档案页使用“启用模板 Tab + 分类分组 + 勾选归类”，创建和配置抽屉不再选择分类模板或旧产品类型/子类型。
  - DEV-394-ALIAS-CLASSIFICATION-TABS：客户商品名页使用客户级启用模板 Tab，单个/批量新增不选模板，删除展示分类和生产/BOM 操作。
  - DEV-394-CLASSIFICATION-TEMPLATE-STRUCTURE：分类模板页只维护模板和分类项结构，不再配置客户和对象归类。
  - DEV-394-TRANSIENT-RETURN-NAV-DOCS：生产 BOM 返回商品档案配置使用前端内存态公共返回导航，刷新后消失，并更新手册/需求/验收文档。
- Verifier:
  - Frontend: `node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/view-routing.test.js`
  - API/backend: `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-02-product-classification-view-tabs.md`
- Deployment: feature branch pushed; merged to `develop` with `c6351ffabcf510cf2c2d0311846ac800269220f6`; status evidence commit `95a6d530cc39c21ca03dfcb5f76c0b1f912b8415` pushed to `origin/develop`; development stack deployed with `./deploy_orderapp.sh development` at `95a6d530cc39c21ca03dfcb5f76c0b1f912b8415`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260602184905`.
- Last update: 2026-06-02 Asia/Shanghai
- Notes: Van requested no browser/manual验收; use code/docs/unit/API/build verification. RED evidence: frontend tests initially failed on missing classification template usage helpers, page-level tabs and BOM return navigation; catalog API tests initially failed because batch customer aliases still accepted `classification_template_id`, classification templates retained customer ownership, and product/customer classification-template usage APIs were missing; support marker tests caught stale PR-393 wording. GREEN evidence before merge and on merged `develop`: `node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/view-routing.test.js` passed 110/110; `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` passed; `npm run build` in Vue shell passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Deploy evidence: Docker build ran `go test ./...` and succeeded; containers `erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert` running; unauthenticated `GET /app/` returned `303`; authenticated `GET /app/vue-shell` returned `200`; authenticated `/app/api/product-settings` and `/app/api/product-classification-template-usages/products` returned `200`; deployed source/dist contain `PR-394-PRODUCT-CLASSIFICATION-VIEW-TABS` and product classification usage markers. Initial deploy was temporarily blocked by SSH closing before banner, then recovered and succeeded.

### PR-393-PRODUCT-CLASSIFICATION-TEMPLATES
- Branch: codex/product-classification-template-drawers-20260602
- Owner/session: Codex / 2026-06-02
- Status: merged and deployed to development
- Scope: 商品分类从商品档案表单里的直接分类下拉和旧拖拽分类树，收敛为“分类模板 + 配置分类抽屉”。商品档案和客户商品名只保存 `classification_template_id`；分类项和商品/客户商品名归属在可叠加、可收起的分类配置抽屉中维护。客户商品名批量添加商品档案时可选择分类模板，不选则默认复制/复用来源商品档案分类模板到客户侧并按同名分类映射。行业字段只能来自行业字段模板定义；生产 BOM 支持返回商品档案配置。
- DEV:
  - DEV-393-CLASSIFICATION-SCHEMA-API：新增分类模板、分类项、商品档案归属和客户商品名归属表/API，扩展商品档案和客户商品名 `classification_template_id`，写操作日志。
  - DEV-393-PRODUCT-ALIAS-CLASSIFICATION：客户商品名单个/批量创建支持分类模板；批量不选时复制/复用来源模板并映射同名分类。
  - DEV-393-DRAWER-STACK-BOM-RETURN：商品档案配置和分类配置支持多层抽屉、收起/展开/关闭；生产 BOM 页面提供“返回商品档案配置”。
  - DEV-393-INDUSTRY-FIELD-TEMPLATE-LOCK：商品生产配置字段保存必须来自行业字段模板，不允许商品档案里临时新增/删除字段定义。
  - DEV-393-MANUAL-DOCS：更新需求、验收、商品/BOM/成本/生产/履约手册。
- Verifier:
  - Frontend: `node --test src/lib/product-settings.test.js src/lib/bom.test.js`
  - API/backend: `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-02-product-classification-template-drawers.md`
- Deployment: feature branch pushed; merged to `develop` with `e7d10ef3f49c98b126a55c991c51ca6e7ce40f94`; development stack deployed with `./deploy_orderapp.sh development`. Backup from first deployment: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260602130506`.
- Last update: 2026-06-02 Asia/Shanghai
- Notes: Van requested no browser/manual验收 for this round; use code/docs/unit/API/build verification only. RED evidence: frontend/support tests initially failed on missing classification template drawer/API markers and stale industry-field add behavior. GREEN evidence before merge: `node --test src/lib/product-settings.test.js src/lib/bom.test.js` passed 102/102; `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` passed; `npm run build` in Vue shell passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Merge-gate evidence after `origin/develop` merge: frontend target passed 102/102; targeted Go packages passed; Vue build passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` and `scripts/verify_kferp.sh backend` exited 0. Deploy evidence: Docker build ran `go test ./...` and succeeded; containers running; unauthenticated `/app/` returned 303 to `/app/orders`; authenticated `/app/vue-shell` returned 200; authenticated `/app/api/product-settings` and `/app/api/product-classification-templates` returned 200; requirement API exposes `PR-393-PRODUCT-CLASSIFICATION-TEMPLATES`; server source contains `classification-config-drawer`.

### PR-392-PRODUCT-CONFIG-ENTRY-TEMPLATE-ALIAS
- Branch: codex/product-config-entry-template-alias-20260602
- Owner/session: Codex / 2026-06-02
- Status: merged and deployed to development
- Scope: 商品档案列表以商品名作为唯一商品档案配置入口；商品档案配置抽屉维护基础信息、商品配置模板、生产 BOM、工艺路线、预期损耗率、行业字段模板和值，并以内页跳转维护当前 BOM 明细。商品配置模板由商品档案引用，分类只负责归类；客户商品名支持批量从商品档案创建；生产 BOM 跳转不刷新左侧菜单。
- DEV:
  - DEV-392-PRODUCT-ARCHIVE-CONFIG-DRAWER：商品档案列表删除生产配置/更换生产 BOM/维护 BOM/BOM 重复按钮，点击商品名打开商品档案配置抽屉。
  - DEV-392-PRODUCT-TEMPLATE-OWNERSHIP：新增 `products.product_config_template_id`，商品创建、商品基础信息、成本/价格/生产读取优先商品档案模板，旧分类模板 legacy fallback。
  - DEV-392-INDUSTRY-FIELDS-IN-PRODUCT-CONFIG：商品生产配置保存 `industry_field_template_id` 和字段快照，行业字段模板在商品档案配置抽屉中以普通表单使用。
  - DEV-392-CUSTOMER-ALIAS-BATCH：新增 `/api/customer-product-aliases/batch` 和客户商品名批量添加抽屉，重复绑定跳过并写操作日志。
  - DEV-392-SPA-BOM-NAV-DOCS：生产 BOM 明细入口使用 `kferp:navigate-view` SPA 导航，更新手册、需求和验收文档。
- Verifier:
  - Frontend: `node --test src/lib/product-settings.test.js src/lib/view-routing.test.js`
  - API/backend: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`
  - Build: `npm run build` in `orderapp-remote/frontend-vue-shell`
  - Changed verifier: `scripts/verify_kferp.sh changed`
  - Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`; `orderapp-remote/docs/OP_MANUAL_COSTING.md`; `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`; `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
  - Review/acceptance: `orderapp-remote/docs/REQUIREMENTS.md`; `orderapp-remote/docs/ACCEPTANCE_TESTS.md`; `orderapp-remote/docs/acceptance/2026-06-02-product-config-entry-template-alias.md`
- Deployment: feature branch pushed, merged to `develop` with `fffe159ec3315937cdb5577481c6676bacb85f07`, pushed to `origin/develop`, and deployed to development with `./deploy_orderapp.sh`. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260602110207`.
- Last update: 2026-06-02 11:05 Asia/Shanghai
- Notes: Van requested no browser/manual验收 for this round; use code/docs/unit/API/build verification only. RED evidence captured in acceptance doc: frontend tests initially failed on missing batch alias payload/product-name config entry/SPA BOM jump; catalog API tests initially failed on missing batch alias and product/industry field contracts. GREEN evidence before merge: `node --test src/lib/product-settings.test.js src/lib/view-routing.test.js` passed 100/100; targeted catalog/costing/sales/production/support Go tests passed; `npm run build` passed with existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` exited 0. Merge-gate evidence on `develop`: frontend target tests passed 100/100; targeted Go/API packages passed; `scripts/verify_kferp.sh changed` passed; `scripts/verify_kferp.sh backend` passed; Vue build passed with existing chunk-size warning. Deploy evidence: Docker build ran `go test ./...` and succeeded; containers `erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert` running; unauthenticated `GET /app/` returned `303` to `/app/orders`; authenticated `GET /app/vue-shell` returned `200`; authenticated `GET /app/api/product-settings` returned `200` and includes `product_config_template_id`; requirement API exposes `PR-392-PRODUCT-CONFIG-ENTRY-TEMPLATE-ALIAS`; server source contains `批量添加商品档案`.

### PR-391-PRODUCT-PRODUCTION-CONFIG-UI-FIX
- Branch: codex/product-production-config-ui-fix-20260601
- Owner/session: Codex / 2026-06-01
- Status: merged and deployed to development
- Scope: 修复 PR-390 后商品档案页看不到“商品生产配置”可编辑入口的问题。商品行新增“生产配置”按钮，抽屉中可维护生产 BOM、BOM 已发布版本、工艺路线、预期损耗率和产品信息字段；产品信息字段用普通表单行维护，不要求用户写 JSON。
- DEV:
  - DEV-391-VUE-PRODUCTION-CONFIG-DRAWER：商品档案列表新增“生产配置”入口和抽屉，保存到 `/api/product-production-configs/:product_id`。
  - DEV-391-PROCESS-ROUTE-OPTIONS：抽屉加载 `/api/process-routes?status=published` 作为工艺路线选项。
  - DEV-391-LANGUAGE-CLEANUP：更换生产 BOM 抽屉不再展示“兼容产出因子/预期产出率”。
  - DEV-391-MANUAL-DOCS：更新商品/BOM/成本手册，明确商品档案 → 生产配置的操作位置。
- Verifier:
  - Unit/frontend: node --test src/lib/menu-ia.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/product-bean-list-split.test.js
  - API/backend: go test ./internal/interfaces/http/catalog ./internal/application/catalog -count=1
  - Frontend/build: npm run build in orderapp-remote/frontend-vue-shell
  - Changed verifier: scripts/verify_kferp.sh changed
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md
- Deployment: feature branch pushed, merged to `develop` with `acb9331921f6dea84ed758d1ead738016a715c76`, followed by doc marker fix `a7a15156e9ce3e410d6881f64dd7e09a243f89f9`, and deployed to development. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260601234922`.
- Last update: 2026-06-01 23:53 Asia/Shanghai
- Notes: RED evidence: `node --test src/lib/product-settings.test.js` initially failed because `openProductProductionConfig(row)` and the drawer save/field controls were absent. GREEN evidence: targeted frontend tests passed 118/118; Vue build passed with existing chunk-size warning; changed verifier exited 0; catalog Go API/application tests passed. Deploy evidence: Docker build ran `go test ./...`; containers running; unauthenticated `/app/` returned `303` to `/app/orders`; BasicAuth `/app/vue-shell/` returned `200`; BasicAuth `/app/api/product-production-configs` and `/app/api/process-routes` returned `200`; deployed source contains `保存商品生产配置`. One deploy attempt failed before restart because the first handoff removed historical PR-374/PR-389 doc markers used by support tests; markers were restored and build passed. A second attempt hit transient Docker registry TLS timeout for `golang:1.22-alpine`; manual retry of compose build succeeded.

### PR-390-PRODUCT-PRODUCTION-CONFIG-OVERHAUL
- Branch: codex/product-production-config-overhaul-20260531
- Owner/session: Codex / 2026-05-31
- Status: merged and deployed to development
- Scope: 一次性把商品/BOM/工艺/价格/生产口径改为“BOM 只做配方库；商品档案承载生产配置和商品分类；客户商品名独立维护销售展示；商品配置模板独立维护模板规则；工艺路线只管工序；预期产出率下线，系统主口径改为预期损耗率”。
- DEV:
  - DEV-390-SCHEMA-BACKFILL：新增商品生产配置、生产配置字段和工艺路线结构，回填旧 BOM 绑定、yield_rate 和特殊属性。
  - DEV-390-API-SERVICE：新增商品生产配置和工艺路线 API，调整 BOM 分组增删改名排序，成本/价格表/录单/生产计划/工单改读商品生产配置。
  - DEV-390-VUE-PRODUCT-PAGES：拆分商品档案、客户商品名、商品配置模板页面入口，商品档案右侧承载生产配置和商品分类。
  - DEV-390-VUE-BOM-LIBRARY：生产 BOM 页面改为分组树，BOM 详情只维护版本和配方明细，下线特殊属性、预期损耗、预期产出率。
  - DEV-390-SNAPSHOT-COMPAT：价格表、订单行和工单冻结商品生产配置快照，历史数据继续 legacy fallback。
  - DEV-390-MANUAL-DOCS：更新需求、验收、操作手册和活动需求记录。
- Verifier:
  - Unit/API/backend: targeted Go tests for catalog, bom, costing, sales, production, support packages; final go test ./...
  - Frontend: targeted node tests for menu, product pages, BOM, price/order display helpers.
  - Build: npm --prefix orderapp-remote/frontend-vue-shell run build.
  - Changed verifier: scripts/verify_kferp.sh changed.
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md; orderapp-remote/docs/OPERATION_MANUALS.md.
  - Review/acceptance: REQUIREMENTS.md; ACCEPTANCE_TESTS.md; orderapp-remote/docs/REQUIREMENTS.md; orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-31-product-production-config-overhaul.md.
- Deployment: merged to `develop` with `e0072e59641a6f81da101f3a01206acb27a8e3d6` and deployed to development. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260601000753`.
- Last update: 2026-06-01 00:18 Asia/Shanghai
- Notes: Van requested no browser/manual验收 for this round; use code/docs/unit/API/build verification only. RED evidence: frontend targeted tests initially failed on missing split pages/BOM tree/product production config markers and later caught stale `special_attrs_json` payload assertions; backend targeted tests initially failed on missing product production config, process route, BOM group delete/sort and snapshot fields. Final local verification passed: `node --test src/lib/menu-ia.test.js src/lib/bom.test.js src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-mode.test.js src/lib/customer-management-source.test.js`; `go test ./...`; `npm run build` in `orderapp-remote/frontend-vue-shell` (existing chunk-size warning only); `scripts/verify_kferp.sh changed`.

### PR-389-BOM-GROUP-SPECIAL-ATTRS
- Branch: codex/bom-version-special-attrs-20260531
- Owner/session: Codex / 2026-05-31
- Status: merged and deployed to development
- Scope: BOM 分组补齐维护入口；特殊属性从 SKU/商品配置模板迁入生产 BOM 版本。生产 BOM 版本携带配方明细、预期损耗率、特殊属性字段和值；SKU 管理不再编辑特殊属性；价格表、成本、生产计划和工单优先读取绑定 BOM 版本特殊属性，旧 SKU 字段仅作兼容 fallback。
- DEV:
  - DEV-389-BOM-GROUP-CRUD：扩展 BOM 分组查询/编辑/停用 API、操作日志和 Vue 分组管理入口。
  - DEV-389-BOM-VERSION-SPECIAL-ATTRS：扩展 production_bom_versions schema/service/API，草稿可编辑特殊属性，已发布只读。
  - DEV-389-MIGRATION-BACKFILL：旧 SKU 特殊属性迁入 BOM 版本；同一 BOM 版本属性冲突时自动复制 BOM/版本并重新绑定商品。
  - DEV-389-COST-PRODUCTION-INTEGRATION：价格表/成本/生产/工单优先读取 BOM 版本特殊属性，fallback 旧 SKU 字段。
  - DEV-389-VUE-UI：SKU 管理移除特殊属性编辑，商品配置模板移除特殊属性定义，BOM 页面增加分组管理和版本特殊属性区。
  - DEV-389-MANUAL-DOCS：更新需求、验收和操作手册。
- Verifier:
  - Unit/API/backend: go test ./... plus targeted BOM/catalog/costing/production/support packages.
  - Frontend: node --test src/lib/bom.test.js src/lib/product-settings.test.js and targeted helpers.
  - Build: npm --prefix orderapp-remote/frontend-vue-shell run build.
  - Changed verifier: scripts/verify_kferp.sh changed.
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OPERATION_MANUALS.md.
  - Review/acceptance: orderapp-remote/docs/REQUIREMENTS.md; orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-31-bom-version-special-attrs.md.
- Deployment: merged to `develop` with `52e824892770f73b8f68b1bcdf2a8f1fdd3e9643` and deployed to development via `./deploy_orderapp.sh`.
- Last update: 2026-05-31 21:19 Asia/Shanghai; RED tests were added before implementation. Final local verification passed: `go test ./...`; `node --test src/lib/bom.test.js src/lib/product-settings.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`; `scripts/verify_kferp.sh changed`. Docker build also ran `go test ./...` during deployment.
- Deploy evidence: backup `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260531211432`; containers running; `/app/` GET returned `303` to `/app/orders`; authenticated `/app/vue-shell/` returned `200`; authenticated `/app/api/production-bom-groups?include_inactive=1` returned `200` with default group data; requirement API includes `PR-389-BOM-GROUP-SPECIAL-ATTRS`.
- Notes: `scripts/reserve_req_id.sh` returned PR-389; entry seeded manually. Van requested no browser/manual验收 for this round to save tokens.

### PR-388-PRODUCTION-BOM-LIBRARY
- Branch: codex/production-bom-library-20260531
- Owner/session: Codex / 2026-05-31
- Status: merged and deployed to development
- Scope: 把“商品内嵌 BOM + 继承/锁定/派生来源”收敛为独立生产 BOM 配方库、BOM 分组、BOM 版本和商品档案默认生产 BOM 版本绑定。客户商品名不产生 BOM；多个商品档案可复用同一个 BOM 版本；商品绑定旧版本只提示非最新版，不再叫锁定版本。
- DEV:
  - DEV-388-SCHEMA-SERVICE-API：新增生产 BOM 分组、BOM、版本、版本明细和商品绑定表，补服务/API/操作日志。
  - DEV-388-LEGACY-BACKFILL：旧 `owned/derived_owned/inherit_current/inherit_version` 兼容回填到商品生产 BOM 绑定，不回改历史业务数据。
  - DEV-388-VUE-BOM-LIBRARY：BOM 页面改为生产 BOM 配方库和分组/版本口径，下线继承/锁定/派生文案。
  - DEV-388-PRODUCT-BINDING：商品管理展示生产 BOM 编号、版本号和非最新版提示，提供更换生产 BOM 入口。
  - DEV-388-COST-PRODUCTION-INTEGRATION：成本核算、产品价格表、生产计划、物料需求和工单优先读取绑定 BOM 版本。
  - DEV-388-MANUAL-DOCS：更新需求、验收和操作手册。
- Verifier:
  - Unit/frontend: node --test src/lib/bom.test.js src/lib/product-settings.test.js
  - API/backend: go test ./internal/interfaces/http/catalog ./internal/interfaces/http/bom ./internal/interfaces/http/support ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/costing -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Changed verifier: scripts/verify_kferp.sh changed
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OPERATION_MANUALS.md
  - Review/acceptance: orderapp-remote/docs/REQUIREMENTS.md; orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-31-production-bom-library.md
- Deployment: merged to `develop` with `a6418c6958cb9a8862b2a59dfaeb9a3e0dbc9605`; follow-up test fake fix `36eeb93596185c3b85f7087e6e1ef0d4573d60e7` deployed to development via `./deploy_orderapp.sh development`.
- Last update: 2026-05-31 Asia/Shanghai; targeted Go, frontend node tests, Vue build, changed verifier, local `go test ./...`, Docker build `go test ./...`, and development smoke passed.
- Deploy evidence: backup `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260531201304`; containers running; `/app/` GET returned 303; authenticated `/app/vue-shell/` returned 200; `/app/api/production-bom-groups` returned 200; requirement API includes `PR-388-PRODUCTION-BOM-LIBRARY`.
- Notes: `scripts/reserve_req_id.sh` returned PR-388; `--claim production-bom-library` hit the known awk multiline bug, so this entry was seeded manually.

### PR-387-VIEW-CONTEXT
- Branch: codex/view-context-20260531
- Owner/session: Codex / 2026-05-31
- Status: deployed
- Scope: 把“工厂总览 / 客户账户”升级为统一“视图上下文”。顶部显示当前视图并支持工厂、客户、订单和外部客户固定视图；视图只负责菜单呈现、默认过滤、URL 保留和跨页面参数传递，不替代后端权限。结合 PR-386 商品模型，客户视图使用客户商品名，执行侧仍使用商品档案、生产 BOM 和价格表快照。
- DEV:
  - DEV-387-PHASE1-FRONTEND-CONTEXT：新增 Vue/Vite `view-context` 抽象，兼容旧 `workspace=customer&customer_id=...` URL 和旧 workspace-mode API。
  - DEV-387-PHASE2-PAGE-ADAPTERS：商品管理、产品价格表、BOM、录单、订单列表、仓库库存、费用管理接入客户/订单上下文。
  - DEV-387-PHASE3-OPTIONS-API：新增视图上下文客户/订单选项 API，并复用客户/订单权限边界。
  - DEV-387-PHASE4-PRESET-CRUD：新增保存视图表和 CRUD API，保存/修改/停用写操作日志。
  - DEV-387-PHASE5-MANUAL-ACCEPTANCE-DEPLOY：更新手册、验收文档，合入 develop 并部署 development stack。
- Verifier:
  - Unit/frontend: node --test src/lib/view-context.test.js src/lib/workspace-mode.test.js and targeted page tests touched by adapters.
  - API/backend: go test ./internal/interfaces/http/support -run 'TestViewContext' -count=1 plus targeted affected packages.
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build.
  - Changed verifier: scripts/verify_kferp.sh changed.
  - Manual: orderapp-remote/docs/OP_MANUAL_WORKSPACE_MODE.md; orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OPERATION_MANUALS.md.
  - Review/acceptance: orderapp-remote/docs/REQUIREMENTS.md; orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-31-view-context.md.
- Deployment: development deployed at origin/develop 4f4e2c87a960779ca7fda4f58c56e41f6851dd67; backup root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260531160343.
- Last update: 2026-05-31 16:10 Asia/Shanghai
- Notes: `scripts/reserve_req_id.sh --claim view-context` hit the known awk multiline bug; `scripts/reserve_req_id.sh` returned PR-387 and this entry was seeded manually. Final evidence before merge: `scripts/verify_kferp.sh changed` exited 0; frontend target `node --test src/lib/view-context.test.js src/lib/workspace-mode.test.js src/lib/operation-manuals.test.js src/lib/menu-ia.test.js` passed 28/28; API/backend `go test ./internal/interfaces/http/support ./internal/interfaces/http/sales -count=1` passed; `npm run build` passed with existing chunk-size warning. Merge/deploy evidence: feature branch pushed, local develop merge commit 4f4e2c87 pushed, deploy script rebuilt image and ran Docker build `go test ./...`; containers running, authenticated `/vue-shell/` returned 200, `/api/view-context/options?type=customer&limit=3` returned 200, PR-387 marker found in requirement API. Van explicitly stopped browser/manual验收 to save tokens; temporary smoke login session was removed.

### PR-386-PRODUCT-MODEL-OVERHAUL
- Branch: codex/product-model-overhaul-20260531
- Owner/session: Codex / 2026-05-31
- Status: deployed
- Scope: 五期统一商品模型改造。商品档案承载库存、BOM、成本、生产和成品批次；客户商品名承载客户侧对外名称、编号、品牌和展示分类并绑定商品档案；工厂自营作为内置客户-like 归属；生产 BOM 作为商品档案制造定义；产品价格表按价格表归属客户下启用的客户商品名生成和发布快照；旧客户 SKU 保持兼容并提供安全收敛检查。
- DEV:
  - DEV-386-PHASE1-LANGUAGE-MANUAL：商品管理/商品档案/生产 BOM/价格表归属口径替换，手册场景和前端测试。
  - DEV-386-PHASE2-CUSTOMER-ALIAS：新增客户商品名模型、API 和商品管理页客户商品名工作区。
  - DEV-386-PHASE3-PRICE-LIST：产品价格表读取客户商品名并发布 alias/product 双快照。
  - DEV-386-PHASE4-ORDER-PORTAL-PRODUCTION：录单、门户、履约、生产计划和工单接入客户商品名展示与商品档案执行链路。
  - DEV-386-PHASE5-LEGACY-SKU-CONVERGENCE：旧客户 SKU 只读收敛检查和新增动作分流。
- Verifier:
  - Unit/frontend: node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js src/lib/costing-bean-list-version-ui.test.js; later add alias/price/order tests as implemented.
  - API/backend: targeted Go tests for catalog/customer alias/costing/sales/portal/production/support packages as each phase lands.
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build.
  - Changed verifier: scripts/verify_kferp.sh changed.
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md; orderapp-remote/docs/OPERATION_MANUALS.md.
  - Review/acceptance: orderapp-remote/docs/REQUIREMENTS.md; orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-31-product-model-overhaul.md.
- Deployment: development deployed at origin/develop ea0ea54f62f8b16993980f51b4b315697ef65eea; backup root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260531150702.
- Last update: 2026-05-31 15:16 Asia/Shanghai
- Notes: Phase 1-5 complete and deployed to development. Final evidence before merge: frontend target `node --test` 211/211 passed; targeted Go packages passed; `npm run build` passed with existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` exited 0; browser flow passed for 商品管理、客户商品名、产品价格表、录单、客户履约、生产计划、生产工单、操作日志. Merge/deploy evidence: feature branch pushed; develop merge commit ea0ea54f pushed; deployment script rebuilt image and ran Docker build `go test ./...`; containers healthy/running; unauthenticated API returned 401; authenticated API returned 200; `/app/` returned 303 then authenticated follow returned 200; PR-386 marker found in requirement API and synced acceptance doc. Van stopped additional browser UI smoke after deployment; temporary smoke login session was removed.

### PR-385-SKU-BOM-INHERITANCE
- Branch: codex/sku-bom-inheritance-20260529
- Owner/session: Codex / 2026-05-29
- Status: deployed
- Scope: SKU复制默认继承来源 SKU 有效 BOM；BOM 维护页支持派生自有 BOM 并快照来源 SKU/BOM 编号、名称和版本；SKU/BOM 页面展示 BOM 来源；成本、生产计划和客户履约按有效 BOM 解析。支持 BOM 版本锁定（inherit_current / inherit_version）。
- Fix (2026-05-30): 补全 SetBomSource API 端点 POST /api/bom/:product_id/source，修正 Repository 写入 product_bom_sources 表，新增 Service 层方法。
- Verifier:
  - Unit: go test ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/catalog ./internal/application/bom -count=1; node --test src/lib/bom.test.js src/lib/product-settings.test.js
  - API: go test ./internal/interfaces/http/bom ./internal/interfaces/http/catalog ./internal/interfaces/http/production ./internal/interfaces/http/costing ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/support -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-29-sku-bom-inheritance.md
- Deployment: development deployed at origin/develop 38e393864b2e89e0b8c9a4082f99165ede59e81e; backup root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260529225513
- Last update: 2026-05-30 00:53 Asia/Shanghai
- Notes: reserve_req_id.sh indicated PR-385 but --claim hit the known awk multiline bug; using manual PR-385 seed/update. Browser verified on development with target SKU 497: inherited source SKU 496 / BOM V001 was read-only, UI derive changed it to derived_owned, edited derived BOM ratio to 90 while source BOM stayed 100; audit logs include inherit_current id 3889, copy_sku id 3890, derive_owned id 3895, item save id 3897. 2026-05-30 bugfix: SetBomSource 之前未在 Repository 接口注册、缺少 API 端点和 Service 方法、SQL 错误写入 products 表不存在的列；现已修正并重新部署。

### PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL
- Branch: codex/process-bom-workorder-sku-model
- Owner/session: Codex / 2026-05-26
- Status: deployed
- Scope: 通用制造模型一期，BOM 预期损耗率优先并兼容 yield_rate；工序卡记录实际投入、实际产出、实际损耗；工单展示冻结预期损耗和工序实际损耗汇总。
- Verifier:
  - Unit: go test ./internal/domain/production ./internal/application/bom -count=1
  - API: go test ./internal/interfaces/http/bom ./internal/interfaces/http/production ./internal/interfaces/http/support -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md; orderapp-remote/docs/OP_MANUAL_PRODUCTION.md; orderapp-remote/docs/OP_MANUAL_COSTING.md
  - Review/acceptance: docs/superpowers/specs/2026-05-26-process-bom-workorder-sku-model-design.md; docs/superpowers/plans/2026-05-26-process-bom-workorder-sku-model.md; docs/acceptance/2026-05-26-process-bom-workorder-sku-model.md
- Deployment: development deployed; feature merge commit ecf45a767d826a8b39d33aa99948f94b83174304, evidence-sync commit f397532f554c392502f64e0c208c43820b3f3000; latest app backup root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260526131856
- Last update: 2026-05-26 13:20 Asia/Shanghai
- Notes: reserve_req_id.sh --claim hit the existing awk multiline bug after indicating PR-375; continued with manual PR-375 seed/update. Browser verified on development: BOM saved expected loss for Codex测试速溶盒装 10条/盒, job card #10 saved 100 input / 92 output / 8% actual loss, work order WO-0000000029 showed expected loss and actual loss summary, 操作日志 showed save_expected_loss_rate and update_actuals.

### PR-372-PRODUCT-CONFIG-DISPLAY-UNIT
- Branch: codex/product-config-display-unit-20260525
- Owner/session: Codex / 2026-05-25
- Status: deployed
- Scope: SKU设置商品配置模板移除固定展示方式，改为价格表展示单位并继承单位模板报价单位。
- Verifier:
  - Unit: node --test src/lib/product-settings.test.js
  - API: go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates -count=1; go test ./internal/interfaces/http/support -run TestDev372 -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_COSTING.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-25-product-config-display-unit.md
- Deployment: development deployed at origin/develop 2865b7c9891607c453d7bbf38b20295bed212fd8
- Last update: 2026-05-25 22:34 Asia/Shanghai
- Notes: reserve_req_id.sh --claim failed on awk multiline string; claimed manually to avoid PR-371 WIP collision. Local Vite renders request failure without a同源后端；deployment后在 development 环境做浏览器复验。

### PR-373-PRODUCT-CONFIG-UI-POLISH
- Branch: codex/product-config-ui-polish-20260525
- Owner/session: Codex / 2026-05-25
- Status: verified
- Scope: 商品配置模板列表改成明确列表行；价格表生成规则三个下拉框三列对齐，说明改为感叹号弹出提示。
- Verifier:
  - Unit: node --test src/lib/product-settings.test.js
  - API: go test ./internal/interfaces/http/support -run TestDev373 -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_COSTING.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-25-product-config-ui-polish.md
- Deployment: pending
- Last update: 2026-05-25 22:58 Asia/Shanghai
- Notes: Browser feedback from Van on product settings page: template row did not look like a list; price rule dropdowns were not aligned; inline note should become exclamation tooltip.

### PR-374-COMPOSABLE-PRODUCT-PRICING
- Branch: codex/composable-product-pricing-20260525
- Owner/session: Codex / 2026-05-25
- Status: verifying
- Scope: 打通商品配置组合式计价，不新增 cost_model 枚举；速溶盒装按 BOM 每盒 10 条原料成本 + 阶梯利润率生成元/盒，价格表/PDF/录单保留自定义单位。
- Verifier:
  - Unit: go test ./internal/domain/costing -run 'TestComposableProductPricingUsesBomUnitCostAndCustomQuoteUnit|TestCustomGradientDisplayUnitDoesNotFallbackToLb' -count=1; node --test src/lib/bean-list-pdf.test.js src/lib/order-entry.test.js
  - API: go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsReadsComposablePriceRulesAndBomUnitCosts -count=1; go test ./internal/interfaces/http/support -run TestDev374 -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_COSTING.md; orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-25-composable-product-pricing.md
- Deployment: pending
- Last update: 2026-05-25 23:35 Asia/Shanghai
- Notes: Van 要求完成后调用浏览器，用测试数据打通测试；部署后需在 development smoke 中覆盖 SKU设置/产品价格表/录单链路。

## Template

```markdown
### PR-000-SHORT-SLUG
- Branch:
- Owner/session:
- Status: planned | red | implementing | verifying | pushed | merged | deployed | blocked
- Scope:
- Verifier:
  - Unit:
  - API:
  - Frontend/build:
  - Manual:
  - Review/acceptance:
- Deployment:
- Last update:
- Notes:
```
