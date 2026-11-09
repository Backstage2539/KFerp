# PR-751 寄件回填候选与现货补足验收

## 2026-11-09 共享批次后续订单校验修复

- Van 已明确两张运单归属，并自行更正第二单地址。首单 SF1224903286779 → SO-20260922-000190 已通过正式页面回填和出库，14 袋、实际物流成本 13.10 元；实际成本与客户应收分别记录。
- Van 明确选择从公司库存转入喜沁 4 袋。正式页面已提交 SE-0000000149：FP-OP-0000000019 → FP-MOVE-0000000006，4 袋、货权转为喜沁；第二单预览确认原批次 8 袋及新转入批次 4 袋并保存。未占用其他订单的 17 袋。
- 新阻断：发货就绪查询把已出库首单保留的历史分配继续计入占用。批次余量 8 袋已正确，本单分配 8 袋，但历史已发 3 袋也被计入，导致误判不足。与既有库存候选口径统一：已存在原子出库扣减记录的订单不再作为未发货预留。
- RED：新增 `TestShippingReturnSharedBatchExcludesAlreadyDeductedOrders`，两个订单共用 8 袋批次；首单 HTTP 出库 4 袋后，第二单拥有剩余 4 袋却 `CanShip=false`。日志 `/tmp/kferp-return-verification/shared-batch-red.log`。
- GREEN：隔离 PostgreSQL `TestShippingReturn|TestOrderProgress|TestFulfillmentExplicitPhysicalReplenishment|TestDirectShipHistorical` 通过（13.510s）；全包后端 verifier 通过。新增测试覆盖未出库两单仍累计占用、库存不足继续拦截、两单顺序 HTTP 回填、各自重复提交、8 袋总扣减、2 条日志、2 条运单及分配历史保留。前端实现未改动。
- 仅调整只读发货条件；不删除历史分配、不修改历史 BOM 或库存余额，不新增迁移。手册补充共享批次说明。
- 后续发布与第二单实际回填回执待补充；以上真实操作属于 Van 已明确授权的业务处理，不作自动测试。

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

- 功能提交 `2ceda2f95` 已推送；集成至 `develop@65a896eee611d7090b115d362f536fb75fefb628`，开发环境 ERP-only 部署成功。
- 开发回滚锚点：源码 `/opt/stacks/erp/orderapp.backup.deploy-20261108233503-65a896eee611`，镜像 `kferp-orderapp-rollback:development-20261108233503-65a896eee611`。
- 开发服务运行且重启数 0，登录页 HTTP 200；候选查询源码 SHA256 与本地一致，运行容器前端资源包含显式补足选项。匿名 API 401 只证明鉴权，不作为业务验收。
- 经确认本次没有夹带其他新增功能，已验证的 develop 合入 `main@54f87c0846dbbf8579aa920a4d02b50ef77e01fb` 并推送；`KFERP_ERP_ONLY=1 ./deploy_orderapp.sh production` 返回成功。
- 正式回滚锚点：源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108234647-54f87c0846db`，镜像 `kferp-orderapp-rollback:production-20261108234647-54f87c0846db`。
- 正式服务运行且重启数 0，Postgres healthy，登录页 HTTP 200；运行镜像 `sha256:8affb203845e1941b1a513df6d565e7396a162288fb5055ee7f22f6bdb8356ca`。候选查询源码 SHA256 `e9c0576c9682350902e55ec435a50ffbfd5efcacff61387ffe8f72a05840be93` 与已验证源一致；前端 `OrderEntryView-MOPN1mBN.js` 已装入容器。
- 正式 Chrome 员工会话只读核查：SO-20260922-000190 页面出现「选择快递单（2）」；弹窗同时显示本次两张运单，分别展示订单日期、快递下单及揽件时间，明确提示实物分配未完成并禁用「使用此单」。未进行补足预览、保存或运单提交。
- 正式只读 SQL 核查：回传行 10/11 均为 pending、selected_order_id 为空；订单 190/173 的快递单号为空。PR-751 和三个 DEV 行均为 review，等待真实业务归属确认及 Van 验收。
- 无数据库迁移，无小程序源码改动；ERP-only 保留原小程序包，不上传、提审或发布微信版本。本节为部署后文档回执，后续文档提交不代表运行版本再次变化。
