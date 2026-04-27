# Costing Settings And Material Metadata Design

## Goal

Extend the Excel costing migration so the ERP owns the configurable costing parameters, material batch/reference data, commercial wholesale tiers, and retail/commercial bean-list previews needed for day-to-day use.

## Requirements

- ERP settings must expose the costing parameters currently seeded from Excel, so operators can update values without changing code.
- Material purchase price is the single source for costing. Costing must keep reading `materials.purchase_price`.
- Every material must have a batch number. When blank, the default batch number is today's date in `YYYYMMDD` format.
- Material records must store bean-card fields from the Excel `物料成本` sheet: origin, processing station, variety, process method, grade, altitude, flavor, and bean-list version.
- Cost trial must show commercial wholesale gradient prices.
- Commercial wholesale tiers are four pound-based tiers:
  - `2-13磅`
  - `14-23磅`
  - `24-47磅`
  - `大于47磅`
- Retail bean list must show 227g and 250g retail prices plus flavor text.

## Architecture

- `internal/domain/costing` adds explicit commercial tier output while keeping the existing Excel-compatible price arrays for compatibility.
- `internal/infrastructure/postgres/costing` continues to load material purchase prices and publishes commercial tiers as 454g package tiers, so order pricing matches by package count in pounds.
- `internal/infrastructure/postgres/materials` extends the `materials` table with batch and bean-card columns; application and HTTP DTOs pass those fields through.
- `internal/interfaces/http/costing` adds list/update APIs for editable costing parameters.
- Vue shell adds a `成本参数设置` view under `设置` and expands the existing `成本核算` and `物料档案/库存` views.

## Testing

- Domain unit tests assert the four commercial tiers and Excel cached prices.
- Material repository tests assert blank batch numbers normalize to today's date.
- HTTP tests assert costing parameter settings can be listed and updated through service boundaries.
- Frontend build verifies the new views compile.

