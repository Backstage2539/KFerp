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

Production-target hotfix based on origin/main@973d5481. A separate newly merged order-catalog change on develop is outside this production repair; synchronize the hotfix into develop separately and release only this production-target branch to main. Runtime and rollback evidence will be added after deployment.
