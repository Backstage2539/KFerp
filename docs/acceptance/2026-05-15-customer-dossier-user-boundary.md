# 2026-05-15 客户档案与用户边界验收记录

## 范围
- 客户档案列表去掉“编辑”操作列，点击客户名称打开详情/编辑抽屉。
- 员工维护、用户权限只面向内部用户。
- 外部用户统一在客户履约运营台按客户管理。
- 客户门户配置页不再直接绑定外部账号，只跳转到履约运营台。

## 验收点
- 客户档案列表存在 `customerDrawerOpen`、`openCustomerDrawer` 和 `closeCustomerDrawer`，不再渲染旧编辑列。
- `/api/auth/internal-accounts` 返回内部账号，用户权限页不出现账号类型切换、渠道客户或外部用户角色控件。
- 客户履约运营台提供外部用户创建、密码重置、登录启停和客户绑定 API/UI。
- 客户门户配置页包含“去履约运营台管理”，且不再调用 `/api/auth/accounts` 或 `saveERPBinding`。

## 验证命令
- `go test ./internal/interfaces/http/support -run TestCustomerDossierUserBoundary -count=1`
- `go test ./internal/application/customerfulfillment -run TestServiceExternalUserManagementValidatesAndDelegates -count=1`
- `go test ./internal/interfaces/http/customerfulfillment -run TestExternalUsersAPIManagesCustomerAccounts -count=1`
- `node --test src/api/auth.test.js src/api/customer-fulfillment.test.js src/lib/customer-portal-theme.test.js`
- `npm run build`
