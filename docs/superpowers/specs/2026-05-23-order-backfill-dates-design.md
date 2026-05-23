# Order Backfill Dates Design

## Goal
Support order backfill by separating the ERP document date from the customer's real order date, while keeping both dates editable in normal order entry and visible on customer-facing sales documents.

## Decisions
- Add `document_date` to orders as the ERP document date.
- Keep `order_date` as the customer's real order/business date.
- New orders default both dates to today.
- Legacy orders backfill `document_date` from `order_date`.
- If older API clients omit `document_date`, the server uses `order_date` as the document date.
- Order numbers are generated from `document_date`.
- Sales order PDF/PNG snapshots show both `单据日期` and `订单日期`.
- Delivery note snapshots show `单据日期`, `订单日期`, and existing `出库日期`.
- The order entry page always shows both date fields and labels the page capability as `订单补录`.

## Scope
This first implementation delivers the date model and document rendering foundation. Multi-group rapid entry and merged multi-order sales documents remain a later workflow layer because they affect order grouping, production, fulfillment, inventory, and sales document ownership.

## Tests
- API save/edit round trip persists and returns both dates.
- Frontend payload includes both dates and the order entry page exposes the labels.
- Sales order and delivery note renderers include the new date labels.
- Operation manual, requirements, acceptance evidence, and PR/DEV seeds document the workflow.
