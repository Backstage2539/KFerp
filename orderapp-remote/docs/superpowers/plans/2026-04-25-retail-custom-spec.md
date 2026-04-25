# Retail Custom Spec Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let retail order entry accept arbitrary package grams while migrating the order-entry page into Vue/Vite.

**Architecture:** Add JSON form/save APIs around the existing order save service. Render the order-entry workflow as a Vue shell internal page and keep legacy `/order` GET as a redirect. Frontend pure helpers calculate retail custom-spec prices and build the save payload.

**Tech Stack:** Go + Echo + pgx, existing sales application service, Vue 3 + Vite, Go tests, Node build.

---

## File Structure

- Create: `order_api.go`
  - JSON endpoints for order form bootstrap and order save.
- Modify: `order_routes.go`
  - Redirect `/order` GET to the Vue shell entry while keeping POST compatibility.
- Modify: `frontend-vue-shell/src/App.vue`
  - Make `order` an internal Vue view.
- Create: `frontend-vue-shell/src/views/OrderEntryView.vue`
  - Vue order-entry page.
- Create: `frontend-vue-shell/src/lib/order-entry.js`
  - Product lookup, retail price, spec, total, and payload helpers.
- Create: `frontend-vue-shell/src/lib/order-entry.test.js`
  - Node unit tests for custom grams and payload.
- Create/modify Go tests:
  - `order_api_test.go`
  - `order_vue_entry_test.go`

## Tasks

- [ ] Add failing frontend helper tests for custom retail grams.
- [ ] Implement `frontend-vue-shell/src/lib/order-entry.js`.
- [ ] Add failing Go API tests for `GET /api/order/form`, `POST /api/order`, and `/order` redirect.
- [ ] Implement `order_api.go` and route registration.
- [ ] Make `/order` GET redirect to `/vue-shell?view=order`.
- [ ] Add `OrderEntryView.vue` and wire it into `App.vue`.
- [ ] Build both test layers: `node --test src/lib/order-entry.test.js`, `go test ./...`, and `npm run build`.
- [ ] Update 5 requirement tables and deploy after merge to `develop`.

