# Production Customer Assets Schema Repair

Date: 2026-07-10

Requirement: PR-522-PRODUCTION-CUSTOMER-ASSETS-SCHEMA

## Incident

- Production customer creation committed the customer row, then the response detail read failed because `customer_assets` was absent.
- The table existed only in a historical manual SQL document and was not part of application startup schema initialization.

## Fix

- Added idempotent customer PostgreSQL schema initialization for `customer_assets` and its customer index.
- Added the customer schema step after core customers and before customer portal initialization.
- Applied the same idempotent DDL to production without deleting or rewriting customer data.

## Evidence

- RED: targeted tests failed before implementation because `customer.EnsureSchema` and the app startup customer step were missing.
- GREEN: customer schema/source tests, app schema-order test, customer repository/API packages and appmain package passed.
- Production: `customer_assets` resolves in the tenant schema and authenticated customer detail returns HTTP 200 with an empty assets array.

## Data Note

Repeated create attempts during the incident committed multiple same-name customer rows before the response failed. No automatic destructive cleanup was performed.
