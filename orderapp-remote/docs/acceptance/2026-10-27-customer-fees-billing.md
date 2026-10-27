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
- 发布流程对应的单 worker 全仓 Go 验证 `GOMAXPROCS=1 GOFLAGS=-p=1 go test -p 1 ./...` 通过，包含本次更新的历史手册断言。另一次默认并行 Go 测试带本地共享测试库运行时遇到 fixture/schema 冲突和 PostgreSQL `out of shared memory`；这不影响单 worker 无数据库发布测试结果。
- `scripts/verify_kferp.sh changed` 通过；合入 `develop` 后在服务器再次串行运行 `GOMAXPROCS=1 GOFLAGS=-p=1 go test -p 1 ./...` 全部通过。开发环境部署成功，版本 `a1f775455be672d13b9f333ef4a1327ffe3b8793`；部署脚本健康检查 `https://dev.qacoohee.com/app/login` 返回 HTTP 200。
- 已登录开发工作台浏览器验收：客户履约运营台费用页显示“合计未付 / 未出账 / 已出账”，默认打开未出账流水；费用类型、上月/当月/自定义日期筛选、当前页三态全选以及发生日期、费用类型、来源单号、费用名称、应收/已收/未收和确认状态列均可见。用未开通履约场景的开发客户只验证页面空态与筛选控件，没有确认任何费用或出账。
- 吴国宏客户记录在生产档案中存在，但开发客户数据未同步该记录。生产仍运行旧版费用页面；本次只读确认版本差异，没有创建账单、登记收款或修改生产记录。因此，吴国宏具体费用在新版开发工作台的逐笔业务验收留待后续同步数据或正式发布后进行。

## 业务限制

自动化验收使用隔离测试数据库和合成费用。没有为吴国宏出正式账单、没有登记收款、没有改动生产数据；没有发布生产或微信小程序。
