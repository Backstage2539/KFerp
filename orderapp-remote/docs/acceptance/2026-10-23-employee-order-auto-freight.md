# PR-720 普通员工录单自动运费与小程序编辑修复

## 需求与边界

员工网页与小程序新建、编辑普通订单时，默认按有效运费模板报价并将报价计入本单应收；支持本单切换启用模板、手填运费、包邮和恢复自动。商品物流重量缺失时，页面指出缺失商品，允许输入含包装总重；报价直接按包裹总重计价，不重复加入模板包装重量，也不写回 BOM。已有订单保留原运费协议，用户明确切换后才重新报价。实收不随报价自动变更。

小程序保存失败根因在原订单商品行重新经过当前 BOM / 发布价格表解析：商品行未改动，仅保存运费时，历史规格版本也可能已不再是当前有效版本，因此整单保存会失败。接口没有保留完整错误位置，客户端只能显示通用 internal error。修复在服务器侧确认全部商品身份、数量、价格、发布表和来源快照均未改变后，锁定并沿用原始商品行快照；只有发生商品编辑时才走现行 BOM / 价格校验。API 记录路由、订单号、处理阶段、错误类别和 SQLSTATE，不记录请求正文、收件信息或原始数据库错误文本。未能从当时日志中恢复生产请求的具体异常，因此不将该订单两次 500 断言为已被逐字复现；隔离回归覆盖了导致该类运费编辑失败的旧 BOM 版本路径。

## RED / GREEN 证据

- RED：新增的 PostgreSQL 回归在实现前提下遇到订单商品仍重新校验当前 BOM / 价格表；改为保留完整原始明细快照后，验证商品明细、售价和 BOM 身份保持不变，运费变更成功。
- RED：实现新重量来源快照字段后，现有 `TestOrderFreightV2LegacyAndConcurrentImport` 发现单纯改备注会改变旧运费协议快照；将新增重量来源字段限制在明确使用新录单协议的报价后，该兼容回归恢复通过。
- GREEN：串行 PostgreSQL 回归覆盖自动运费、含包装包裹总重、快照保存、旧延期协议未变、未改商品快照的运费编辑、实际运费处理和确认流程。

## 验证

- `ORDERAPP_TEST_DATABASE_URL=<隔离测试库> go test -p 1 ./internal/interfaces/http/customerportal ./internal/interfaces/http/freight ./internal/interfaces/http/support`：通过。
- `ORDERAPP_TEST_DATABASE_URL=<隔离测试库> go test -p 1 ./internal/domain/freight ./internal/infrastructure/postgres/customerfulfillment -run 'TestOrderFreightAutoCurrent|TestOrderFreightV2LegacyAndConcurrentImport|TestOrderFreightV2SalesAndActual|TestOrderFreightV2PendingConfirmation' -count=1`：通过。
- `env -u DATABASE_URL -u ORDERAPP_TEST_DATABASE_URL go test -p 1 ./...`：全 Go 包通过，与远程发布流水线的单 worker 测试设置一致。
- `node --test --test-concurrency=1 src/lib/*.test.js src/api/*.test.js && npm run build`（`frontend-vue-shell`）：通过。
- `npm test -- --maxWorkers=1 --minWorkers=1 --no-file-parallelism && npm run typecheck && npm run build:mp-weixin:development`（`miniapp`）：71 个测试文件、432 项测试通过；类型检查和开发包构建通过。
- `git diff --check`：通过。
- 使用隔离 PostgreSQL 数据库执行无筛选全量测试时，仓库中其他既有数据库 fixture、过期 PDF 断言和共享内存资源问题导致部分不相关用例失败；本次涉及的接口包和指定运费 PostgreSQL 用例单独串行通过。发布流水线不向 Go 测试进程提供数据库连接，因此其全包检查为绿色；真实数据库行为由上述针对性隔离回归覆盖。

## 操作影响与手册

- 用户触发的网页、小程序保存沿用订单操作日志；运费模板版本、重量来源、金额和操作人进入运费快照 / 审计记录。
- 网页操作入口：订单录入与销售资料中的“普通录单自动运费与小程序编辑”。小程序使用同一员工录单接口和报价口径。
- 操作手册：`docs/OP_MANUAL_ORDER_SALES.md`、`docs/OP_MANUAL_FREIGHT.md`；手册索引：`docs/OPERATION_MANUALS.md`。
- 不对现有生产订单批量重算，不执行生产环境写入。

## 开发环境交付

- 分支：`codex/employee-order-auto-freight-20261023`。
- PR / DEV：PR-720、DEV-845 至 DEV-848；开发环境代码已部署到 `2eb937868cbcbff3af9ee8b7616168756290c464`。
- 小程序开发包：`miniapp/dist/build/mp-weixin`，构建目标 `https://dev.qacoohee.com/app`。当前本地构建为 237 个文件、约 1.3 MB；`app.json` SHA-256 `4885c70ba9db557bd8b7acfcbfba3de98c5914a6561bf66817b1146a8aeb3e37`；员工录单页 SHA-256 `4a159cda82ad12b91a7537c9b3f490e8e5ec2b0d5fa086ab3d75207b5ce799e1`。
- 小程序仅生成并验证开发包；未上传微信、未提审、未发布。业务验收由 Van 另行确认。

## 生产交付与运行核对

- PR #264 已合入 `main`，生产提交 `6b3e67af6bda35572d91f95d4178887eb73be033`。该发布基于当时最新 `main`，也包含已验证的 PR #266 工单排班状态修复；没有带入仅存在于 `develop` 的其他提交。
- `bash ./deploy_orderapp.sh --preflight production` 对最终生产候选通过：Vue/Vite 全量测试和构建、小程序 71 个测试文件/432 项测试、类型检查、24 页生产构建、服务端 Go 全量测试、镜像内 Go 全量测试及生产镜像构建均通过。
- `bash ./deploy_orderapp.sh production` 完成，最终对外部署提交与 `origin/main` 一致。仅重建并重启 `erp_prod_orderapp`；运行状态为 `running`、重启计数 0，PostgreSQL 健康。未改写任何业务订单。
- 生产源码备份：`/opt/stacks/erp-production/orderapp.backup.deploy-20261023133639-6b3e67af6bda`；回滚镜像：`kferp-orderapp-rollback:production-20261023133639-6b3e67af6bda`。
- 运行烟测：`/app/login` 返回 200；未认证 `/app/` 和需求 API 返回 401；使用运行容器认证配置的只读需求 API 返回 200，产品需求可见 PR-720，开发需求可见 DEV-845 至 DEV-848。没有执行生产订单写入或批量回算。
- 小程序生产包位于 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。`RELEASE_INFO` 指向上述生产提交和 `https://erp.qacoohee.com/app`；`PAGE_FILE_MANIFEST` 共 96 个文件且已通过部署脚本校验，清单 SHA-256 为 `d52b9af4b3640a76c0396343e593aacd330ddc021ff1003c6fadc240f968e5bb`。此前本地包保存在 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20261023134649-6b3e67af6bda`。
- 小程序包仅在本机生成并校验；没有上传微信、提审或发布。现有订单 `SO-20261023-0001` 未被自动改价；如需使用新报价，应由员工打开订单、补填含包装包裹重量并主动恢复自动运费后保存。
