# ACTIVE_REQUIREMENTS

Purpose: short-lived coordination for Codex workflows. Keep active requirement ids, branches, verifier commands, deployment ownership, and unresolved blockers here so future sessions do not have to recover this from chat history.

This is not long-term memory. Move durable product/deployment decisions to `MEMORY.md` or source docs, then remove stale entries from this file.

## Active

### PR-372-PRODUCT-CONFIG-DISPLAY-UNIT
- Branch: codex/product-config-display-unit-20260525
- Owner/session: Codex / 2026-05-25
- Status: deployed
- Scope: SKU设置商品配置模板移除固定展示方式，改为价格表展示单位并继承单位模板报价单位。
- Verifier:
  - Unit: node --test src/lib/product-settings.test.js
  - API: go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates -count=1; go test ./internal/interfaces/http/support -run TestDev372 -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_COSTING.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-25-product-config-display-unit.md
- Deployment: development deployed at origin/develop 2865b7c9891607c453d7bbf38b20295bed212fd8
- Last update: 2026-05-25 22:34 Asia/Shanghai
- Notes: reserve_req_id.sh --claim failed on awk multiline string; claimed manually to avoid PR-371 WIP collision. Local Vite renders request failure without a同源后端；deployment后在 development 环境做浏览器复验。

### PR-373-PRODUCT-CONFIG-UI-POLISH
- Branch: codex/product-config-ui-polish-20260525
- Owner/session: Codex / 2026-05-25
- Status: verified
- Scope: 商品配置模板列表改成明确列表行；价格表生成规则三个下拉框三列对齐，说明改为感叹号弹出提示。
- Verifier:
  - Unit: node --test src/lib/product-settings.test.js
  - API: go test ./internal/interfaces/http/support -run TestDev373 -count=1
  - Frontend/build: npm --prefix orderapp-remote/frontend-vue-shell run build
  - Manual: orderapp-remote/docs/OP_MANUAL_COSTING.md
  - Review/acceptance: orderapp-remote/docs/ACCEPTANCE_TESTS.md; orderapp-remote/docs/acceptance/2026-05-25-product-config-ui-polish.md
- Deployment: pending
- Last update: 2026-05-25 22:58 Asia/Shanghai
- Notes: Browser feedback from Van on product settings page: template row did not look like a list; price rule dropdowns were not aligned; inline note should become exclamation tooltip.

## Template

```markdown
### PR-000-SHORT-SLUG
- Branch:
- Owner/session:
- Status: planned | red | implementing | verifying | pushed | merged | deployed | blocked
- Scope:
- Verifier:
  - Unit:
  - API:
  - Frontend/build:
  - Manual:
  - Review/acceptance:
- Deployment:
- Last update:
- Notes:
```
