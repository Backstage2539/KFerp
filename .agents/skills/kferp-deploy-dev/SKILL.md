---
name: kferp-deploy-dev
description: Use when working in KFerp and Van asks to deploy, publish, release, smoke test, merge to develop, or put changes into the development environment.
---

# KFerp Development Deploy

Use this skill for development-environment integration and deployment. It assumes implementation is already complete or the current thread owns the feature branch.

## Gates

- Never deploy from a stale local `develop`.
- Never force-push `develop`.
- Do not deploy another workflow's new commit unless that workflow has finished verification or Van explicitly asks.
- Feature branch must be pushed before merge.
- Relevant unit/API/frontend/build/manual/review evidence must be complete before integration.

## Workflow

1. Confirm current branch/worktree and `git status --short`.
2. `git fetch origin`.
3. Merge or rebase latest `origin/develop` into the feature branch; resolve conflicts on the feature branch.
4. Rerun relevant tests after the merge:
   - `scripts/verify_kferp.sh changed`
   - targeted unit/API/frontend tests
   - `scripts/verify_kferp.sh backend` and frontend build when touched
5. Push the feature branch.
6. Fast-forward or cleanly merge to `develop`; push `develop`.
7. Verify intended deployment commit:
   - `git fetch origin`
   - `git log --oneline -3 origin/develop`
   - `git rev-parse origin/develop`
8. Deploy development with the repo's deploy script and server workflow. If SSH/server actions are needed, use the `kferp-deploy` skill.
9. Smoke test:
   - containers running
   - unauthenticated `/app/` returns 401
   - authenticated `/app/` or shell route returns 200/303 as expected
   - requirement API exposes the PR when applicable
   - feature-specific API/doc/source marker exists
10. Update `ACTIVE_REQUIREMENTS.md` and daily memory with deployed commit, backup path, and smoke evidence.

## Final Report

Include branch, pushed commit, `origin/develop` commit, deploy command, backup path if any, smoke results, and any checks not run.
