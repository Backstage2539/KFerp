# 客户费用正式出账与账单导出验收（PR-740）

## 验收口径

- 费用页以合计未付、未出账、已出账分区；合计未付只累加未出账和已出账的未收金额。
- 未出账流水可按费用类型、日期筛选和分页选择；出账须预览逐笔明细和分类小计，再显式确认。
- 订单款项按逐单未收余额纳入；库存代发不重复计购货款。预估运费不计入可出账金额；已入账实际运费、已提交代发单服务费可出账。
- 正式账单不可覆盖，来源不可重复出账；重复请求返回同一账单。确认时金额快照、来源关联及操作日志原子保存，不登记收款。
- 账单详情、Excel、PDF 使用相同来源明细；PDF 支持系统公章和账单专属位置设置。
- 订单行未付款保持红色；只有已发货且已付款整行绿色，“已发货”文字保持绿色。

## RED / GREEN 与验证记录

- 前端金额汇总、筛选选择、订单行颜色、账单 API helpers 先以缺失实现或旧界面行为触发失败，再补实现。最终 `node --test`：1,643 项通过、0 失败。用例覆盖未收金额不重计、当前页三态选择、筛选后只保留可选费用、未付款已发货红色、已付款已发货绿色。
- 隔离 PostgreSQL 定向用例：
  `DATABASE_URL='postgresql:///postgres?host=/tmp' go test -p 1 ./internal/appmain ./internal/application/customerfulfillment ./internal/infrastructure/postgres/customerfulfillment ./internal/interfaces/http/customerfulfillment ./internal/infrastructure/excel ./internal/infrastructure/pdf -run 'TestReleaseLegacyBillingDrafts|TestIssueBilling|TestCreateSettlementSelectedSourcesSnapshotsOnlyUnpaidAndAudits|TestBillingPDFSettings|Billing|CustomerBill|CustomerFulfillmentBilling' -count=1`
  通过。覆盖旧草稿来源释放、正式出账快照、同幂等键重放、不同键拒绝重复来源、预览版本变化时整体回滚、折扣与部分付款只记未收余额、操作日志、公章设置兼容读取/保存/审计、HTTP 客户范围与文件下载。
- `npm run build` 通过；Vite 显示现有的大型 PDF worker / preview chunk 体积提示。
- PDF 可视检查：使用 70 笔长中文费用名称生成横向 A4 样例共 11 页；已查看首页、续页和末页。单元格边框对齐，长名称在格内换行，续页重复标题和列标题，末页最后一笔与合计同页，页码正确。第一次渲染发现边框错位和合计单独成页，修正后复查通过。
- `go test ./...` 全仓并行执行未通过：运行中出现多处测试 fixture/schema 缺表或缺列，以及 PostgreSQL `out of shared memory`；改动相关 billing 定向包测试通过。该项结果保留为全仓测试限制，不报告为全仓通过。
- `scripts/verify_kferp.sh changed`、合入最新 develop 后回归、开发环境部署和浏览器验收将在交付时补记。

## 业务限制

本验收仅使用隔离测试数据库和合成费用。没有为吴国宏出正式账单、没有登记收款、没有改动生产数据；没有发布生产或微信小程序。
