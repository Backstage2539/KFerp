---
name: kferp-bugfix
description: Use when working in KFerp and Van reports a bug, regression, incorrect display or calculation, broken workflow, error, failed test, or behavior that is "not right".
---

# KFerp Bugfix

Use this skill so Van can report bugs naturally while Codex supplies the verifier.

## Defaults

- Do not ask Van for test criteria unless the expected behavior is genuinely ambiguous.
- Reproduce the issue before changing production code.
- Follow TDD: create or adjust the smallest failing unit/API/frontend/PDF test first and record the RED output.
- Keep the fix scoped to the failing behavior; avoid opportunistic refactors.
- Update manuals only when the fix changes user-visible behavior, fields, buttons, workflow order, permissions, import/export, or failure handling.
- Do not deploy unless Van asks for deployment or the current thread already owns that deployment.
- Simple bug mode: if Van says "简单BUG" or the expected change is local and self-contained, run only targeted TDD, merge the verified fix into `develop` after GREEN, and do not run extra system/browser acceptance or deploy.

## Workflow

1. Identify the affected surface: backend domain/service, API handler, Vue/Vite frontend, PDF/PNG generator, data migration, or deployment config.
2. Find the current implementation with `rg` and read the nearest tests before editing.
3. Build the verifier:
   - backend logic: targeted Go test in the nearest package
   - API behavior: handler/API test, or curl smoke only when handler tests cannot cover it
   - frontend behavior: `node --test` test near `orderapp-remote/frontend-vue-shell/src`
   - PDF/PNG layout: generator test plus rendered artifact or page/screenshot inspection when visual overlap/spacing is the bug
4. Run the new/changed test and confirm it fails for the reported reason.
5. Implement the smallest fix and rerun the targeted verifier.
6. For simple bug mode, stop after targeted GREEN, merge into `develop`, and do not deploy. For normal bugfixes, run broader checks for touched areas.
7. Update `ACTIVE_REQUIREMENTS.md` with branch, requirement id if any, verifier commands, and status unless the task is simple bug mode and the update would only add process overhead.
8. Final report must include: reproduction, RED evidence, GREEN evidence, files changed, manual impact, and whether deployment was done.

## Standard Checks

Use `scripts/verify_kferp.sh` when it covers the touched area:

- `scripts/verify_kferp.sh changed`
- `scripts/verify_kferp.sh backend`
- `scripts/verify_kferp.sh frontend-tests`
- `scripts/verify_kferp.sh frontend-build`

For narrow fixes, run targeted tests first; use full checks before merge/deploy.
For simple bug mode, targeted RED/GREEN evidence is enough unless the local fix grows into a broader change.
