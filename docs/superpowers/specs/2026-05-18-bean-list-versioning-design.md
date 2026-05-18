# Bean List Versioning Design

## Context

Bean list publishing already stores immutable publication snapshots in `bean_list_publications`. Customer portal and miniapp display the latest customer-owned publication first and fall back to the latest official publication. The missing product behavior is version selection, server-side one-time generation, order-entry version binding, and a first-use update prompt for customers.

## Product Rules

- Every published bean list is a versioned immutable snapshot and remains in the version list.
- A customer with customer-owned published bean lists may choose either "follow latest" or a fixed customer-owned publication version.
- A customer without customer-owned bean lists does not see a version selector and always uses the latest official bean list.
- The miniapp defaults to the current effective version: fixed customer publication, latest customer publication, or latest official publication.
- ERP order entry must select a bean list version. The default is the same current effective version.
- Orders store the selected `bean_list_publication_id` and `version_no` so historical order pricing does not drift when a bean list changes.
- For each customer and bean list version, the first order attempt shows the version update summary once. After confirmation, that customer is not prompted again for the same version.

## Data Model

Extend `customer_portal_profiles`:

- `bean_list_mode TEXT NOT NULL DEFAULT 'latest'`
- `bean_list_publication_id BIGINT NOT NULL DEFAULT 0`

Add `bean_list_publication_assets`:

- `publication_id`, `asset_type`, `content_type`, `cache_key`, `payload`
- Unique on `(publication_id, asset_type)`
- Used for server-side generated artifacts so one publication is rendered once per artifact type.

Add `customer_bean_list_acknowledgements`:

- `customer_id`, `publication_id`, `acknowledged_at`, `acknowledged_by`
- Unique on `(customer_id, publication_id)`

Extend `orders`:

- `bean_list_publication_id BIGINT NOT NULL DEFAULT 0`
- `bean_list_version_no TEXT NOT NULL DEFAULT ''`

The existing `bean_list_publications` table remains the canonical version list.

## Backend Behavior

Customer portal repository resolves the effective bean list in this order:

1. Fixed customer-owned publication configured on the profile, only if it still belongs to that customer and is published.
2. Latest customer-owned published publication.
3. Latest official published publication.

The service page returns both the effective bean list and metadata:

- whether a customer-owned list exists
- available customer-owned versions for settings
- cache key for miniapp local cache
- whether this customer has acknowledged this effective publication
- update diff summary versus the customer's previous acknowledged or previous available publication

Admin profile save validates that a fixed publication belongs to the selected customer. If the customer has no customer-owned publication, the fixed value is cleared and the UI hides the selector.

Order form API returns the effective bean list for the selected customer. Order save accepts a selected publication ID; if omitted, it resolves the effective one. The order stores the publication ID/version. Price lookup uses the selected publication snapshot when it can match a product and tier; existing product price tiers remain the fallback.

Miniapp acknowledgement API records that a customer saw and accepted the version prompt before order submission proceeds.

## Frontend Behavior

Costing bean list publishing keeps the version list behavior and adds server-side cached PDF generation through the publication asset endpoint.

Customer portal configuration shows "Bean list display version" only when the customer owns at least one published bean list. Options are "follow latest" and customer-owned version rows.

ERP order entry shows a bean list version selector after customer selection. The default follows the effective version. If a newer effective version is unacknowledged for that customer, order save is blocked by a confirmation dialog that shows the diff.

Miniapp service page keeps native display and local cache. When `cache_key` is unchanged, local content is reused. When it changes, the page fetches the new version, caches it, and exposes update prompt data for order submission.

## Diff Rules

Diff is generated from publication `content_json.groups[].items[]`.

The stable key is `code` when present, otherwise normalized `name`. The diff reports:

- added items
- removed items
- changed prices
- changed recommended use, flavor, or description

Changed fields are returned as structured data so Vue and miniapp can highlight the changed row or field without pure text diffing.

## Audit And Manuals

Audit entries are required for:

- publishing and withdrawing bean lists, already covered
- saving a customer's bean list display mode
- generating a server-side publication asset
- acknowledging a customer bean list version
- saving an order with a selected bean list version

Manuals to update:

- `OP_MANUAL_COSTING.md`
- `OP_MANUAL_CUSTOMER_PORTAL.md`
- `OP_MANUAL_ORDER_SALES.md`
- mirrored files under `orderapp-remote/docs/`

Acceptance evidence goes into `ACCEPTANCE_TESTS.md` and a new acceptance note under `docs/acceptance/`.

## Test Strategy

- Unit tests for effective publication resolution, diff calculation, cache key behavior, and order-entry helper defaults.
- API tests for customer portal bean list service page, acknowledgement, customer profile fixed version validation, order form defaults, and order save version persistence.
- Existing miniapp tests extended for cache and update prompt behavior.
- Build checks for Vue/Vite frontend and miniapp TypeScript tests.
