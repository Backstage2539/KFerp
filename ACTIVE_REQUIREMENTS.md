# ACTIVE_REQUIREMENTS

Purpose: short-lived coordination for Codex workflows. Keep active requirement ids, branches, verifier commands, deployment ownership, and unresolved blockers here so future sessions do not have to recover this from chat history.

This is not long-term memory. Move durable product/deployment decisions to `MEMORY.md` or source docs, then remove stale entries from this file.

## Active

### PR-389-BOM-GROUP-SPECIAL-ATTRS
- Branch: codex/bom-version-special-attrs-20260531
- Owner/session: Codex / 2026-05-31
- Status: verified
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
- Deployment: not requested yet.
- Last update: 2026-05-31 Asia/Shanghai; RED tests were added before implementation. Final local verification passed: `go test ./...`; `node --test src/lib/bom.test.js src/lib/product-settings.test.js`; `npm run build` in `orderapp-remote/frontend-vue-shell`; `scripts/verify_kferp.sh changed`.
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
