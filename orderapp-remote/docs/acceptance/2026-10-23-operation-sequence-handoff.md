# PR-723 工序顺序与实际交接

## 范围
用户确认：前序给多少就是多少，不设置计划/最低数量要求，只卡工序，前序产出必须经过当前工序。落实为同工单所有前序未取消任务完成后才能开始、继续或完成本工序。计划数量不改写；实际交接按冻结库存单位展示，单批报工默认前序实际产出、多批由操作员分别记录。首道 WIP、质检、排班、计件与耗料规则保持。

## RED / GREEN
- API RED `/tmp/kferp-pr723-api-red.log`：4种低产/单位样例均被旧数量门槛阻断；开始/继续/完成可绕过较早未完成工序；工位仍显示可开始。
- API GREEN `/tmp/kferp-pr723-api-green.log`：真实PostgreSQL隔离schema，3个顶层用例含kg/g/件/零产出、顺序保护、取消兼容、失败无审计写入、成功日志和计划不变；9.445s。
- 前端 RED `/tmp/kferp-pr723-ui-red.log` → GREEN `/tmp/kferp-pr723-ui-green.log`：2个新行为用例及20个现有回归通过。
- 手册：`docs/OP_MANUAL_PRODUCTION.md` PR-723，前端实际交接说明同步。

## 交付
独立分支 codex/operation-sequence-handoff-20261023，从origin/develop a82da57e开始；原ccd2工作区保持。PR-723 / DEV-855～857，Van验收独立。生产发布与回滚回执在验证完成后补记。

## 回归与页面验收
- 取消前序的边界 RED `/tmp/kferp-pr723-cancel-red.log`：后续工序被误当首道用料核对；修正后纳入真实 API GREEN。
- 最终真实 PostgreSQL 回归 `/tmp/kferp-pr723-api-final.log`：`TestPR723|TestJobCardStartUsesFrozenBatchTailRemainder|TestPR722BulkPickingAPI` 全部通过（34.413s），含现有首道 WIP 与整袋领料保护。
- `scripts/verify_kferp.sh backend` 全通过；前端全量 1559 通过 / 0 失败；Vue build 和 changed 检查通过。日志位于本任务 `outputs/pr723-production/`。
- 全量 PostgreSQL 扩展筛选发现既有 `TestJobCardStartUsesWorkOrderIdentityAndAutoDefaultLead` 使用已迁移的工序负责人设置接口；未修改基线 c8657889 同样失败，记录于 `kferp-pr723-baseline-old-fixture.log`。本次不改旧测试所依赖的无关人员设置功能。
- 浏览器使用真实 WorkstationView + 本地内存 API：低于计划的 11.2kg 可开始；完成面板默认实际投入 11.2kg，按 10.9kg 实际产出提交后刷新成功；前序未完成时无开始按钮；390px 窄屏同流程成功、字段无横向溢出。生产业务按钮未点击。
- 人工代码复核：工单行锁内统一 start/resume/complete 顺序保护；无计划数量比较；同工单全部更早有效任务完成；实际汇总仅相邻前序且按库存单位；多批不重复预填；原冻结计划与审计流程不改写。


## 生产发布回执（2026-10-23 20:15 CST）
- develop PR #277 → `80945b5e2eb5eb569e57cae3771aadf94430a3e6`；独立 main PR #278 → `45faf9642eb7ff8148ab2b9b4832d62eb86961f5`。
- 干净 main 执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`，共享服务器发布锁保护，退出 0。服务器前端 1559、miniapp 432、类型检查/24页构建、Go 全量、镜像内 Go 测试全部通过；本次未上传或发布微信小程序。
- 运行版本 `45faf9642eb7ff8148ab2b9b4832d62eb86961f5`；镜像 `sha256:d39ee684a80be76dae659d574a928b6f79edc4e3f20b25c3847373b4e747e1b4`；应用 running，数据库 healthy，启动无异常。未登录401，认证 shell/API/生产手册200。
- 工位资源 `WorkstationView-cgMH9yTG.js` 公网/服务器/容器 SHA256 一致：`fa9361529d8abdb0dccdb00179cd55553d45fe8622ca96acfe622f43d1f60f65`。
- 生产只读 API 与实际浏览器一致：任务7小菠萝前序合计11.2kg、任务22初晓前序6.5kg，均 predecessor_completed/can_start=true、is_blocked=false；页面显示对应实际产出与开始入口，没有旧数量不足提示。
- 工单5/19及其任务 before/after 摘要与哈希完全一致。生产不执行开始/完成/领料/过账。DEV-855/856/857已完成，PR-723保持review待Van；发布完成状态通过单行进度回写并写入操作日志，后续种子同步完成态。
- 回滚镜像：`kferp-orderapp-rollback:production-20261023200627-45faf9642eb7`，指向上一版 b07c4234 的 `sha256:a10f9a853a922b9cdaeaeb6de2c2692bc2c5fc23e7c06b3cc7c3496099baa17e`；源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261023200627-45faf9642eb7`。保持PR722最低安全回滚01013bde，禁止退回不支持精确预留撤销的更旧版本。
- 本地证据 `outputs/pr723-production/`：`deploy.log`、`before.json`、`after.json`、`checks.json`、`runtime.txt`、`progress-receipt.txt`、`production-actual-handoff.png`，以及RED/GREEN与隔离页面截图。
- 本回执仅更新文档及进度种子，不再次重启生产；当前运行版本仍为45faf964。原ccd2目录已有改动保留。Van业务验收未代签。
