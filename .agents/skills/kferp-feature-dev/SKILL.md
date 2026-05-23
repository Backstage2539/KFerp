---
name: kferp-feature-dev
description: Use when working in KFerp and Van asks to add, change, implement, support, design, or extend a product feature or business behavior.
---

# KFerp Feature Development

Use this skill so Van can describe product needs naturally while Codex turns them into KFerp's PR/DEV/test/manual/review workflow.

## Defaults

- Codex, not Van, creates the verifier from the request and touched area.
- Start on a feature branch/worktree named `codex/<task-name>` unless already isolated.
- Reserve a PR id before broad edits when the task is a product requirement.
- Maintain PR/DEV progress in the UI seed/source docs that already back KFerp's requirement tables.
- Follow TDD: RED unit/API/frontend tests before implementation.
- Update source manuals and `orderapp-remote/docs/` manuals for any user workflow change.
- Do not mark PR done; Van accepts product requirements.

## Workflow

1. Read `HOW_TO_WORKFLOW.md`, `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, and the nearest domain/manual docs.
2. Reserve or choose the requirement id:
   - run `scripts/reserve_req_id.sh` to inspect the next available id
   - run `scripts/reserve_req_id.sh --claim <short-slug>` only when starting a new PR entry
3. Convert Van's request into:
   - PR: business goal, scope, acceptance criteria, owner
   - DEV: independently implementable tasks
   - verifier: unit/API/frontend/build/manual/review evidence required for this specific change
4. Write tests first and verify RED.
5. Implement the minimum code and docs to satisfy the tests and acceptance criteria.
6. Run targeted checks, then broader checks by touched area.
7. Update `ACTIVE_REQUIREMENTS.md` with requirement id, branch, status, verifier commands, and deployment state.
8. Final report must include PR/DEV ids, verifier evidence, manual paths, branch/deploy status, and open acceptance items.

## Verifier Selection

- Backend business rule: Go unit/service/repository test plus `go test` for affected package.
- API contract: handler/API test; add curl smoke only for deployed verification.
- Vue/Vite UI behavior: frontend `node --test`, `npm run build`, and browser check when layout/interaction matters.
- PDF/PNG/print output: use `kferp-pdf-output`.
- Template page touched: use `kferp-vue-change` before adding user-facing behavior.
- Deployment requested: use `kferp-deploy-dev`.
