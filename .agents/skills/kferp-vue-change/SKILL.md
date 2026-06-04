---
name: kferp-vue-change
description: Use when working in KFerp and a task touches user-visible frontend pages, Vue/Vite code, templates, forms, tables, customer UI, navigation, or in-app manuals.
---

# KFerp Vue/Vite Frontend Change

Use this skill to enforce KFerp's unified frontend architecture without Van having to repeat it.

## Hard Rules

- User-facing frontend changes target Vue 3 + Vite.
- Do not add new user-facing behavior to `templates/*.html`.
- If a touched page is still template-based, treat that as migration debt and migrate/refactor the page or affected entry point into Vue/Vite before implementing the feature.
- New pages and changed pages must use JSON APIs rather than new HTML template data wiring.
- Manuals shown in the frontend must stay aligned with Markdown/manual docs.
- 跨页面跳转必须使用 `kferp:navigate-view` 传递 `returnNavigation`；目标页面在左上角展示“返回来源操作”的入口。返回上下文只保存在前端内存里，刷新后消失，不能写进持久业务数据或让刷新后的页面保留过期返回入口。

## Workflow

1. Locate the visible route, Vue component, legacy template, API client, and existing tests.
2. If the page is template-based:
   - identify the minimal Vue/Vite route/component needed for the touched workflow
   - keep legacy template only as transitional redirect/read-only compatibility when necessary
3. Write frontend tests before UI implementation:
   - pure helpers: test in `src/lib/*.test.js`
   - API client behavior: test in `src/api/*.test.js`
   - route/view behavior: nearest `src/views/*.test.js`
4. Use the shared API client; do not introduce bare `fetch` where auth or `/app` prefix handling is required.
5. Implement UI with existing component/style patterns. Avoid layout shifts and text overflow.
6. Run:
   - targeted `node --test`
   - `scripts/verify_kferp.sh frontend-build`
   - browser/screenshot verification for interaction or responsive layout changes
7. Update relevant operation manual docs and frontend manual entry when workflow, fields, buttons, permissions, import/export, or errors change.

## Final Evidence

Report changed route/component, legacy template impact, frontend tests, build result, browser verification when applicable, and manual paths.
