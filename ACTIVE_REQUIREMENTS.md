# ACTIVE_REQUIREMENTS

Purpose: short-lived coordination for Codex workflows. Keep active requirement ids, branches, verifier commands, deployment ownership, and unresolved blockers here so future sessions do not have to recover this from chat history.

This is not long-term memory. Move durable product/deployment decisions to `MEMORY.md` or source docs, then remove stale entries from this file.

## Active

### PR-394-PRODUCT-CLASSIFICATION-VIEW-TABS
- Branch: codex/product-classification-view-tabs-20260602
- Owner/session: Codex / 2026-06-02
- Status: in_progress
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
- Deployment: pending
- Last update: 2026-06-02 Asia/Shanghai
- Notes: Van requested no browser/manual验收; use code/docs/unit/API/build verification. RED evidence: frontend tests initially failed on missing classification template usage helpers, page-level tabs and BOM return navigation; catalog API tests initially failed because batch customer aliases still accepted `classification_template_id`, classification templates retained customer ownership, and product/customer classification-template usage APIs were missing; support marker tests caught stale PR-393 wording. GREEN evidence: `node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/view-routing.test.js` passed 110/110; `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1` passed; `npm run build` in Vue shell passed with existing chunk-size warning; `scripts/verify_kferp.sh changed` exited 0.

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
