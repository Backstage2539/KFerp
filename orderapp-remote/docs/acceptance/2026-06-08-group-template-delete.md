# PR-455 分组模板删除

## 需求
- 分组模板没有启用/停用功能；模板删除就是删除。
- 系统设置的分组模板表单在编辑已有模板时显示 `删除模板`。
- 删除模板会删除该模板、模板下大类/小类、用途和对象归类，并写操作日志。
- 删除后商品档案、生产 BOM、仓库库存和商品价格表不再能选择该模板。

## RED
- Frontend: `node --test src/lib/materials-ui.test.js`，实现前失败于系统设置分组模板页缺少 `删除模板`，且仍显示模板启用/停用状态。
- API: `go test ./internal/interfaces/http/catalog -run TestBusinessGroupsAPIDeletesTemplate -count=1`，实现前编译失败于缺少 `DeleteBusinessGroupCommand` 和 `DELETE /api/business-groups/:id`。
- Support: `go test ./internal/interfaces/http/support -run TestDev455GroupTemplate -count=1`，实现前失败于缺少 PR-455 种子、文档、删除接口和前端合同标记。

## GREEN
- Frontend targeted: `node --test src/lib/materials-ui.test.js` 通过。
- API targeted: `go test ./internal/interfaces/http/catalog -run 'TestBusinessGroupsAPIDeletesTemplate|TestBusinessGroupItemsAPIWritesGenericGroupItems' -count=1` 通过。
- Support contract: `go test ./internal/interfaces/http/support -run TestDev455GroupTemplate -count=1` 通过。
- Related frontend: `node --test src/lib/materials-ui.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` 通过 176/176。
- Vue build: `npm run build` 通过，保留既有 Vite chunk-size warning。
- Backend: `go test ./...` 通过。
- Verifier: `scripts/verify_kferp.sh changed` 通过。
- Diff hygiene: `git diff --check` 通过。

## Manual
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`

## Browser Acceptance
- Local Browser: mocked Vue shell at `http://127.0.0.1:5196/vue-shell/?view=groupTemplates&pr455=delete2` rendered the system `分组模板` section. Template panel showed `删除模板`, did not show template `启用/停用` status, and did not expose `移动到分类` / object assignment controls.
- Local Browser: clicking `删除模板` with local mock confirmation sent `DELETE /api/business-groups/901`; after reload, `.template-chip` count was 0, `.category-editor` count was 0, `删除模板` button count was 0, and empty template notice was visible. Browser console error count was 0.
- Pending development browser acceptance after deploy.
