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

## Release state

Development deployment, clean-main integration, production deployment, runtime/static-asset verification and rollback anchors are pending. The release-equivalent Go suite, frontend suite/build, change checks, and targeted isolated database/API tests pass. Development verification must still use isolated data; production smoke remains read-only. Van's business acceptance is pending.

The shared development/production host was checked read-only before preflight: its system disk is 60 GB with 3.8 GB free (94% used), below the agreed 6 GB minimum. Docker BuildKit cache is empty (0 B); 2.221 GB of reclaimable Docker images were left intact to preserve rollback images. The deploy lock was available, both app containers were running, and both PostgreSQL containers were healthy. No remote build or deployment was started. The initial capacity block was resolved under the user's subsequent cleanup authorization: 202 old rollback image tags from before 2026-10-23 were removed without force and only when their image was unused by every container. All newer rollback images, all source backups and database volumes were retained. Available space rose from 3.8 GB to 28 GB. Cleanup audit: `/root/pr729-image-cleanup-20261025172850.json`.

## Integration continuation

PR #299 merged to `develop` at `b6a226a1735f2e0c25e307e12490d721dec2393f`. Development deployment is running from a clean clone at that exact commit. The clean-main integration starts at `a4e838522ddd9540a188f59a35bafa715ca089c8` and cherry-picks only this change, retaining main-specific BOM fixes and prior acceptance evidence. On the integration tree, all Go tests without the optional test-database environment, all frontend tests/build, and isolated PostgreSQL completion/reconciliation/replenishment/location-repair tests passed. Production business data snapshots were recorded before release; no production business writes were performed.
