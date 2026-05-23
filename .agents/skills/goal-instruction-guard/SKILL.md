---
name: goal-instruction-guard
description: Use when Van asks to start, enter, create, run, resume, or prepare Codex goal mode, `/goal`, a durable objective, autonomous long-running work, or a goal instruction.
---

# Goal Instruction Guard

Use this skill before creating a Codex goal. Its job is to prevent vague long-running objectives from entering goal mode.

## Core Rule

Do not enter goal mode until the instruction is an auditable work order. A valid goal has:

- One objective: one durable outcome, not a backlog or mixed list.
- Verifiable end state: what must be true when done.
- Scope: files, modules, systems, environment, or explicit boundaries.
- Context to read first: docs, issue, logs, plan, screenshots, or repo files.
- Verification: commands, checks, artifacts, or measurable acceptance criteria.
- Evidence: what the final report must include.
- Stop rule: when to pause instead of guessing.

If any of `scope`, `verification`, or `stop rule` is missing, do not create the goal.

## When The Goal Is Not Ready

Respond in Chinese and stop. Do not call a goal tool. Use this shape:

```text
暂不进入 goal 模式。这个指令还缺少可审计的完成条件，我先按 goal 写法改成一版，你复制修改后再发给我：

/goal Complete [one objective].
Context: [files/docs/issues/logs/screenshots to read first].
Scope: [allowed files/modules/systems] only; do not change [explicit exclusions].
Verification: Run [commands] and/or produce [artifacts]; all must pass.
Evidence: Report [changed files, command outputs, screenshots, deploy commit, etc.].
Stop rule: Pause and ask if [unknowns, credentials, destructive actions, production risk, unclear acceptance].
```

Use Van's original wording to fill as much as possible. Mark unknown parts as `[请补充: ...]` rather than inventing risky requirements.

## When The Goal Is Ready

Before creating the goal, restate the contract briefly:

- Objective
- Stop condition
- Verification
- Stop rule

Then enter goal mode using the available goal mechanism. If no goal tool is available, tell Van to paste the generated `/goal` block into the Codex surface that supports goals.

## Good Goal Pattern

```text
/goal Complete PR-000-[slug] for KFerp.
Context: Read AGENTS.md, HOW_TO_WORKFLOW.md, ACTIVE_REQUIREMENTS.md, REQUIREMENTS.md, ACCEPTANCE_TESTS.md, and the affected manual/source files first.
Scope: Implement only [feature/bug area] on branch codex/[slug]; keep unrelated files unchanged; do not deploy unless explicitly requested.
Verification: Run targeted RED/GREEN tests, scripts/verify_kferp.sh changed, and the relevant backend/frontend/build/API checks for touched areas.
Evidence: Report PR/DEV ids if created, changed files, RED/GREEN evidence, manual paths, verification commands, and remaining risks.
Stop rule: Pause if acceptance criteria, data ownership, credentials, production impact, destructive operations, or cross-workflow conflicts are unclear.
```

## Reject These For Goal Mode

- "继续做完"
- "把系统优化一下"
- "修所有问题"
- "重构一下前端"
- "部署看看"
- "按你觉得好的方式做"

Convert them into one bounded objective with scope, verifier, evidence, and stop rule before entering goal mode.
