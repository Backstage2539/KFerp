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
- Final post-fix frontend targeted: `node --test src/lib/materials-ui.test.js src/lib/bom.test.js src/lib/product-settings.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` 通过 176/176；覆盖仓库分类候选不再重复显示模板名。
- Final deploy build: `./deploy_orderapp.sh` 完成 Vue shell build、miniapp typecheck/build、Docker build 内 `go test ./...` 和 orderapp 镜像构建；最终 `erp_orderapp` 已用新镜像启动。
- Development smoke: `origin/develop=56d44772b45223c5deb8339761ea77019c5b0cf2`；备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608161536`；`erp_orderapp` up，`erp_postgres` healthy；未认证 `/app/` 返回 `303` 到 `/app/orders`；认证 `groupTemplates`、`productSettings`、`bom`、`warehouseInventory` 均返回 `200`；需求 API 暴露 PR-453；`/app/api/business-groups` 返回 `200`。

## Manual
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`

## Browser Acceptance
- Local Browser: Vite `http://127.0.0.1:5194/vue-shell/?view=groupTemplates` 跳转到 `/login`；本地前端没有可用 `/api/auth/me` 和业务 API 代理，不能作为真实页面验收。
- Development Browser: `groupTemplates` 和旧 `groupManagement` 路由都显示 `系统设置 / 分组模板` 区块；区块只维护模板、大类、小类，不显示对象列表、勾选或移动对象入口。
- Development Browser: 商品档案显示 `选择分组模板` 后才进入分类和 `移动到分类` 流程；页面无 `目标分组` / `移动到分组`。
- Development Browser: 生产 BOM 显示 `选择分组模板`、`移动到分类`、`目标分类`；页面不出现 `使用分组`。
- Development Browser: 仓库库存显示 `库存分组模板` 和分类候选，分类候选为大类/小类路径，不再重复显示模板名前缀；页面无旧移动分组文案。
- Development Browser: 商品价格表显示 `价格表配置` 按钮和 `计价模式规则` 弹窗按钮，`模板继承规则` 不再常驻。
