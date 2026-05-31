# ACTIVE_REQUIREMENTS

Purpose: short-lived coordination for Codex workflows. Keep active requirement ids, branches, verifier commands, deployment ownership, and unresolved blockers here so future sessions do not have to recover this from chat history.

This is not long-term memory. Move durable product/deployment decisions to `MEMORY.md` or source docs, then remove stale entries from this file.

## Active

### PR-386-PRODUCT-MODEL-OVERHAUL
- Branch: codex/product-model-overhaul-20260531
- Owner/session: Codex / 2026-05-31
- Status: verified
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
- Deployment: not deployed; do not deploy until all five phases pass final verification and merge/development deploy is explicitly reached by workflow.
- Last update: 2026-05-31 Asia/Shanghai
- Notes: Phase 1-5 complete and locally verified. Final evidence: frontend target `node --test` 211/211 passed; targeted Go packages passed; `npm run build` passed with existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` exited 0; browser flow passed for 商品管理、客户商品名、产品价格表、录单、客户履约、生产计划、生产工单、操作日志. During browser verification, Phase 3 also fixed price-list preview/PDF grouping to reuse alias-filtered `visibleCostingItems` and align category keys with PDF product selection.

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
