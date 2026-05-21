# 验收记录：客户账户隐藏生产与工作台模式手册

日期：2026-05-21

## 范围

- 内部员工切到客户账户模式后，菜单只保留客户账户、客户商品与配方、客户财务三组入口。
- 客户账户模式不展示生产计划/开始生产、生产中、生产工单、生产质检、生产成本、生产手册或工作台模式手册。
- 生产排产、开始生产和生产过程管理仍从工厂总览的生产管理进入。

## 验收点

- [ ] 客户账户模式菜单不包含 `producePlan` 和 `workspaceModeManual`。
- [ ] 客户账户模式菜单不包含生产管理相关入口：`productionManual`、`productionAcceptance`、`produceRunning`、`workOrders`、`jobCards`、`qualityInspections`、`produceLogs`、`productionCosts`。
- [ ] 工厂总览仍保留生产管理入口和工作台模式手册，不影响内部全局运营。
- [ ] 操作手册说明客户账户不承载生产排产/开工，生产操作需切回工厂总览。

## 验证证据

- RED：`node --test orderapp-remote/frontend-vue-shell/src/lib/workspace-mode.test.js` 先失败在 `workspaceModeManual should stay out of the customer workspace`。
- 单元测试：`node --test orderapp-remote/frontend-vue-shell/src/lib/workspace-mode.test.js orderapp-remote/frontend-vue-shell/src/lib/menu-permissions.test.js orderapp-remote/frontend-vue-shell/src/lib/operation-manuals.test.js orderapp-remote/frontend-vue-shell/src/lib/menu-ia.test.js`，29 个用例通过。
- 完整前端回归：`node --test src/lib/*.test.js src/api/*.test.js`，258 个用例通过。
- API/支持层回归：`go test ./internal/interfaces/http/support -run 'TestDeployScriptSyncsOperationManualDocs|TestDocsRawRouteServesOperationManual' -count=1` 通过。
- Go 全量回归：`go test ./...` 通过。
- 构建回归：`npm run build` 通过。
- 质量检查：`git diff --check` 通过；冲突标记检查无命中。
