# PR-718 留厂订单状态与寄递信息修正

状态：技术验证完成，develop/main及生产交付进行中，Van业务验收待确认。

## 修正及边界

- `delivery_mode=customer_stock` 唯一识别留厂；优先已入仓/部分入仓，其次实际生产状态，生产完成映射待入客户仓，缺省待生产。库存代发仍走寄递流程。
- 履约/通用列表和详情隐藏无关寄递操作；编辑保留付款、商品及目标仓。稳定Teleport容器随草稿交付方式显示，关闭、切单、复制重置草稿显示状态。
- 不修改两张生产订单数据、不迁移历史ship_status；同一留厂订单整单保存保留delivery_status，普通订单转留厂及新建仍初始化pending_receipt。未新增API、数据库状态、小程序变更或生产待办模块。
- 同步唯一源手册OP_MANUAL_ORDER_SALES.md、OP_MANUAL_CUSTOMER_FULFILLMENT.md，沿用Vue的orderSalesManual/customerFulfillmentManual动态入口。DEV-718-05/06跟踪，PR保留review。

## RED / GREEN

原始输出及截图在工作树同级 `evidence/retained-stock-display`。

- `vue-red.log`：实际组件暴露留厂待发货/收件区域；`backend-red.log`：历史冻结商品允许HTTP整单保存，但部分入仓状态被重置pending_receipt，收到数量仍为2。
- `status-alias-red.log`补充已有“已生产完成”名称；`drawer-action-red.log`及`courier-note-red.log`补查出库按钮与快递备注。修复后26项定向Vue全部通过（`vue-green.log`）。
- 实际OrdersView + OrderEntry组件覆盖隐藏区域、付款保留、交付方式切换、重开及Teleport无异常；CustomerFulfillment实际列表/详情覆盖留厂与库存代发区别。
- save runtime验证无需收件地址，无运费报价，隐藏收件/运单/发货字段保留，付款金额与收款方式可提交；目标客户仓不合法仍拒绝。
- 真实PostgreSQL+HTTP：冻结商品保存200，部分/全部入仓状态、收到数量、实物库存、历史收件和零运费保持，操作日志有update；普通明细重写被实际库存依赖守卫拒绝，回滚后库存和状态保持。新建及普通订单转留厂初始化正确。
- 运营队列验证留厂不进入待发货/缺收件/代发/待备货，仍计入待收款/待确认；原24件留厂生产、分批入仓及两次代发回归通过。DTO读取真实交付快照通过。
- 本地全量Vue1521/1521和Vite构建通过；最后快递备注隐藏后定向26与Vite重验通过。全量Go通过，未为全量测试配置全局数据库；真实DB仅在上述隔离回归提供。
- `db-all.log`首次补充转留厂测试时因普通单fixture遗漏现有运费协议失败；修复测试fixture使用既有合法员工历史入口后，`transition-green.log`通过，不修改运费规则。
- 1280×720和620×900本地实际Vue页面核验：`editor-desktop.png`、`editor-narrow.png`、`fulfillment-list.png`。交付方式切换恢复寄递字段，再切回隐藏，无浏览器错误。演示数据只读，不替代生产业务验收。

## 发布记录

将记录develop/main合并、实际运行版本、生产只读复核及回滚锚点。生产目标SO-20261022-0002（115）、SO-20261022-0001（113），现有customer_stock/pending_receipt/待生产数据无需修复。
