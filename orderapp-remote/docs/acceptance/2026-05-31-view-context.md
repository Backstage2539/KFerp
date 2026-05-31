# PR-387 通用视图上下文验收记录

## 范围

- 顶部从固定“工厂总览 / 客户账户”升级为“当前视图”。
- 内置视图包含工厂总览、客户、订单和外部客户固定视图。
- 视图上下文只负责菜单呈现、默认过滤、URL 保留和跨页面参数传递，不替代权限、客户隔离、商品/BOM/价格后端校验。
- 沿用 PR-386 商品模型：客户侧展示客户商品名，执行侧使用商品档案、生产 BOM 和价格表快照。

## PR / DEV

- PR：`PR-387-VIEW-CONTEXT`
- DEV：
  - `DEV-387-PHASE1-FRONTEND-CONTEXT`
  - `DEV-387-PHASE2-PAGE-ADAPTERS`
  - `DEV-387-PHASE3-OPTIONS-API`
  - `DEV-387-PHASE4-PRESET-CRUD`
  - `DEV-387-PHASE5-MANUAL-ACCEPTANCE-DEPLOY`

## RED 证据

- `node --test src/lib/view-context.test.js`：实现前因缺少 `src/lib/view-context.js` 失败。
- `go test ./internal/interfaces/http/support -run TestViewContext -count=1`：实现前因缺少 `ensureViewContextPresetTables` 和视图上下文 API 失败。
- `node --test src/lib/view-context.test.js src/lib/workspace-mode.test.js`：补充保存视图 UI 断言后，因 `App.vue` 尚未包含保存/停用/恢复默认视图控件失败。

## GREEN 证据

- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/view-context.test.js src/lib/workspace-mode.test.js`
- `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestViewContext -count=1`
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/view-context.test.js src/lib/workspace-mode.test.js src/lib/operation-manuals.test.js src/lib/menu-ia.test.js`
- `cd orderapp-remote && go test ./internal/interfaces/http/support ./internal/interfaces/http/sales -count=1`
- `cd orderapp-remote/frontend-vue-shell && npm run build`
- `scripts/verify_kferp.sh changed`

## 手册证据

- `orderapp-remote/docs/OP_MANUAL_WORKSPACE_MODE.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`

## 验收项

- [ ] 顶部显示“当前视图”，可切换工厂总览、客户和订单视图。
- [ ] 旧 `workspace=customer&customer_id=...` URL 可恢复客户视图，新 URL 使用 `view_context`。
- [ ] 客户视图进入商品管理默认显示客户商品名，BOM 仍是商品档案的生产 BOM。
- [ ] 客户视图下产品价格表锁定当前客户，并只使用该客户启用且纳入价格表的客户商品名。
- [ ] 订单视图进入订单列表只显示该订单，且派生订单客户上下文。
- [ ] 外部客户账号只能看到绑定客户视图，跨客户/跨订单上下文 API 返回 403。
- [ ] 保存当前视图、修改保存视图、停用保存视图写入操作日志。
- [ ] 生产计划和工单仍是工厂执行入口，只使用客户/订单过滤和追溯标签。

## 浏览器验收

- 2026-05-31 Van 指示后续不用做浏览器/人工验收，以节省 token；本需求收口到代码落盘、文档推送、单测和 API 测试。
- 曾创建的临时 smoke 登录会话已从 `login_sessions` 删除。

## 部署记录

- Feature branch：`codex/view-context-20260531`
- Feature commit：`2c015a69 feat(view-context): add unified current view context`
- Develop merge commit：`4f4e2c87 Merge PR-387 view context`
- Deploy target：development stack
- Deploy command：`./deploy_orderapp.sh development`
- Backup：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260531160343`
- Smoke：
  - `docker compose ps`：`erp_orderapp` running，`erp_postgres` healthy，`erp_caddy` running，`erp_docconvert` running。
  - Docker build：镜像内 `go test ./...` 通过。
  - Authenticated `/vue-shell/`：200。
  - Authenticated `/api/view-context/options?type=customer&limit=3`：200。
  - Authenticated `/api/req/product?limit=500`：包含 `PR-387-VIEW-CONTEXT`。
