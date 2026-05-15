# 员工维护与用户权限合并验收

## 范围

- “用户权限”独立页面和菜单配置项合并到“员工维护”。
- 员工维护同页维护内部员工资料、登录启停、密码和内部角色。
- 外部用户仍只在客户履约运营台维护。

## 证据

- RED：`go test ./internal/interfaces/http/support -run TestEmployeePermissionPageMergeWiring -count=1` 曾失败，失败点为 `CompanyStaffView.vue` 缺少 `fetchInternalAuthAccounts`。
- RED：`node --test src/lib/menu-ia.test.js` 曾失败，失败点为主菜单仍包含 `userPermissions`。
- RED：`go test ./internal/infrastructure/postgres/authz -run TestDefaultViewPermissionsCoverVueShellMenuKeys -count=1` 曾失败，失败点为默认视图权限仍包含 `userPermissions`。
- GREEN：员工维护页已包含 `fetchInternalAuthAccounts`、`resetEmployeePassword`、`saveEmployeeRoles`、账号启停、设置密码/重置密码和内部权限勾选。
- GREEN：系统菜单移除独立“用户权限”，`App.vue` 将历史 `view=userPermissions` 归一化为 `employees`。
- GREEN：默认视图权限移除 `userPermissions`，部署初始化时会删除历史 `auth_view_permissions.userPermissions` 配置行。
- 本地验证：`go test ./internal/interfaces/http/support -run TestEmployeePermissionPageMergeWiring -count=1` 通过。
- 本地验证：`go test ./internal/infrastructure/postgres/authz -count=1` 通过。
- 本地验证：`node --test src/lib/menu-ia.test.js src/api/auth.test.js` 通过，15/15。
- 本地验证：`npm run build` 通过，仅保留既有大包提示。
- 本地验证：`go test ./... -count=1` 通过。

## 交付记录

最终合入和部署提交以本需求最终回复中的 `origin/develop` 为准。
