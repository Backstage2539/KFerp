# PR-453 分组模板移入系统设置

## 需求
- 分组模板是系统级基础资料，入口统一为 `系统设置 / 分组模板`。
- 分组模板只维护模板名、大类和小类，不维护商品、BOM、仓库等对象。
- 商品档案、生产 BOM、仓库库存先选择分组模板，再显示分类 Tab 和 `移动到分类`。
- 生产 BOM 不再显示 `使用分组`；所有启用分组模板可直接选择。
- 对象归类仍写入 `business_group_assignments`；`usage_key` 只表示内部使用场景。

## RED
- Frontend: `node --test src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/materials-ui.test.js`，实现前失败于缺少 `系统设置 / 分组模板` 菜单、商品/BOM/仓库模板先选流程和模板页对象隔离。
- Support: `go test ./internal/interfaces/http/support -run TestDev453GroupTemplateSystemSettingsContracts -count=1`，实现前失败于缺少 PR-453 种子、文档和前端合同标记。

## GREEN
- Frontend targeted: `node --test src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/materials-ui.test.js` 通过。
- Vue build: `npm run build` 通过，保留既有 Vite chunk-size warning。
- Support/API contracts: `go test ./internal/interfaces/http/support -run 'TestDev453|TestDev442|TestDev450|TestDev451' -count=1` 通过。
- Support suite: `go test ./internal/interfaces/http/support -count=1` 通过。
- Backend: `go test ./...` 通过。
- Verifier: `scripts/verify_kferp.sh changed` 通过。
- Diff hygiene: `git diff --check` 通过。

## Manual
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`

## Browser Acceptance
- Local Browser: Vite `http://127.0.0.1:5194/vue-shell/?view=groupTemplates` 跳转到 `/login`；本地前端没有可用 `/api/auth/me` 和业务 API 代理，不能作为真实页面验收。
- Development deploy 后验收：系统设置、商品档案、生产 BOM、仓库库存、商品价格表、工单 BOM 选择、仓库下拉。
