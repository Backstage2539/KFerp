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

