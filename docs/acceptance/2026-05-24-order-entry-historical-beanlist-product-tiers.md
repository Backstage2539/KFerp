# PR-353-ORDER-ENTRY-HISTORICAL-BEANLIST-PRODUCT-TIERS

## Requirement
When order entry selects a still-published historical bean-list version, product candidates and price tiers must come from that selected publication snapshot. The order form API must return product tiers for every published commercial and green bean-list publication that is available for selection, not only the latest publication.

## Root Cause
`fetchCommercialOrderPublicationTiers` and `fetchGreenBeanOrderPublicationTiers` iterated over all published publications but overwrote the accumulated product tier map on each row. The final API response only carried one publication's tiers for each product. When the UI selected another published historical commercial version, commercial products no longer matched the selected publication and the dropdown appeared to contain only green-bean products.

## Evidence
- RED: `go test ./internal/infrastructure/postgres/sales -run TestMergeOrderPublicationTierMapsKeepsMultiplePublishedVersions -count=1` failed because multi-publication tier merging did not exist.
- GREEN: `go test ./internal/infrastructure/postgres/sales -run TestMergeOrderPublicationTierMapsKeepsMultiplePublishedVersions -count=1` passed after merging tier maps instead of overwriting.
- API coverage: `TestOrderAPIFormReturnsAllPublishedCommercialBeanListTiersForVersionSwitching` documents the `/api/order/form` contract that one product can carry tiers from multiple published commercial bean-list publications. It runs when `ORDERAPP_TEST_DATABASE_URL` or `DATABASE_URL` is available.

## Acceptance Checklist
- [x] Published historical commercial bean-list tiers are preserved alongside latest commercial tiers.
- [x] Published historical green bean-list tiers use the same accumulation path.
- [x] Frontend product filtering can match the selected historical publication because product tiers include that publication ID.
- [x] Operation manual updated: `OP_MANUAL_ORDER_SALES.md`.
