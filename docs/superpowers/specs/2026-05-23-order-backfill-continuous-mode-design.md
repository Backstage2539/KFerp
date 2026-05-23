# Order Backfill Continuous Mode Design

## Goal

Make `订单补录` visible as an actual entry mode instead of only a hint. Operators can still edit `单据日期` and `订单日期` for one-off backfill, and can turn on continuous mode to save one order and immediately keep entering the next order with shared header context.

## Scope

- Add a toggle on the Vue/Vite order entry page for new orders only.
- When the toggle is off, saving behaves exactly as today: save the order and follow the backend redirect.
- When the toggle is on, the primary action becomes `保存并继续补录`.
- After a successful continuous save, stay on the entry page, show the saved order number, keep shared header context, and reset order-specific fields.

## Preserved Context

Continuous backfill keeps these fields for the next order:

- Customer and customer query.
- `单据日期` and `订单日期`.
- Customer-derived source/order type fields.
- Payment status and receipt method.
- Shipping status, logistics company, logistics product, freight amount, freight note, rounding setting.
- Bean-list publication selections for roasted, green, and drip products.

## Reset Context

Continuous backfill clears these fields for the next order:

- Product detail rows, replacing them with one blank row.
- Tracking numbers.
- Payment amounts, payment voucher id, voucher preview state, and selected file.
- Order notes, order discount, outsourced fee fields, field errors, and transient row menus.

## Data Flow

No backend API change is required. The normal `/api/order` save payload already includes both dates and all header fields. Continuous mode changes only the post-save client behavior:

1. Build and validate the existing order payload.
2. Save with `/api/order`.
3. If `continueBackfill` is false, keep the current redirect behavior.
4. If `continueBackfill` is true, do not redirect. Reset the client form for the next order while keeping the preserved context.

## Testing

- Frontend unit/static tests cover the toggle, labels, save actions, no-redirect continuous branch, and reset/preserve field behavior.
- Support seed tests cover PR/DEV/UT/API/REV rows and manual/acceptance docs.
- Existing order API tests cover date payload behavior from PR-342, so no new backend API endpoint test is needed.
