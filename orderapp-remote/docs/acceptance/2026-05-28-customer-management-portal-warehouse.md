# PR-383 客户管理菜单、门户配置和客户仓库验收记录

## 范围
- 客户管理菜单：客户档案、门户客户配置、客户门户能力模板、客户履约手册。
- SKU 设置切换页面后不残留 SKU 内容。
- 门户客户配置移除豆单展示版本和代加工仓库编辑，只展示客户仓库。
- 仓库库存支持绑定客户，并写操作日志。
- 客户档案客户类型和默认订单类型支持自定义新增。

## 验收项
- [ ] 客户管理菜单下能进入客户档案、门户客户配置、客户门户能力模板、客户履约手册。
- [ ] 从 SKU 设置切换到其他页面，无需刷新即可看到目标页面内容。
- [ ] 门户客户配置没有“豆单展示版本”和“代加工仓库”编辑入口。
- [ ] 仓库库存绑定客户后，门户客户配置详情展示该客户仓库。
- [ ] 客户档案编辑抽屉可通过加号新增客户类型和默认订单类型。
- [ ] 操作日志可查客户类型/订单类型新增、仓库绑定客户、门户配置保存等写操作。

## 验收证据
- 单元测试：`node --test src/lib/menu-ia.test.js src/lib/customer-types.test.js src/lib/customer-management-source.test.js`
- API 测试：`go test ./internal/application/customer ./internal/application/stock ./internal/interfaces/http/customer ./internal/interfaces/http/stock ./internal/interfaces/http/customerportal ./internal/infrastructure/postgres/customerportal -count=1`
- 浏览器验收：部署到 development 后补充实际点击路径和结果。

