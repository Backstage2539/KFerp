# Excel Costing DDD Adaptation Design

## Goal

Migrate the Excel cost calculation and bean-list pricing workflow into the current DDD ERP architecture on `develop`, without restoring old root-package business code.

## Scope

- Add a pure costing domain engine for formulas copied from the Excel workbook.
- Add an application service for request validation, calculation, saved runs, bean-list previews, and price publishing.
- Add a Postgres adapter for costing parameters, calculation run persistence, product/BOM/material input loading, and catalog price publishing.
- Add HTTP JSON routes under a new `internal/interfaces/http/costing` module.
- Add a Vue 3 view inside the existing Vite shell for cost calculation and bean-list preview.
- Seed the 5 workflow tables with PR/DEV/UT/API/REV rows for this migration.

## Architecture

The feature follows the same boundaries as the current `develop` branch:

- `internal/domain/costing` owns pure formula types and functions. It has no database, HTTP, or application imports.
- `internal/application/costing` owns use-case orchestration and interfaces. It depends only on the domain package and a repository interface.
- `internal/infrastructure/postgres/costing` implements the application repository. It can read `products`, `product_bom`, `product_bom_items`, `materials`, and write costing tables plus catalog price tiers.
- `internal/interfaces/http/costing` owns Echo route registration and JSON response handling.
- `internal/appmain` stays as the composition root and only wires the service, repository, routes, and schema step.

## Data Flow

1. `GET /api/costing/parameters` loads cost parameters from `cost_parameters`, seeded with defaults if missing.
2. `POST /api/costing/calculate` accepts explicit product inputs and returns calculated wholesale, retail, and drip-bag prices.
3. `GET /api/costing/bean-list` reads active products with BOM material costs and returns calculated bean-list rows.
4. `POST /api/costing/runs` saves a calculation snapshot as a draft run.
5. `POST /api/costing/runs/:id/publish` publishes the saved run to product retail prices and `product_price_tiers`, preserving existing product roast level and writing audit logs.

## Testing

- Unit tests cover Excel cached golden values and invalid inputs in the domain engine.
- Application tests cover request validation and repository orchestration.
- HTTP tests cover route registration and calculate endpoint behavior without relying on UI.
- Architecture tests are left intact and should continue blocking root-package or appmain business-code regressions.
- Frontend build verifies the Vue view compiles in the current Vite shell.

