# Production Acceptance And WIP Control Design

## Goal

Continue the ERPNext-inspired manufacturing work with a focused P0/P1 slice: make the production flow easy to verify after deployment, and make WIP reservations visible and controllable before they cause hidden stock conflicts.

## Scope

- P0: add a production acceptance checklist API and Vue page for smoke-checking the core manufacturing flow.
- P1: add WIP reservation read models, manual release/adjust controls, work-order reservation summaries, and WIP transfer suggestions in the material plan.
- P2/P3 are recorded as follow-up development demand, not implemented in this slice: quality gates that block inventory/shipping, variance dashboards, and richer print/label analytics.

## Product Behavior

- Operators can open a production acceptance page and see whether the system has the minimum data needed for the production loop: warehouses, material batches, WIP stock, running/completed work orders, production logs, quality records, and traceable finished batches.
- Production planners can see `WIP可用(g)` and `建议领到WIP(g)` in the material plan. The suggestion is the amount that should be transferred from raw stock to WIP before starting production, capped by raw-stock availability.
- Production and warehouse users can view WIP reservations by work order/material, including required, reserved, consumed, remaining reserved, WIP total, and available-to-new-work-order grams.
- If a reservation is wrong, an authorized production operator can adjust the reserved grams, but not below already consumed grams and not above WIP capacity after other open reservations are considered.
- If an abandoned work order still holds reservation, an authorized production operator can manually release that work order's open reservations. The action is audited.

## Architecture

- Domain rules live in `internal/domain/production`: reservation remaining quantity and adjustment validation.
- Application service owns command/query normalization and exposes acceptance, reservation list, adjustment, and release use cases.
- Postgres production adapter owns SQL read models and transactional writes.
- HTTP routes stay in the production module under `/api/produce/*`.
- Vue/Vite receives one new production page and targeted enhancements to existing pages. No legacy HTML template feature work.

## API

- `GET /api/produce/acceptance-smoke`
  - Returns checklist rows: code, title, status, count, detail, target view.
- `GET /api/produce/wip-reservations?status=reserved&work_order_no=&material_id=&limit=200`
  - Returns reservation rows and summary totals.
- `POST /api/produce/wip-reservations/adjust`
  - Body: `reservation_id`, `reserved_g`, `reserved_units`, `note`.
- `POST /api/produce/wip-reservations/release`
  - Body: `running_item_id`, `work_order_no`, `note`.

## UI

- Add `生产验收` under production management.
- Add WIP reservation summary columns to `生产工单`.
- Add WIP reservation drawer/section to `仓库库存`.
- Add `WIP可用(g)` and `建议领到WIP(g)` to `生产计划/开始生产` material plan.
- Update production manual and source Markdown with the new verification and reservation operations.

## Testing

- Unit tests for domain reservation rules and service normalization.
- API tests for acceptance smoke, reservation listing, adjustment, release, and material-plan response fields.
- Source guard tests for Vue wiring and manual updates.
- Full verification: `go test ./... -count=1`, `node --test src/lib/*.test.js`, `npm run build`, `git diff --check HEAD`.
