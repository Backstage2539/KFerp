# 验收记录：客户模板实时引用与客户侧订单列表

## 范围

- 履约客户账号登录后的客户侧工作台订单列表，与 ERP 后台客户履约账户底部“履约客户订单”列表使用同一数据源和字段。
- 客户能力模板改为实时引用，支持手动复制子模板、改名、失效和模板树折叠。
- 引用失效模板的客户不能继续保存、应用或绑定 ERP 工作台，必须重新选择启用模板。

## 验收点

- 通过：客户侧工作台移除旧 `direct_ship_orders` 小表，底部改用 `fetchCustomerFulfillmentOrders`、订单费用 helper、订单详情、销售单和出库单抽屉。
- 通过：模板 API 返回并保存 `parent_template_key`、`active`、`sort_order`，新增 `POST /api/customer-portal/admin/capability-templates/:key/copy` 手动复制子模板。
- 通过：模板运行时只接受 active 模板；客户门户、小程序上下文、客户履约工作台和 ERP 履约订单 scope 都按实时模板解析能力。
- 通过：模板设置页支持复制模板、模板失效、子模板缩进和折叠；客户门户配置页只展示 active 模板，已引用失效模板时提示“模板已失效”。
- 通过：操作手册和需求表已更新。

## 证据

- `go test ./... -count=1`
- `node --test src/lib/customer-fulfillment.test.js src/api/customer-fulfillment.test.js src/lib/customer-portal-theme.test.js`
- `npm run build`
- `curl -I 'http://127.0.0.1:5179/vue-shell/?view=customerCapabilityTemplates'`
- `curl -I 'http://127.0.0.1:5179/vue-shell/?view=customerProcessingPortal'`
