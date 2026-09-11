# PR-651 Production Plan Capacity Workspace — Design QA

**Source visual truth**

- `/var/folders/p7/235nxhzn3_bbbyjsb56491xr0000gq/T/codex-clipboard-e2847a83-d25b-4cbf-bf7c-2ad373461bd9.png`

**Browser-rendered implementation evidence**

- Desktop, first actual operation: `/private/tmp/kferp-pr651-qa/capacity-operation-1.png`
- Desktop, second actual operation and source-order table: `/private/tmp/kferp-pr651-qa/capacity-operation-2.png`

**Viewport and normalization**

- Source pixels: `1488 × 1058`.
- Desktop implementation pixels: `1488 × 1058`; CSS viewport `1488 × 1058`; `deviceScaleFactor: 1`.
- The full-view comparison used equal desktop pixel dimensions without resampling. The source includes the shared ERP shell; the implementation capture isolates the real capacity-workspace component because the shell is supplied by the existing Vue application at runtime.

**State**

- Draft production plan `PP-0000000109`.
- Actual frozen operation names are `滚筒烘焙` and `手工装袋`.
- `滚筒烘焙` is fully covered; `手工装袋` is short by `2袋`.
- The second operation preserves three source-order tasks: `10袋 / 8袋 / 2袋`.

**Full-view comparison evidence**

- Fonts and typography: both views use a compact Chinese system sans-serif hierarchy with a strong plan number, medium section titles, and subdued explanatory copy. Text remains readable throughout the desktop viewport.
- Spacing and layout rhythm: the implementation keeps the reference's white panels, light borders, restrained radii, green section markers, right-side review region, and sticky action footer. The dedicated workspace gives allocation controls more horizontal room than the source plan-detail composition.
- Colors and visual tokens: semantic green, pale green surfaces, muted gray text, amber shortage state, and white canvas follow the reference palette and the existing KFerp visual system.
- Image quality and asset fidelity: the target contains one decorative coffee-bean image in the plan summary. The capacity-only implementation intentionally does not introduce or approximate that decorative asset; it uses no placeholder, emoji, CSS drawing, or replacement image.
- Copy and content: the implementation uses actual route operation names, separates `计划数量 / 已安排 / 还需安排`, preserves each source order, and explains that confirmation saves the split without submitting the plan or generating work orders.
- Overall composition: the implementation matches the target's dense ERP information hierarchy while adapting it to a focused, editable capacity-allocation workspace.

**Focused region comparison evidence**

- The second-operation capture was compared with the reference task table and right-side review region at the same desktop pixel dimensions. Order number, customer, task quantity, shortage status, and allocation controls remain visible together, so no additional crop was needed.

**Primary interactions tested**

- Switched from `手工装袋` to `滚筒烘焙` through the actual operation tab and verified the active operation content changed.
- Verified the draft exposes `保存草稿` while `确认安排` remains disabled when one task is short.

**Console errors checked**

- Browser console error and warning log: empty.

**Findings**

- No actionable P0, P1, or P2 visual mismatch remains.
- The missing shared ERP sidebar in the isolated component capture is expected; the deployed route renders inside the existing Vue shell.

**Comparison history**

- Iteration 1: compared the source with both actual-operation states at `1488 × 1058`. No actionable P0/P1/P2 issue was found, so no visual fix iteration was required.

**Implementation checklist**

- [x] Use frozen operation names instead of fixed process labels.
- [x] Preserve source-order task quantities.
- [x] Show native weight and sales units.
- [x] Keep draft save available and gate confirmation on per-task coverage.
- [x] Match the reference's restrained green ERP style.
- [x] Verify the source's desktop viewport and retain the component's existing responsive breakpoints.

**Follow-up polish**

- None required for handoff.

final result: passed
