# 验收记录：客户账户锁定客户视角

## 范围
- 工厂总览不再展示履约运营台。
- 内部员工客户账户围绕顶部当前客户处理履约、订单、库存、SKU、豆单、BOM 和费用，不展示门户与能力。
- 渠道客户账号登录时不展示工厂/客户模式切换，只能看自己绑定客户的履约与费用。

## 验收点
- [ ] 工厂总览菜单没有“客户履约账户/履约运营台”入口，客户门户配置和能力模板仍在工厂内部设置里。
- [ ] 客户账户菜单只有客户账户、客户商品与配方、客户财务三类入口。
- [ ] 客户账户进入 SKU设置、BOM配方维护、产品豆单、库存和费用管理后，页面内不再需要选择客户。
- [ ] 客户账户 SKU 和 BOM 只展示公共配置与当前客户配置，不展示其他客户配置；公共配置不能被客户视角误改。
- [ ] 客户账户库存只展示当前客户相关成品库存，不展示工厂原料库存、跨客户追溯或其他客户库存。
- [ ] 客户账户费用管理固定为当前客户，新增费用默认关联当前客户，费用列表按当前客户过滤。
- [ ] 渠道客户账号登录后看不到“工厂总览 / 客户账户”和“当前客户”选择器，只能进入自己客户的履约与费用。

## 验证证据
- 单元测试：`node --test src/lib/workspace-mode.test.js src/lib/workspace-context-pages.test.js src/lib/bom.test.js`
- API 测试：`go test ./internal/application/stock ./internal/interfaces/http/stock ./internal/infrastructure/postgres/stock -count=1`
- 手册：`OP_MANUAL_WORKSPACE_MODE.md`、`OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`OP_MANUAL_CUSTOMER_PORTAL.md`
