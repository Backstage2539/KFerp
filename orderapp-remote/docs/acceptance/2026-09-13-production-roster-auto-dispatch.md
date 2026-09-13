# PR-657 生产排班与工位自动派工验收记录

日期：2026-09-13（Asia/Shanghai）
范围：`develop` / development；production 不在本次范围。
验收边界：自动化与隔离数据验证属于技术验收，Van 在 development 的真实业务判断仍待完成。

## 最终行为

- “生产排程”更名为“生产排班”。排班按周一至周日维护员工每天上班、休息、未排班，并在同页显示工位每天唯一负责人。
- 工位维护一名主负责人和有序替补。负责人按“有效当日调整 → 上班主负责人 → 按序上班替补”解析；一人可以负责多个工位，无人值班时禁止新任务开工。
- 员工工位页默认“我的今日工位”。待执行任务不保存每日人员副本，读取和开工时解析当天工位负责人；开工后冻结实际负责人。
- 工位换班立即影响未开工任务。执行中任务由新负责人确认“接手工位”，交接记录保存任务范围及每张任务的原负责人。
- 尚未开工的商品需求可撤回，与新增待计划需求原子合并成新草稿；共享上游会扩大并展示撤回范围，已开工、耗料或产出会阻止处理，WIP 实物保留。
- 页面验收发现并修复整单已进入生产中时无法预览剩余未开工商品的问题；同一计划已开工商品继续执行，未开工商品仍可独立撤回重排。
- 页面用 `V0 / 本周排班尚未保存` 区分首次排班，员工无工位时分别说明休息、尚未排班和本周未保存；全厂无任务工位显示“今日工位暂无任务”。
- 撤回重排摘要沿用商品业务单位，例如 `11 袋`，不会统一写成 kg。
- 旧逐任务排程写入口返回功能已迁移提示；历史任务人员与旧工序人员关系保留只读。

## TDD 与接口证据

- RED：排班负责人解析、未排班、失效调整、版本冲突、幂等、无人值班门禁、工位交接、共享上游撤回及需求合并测试在实现前失败。
- GREEN：`production_roster_test.go`、`production_roster_postgres_test.go`、`production_replan_test.go`、`production_replan_scope_test.go`、`production_flow_api_test.go` 通过。
- 真实 PostgreSQL：隔离员工、工位、周出勤、替补负责人、我的今日工位、任务自动归属、交接及操作日志生命周期通过；撤回旧需求与新增需求合并为单一草稿项、旧新单据关联和幂等通过。
- 生产中计划的部分撤回：两项商品中一项已开工、另一项未开工时，仅未开工项进入新草稿，原运行工单和原计划生产中状态保持不变。
- 关键回归：1,749g 分批保持 875g＋874g；真实 1g 缺口仍阻断；已消耗批次不重复计入；已取消工单不回到工位队列。

## 页面与构建

- Vue 页面：`ProductionScheduleView.vue`、`ManufacturingWorkstationsView.vue`、`WorkstationView.vue`、`ProductionReplanWorkspace.vue`。
- 页面契约：员工周表、工位周表、复制上周预览、临时换人、恢复自动安排、我的今日工位、待交接、撤回重排和窄屏卡片均有前端测试。
- 操作手册：`docs/OP_MANUAL_PRODUCTION.md` 顶部 PR-657 章节；页面帮助沿用该手册入口。
- 完整检查：Go 全包通过；Vue `1218/1218` 通过；小程序 `238/238` 通过；Vite 正式构建通过。

## 操作日志

- 周排班保存：`production_roster/save`
- 工位人员配置：`manufacturing_workstation/update` 或 `create`
- 工位交接：`production_workstation_handover/handover`
- 撤回并重新安排：旧计划 `production_plan/replan`，新计划 `production_plan/create_replan_draft`

## 开发环境与截图

- development 应用版本：`e82904f84693b68a8b1cd40383cb7913a3e3c736`。
- 源码回滚点：`/opt/stacks/erp/orderapp.backup.deploy-20260913165023-e82904f84693`。
- 镜像回滚点：`kferp-orderapp-rollback:development-20260913165023-e82904f84693`。
- smoke：登录页与 Vue shell 均为 HTTP 200；`erp_orderapp` 为 Up；PostgreSQL 为 healthy；部署后浏览器控制台无错误。
- development 当前数据尚未完成工位人员和本周出勤首次配置，因此页面如实显示“待补人员配置”“本周排班尚未保存”，没有从旧工序人员关系猜测工位资格。
- 实际单据 `PP-0000000114` 仅作只读对照，确认生产中计划的未开工商品可预览撤回范围、数量单位和新草稿合计；未点击最终提交。所有排班、交接、开工和撤回写操作只在隔离数据中验证。

截图目录：`/private/tmp/kferp-pr657-acceptance/`

1. `01-weekly-employee-roster.png`：本周员工排班与首次保存状态。
2. `02-workstation-week.png`：工位七天负责人表及待补配置。
3. `03-my-today-workstations.png`：员工本人今日工位空状态。
4. `04-factory-workstations-and-quantities.png`：全厂任务规格、投料、产出和批次。
5. `05-workstation-staff-config.png`：主负责人、有序替补和未来七天影响。
6. `06-narrow-roster.png`：430px 窄屏排班布局。
7. `07-replan-preview.png`：生产中计划未开工商品撤回重排预览。

技术验收完成；Van 的真实排班、换班、开工与撤回业务验收仍待进行。
