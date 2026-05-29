# 2026-05-29 客户门户开关与客户仓库验收记录

## 范围
- 客户管理菜单合并客户档案、客户门户配置、客户门户能力模板和客户履约手册。
- 客户档案增加“开通客户门户/工作台”开关，客户类型和订单类型支持新增。
- 客户门户配置只展示已开通门户/工作台客户，能力模板仍在门户配置页绑定。
- 客户门户配置移除豆单展示版本和代加工仓库字段。
- 仓库库存增加“客户仓库”分类，客户仓库通过“仓库设置”绑定或取消绑定客户。

## 验收证据
- 单元/API：`go test ./internal/application/customer ./internal/interfaces/http/customer ./internal/application/customerportal ./internal/interfaces/http/customerportal ./internal/infrastructure/postgres/sales -run 'TestNormalize|TestCustomer|TestPortal|TestOrderListWhere|TestOrderFulfillment|TestFulfillment' -count=1`
- 前端构建：`npm run build --prefix orderapp-remote/frontend-vue-shell`
- 操作手册：已更新 `OP_MANUAL_CUSTOMER_PORTAL.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_INVENTORY_MATERIALS.md`。

## 浏览器验收
- 待部署前后用浏览器执行：客户档案开通门户/工作台 -> 客户门户配置绑定客户门户能力模板 -> 仓库库存进入客户仓库并绑定客户 -> 操作日志查询相关变更。
