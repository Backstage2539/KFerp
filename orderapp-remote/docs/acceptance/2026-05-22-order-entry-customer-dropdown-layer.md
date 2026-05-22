# PR-327-ORDER-ENTRY-CUSTOMER-DROPDOWN-LAYER

## 验收范围
- 手机窄屏录单时，客户输入框打开的客户候选列表必须显示在客户负责人只读字段和后续字段之上。
- 客户候选列表的层级必须提升父级 combobox stacking context，而不是只提高列表自身 `z-index`。
- 选择客户或离开客户输入后，客户候选列表关闭，不遮挡后续字段操作。

## 验收证据
- 单元：`order entry raises the active combobox above following fields` 验证客户 combobox 有 `open` 类和更高 `z-index`，并确认录单页已改为客户负责人只读字段、无订单负责人候选框。
- 前端：`node --test src/lib/order-entry.test.js` 通过，覆盖录单相关回归。
- 前端：`node --test src/lib/*.test.js src/api/*.test.js` 通过，覆盖 Vue shell 前端单元/API wrapper 回归。
- 构建：`npm run build` 通过。
- API：`go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsCustomerDefaultsForOrderEntry -count=1` 通过，确认录单表单 API 返回客户负责人默认值。
- 浏览器：本地 Vite + mock API 打开 `/vue-shell/?view=order`，输入客户后 `.customer-combobox.open` 计算 `z-index=30`，客户候选列表可见且高于后续字段。
- 手册：客户负责人口径已并入 PR-327-CUSTOMER-RESPONSIBLE-EMPLOYEE 的操作手册更新。
