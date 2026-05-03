# Customer Portal Business Tasks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement and verify task-by-task.

**Goal:** Complete the first usable customer-facing business slice behind the mini-program service entries.

**Architecture:** Keep customer type flexible by routing every mini-program business feature through the current bound `customer_id` plus enabled service capabilities. Customer-visible data is served by `/api/mini/*`; internal ERP APIs remain employee-only.

**Tech Stack:** Go/Echo/Postgres for API and persistence, uni-app Vue 3 TypeScript for mini-program pages, PR/DEV/UT/API/REV rows for workflow evidence.

---

## Task 1: Customer Service Page API

- Add `GET /api/mini/services/:key`.
- Authorize by mini token, current customer binding, and service capability.
- Return one page-shaped payload with summary metrics and service-specific lists.

## Task 2: Customer-Visible Queries

- Reuse existing customer bean list snapshots, products, orders, production status, payment status, shipping status, and tracking number.
- Always filter order data by the bound `customer_id`.

## Task 3: Direct Ship Batch

- Add `direct_ship_import_batches`.
- Add `POST /api/mini/direct-ship/batches`.
- Store only the upstream customer batch. Downstream recipients stay out of `customers`.

## Task 4: Processing Request

- Add `processing_job_requests`.
- Add `POST /api/mini/processing-requests`.
- First version records customer-submitted processing requests and status; internal staff will later link work orders.

## Task 5: Inventory And Settlement

- Add `customer_inventory_items` as a customer custody snapshot table.
- Add `customer_fee_items` and `customer_settlement_batches`.
- Support product, processing, shipping, direct ship service, packaging, storage, and adjustment fees.

## Task 6: Miniapp Service Page

- Replace the placeholder page with real API loading.
- Show service summaries and lists.
- Support submitting direct ship batches and processing requests from the mini-program.

## Verification

- Unit tests: customerportal application service, Postgres schema guard, support requirement seed guard, miniapp helpers.
- API tests: mini service page, direct ship submit, processing submit, token/capability errors.
- Build checks: miniapp typecheck, miniapp mp-weixin build, full Go test suite, `git diff --check`.
