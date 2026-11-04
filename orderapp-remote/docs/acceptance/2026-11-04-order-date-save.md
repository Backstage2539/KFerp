# 留厂及代发订单日期保存验收

- 范围：关联代发、生产或入库的留厂销售单，以及保留旧地区写法的库存代发订单。正式业务数据只读检查，不代替现场修改订单。
- 留厂 RED：隔离 PostgreSQL 员工订单保存 API，仅改日期时复现“留厂销售已有关联代发、生产或入库，不能修改商品数量”。
- 代发 RED：隔离 PostgreSQL 员工代发编辑 HTTP API，原始地区为云南省/大理市/大理古城，冻结运费地区为云南省/大理白族自治州/大理市，仅改日期预览返回“收件地区无法唯一识别，请选择省市区”。
- 地区 GREEN：`TestFulfillmentEditHistoricalRegionDateOnlyHTTP` 和既有 `TestFulfillmentOrderEdit` 回归通过；覆盖原地址及费用快照、操作日志、数量变化重算、真实地址变化、篡改地区和冻结快照不一致。
- 留厂 GREEN：`TestRetainedOrderHTTPSavePreservesReceivedProgress` 使用真实数据库及员工 HTTP API，覆盖页面提交格式、自动价/手工价/换算价、入库进度、原明细快照、权益、库存及日志；实际修改数量、价格、折扣、备注、来源和规格继续受原有限制。
- 其他回归：`scripts/verify_kferp.sh changed` 与 `scripts/verify_kferp.sh backend`；最终合并后重新验证。
- 用户操作说明：`docs/OP_MANUAL_ORDER_SALES.md`，系统内订单销售手册入口沿用原有 Vue 手册入口。
- 上线后业务验收由现场人员操作；不自动更改现有订单、客户地址、库存或费用。
