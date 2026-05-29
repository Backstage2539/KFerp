# 客户账号费用菜单验收记录

## 范围
- PR-303-CUSTOMER-ACCOUNT-FINANCE-MENUS
- 分支：`codex/customer-account-workspace-20260520`

## 验收点
- 渠道客户账号登录后，顶部不展示“工厂总览 / 客户账户”和“当前客户”选择器。
- 左侧菜单拆为“工作台”和“费用相关”。
- “工作台”只保留客户履约运营台和履约订单，不嵌入费用明细或结算单。
- “费用相关”包含费用明细、经营报告和结账相关。
- 费用明细、经营报告、结账相关都使用账号绑定客户作为 `customer_id`，不需要页面内选择客户。
- 客户侧费用明细只读，不展示新增费用表单、员工筛选或客户选择器。
- 客户侧结账相关只读，不展示结账、结账后调整或会计交接导出。
- 后端客户费用读接口按当前登录员工绑定客户派生客户范围，请求其他 `customer_id` 会被拒绝。

## 自动化证据
- 前端单元测试：`node --test src/lib/workspace-mode.test.js src/lib/workspace-context-pages.test.js src/lib/customer-portal-theme.test.js src/lib/finance.test.js`
- 后端单元/API/仓储测试：`go test ./internal/application/finance ./internal/interfaces/http/finance ./internal/infrastructure/postgres/finance -count=1`
- 客户费用范围 API 测试：`TestCustomerAccountFinanceReadAPIDerivesBoundCustomerAndRejectsCrossCustomer`

## 手册
- `OP_MANUAL_WORKSPACE_MODE.md`
- `OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `OP_MANUAL_FINANCE.md`
