# 验收记录：客户模板实时引用与客户侧订单列表

## 范围

- 履约客户账号登录后的客户侧工作台订单列表，与 ERP 后台客户履约账户底部“履约客户订单”列表使用同一数据源和字段。
- 客户能力模板改为实时引用，支持手动复制子模板、改名、失效和模板树单详情展开。
- 引用失效模板的客户不能继续保存、应用或绑定 ERP 工作台，必须重新选择启用模板。

## 验收点

- 通过：客户侧工作台移除旧 `direct_ship_orders` 小表，底部改用 `fetchCustomerFulfillmentOrders`、订单费用 helper、订单详情、销售单和出库单抽屉。
- 通过：模板 API 返回并保存 `parent_template_key`、`active`、`sort_order`，新增 `POST /api/customer-portal/admin/capability-templates/:key/copy` 手动复制子模板。
- 通过：模板运行时只接受 active 模板；客户门户、小程序上下文、客户履约工作台和 ERP 履约订单 scope 都按实时模板解析能力。
- 通过：模板设置页复制模板时只填写显示名称，系统自动生成安全 key；复制出的子模板紧贴母模板下方缩进展示。
- 通过：模板设置页默认只展开一个模板详情，其他模板只显示名称和说明；展开任意模板会自动收起其他详情。
- 通过：模板设置页支持模板失效；失效模板仍可保存，保存按钮不置灰；客户门户配置页只展示 active 模板，已引用失效模板时提示“模板已失效”。
- 通过：小程序登录或刷新上下文遇到失效模板时，返回并展示“客户配置已更新，请联系管理员处理”，不再显示 `invalid request`。
- 通过：操作手册和需求表已更新。

## 证据

- `go test ./... -count=1`
- `node --test src/lib/customer-fulfillment.test.js src/api/customer-fulfillment.test.js src/lib/customer-portal-theme.test.js`
- `npm run build`
- `curl -I 'http://127.0.0.1:5179/vue-shell/?view=customerCapabilityTemplates'`
- `curl -I 'http://127.0.0.1:5179/vue-shell/?view=customerProcessingPortal'`
