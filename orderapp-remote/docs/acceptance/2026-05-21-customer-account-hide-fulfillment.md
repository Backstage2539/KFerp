# 验收记录：客户账户模式隐藏履约运营台

日期：2026-05-21

## 范围

- 设置菜单新增“界面设置”。
- 新增“客户账户模式隐藏履约运营台”开关，默认开启。
- 开启后只隐藏客户账户模式中的内部履约运营台入口；不删除履约运营台页面、客户侧工作台、履约导入/订单 API 或客户门户模板能力。

## 验证

- `node --test src/lib/menu-permissions.test.js src/api/ui-settings.test.js`
- `node --test src/lib/menu-ia.test.js src/lib/view-routing.test.js src/api/auth.test.js src/api/ui-settings.test.js`
- `go test ./internal/interfaces/http/support -run 'TestUISettingsAPI|TestAuthMe|TestAuthorizationMiddleware' -count=1`
- `go test ./internal/application/authz ./internal/infrastructure/postgres/authz -count=1`

## 整体验证

- `node --test src/lib/*.test.js src/api/*.test.js`：241 个用例通过。
- `go test ./...`：全部包通过。
- `npm run build`：Vue/Vite 构建通过。
- `git diff --check`：通过。
