# Production named price-table draft quota repair

## Report and reproduction

User reported production `载入最新发布版` and `发布价格表` fail at `kferp:named-price-tables:kferp:price-list-generation:factory:official:all:product-catalog:56` with `Storage.setItem` quota exhaustion. Previous c591b765 protected the pre-replacement backup only; the subsequent batch save still used raw localStorage and prevented the publish request.

## Change

- Named batches now persist as serialized snapshots in IndexedDB, waiting for transaction commit; scoped writes are ordered and reads await outstanding writes.
- Existing localStorage batches remain readable when no IndexedDB record exists. New edits do not consume the small legacy cache or restore its stale value over newer data.
- Restored named editors use one persistence path instead of writing duplicate standalone caches.
- Explicit actions await storage safely. Customer/category switches invalidate pending actions; repeated publish clicks are blocked. Storage errors stay visible while current edits remain available for saving to the server.
- No schema/API/business-price changes. Existing save/publish APIs retain operation-log behavior. No real customer price list was published as part of verification.

## RED / GREEN

- Baseline quota tests: 0/3, raw quota error and no durable transaction wait.
- Async scope/duplicate-publish regression tests: 0/3 before guards; pass after guards.
- Targeted batch/editor/refresh/restore suite: 54/54.
- Full Vue suite: 1454/1454; production Vue build passed.
- Existing HTTP handler regression `TestNamedPriceTableBatchAPIAuthScopeAndPartialPDF` passed.
- Native browser fixture on isolated localhost origin filled localStorage to actual QuotaExceededError, backed up the old draft, loaded and durably saved the published snapshot, then reloaded and verified its fixed price. Legacy cache remains intact. Screenshot: `/tmp/kferp-draft-quota-browser-pass.png`.
- Manual: `orderapp-remote/docs/OP_MANUAL_COSTING.md`, existing Vue costingManual entry serves the same source.

## Delivery boundary

Production-target hotfix based on origin/main@973d5481. A separate newly merged order-catalog change on develop is outside this production repair; synchronize the hotfix into develop separately and release only this production-target branch to main. The hotfix was integrated separately into develop4d03e0cd and main b6efdd71; develop Vue1457/build also passed.


## Production delivery verified

- Feature: `2b946de6`; production merge and running release: `b6efdd71d202d3d3a8d7ddea025a957709aeae53`.
- Command: `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production` from clean pushed main.
- Standard release gates passed: Vue1454/build, miniapp371/typecheck/build/24 pages, all Go tests on host and inside image, image build, application readiness and external login smoke.
- Source rollback: `/opt/stacks/erp-production/orderapp.backup.deploy-20261021112612-b6efdd71d202`.
- Image rollback: `kferp-orderapp-rollback:production-20261021112612-b6efdd71d202`.
- Running image: `sha256:838fd02e239b6e5384d2f1308ac09ef6dfba5a2ed76ddfda84fa43308fefc1fa`; started `2026-10-21T03:34:24.601002581Z`.
- Public costing shell/login/requirement read: HTTP200. Unauthenticated publication API: HTTP401. Database healthy.
- `/app/vue-shell/assets/CostingView-B1E4XUlL.js` contains the new persistent draft store marker and returns HTTP200; served file SHA256 matches both promoted source and running container. Updated manual guidance present in production.
- Native browser proof and full logs retained in `/Users/yiiiple-work/.codex/visualizations/2026/10/21/01a15c6e-3fac-7760-8d44-9810f2744dc2/`.
- No live price table was republished for testing. User business acceptance remains separate. Fixed local miniapp directories were not replaced; no WeChat upload/publication.
