# PR-729 Production recovery and packaging closeout

## Scope and safety boundary

This change repairs the production screens and posting rules needed to continue packaging work: valid warehouse API calls, readable warehouse names, exposed work-order actions and progress, work-order-scoped historical correction, partial operation completion, audited material-batch location repair, and shortage-linked replenishment from the original frozen recipe and route.

The source worktree is `codex/production-flow-repair`, based on `origin/develop@f25458c4cca46a181f11831db0e4d04a1b7cb44c`. Production verification is read-only. The seven historical packed orders have not been corrected or received: their actual bag counts and actual material use require site confirmation. Existing duplicate warehouse balances have not been changed: staff must confirm physical locations and reservations first. No production business records were written during implementation.

## RED/GREEN evidence

- Frontend regression tests first failed because the completed-material filter also offered a product dependency as a linked replenishment source. The filter now requires a completed upstream material work order with a positive shortage. The Vue helper and work-order contract tests pass.
- The first PostgreSQL linked-replenishment run exposed an aggregate `NULL` scan for plans without replenishment links. The query now coalesces its empty result and the ordinary production-plan submission path passes again.
- A second isolated run exposed that some test/deployment schemas do not have the optional customer-processing reservation table. Preview now checks for the relation before reading its reservations; when present, those reservations are deducted from available inventory.
- The linked API integration test verifies a 12.7 kg packaging shortage with 2 kg of free same-owner WIP creates a 10.7 kg replenishment draft; existing 10 kg reservation remains protected. It then verifies the draft covers the remaining shortage, same-request retry returns the original plan, changed content under the same request key is rejected, the cloned BOM and process-route snapshots match the source work order, submission adds the shortage dependency to the selected consumer, and an outdated preview returns conflict without creating a draft.
- `TestLinkedReplenishmentPreviewCreateIdempotencyAndWorkOrderAllocation` and `TestLinkedReplenishmentRejectsStalePreviewVersion` passed serially against isolated PostgreSQL schemas after the final consumer-source integrity predicate. The cases cover free-stock offset, protected reservations, frozen BOM/route snapshots, draft coverage, request-key reuse/rejection, consumer allocation, and stale-preview rejection. A parallel run with the whole Go suite hit local PostgreSQL `out of shared memory` during schema setup and is not counted as product evidence.
- `env -u ORDERAPP_TEST_DATABASE_URL go test ./...` passed all Go packages. This matches the Dockerfile's Go test step, which has no test database configured.
- `scripts/verify_kferp.sh frontend` passed all 1,596 frontend tests and the Vite production build (6,750 modules). Vite emitted the existing large-chunk advisory. `scripts/verify_kferp.sh changed` also passed.
- A separate database-enabled `go test -p 1 ./...` run exposed failures across unrelated legacy test fixtures (including missing schema objects and stale product-spec expectations) and was stopped after about nine minutes while the production package was still running. This run is inconclusive for the broad DB suite; the targeted production and stock PostgreSQL/API tests above are the relevant database evidence.

## Business-data verification still required

Before site staff performs the seven historical closeouts, verify each actual packed bag count, frozen bag specification, actual consumed materials, target warehouse name and any prior receipts. Submit one work order at a time or use the supported batch interface, then reconcile stock, cost, receipt document and operation log. Do not use old report quantity or planned quantity as a substitute.

Before correcting the three material and three packaging-batch location discrepancies, compare original receipts/transfers, reservations and physical warehouse locations. The repair preview must balance each batch's locations to its remaining batch quantity. Move reservations only to confirmed locations. The repair changes accounting locations and reservation references; it does not move physical goods.

For the nine unfinished packaging work orders, use the work-order list's direct operation action. Where a completed upstream material work order leaves a shortage, use “关联补产” to preview same-owner free stock, other work-order reservations and in-flight/draft supply before creating the linked plan. Complete the supplement through the ordinary plan workflow, then continue the packaging operation. Actual output and warehouse placement remain site-entered actions.

## Browser end-to-end follow-up

The final browser acceptance uses a separately restored development database, `pr729_final`, on the internal-only Docker network `pr729-browser-isolated`, reached through a local SSH tunnel. No notification or logistics provider can be reached. The test employee and recipient are isolated fixtures. Business writes were performed by clicking the Vue pages, not by API scripts or SQL updates. SQL is used only for fixture setup and final read-only reconciliation.

Browser order `SO-20261025-0001` (isolated ID 1638) contains 24 bags of 初晓, frozen 227g specification. The browser created and submitted plan `PP-0000000118`, then ran material work order `WO-PP-0000000118-0000000121` and packaging work order `WO-PP-0000000118-0000000120`:

1. Create the order; save returns to the Vue order list.
2. Create the plan, review material sources, set the roster and start the work orders. Transfer the missing 3,385g raw ingredient and 24 bags into WIP using normal stock-entry controls.
3. Finish roast batch 1: 4.086kg, 30 minutes; automatically consume ingredients and receive into WIP (SE50/51).
4. Finish packaging 1 bag, 227g actual input, 1 minute; automatically receive 1 finished bag (SE52/53), keep the operation active, and open Continue without taking over another employee's task.
5. Finish roast batch 2: 1.362kg, 12 minutes; 846g remaining ingredient is accepted exactly (SE54/55).
6. Continue packaging the remaining 23 bags, 5,221g input, 20 minutes, and finish the operation (SE56/57). Both work orders are completed and the order becomes production complete.
7. Enter synthetic recipient/tracking `PR729-QA-20261025-0001` in the order, select shipped and save. The batch confirmation lists 1 + 23 units, without misleading 0g quantities.
8. Read-only reconciliation confirms finished warehouse balance 0, both finished batches remaining 0, shipment deductions 1 + 23, and two `sales_order_shipment` ledger rows totaling -24 units. Packaging consumed 5,448g roast and 24 bags exactly once. Packaging labor totals 21 minutes and 12 currency units; roast labor totals 42 minutes and 71.4. Source documents and operation/order audits exist.

The first shipping attempt exposed a real bug: status changed to shipped while all 24 bags remained. That screenshot is retained as RED, not passed acceptance. After fixing the ordinary order-save transaction, the isolated order was reset to unshipped through the browser and shipped again. The final read-only ledger reconciliation is GREEN. Inventory shortage rolls back the saved status, and repeated saved shipping state does not deduct twice.

The requested inline receipt was separately clicked through on the cloned `WO-PP-0000000114-0000000116`: explicitly entered test quantity 4.54kg, split 3.4 + 1.14kg, and test reason; the current row reported receipt into WIP with SE58/59 and refreshed without navigating. The live work order was untouched. Actual quantity remains blank until a person supplies it. The dead drawer buttons and independent production-record/reconciliation module are removed; historical links return to work orders.

Additional RED/GREEN cases found during browser acceptance cover split-gram rounding, small-batch suggested input, direct partial completion employee ownership, order production readiness, and clearing previous batch time/draft. Time subtraction handles both historical `ActualMinutes` and `actual_minutes` event payload keys. The API asserts the second batch suggests zero already-posted minutes.

Final gates: all 1,604 Vue tests and Vite build pass; all Go packages pass without DB configuration; targeted `TestOperationCompletion*`/`TestLegacyProduction*` PostgreSQL API tests pass; `TestPR729OrderSaveShipmentDeductsActualFinishedStock` passes with actual batch, warehouse, insufficient-stock rollback and repeated-save checks. The broad historical DB-suite limitation above remains unchanged.

Local evidence: `/private/tmp/pr729-browser-evidence/` contains screenshots `final-01-order.png`, `final-02-roast-first.png`, `final-03-partial-continue.png`, `final-04-all-received.png`, `final-06-shipment-posted.png`, `06-inline-receipt-complete.png`, and `final-reconcile-after-shipment.log`. Targeted RED/GREEN and full gate logs are `/private/tmp/pr729-*.log`. These are operational evidence locations, not application data.

## Release state

PR #299 is merged into develop at `b6a226a1735f2e0c25e307e12490d721dec2393f` and the original development deployment passed. The inline receipt/browser fixes are being integrated as a follow-up before the clean-main PR #300 is released. Final deployed commits, backups and read-only smoke are recorded after release.

The earlier disk gate was cleared under the user's cleanup authorization: unused old rollback images were removed while retaining running images, recent rollback anchors, database/volumes and source backups. The host has about 27GB free. Production pre-release backup `/opt/stacks/erp-production/backups/pr729-pre-release-20261025174556.dump` was validated. Production business checks remain read-only; site counts, actual batch positions and Van's business acceptance remain pending.
