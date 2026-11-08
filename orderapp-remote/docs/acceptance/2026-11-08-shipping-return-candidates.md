# PR-751 寄件回填候选与现货补足验收

## 问题与范围

生产喜沁咖啡回传两张 10 月运单时，候选只有 9 月订单。旧流程在匹配阶段排除未就绪库存订单，并只读取导入时保存的候选；客户转入现货后，普通订单编辑按原规则保留待入库预订。

本次将收件匹配与可发货条件分开，实时显示匹配但未就绪的订单及原因；ERP 订单日期、快递下单时间、揽件时间分别展示。未就绪候选也参与唯一性判断，防止自动选到同收件人的旧单。增加默认关闭的「使用客户现货补足待入库预订」选项，先展示来源批次再确认保存；限原订单仓库、本客户、同规格可用现货，不挪用其他单，保留原实物批次和历史版本，释放对应待入库预订，不改购货或入库事实。之后按既有回填流程确认实际成本并发货。

## RED

- `TestShippingReturnMatchesPendingOrderCreatedAfterImport`：导入后收件匹配的待入库订单，旧查询返回空候选。
- `TestFulfillmentExplicitPhysicalReplenishment`：已有足额现货，显式选择补足后旧预览仍为 `StockReady=false`。
- `shipping-return-runtime.test.js`：禁用候选仍触发一次提交。
- 原始输出保存在本机 `/tmp/kferp-return-verification/red.log`、`frontend-red.log`。

## GREEN

- 隔离 PostgreSQL，`ORDERAPP_TEST_DATABASE_URL` 指向本机专用 55491 端口；逐测试隔离 schema。`go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestShippingReturn|TestFulfillmentOrderEdit|TestFulfillmentExplicitPhysicalReplenishment|TestDirectShipHistorical' -count=1` 通过。
- 覆盖导入后新匹配、待入库可见且不能发货、歧义防误配、显式补足、不勾选保留原预订、来源历史版本守卫、库存变化拒绝、操作日志失败回滚、原购货入库数量不变、重复提交、补足后无需重导即可回填、HTTP 发货与历史批次一次扣减。
- `scripts/verify_kferp.sh backend` 通过（全包非数据库测试；数据库场景单独执行如上）。
- Vue 全量 1767 项通过，Vite 构建通过；新增真实组件测试验证默认未选、未预览不能保存、展示具体批次、保存使用已展示预览而不自动换来源、重开清空选择。
- Chrome 本机隔离页面观察：未就绪候选显示三个日期和阻断原因，「使用此单」禁用；补足默认未选，勾选并核算后展示客户货权、仓库、批次、数量。使用纯测试收件信息及模拟 API，未访问正式写入接口。

## 手册

- `docs/OP_MANUAL_SHIPPING_RETURN.md`（Vue shippingReturnManual 复用此单一源）。
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`；Vue 编辑页面与回填弹窗同步提示。
- PR-751-SHIPPING-RETURN-CANDIDATES，DEV-PR751-01 至 03：待 Van 现场验收，不代验收。

## 生产业务边界

只读核查批次 2 的两张运单仍待选择。SF1224903286779 的 10 月 10 日记录与订单 SO-20260922-000190 日期和收件信息相符，但同收件人还有多张订单；SF5146788624713 的收件信息与 SO-20261031-000173 不同。已经请求 Van 明确两张运单的订单号。在归属确认前，不替用户选择、补足或发货，也不改变真实库存、运费、订单日期或收件人。结算重量未在本次修复中推断或补写。

## 发布回执

待记录合并、部署版本、服务检查和生产只读候选检查。无数据库迁移，无小程序源码改动。
