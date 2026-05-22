# PR-327-ORDER-ENTRY-CUSTOMER-DROPDOWN-LAYER

## 验收范围
- 手机窄屏录单时，客户输入框打开的客户候选列表必须显示在订单负责人输入框和后续字段之上。
- 客户候选列表的层级必须提升父级 combobox stacking context，而不是只提高列表自身 `z-index`。
- 切换焦点到订单负责人时，客户候选列表关闭，负责人候选列表正常打开，避免两个候选列表同时残留。

## 验收证据
- 单元：`order entry raises the active combobox above following fields` 先红测复现缺少打开态层级，再绿测验证客户/负责人 combobox 有 `open` 类和更高 `z-index`。
- 前端：`node --test src/lib/order-entry.test.js` 通过，覆盖录单相关回归。
- 前端：`node --test src/lib/*.test.js src/api/*.test.js` 通过，覆盖 Vue shell 前端单元/API wrapper 回归。
- 构建：`npm run build` 通过。
- API：`go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsResponsiblePersonOptions -count=1` 通过，确认录单表单 API 仍返回负责人候选。
- 浏览器：本地 Vite + mock API 打开 `/vue-shell/?view=order`，输入客户后 `.customer-combobox.open` 计算 `z-index=30`，`.responsible-combobox` 计算 `z-index=2`，客户候选列表可见。
- 手册：本次只修复候选列表层级，不改变入口、字段、权限、保存流程、导入导出或异常处理，因此不更新操作手册。
