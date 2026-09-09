# PR-643 客户订单与往来账单

状态：自动验证通过，等待 Van 开发环境业务验收。需求：PR-643；开发：DEV-643-ORDERS / DOCUMENTS / ACCOUNTS。

## 实现

- 客户专用订单、费用、周月账单和正式结算单接口；普通历史订单按账号绑定客户查询，安全 DTO 不返回工厂成本或内部订单编辑数据。
- 收件信息解析、订单锁与锁后发货状态重验、修改审计日志。
- 单张与跨页合并销售单预览、生成、文件下载全链路校验客户和文件归属；工厂模板在客户侧只读。
- 销售单新增收件快照，PDF/PNG 共用分组订单号与日期；渲染版本升级为 sales-order-customer-recipient-v6。新生成文件使用当前收件信息；历史文件不覆盖。
- 订单金额使用分整数，预付款计入已付；全结果汇总再分页。独立费用和结算单分列，不重复叠加。草稿不作为正式单据，入结算不代表已付款。
- 旧客户财务入口转向新页面；内部财务页面保留，客户直接访问内部财务和旧文档下载接口被拒绝。

## RED → GREEN

- 首次 account 日期/付款、地址解析和跨页选择测试失败于未实现接口与辅助函数，完成后通过。
- API 回归发现“已出库/已完成”等状态仍可修改收件信息：200，预期 400；补齐终态后通过。
- 发货事务持锁时请求修改地址，提交发货后修改仍返回 200；改为订单锁后独立读取状态，修复后 400。
- 新销售单快照没有当前收件地址；补齐快照和 PDF/PNG 输出后通过。

## 验证证据

- `scripts/verify_kferp.sh all`：全部 Go 测试通过（数据库依赖测试按项目常规无环境跳过）；Vue 1124/1124，Vite 构建通过。
- 本机隔离 PostgreSQL：`ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp'`；测试自行创建独立 schema 并清理。
- `TestCustomerAccountHistorySummaryFeesAndIsolation`：205 笔普通历史订单、预付款/全款/作废、另一个客户、跨页不截断、费用进入草稿未付款、正式单据、真实费用关联。
- `TestCustomerAccountAPIScopeAndDocumentDenials`：两个客户、跨客户参数、订单详情、合并预览/生成/下载及正式结算单越权。
- `TestCustomerAccountDocumentsAndRecipientLifecycle`：实际生成单张/合并 PDF 与 PNG，MIME/文件签名与图片解码、下载归属、历史快照保持、收件修改审计及并发发货限制。
- `TestCustomerAccountExportsAllRowsAndChinesePDF`：205 行 Excel 数据与合计、中文多页 PDF。
- 全部 PDF 渲染测试及销售领域测试通过；现有连续录单/收件隔离回归通过。
- 浏览器本地测试数据预览：跨页已选 1→2 保留；详情抽屉解析姓名/手机/地址；390×844 手机账单布局；浏览器无错误日志。该检查使用本地模拟数据，不代表真实客户业务验收。
- 已人工查看单张 PDF、合并 PDF、合并 PNG、205 笔账单第一页：文字、收件信息、订单号、金额和分页无重叠。
- 本机证据目录：`/private/tmp/kferp-customer-account-evidence-20260909/`，包含 verify-all.log、postgres-api-tests.log、pdf-tests.log、销售单及账单样本。

## 手册与发布

- 手册：`docs/OP_MANUAL_CUSTOMER_ACCOUNT.md`，总索引及履约/订单销售手册同步更新；页面内“查询与对账说明”可见。
- 开发环境交付。没有修改历史业务价格、订单金额、付款记录，也没有新增普通销售正式结算流程。
- 合并与部署信息记录在 ACTIVE_REQUIREMENTS.md 和交付报告；Van 验收未代签。
