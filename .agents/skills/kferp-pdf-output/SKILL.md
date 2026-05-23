---
name: kferp-pdf-output
description: Use when working in KFerp and a task touches PDF, PNG, print/export previews, bean-list documents, sales-order outputs, pagination, typography, seals, or generated visual artifacts.
---

# KFerp PDF/PNG Output

Use this skill for generated documents where API tests alone are not enough.

## Defaults

- Treat PDF/PNG output as a visual artifact with regression risk.
- Verify both data semantics and rendered layout.
- Update cache keys when a stored/cached PDF rendering changes so old assets regenerate.
- If sales-order output changes, check both PDF and PNG paths unless the code proves only one is affected.
- If bean-list output changes, check publication asset storage/cache behavior.

## Workflow

1. Locate the generator, snapshot/data source, cache key, and existing PDF/PNG tests.
2. Build the verifier:
   - data fields and ordering: unit test near generator/snapshot mapper
   - pagination/overlap/layout: generator test plus rendered page inspection
   - API download: handler/API test or deployed curl checking MIME and `%PDF`
3. Run the new/changed test and confirm RED.
4. Implement layout/data/cache changes.
5. Render at least one representative artifact when visual spacing, pagination, typography, or overlap changed.
6. Inspect output with page screenshots or a PDF renderer; verify no overlap, missing text, wrong unit, stale cache, or unexpected blank areas.
7. Update operation manuals and acceptance docs when the user-facing output or download workflow changes.
8. Final report must include cache key impact, generated artifact path or API evidence, visual verification result, and tests run.

## Common Checks

- `go test ./internal/infrastructure/pdf/...`
- relevant API handler tests
- deployed PDF curl: status, `Content-Type: application/pdf`, body starts `%PDF`
- source/bundle/doc marker check after deployment when applicable
