# 留厂及代发订单日期保存验收

## 2026-11-08 历史生产版本跟进

- 正式发布：按 Van 要求完整合并 develop，运行版本 `d48ad618d27d69cd099c3df4e921ce24fc19a110`；正式登录页面、认证订单表单和手册正常，服务运行、数据库健康、订单页面源码散列一致。SO-20261104-0006 仍为原日期 2026-11-04、未作废；未代现场保存订单。正式环境实际保存待现场验收。
- 正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108221728-d48ad618d27d`，回滚镜像 `kferp-orderapp-rollback:production-20261108221728-d48ad618d27d`。发布后文档回执不重新构建。

- SO-20261104-0006 的历史生产版本与当前目录不同。页面初始化及后台更新目录时不得自动替换原单版本与报价来源；主动更换价格表仍按新选择处理。正式订单仅只读核查。
- 合成页面样本先 RED 后 GREEN，验证自动刷新仍保留原快照及主动换表；隔离 PostgreSQL 员工接口原日期保存、库存权益和真实商品修改保护回归通过。前端 1765 项及构建通过。
- 已部署开发环境 `8c270204cde1cacb7659d86e28240759797c9fb0`；服务运行、数据库健康，认证表单和手册 200、匿名 API 401，源码散列一致。备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108215445-8c270204cde1`，开发小程序产物沿用原版本。

- 范围：关联代发、生产或入库的留厂销售单，以及保留旧地区写法的库存代发订单。正式业务数据只读检查，不代替现场修改订单。
- 留厂 RED：隔离 PostgreSQL 员工订单保存 API，仅改日期时复现“留厂销售已有关联代发、生产或入库，不能修改商品数量”。
- 代发 RED：隔离 PostgreSQL 员工代发编辑 HTTP API，原始地区为云南省/大理市/大理古城，冻结运费地区为云南省/大理白族自治州/大理市，仅改日期预览返回“收件地区无法唯一识别，请选择省市区”。
- 地区 GREEN：`TestFulfillmentEditHistoricalRegionDateOnlyHTTP` 和既有 `TestFulfillmentOrderEdit` 回归通过；覆盖原地址及费用快照、操作日志、数量变化重算、真实地址变化、篡改地区和冻结快照不一致。
- 留厂 GREEN：`TestRetainedOrderHTTPSavePreservesReceivedProgress` 使用真实数据库及员工 HTTP API，覆盖页面提交格式、自动价/手工价/换算价、入库进度、原明细快照、权益、库存及日志；实际修改数量、价格、折扣、备注、来源和规格继续受原有限制。
- 其他回归：`scripts/verify_kferp.sh changed` 与 `scripts/verify_kferp.sh backend`；最终合并后重新验证。
- 用户操作说明：`docs/OP_MANUAL_ORDER_SALES.md`，系统内订单销售手册入口沿用原有 Vue 手册入口。
- 上线后业务验收由现场人员操作；不自动更改现有订单、客户地址、库存或费用。
