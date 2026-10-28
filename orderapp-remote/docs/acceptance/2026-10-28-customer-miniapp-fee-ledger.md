# PR-741 客户小程序费用明细与下载验收记录

## 范围

- 客户小程序删除独立对账单、确认对账和提出异议入口，把货款与费用合并为按订单分组的“费用明细”。
- “应付”、商品货款、加工费、代发服务费、运费切换页面明细；全账期汇总不随分类或分页变化。
- 商品货款按订单日期归期；运费和服务费按费用入账日期归期。跨期费用关联原订单，库存代发不重复计购货款，预估运费不计入确认应付。
- 运费和代发服务费关联逐票发货信息；寄件导入可选记录结算重量及揽件时间。历史数据不补录，缺字段显示“未记录”。
- PDF/Excel 忽略页面分类和分页，导出当前账期全部费用。下载不生成正式账单或登记收款。

## TDD 与验证

- RED：实现前新增了分组/分类分页、跨期费用、旧模板和寄件字段、API 账期/导出查询、小程序交互来源及 PDF/Excel 输出断言；原有代码缺少这些字段和行为时定向编译/测试失败。对应回归测试保留在本记录列出的测试文件中。
- GREEN：`go test ./...`（`orderapp-remote`）通过。首次全量运行发现两条旧静态断言仍期待已删除的客户侧对账/异议文案；更新断言为新费用明细语义后全量通过。
- PostgreSQL：在本机一次性数据库 `codex_pr741_customer_fee_test_shipping_final_20261028` 中再次运行 `TestCustomerAccountHistorySummaryFeesAndIsolation`，通过；测试数据库在运行结束后删除。覆盖客户隔离、跨期运费仍关联原订单、同单两票运费、按运单匹配揽件时间与结算重量，以及缺失字段保持空值；样例分类金额为货款 ¥18,450.00、运费 ¥2,064.60，跨期订单小计 ¥14.60。
- API：`go test ./internal/interfaces/http/customerportal` 及客户账户处理器用例通过；覆盖登录上下文客户范围、`view=order_fees`、费用分类参数及文档导出忽略分类/分页。
- 小程序：`npm test` 72 个文件、436 项通过；`npm run typecheck` 通过；`npm run build:mp-weixin:development` 通过，输出为本地开发构建包，没有上传微信。
- Vue/Vite：`node --test src/lib/shipping-return-runtime.test.js` 5/5 通过；`npm run build` 通过。
- Excel：测试确认 3 张工作表、逐笔金额为数值、运单号按文本输出，页面分类不限制导出源数据。
- PDF：70 笔中文长名称费用生成 5 页；检查第 1、2 页，重复表头、长名称换行、表格边界和页码正常。样例文件位于 `/tmp/pr741-pdf/customer-order-fees.pdf`，不作为业务数据或版本文件提交。
- `scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。

## 手册与实现位置

- 客户履约与小程序操作说明：`orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`、`orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`。
- 寄件导入字段说明：`orderapp-remote/docs/OP_MANUAL_SHIPPING_RETURN.md`。
- 前端手册入口继续读取 `OP_MANUAL_CUSTOMER_FULFILLMENT.md`。
- 主要回归测试：
  - `orderapp-remote/internal/application/customerfulfillment/account_test.go`
  - `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/account_test.go`
  - `orderapp-remote/internal/interfaces/http/customerportal/mini_direct_ship_api_test.go`
  - `orderapp-remote/internal/domain/shippingreturn/return_test.go`
  - `orderapp-remote/internal/infrastructure/excel/customer_account_test.go`
  - `orderapp-remote/internal/infrastructure/pdf/customer_account_test.go`
  - `miniapp/src/utils/customerClosedLoopPages.test.ts`

## 开发部署与待验收

- `develop` 与开发运行版本：`0d0c34bae9dc180dd9e89daeb75fb21de4a34d55`。部署命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；仅重建并重启 `erp_orderapp`。旧源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20261028003830-0d0c34bae9dc`；回滚镜像：`kferp-orderapp-rollback:development-20261028003830-0d0c34bae9dc`。
- 部署后容器状态正常；开发登录页 HTTP 200、未认证需求 API 401、认证 API 200；PR-741 显示为“review / Van”，DEV-PR741-01..04 显示为“done / Codex”。
- 开发小程序包路径：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，构建清单含 24 个页面、96 个文件。微信开发者工具成功载入该开发包，首页显示“开发环境 · 测试数据”，问题计数为 0。当前模拟器处于客户账号登录页；需要客户账号会话才能进入费用页核对订单分组、分类切换和下载内容。真机镜像当前未连接，待 Van 登录后完成费用页及真机验收。
- 本次没有上传、审核或发布微信小程序；没有补录历史寄件时间或重量，也没有写入生产业务数据。
