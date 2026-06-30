# E2E Raw Material Order Production Flow Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use `superpowers:test-driven-development` for every blocker fix and `browser:control-in-app-browser` for ERP browser proof. Track status in `ACTIVE_REQUIREMENTS.md` under PR-508.

**Goal:** 打通并留证一条 KFerp 主链路：原料建档/入库 -> 商品/子 SKU 和生产 BOM -> 下单 -> 生产计划/工单/工序卡 -> 领料/完工入库/库存追溯/订单履约。过程中遇到阻断必须先复现、再修复、再验证。

**Architecture:** 先用现有 Vue/Vite 页面和已发布 API 走通流程，不改变业务模型边界。原料库存以库存单位入库；商品档案以父商品 + 子 SKU / 销售规格模板表达；BOM 绑定产出子 SKU；订单使用已发布价格快照；生产计划只消费可解析的库存缺口；工单执行通过 WIP、Stock Entry、完工入库和生产日志闭环。

**Tech Stack:** Go HTTP/application tests, Vue/Vite source/helper tests, PR/DEV support seed tests, in-app browser, existing operation manuals and acceptance docs.

---

## Scope Guard

- Do not invent a new manufacturing path. Use the current pages and APIs: 原料、原料入库、商品档案、生产 BOM、录单、生产计划、生产工单、工序卡、库存作业、操作日志。
- Do not move unit conversion ownership into 商品价格管理. Unit templates / sales spec templates remain the source for product sales and inventory conversion.
- Any user-triggered write in the verified flow must be observable in 操作日志 or a documented existing blocker.
- Browser proof is required before completion. API/source tests alone are not enough for this goal.

## File Map

- `ACTIVE_REQUIREMENTS.md`: PR-508 active coordination, baseline, blockers, browser evidence.
- `orderapp-remote/internal/interfaces/http/support/req_store.go`: PR/DEV/API/REV seed rows.
- `orderapp-remote/internal/interfaces/http/support/dev_508_e2e_raw_material_order_production_flow_test.go`: contract test for PR-508 tracking/docs.
- `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/docs/REQUIREMENTS.md`, `orderapp-remote/docs/ACCEPTANCE_TESTS.md`: durable requirement and acceptance entries.
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`, `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`, `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`, `orderapp-remote/docs/OP_MANUAL_STOCK.md`: workflow/manual anchors.
- `orderapp-remote/docs/acceptance/2026-06-30-e2e-raw-material-order-production-flow.md`: running acceptance evidence.

## Task 1: PR/DEV tracking and contract verifier

- [x] Create failing support test requiring PR-508 seed rows and docs/manual/acceptance markers.
- [x] Run: `go test ./internal/interfaces/http/support -run TestDev509 -count=1`. Expected: FAIL before seeds/docs exist.
- [x] Add PR-508 rows to `req_store.go` and PR-508 sections to active/durable docs.
- [x] Re-run the same support test. Expected: PASS.

## Task 2: Browser/API e2e path

- [x] Open the ERP in the in-app browser and verify login/session state.
- [x] Build a timestamped test-data prefix for PR-508.
- [x] Walk the UI/API path: 原料 -> 原料入库 -> 商品/子 SKU -> BOM -> 下单 -> 生产计划 -> 工单/工序卡 -> 库存作业/完工入库 -> 订单/库存/操作日志 review.
- [x] Record each successful object id/name and each blocker in the PR-509 acceptance file.

## Task 3: Fix blockers with RED/GREEN

- [x] For every blocking defect, write the narrowest failing Go/Vue test or browser reproduction note first.
- [x] Implement the smallest repo-consistent fix where the issue was inside PR-508 scope; browser-only data/infrastructure blockers are recorded with exact evidence.
- [x] Re-run the targeted test, the affected package/view tests, and the browser step that originally failed.

## Task 4: Final verification and delivery

- [ ] Run the targeted backend and frontend suites for materials, catalog, BOM, sales, production, stock, manuals, and support.
- [ ] Run `npm run build`, `scripts/verify_kferp.sh changed`, and `git diff --check` if the change set reaches release scope.
- [ ] Merge/deploy development only after branch push, latest `origin/develop` sync, and required checks pass.
