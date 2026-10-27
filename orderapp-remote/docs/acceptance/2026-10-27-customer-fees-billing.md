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
- 吴国宏客户记录在开发环境中不可用，因此没有用开发数据代替生产业务验收。生产正式发布后已只读确认应用健康；未登录的账单接口返回 401。

## 生产发布与数据迁移核对

- `main` 运行提交：`e89799f900cd2e334678f233b808d62d3136f253`；命令 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production` 完成。发布时 Vue/Vite 测试和构建、436 项小程序测试与类型检查/构建、24 个页面声明校验、服务器全量 Go 测试及镜像内 Go 测试均通过。
- 生产运行状态：`erp_prod_orderapp` running、重启次数 0，PostgreSQL healthy；`/app/login` HTTP 200，匿名账单 API HTTP 401。Caddy 与 docconvert 保持运行。
- 已登录生产工作台核对吴国宏费用页：合计未付 ¥423.20、未出账 ¥423.20、已出账 ¥0.00，共 6 笔；订单款项 ¥397.00、实际运费 ¥20.20、代发服务费 ¥6.00，均显示已确认。页面未执行勾选或出账操作。
- 回退点：源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261027214517-e89799f900cd`；镜像 `kferp-orderapp-rollback:production-20261027214517-e89799f900cd`。
- 发布前数据库备份 `/opt/stacks/erp-production/backups/pre-customer-fees-billing-20261027.dump`，大小 15,155,525 字节，`pg_restore -l` 校验通过，SHA-256 `87411f1630bd9c2228223ce687c421e3ce56ac098b887176539c37dd2fac1a0f`。
- 应用启动兼容迁移：吴国宏客户 ID 488 的 2 笔旧月结草稿（¥26.20）已改为 `reversed`；每笔释放 2 条费用来源关联，共 4 条；剩余旧草稿数为 0。`audit_logs` 中存在两条 `legacy_draft_release` 记录，分别对应 `CS-488-20261001-20261022`、`CS-488-20261001-20261031`。没有生成正式账单或登记收款。
- 小程序完成生产配置的测试、类型检查与构建，但没有上传微信、提交审核或发布。自动化功能测试使用隔离数据库和合成费用。
