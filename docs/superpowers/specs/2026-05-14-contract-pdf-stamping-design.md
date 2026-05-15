# Contract PDF Stamping Design

## Goal

Add a contract stamping workspace where an operator can upload a contract, convert DOCX uploads to PDF, place a selected company seal on one or more PDF pages, drag each seal position, and save the stamped PDF for download.

## Product Scope

- Add a Vue/Vite page `contracts` under the order sales area.
- Upload accepts `.pdf` and `.docx`.
- PDF uploads become immediately stampable.
- DOCX uploads are stored as originals and converted to PDF by the backend before stamping.
- Seals reuse the sales-order seal pipeline: existing seal assets, transparent/cropped PNG handling, and sales-order seal settings remain the source for company seal configuration.
- The user can choose a seal asset, add it to any PDF page, drag page-level positions independently, and save a stamped PDF version.
- Saved stamped versions are stored by the backend and can be downloaded later.

## Architecture

Backend uses a new `contracts` application service, PostgreSQL repository, and HTTP module. The service owns file validation, object-key generation, DOCX-to-PDF conversion, and stamped-PDF storage. The repository owns metadata tables and download lookups.

Frontend uses PDF.js to render each PDF page to a canvas and `pdf-lib` to write the selected seal image into the PDF. Placement data is stored in PDF point units with top-left page coordinates so display scaling and final PDF output stay aligned.

## Data Model

- `contract_documents`: one uploaded contract, including source metadata, original object key, converted/original PDF object key, bytes, source kind, creator, and timestamps.
- `contract_stamped_versions`: saved stamped PDF versions, including contract id, version number, seal asset id, placement JSON, stamped PDF object key, hash, creator, latest flag, and timestamps.

## API Contract

- `GET /contracts` redirects to `/vue-shell?view=contracts`.
- `GET /api/contracts` lists uploaded contracts and latest stamped version metadata.
- `POST /api/contracts` uploads a PDF or DOCX contract.
- `GET /contracts/:id/pdf` downloads or previews the converted/source PDF.
- `POST /api/contracts/:id/stamped` saves the frontend-generated stamped PDF and placement JSON.
- `GET /contracts/:id/stamped/:version_id.pdf` downloads a stamped version.
- `GET /contracts/:id/stamped-latest.pdf` downloads the latest stamped version.
- `GET /api/settings/sales-order/seals` lists reusable company seal assets and marks the current sales-order seal.

## DOCX Conversion

The runtime converter is LibreOffice/`soffice` in headless mode:

```text
soffice --headless --convert-to pdf --outdir <output-dir> <source.docx>
```

The command path is configurable through `DOCX_CONVERTER_CMD`, defaulting to `soffice`. If conversion fails, the API returns a clear bad-request error and removes any partially written converted PDF.

## Frontend Workflow

1. Upload PDF or DOCX.
2. Select a contract from the list.
3. Select a seal from available sales-order seal assets.
4. Render PDF pages in the workspace.
5. Add a seal to the current page or all pages.
6. Drag the seal independently on each page.
7. Save stamped PDF, which uploads the generated PDF plus placements to the backend.
8. Download latest or historical stamped PDF.

## Error Handling

- Unsupported file types are rejected before metadata is persisted.
- Empty or oversized files are rejected.
- DOCX conversion errors leave no converted orphan file.
- Stamped PDF uploads must be valid PDF bytes and reference an existing contract.
- Failed metadata inserts clean up written files.
- If no seal exists, the workspace points the operator to sales-order seal settings.

## Testing

- Unit tests cover file kind detection, DOCX conversion command behavior with a fake converter, placement-to-PDF coordinate conversion, and seal selection helpers.
- API tests cover PDF upload, DOCX upload through a fake converter, stamped PDF save/download, and invalid upload rejection.
- Source/seed guard tests cover PR/DEV rows, menu entry, manual entry, and key workflow text.
- Frontend build verifies PDF stamping dependencies compile in the Vue shell.

