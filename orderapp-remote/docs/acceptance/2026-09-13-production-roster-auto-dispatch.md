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
- 旧逐任务排程写入口返回功能已迁移提示；历史任务人员与旧工序人员关系保留只读。

## TDD 与接口证据

- RED：排班负责人解析、未排班、失效调整、版本冲突、幂等、无人值班门禁、工位交接、共享上游撤回及需求合并测试在实现前失败。
- GREEN：`production_roster_test.go`、`production_roster_postgres_test.go`、`production_replan_test.go`、`production_replan_scope_test.go`、`production_flow_api_test.go` 通过。
- 真实 PostgreSQL：隔离员工、工位、周出勤、替补负责人、我的今日工位、任务自动归属、交接及操作日志生命周期通过；撤回旧需求与新增需求合并为单一草稿项、旧新单据关联和幂等通过。
- 关键回归：1,749g 分批保持 875g＋874g；真实 1g 缺口仍阻断；已消耗批次不重复计入；已取消工单不回到工位队列。

## 页面与构建

- Vue 页面：`ProductionScheduleView.vue`、`ManufacturingWorkstationsView.vue`、`WorkstationView.vue`、`ProductionReplanWorkspace.vue`。
- 页面契约：员工周表、工位周表、复制上周预览、临时换人、恢复自动安排、我的今日工位、待交接、撤回重排和窄屏卡片均有前端测试。
- 操作手册：`docs/OP_MANUAL_PRODUCTION.md` 顶部 PR-657 章节；页面帮助沿用该手册入口。
- 完整 Go、Vue、Vite 和开发环境 smoke 结果在最终部署后补充。

## 操作日志

- 周排班保存：`production_roster/save`
- 工位人员配置：`manufacturing_workstation/update` 或 `create`
- 工位交接：`production_workstation_handover/handover`
- 撤回并重新安排：旧计划 `production_plan/replan`，新计划 `production_plan/create_replan_draft`

## 开发环境与截图

待功能分支合入最新 `develop`、完整检查通过并部署 development 后填写运行版本、回滚点、smoke 与以下截图：

1. 本周员工排班
2. 自动工位安排
3. 我的今日工位
4. 当日换班与待交接
5. 撤回并重新安排
6. 窄屏布局
