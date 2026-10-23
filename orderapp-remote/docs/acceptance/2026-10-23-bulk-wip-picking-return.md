# PR-722 整袋领料到 WIP 与返回工位

## 范围和生产只读基线

2026-10-23。实现分支 codex/bulk-wip-picking-return-20261023，自 origin/develop 8091b262；原 ccd2 工作区已有修改保持。DEV-851 兼容快照与精确撤销，DEV-852 超领及库存保护，DEV-853 返回和草稿保护，DEV-854 验证、手册、生产交付。Van 业务验收独立。

生产运行 6b3e67af。SE-0000000037 / work order 19 / job card 21 为草稿，4 行均为 60000g；只读基线哈希 e520c42ad38ec4c740b61670c0e30976cebbfa11bf1ebf510af66153300e62b7（包含工单、工序卡、当天排班）。不代用户提交该单。

## 原因与实现

自动领料路径在数量校验、预留移动两处把实际数量限制为未领预留，与旧批量领料提示矛盾。实际库存移动与预留转移需分离，剩余 WIP 保持同货主自由库存，BOM/消耗/产量不改变。

返回入口原先只在无可领明细提示块内显示，工位来源又被预览请求覆盖为 work_order。修复返回链接位置、范围/任务上下文及未保存修改的三选项处理。导航上下文仅保留前端内存。

内部 nullable picking_reservation_snapshot 保存本行实际预留移动。NULL 为旧单；新单即使只转移自由库存也保存 version/moves 空记录；事务内精确撤销。兼容提交仍保留旧上限，再由后续提交放开数量，兼容镜像为最低安全回滚版本。

## RED / GREEN

- HTTP/PostgreSQL RED：/tmp/kferp-pr722-api-red.log；真实自动领料计划预留 15.57kg，60kg 草稿保存重载正常，但提交返回 400 待领预留已变化；重复行与并发整袋领取同样复现。
- 兼容仓储 RED /tmp/pr722-compat-red.log → GREEN /tmp/pr722-compat-green.log；兼容既有自动领料 API /tmp/pr722-compat-api.log。独立 main 基线全 Go /tmp/kferp-pr722-compat-main-backend.log 与 PostgreSQL /tmp/kferp-pr722-compat-main-pg.log 通过。
- 批量领料 HTTP/PostgreSQL GREEN /tmp/kferp-pr722-api-green.log；同工单与不同工单共享批次并发重复三次 /tmp/kferp-pr722-api-concurrency-green.log。覆盖草稿重载、60kg提交、幂等提交、自由库存补领、非倒序撤销、kg/g/件、质量冻结、货主/来源身份、其他预留、审计失败原子回滚。
- 审查发现混合动作锁顺序风险；真实 HTTP + PostgreSQL 锁屏障复现 SQLSTATE 40P01（/tmp/kferp-pr722-api-lock-order-old-red.log），统一 draft/create-submit/submit/cancel 的工单→物料→批次加锁顺序后 GREEN（/tmp/kferp-pr722-api-lock-order-green.log）。
- Vue runtime RED /tmp/kferp-pr722-frontend-red.log → 12 新用例 GREEN /tmp/kferp-pr722-frontend-green.log；完整前端 1,557 项通过 /tmp/kferp-pr722-frontend-all.log，构建 /tmp/kferp-pr722-frontend-final-build.log。
- 真实 PostgreSQL 全 stock 包有与本任务无关的既有商品 BOM fixture 失败，8091b262 overlay 同样复现（/tmp/pr722-stock-baseline-existing-failures.log）；本任务新旧领料、撤销、并发用例单独运行通过，不把跳过数据库的全 Go 检查当作这些数据库用例已通过。
- 最终真实 PostgreSQL API：11 个新增顶层用例及4个现有 AutoPicking 回归全部通过（24.125s，/tmp/kferp-pr722-api-final-green.log），包含客户货主60kg闭环和商品组件新批次/预留/审计/撤销闭环。客户撤销流水缺 owner 的 RED /tmp/kferp-pr722-api-owner-ledger-red.log；最终两笔流水均保留货主。兼容镜像同样回补此项后才作为最低安全回滚。
- 完整 Go 包检查 /tmp/kferp-pr722-final-backend.log 通过；审查的锁顺序 finding 已修复复审。
- 所有业务写入测试在本机隔离 schema 或隔离浏览器数据中进行。

## 手册与页面验证

单一来源手册：docs/OP_MANUAL_STOCK.md、docs/OP_MANUAL_PRODUCTION.md 的 PR-722 节；Vue 页面的数量提示同步更新。本地隔离接口下使用实际 StockOperationsView/StockEntriesView/WorkstationView，浏览器已验证：

- 全厂工位进入领料，改为60kg；继续编辑保持输入，保存草稿并返回保留 all/工位/工单/任务21，重进草稿仍60kg。
- 提交成功留在单据页，手动返回后重新显示 WIP 齐套并定位原任务；任务离队时自动打开只读工单详情。
- 保存失败显示错误，三选项弹窗和60kg输入保留；放弃返回保留 mine 范围，不提交。
- 390px 窄屏抽屉可操作；提交失败后两行60kg均保留，返回原我的工位。
- 预览失败仍同时显示页面及抽屉返回入口；直接进入无 return_navigation 时无虚假入口。
- 截图 /tmp/kferp-pr722-ui-proof/save-failure.png；仅隔离 UI mock 保存/提交，生产零业务写入。

## 集成与发布

兼容提交80c38c05793377731375abfd1dc685d162f742ae，经开发PR #269合入74b1947a；生产 cherry-pick606fbbdc经PR #270合入8a86ee97。初版兼容发布已完成，后续补齐客户货主撤销流水；最终兼容和功能发布均已完成，见下方正式回执。发布全程使用共享锁，并保护数据库、数据卷、源码与镜像回滚依据。


## 发布候选复核

最终功能提交 d1dcd283 经开发 PR #273 合入 develop 5a3d7b18。生产候选基于 main 01013bde，仅 cherry-pick 本任务功能，不夹带开发环境其他改动。

初版兼容生产 8a86ee97 发布成功，API、页面、nullable JSONB、数据库健康均通过；草稿37/工单19/工序卡/排班整体只读哈希与基线一致。随后客户撤销流水补丁3c4b2a9d经开发PR #271、生产PR #272合入 main 01013bde；该修正版兼容镜像才是最终最低回滚版本，旧8a86ee97不作为客户货主安全回滚。

当前 main 候选完整 Go、真实 PostgreSQL 11个新API+4个AutoPicking、1,557项前端测试及构建再次通过。期间本机 Go 编译缓存耗尽空间；停止失败检查、清理可再生32GB缓存后重新完整验证通过。保留源码、数据库和回滚镜像。最终运行版本、镜像、生产页面截图与只读哈希见下方回执。


## 正式生产交付回执（2026-10-23 19:04 CST）

- 最终功能通过 PR #273 合入 develop，通过 PR #274 合入 main；正式生产运行提交 `b07c4234b083cf113f963fdfcd202160840ecae3`。从干净 main 执行标准 production 发布，退出0；后续回执文档提交与此运行版本分开记录，不为文档重启生产。
- 当前镜像 `sha256:a10f9a853a922b9cdaeaeb6de2c2692bc2c5fc23e7c06b3cc7c3496099baa17e`；启动时间 `2026-10-23T11:04:11.829451567Z`，运行正常、重启0，PostgreSQL healthy。匿名业务 API 返回401，登录页200。
- 公网/宿主/容器前端完全一致：`StockOperationsView-DulXP_Uq.js`，SHA256 `9a4e0735afb8190e6b4f4fe981cad1e94fd4a9cf6615b91312d0232f724d4c3a`；shell SHA256 `4f32cb700b5dc9241e954c1bd38843055eeecf0b8216f22d5bedaa717fc9d295`。领料抽屉编入 StockOperationsView chunk；最初烟测按独立 StockEntriesView chunk 查找失败，改为实际打包入口后全部检查通过，未改动产品代码。
- 新 nullable JSONB 列存在；生产/库存手册 PR-722 内容在线可读；现有库存操作日志 API 返回200。PR-722 为 review，DEV-851～854 均 done；Van 业务验收待进行。
- 生产浏览器只读验证：直接打开草稿37无虚假返回入口；工位全厂→布勒→初晓任务21→领料，页面和抽屉均有“返回工位”，新说明可见且不显示技术来源字段；四行均60kg。点击返回恢复 `scope=all`、布勒、工单19、任务21，任务高亮并重新加载齐套状态。未编辑、保存或提交生产单据。
- 浏览器前后两次只读核对，草稿37仍 draft、4×60000g；工单19、工序卡及排班整体哈希均为 `e520c42ad38ec4c740b61670c0e30976cebbfa11bf1ebf510af66153300e62b7`，与发布前完全一致。成功过账、失败恢复、三选项、撤销、并发验证仅在隔离数据完成，未对生产草稿代为过账。
- 最终 main 检查：完整 Go 通过；真实 PostgreSQL 执行 `ORDERAPP_TEST_DATABASE_URL='postgresql:///postgres?host=/tmp' go test ./internal/interfaces/http/production -run 'TestPR722BulkPickingAPI|TestAutoPicking' -count=1`，11个新增顶层用例与4个AutoPicking回归通过；前端1557项通过、0失败、0跳过，构建成功。既有全 stock PostgreSQL fixture 失败边界见上文。
- 未执行微信上传、审核或发布。原 ccd2 工作区改动保留。

### 最低安全回滚

- 先于最终功能部署并验证的兼容提交：`01013bded26f3f46fcf6bc5ff0690a5e5dc690db`，保留旧领取上限，但支持新快照、精确撤销及客户货主流水。
- 安全镜像 `sha256:4b4fb42e2afa63848ecb198210c29da28f6eb57540cf9b926ab5632138f6733b`，以下两个标签已确认均指向此镜像：`kferp-orderapp-rollback:pr722-minsafe-01013bde`、`kferp-orderapp-rollback:production-20261023185501-b07c4234b083`。
- 对应兼容源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261023185501-b07c4234b083`，RELEASE_INFO 已核对为01013bde。回滚时保留JSONB列和已有快照，沿用共享发布锁，仅替换应用镜像/匹配源码，重新核对健康与只读单据；回滚后数量旧上限恢复。
- 新超领单据出现后，不得使用8a86ee97、6b3e67af或更旧程序作为回滚。数据库与数据卷不回退、不清空。

### 可复核本地证据

`outputs/pr722-production/` 保存 `final-deploy.log`、`final-smoke.json`、`final-after-ui-smoke.json`、`final-rollback.txt`、最终 Go/API/前端检查日志，以及 `production-return-draft37.png`、`production-return-workstation.png`、隔离页面失败与窄屏截图。以上输出仅为验收证据，不作为生产业务数据修改来源。
