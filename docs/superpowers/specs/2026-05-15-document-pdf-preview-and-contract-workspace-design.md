# Document PDF Preview And Contract Workspace Design

## Goal

Upgrade sales-order and delivery-note document handling so their preview behaves like the contract stamping workspace: render the real PDF page, place the seal directly on that page, and keep the generated output aligned with what the operator sees. Also improve the contract stamping workspace so it is usable as a saved document register, not only a rough upload tool.

## Product Scope

- Sales-order preview renders a real PDF preview instead of an HTML approximation.
- Delivery-note preview renders a real PDF preview instead of an HTML approximation.
- Preview PDFs must be visibly marked as preview copies so they are not mistaken for final customer-facing files.
- Seal placement uses the same PDF-page overlay model as contract stamping.
- Confirming PDF generation saves a normal historical sales-order or delivery-note version using the current seal placement.
- Existing latest download, historical version download, image generation, and WeChat share flows remain available.
- Contract stamping workspace gets a denser document-register layout, clearer selected document state, stronger action grouping, and a better PDF workspace.
- Contracts can be saved as database metadata after upload, including title and note.
- Contracts can be deleted from the active list through a soft delete. Stamped version history and audit evidence remain recoverable from the database/files.
- The existing stamped PDF download route fix is included so historical stamped versions such as `/contracts/1/stamped/1.pdf` resolve correctly.

## Non-Goals

- Do not replace the backend sales-order or delivery-note PDF renderers with frontend-only PDF generation.
- Do not remove existing PDF version history or audit behavior.
- Do not physically delete contract source or stamped PDF files in this iteration.
- Do not add customer/order binding to contracts unless a later requirement asks for it.

## Recommended Architecture

### Shared Frontend PDF Workspace

Add a reusable Vue component for PDF preview stamping. It accepts:

- PDF bytes or a URL to load.
- A selected seal asset.
- Existing placement coordinates.
- Optional read-only or editable mode.
- A visible preview label such as `PREVIEW`.

It uses the same core helpers as contract stamping:

- PDF.js renders each PDF page to an image/canvas.
- The seal overlay uses PDF coordinates scaled to the displayed page.
- Dragging changes page-level placement values.
- The overlay style, drag math, and payload serialization live in shared lib code.

### Backend Preview PDFs

Add preview endpoints that return PDF bytes without creating a historical document version:

- `GET /api/orders/:id/sales-order-preview.pdf`
- `GET /api/orders/:id/delivery-note-preview.pdf`

The preview renderer uses the same snapshot path as the existing JSON preview and formal generate path, but draws an explicit preview marker. The marker can be a light diagonal or top-right `PREVIEW` label that does not hide core business content.

### Formal Generation

Formal generation remains backend-owned:

- `POST /api/orders/:id/sales-orders`
- `POST /api/orders/:id/delivery-notes`

When the operator drags the seal in the PDF preview, the frontend saves the placement through the existing sales-order seal position API before generation, or passes the same values in the generation command if a narrower per-document override is needed. The first implementation should keep the existing global company seal setting behavior unless tests show a per-document override is required.

### Contract Workspace Persistence

Extend contract metadata:

- `title`
- `note`
- `deleted_at`
- `deleted_by`

Add API:

- `PUT /api/contracts/:id` saves editable metadata.
- `DELETE /api/contracts/:id` soft-deletes a contract from the active list.

`GET /api/contracts` returns only active contracts by default. Download endpoints reject deleted contracts for ordinary active-list use. Database rows and files remain for audit.

## UI Design

### Sales Order

- Replace the HTML preview card with the shared PDF preview workspace.
- The workspace header shows version hint, preview status, and the action to refresh preview.
- The PDF page itself shows the preview marker.
- Seal dragging happens on top of the PDF page, using the same seal visual as contract stamping.
- Existing version tables and download buttons stay below.

### Delivery Note

- Keep the outbound maintenance form above the preview.
- Replace the HTML preview card with the shared PDF preview workspace.
- The PDF page shows the preview marker.
- Seal dragging and save behavior match sales order.

### Contract Stamping

- Use a two-column document register: left list/search/upload, right selected document detail and PDF workspace.
- Move upload into a compact toolbar.
- Add editable title/note fields in the selected document detail area.
- Add `Save contract` and `Delete contract` actions with clear disabled/loading states.
- Group stamping actions together: select seal, stamp current page, stamp all pages, save stamped PDF, download latest.
- Improve empty/loading/error states so the page explains the next action without large blank regions.
- Keep the PDF pages centered and scrollable like a document viewer.

## Error Handling

- Preview PDF failures show an error and do not create a historical document version.
- Formal PDF generation still cleans up orphan files on failure.
- Saving contract metadata validates a positive contract id and a non-empty title.
- Deleting a contract requires a positive id and records actor metadata.
- Deleted contracts do not appear in the active list.
- Historical stamped PDF downloads must parse `.pdf` suffixed ids correctly.

## Testing

- Unit tests for shared PDF placement helpers and preview marker renderer behavior.
- API tests for sales-order preview PDF and delivery-note preview PDF returning PDF bytes without creating versions.
- API tests for formal generation still creating versions after preview.
- API tests for contract metadata save and soft delete.
- Regression test for historical stamped PDF download route parsing.
- Vue/source tests for SalesOrderView, DeliveryNoteView, and ContractsView wiring to the shared PDF workspace.
- Manual documentation and acceptance evidence updated in `OP_MANUAL_ORDER_SALES.md`, `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, and req store rows.

