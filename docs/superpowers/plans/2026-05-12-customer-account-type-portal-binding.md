# Customer Account Type And Portal Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Keep the acceptance and operation manual entries in sync with the code.

**Goal:** Separate internal employee permissions from channel customer accounts, move ERP customer binding into customer portal configuration, enforce one-to-one binding, and add customer types with retail mall defaults.

**Architecture:** Add account/customer type fields at the core schema boundary, keep the existing customer fulfillment binding table but tighten constraints and API validation, and make customer portal admin the single setup surface for wholesale portal customers.

---

### Task 1: Failing Tests

- [ ] Add customer API coverage for `customer_type` defaulting and round-trip JSON.
- [ ] Add customer portal service coverage for the `retail_mall` template and mall order-history access.
- [ ] Add source guards for wholesale-only portal listing, one-to-one ERP binding, account type API/UI, and hidden customer roles.
- [ ] Add documentation/seed guards for PR/DEV/UT/API/REV entries.

### Task 2: Core Data And APIs

- [ ] Add `customers.customer_type` with default `retail`.
- [ ] Add `company_employees.account_type` with default `internal_employee`.
- [ ] Extend customer application, repository, and HTTP DTOs with normalized `customer_type`.
- [ ] Extend auth account DTOs with `account_type` and add an account type update endpoint.
- [ ] Make channel customer actors receive only the customer portal workspace permissions derived from account binding, not editable internal roles.

### Task 3: Portal And Binding

- [ ] Add `retail_mall` capability template and allow the orders service with mall capability.
- [ ] Default retail/ecommerce customers to enabled mall profile and capabilities.
- [ ] Filter customer portal admin list to `customer_type='wholesale'`.
- [ ] Return ERP binding summary in portal admin customer rows/details.
- [ ] Move ERP binding API into customer portal admin routes.
- [ ] Update customer fulfillment binding repository to enforce customer/account one-to-one and require `channel_customer`.
- [ ] Remove ERP binding controls from fulfillment operations page.

### Task 4: Frontend

- [ ] Add customer type selector to customer profile create/edit and list editing surfaces.
- [ ] Update customer portal settings list with ERP binding select/status and bound account display.
- [ ] Update user permissions page to distinguish account type, hide customer business roles, and disable role editing for channel customers.

### Task 5: Documentation And Verification

- [ ] Update `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `OP_MANUAL_CUSTOMER_PORTAL.md`, and `OP_MANUAL_SETTINGS_AUDIT.md`.
- [ ] Sync docs into `orderapp-remote/docs` if this repo expects mirrored operation docs.
- [ ] Run targeted Go tests and frontend tests.
- [ ] Run `npm run build` for the Vue shell.
