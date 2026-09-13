# PR-658 排班替补生效、请假自动补位与整日换人

日期：2026-09-13（北京时间）。目标：develop / development。业务验收由 Van 完成。

## 最终实现

- 工位替补选中即加入有序列表；支持上移、下移、移除，保存后重新读取并展示主负责人及替补顺序。
- 工位格子使用“调整负责人”面板，候选为当天上班的启用内部员工，区分主负责人、替补、其他员工。名单外临时员工不会加入长期名单。
- 出勤与负责人编辑自动更新预览；过期响应不覆盖新修改，编辑清除旧保存成功提示，失败保留输入。
- 上班改为休息/未排班，会解除该人当天人工指定并自动匹配替补；外部停用产生的失效调整仍要求处理。
- 整日换人支持日期、A、B、部分工位与可选设 A 休息；保留请假前范围和最近保存的补位清单。整批保存，预览版本及具体任务范围校验，重复请求返回原结果。
- 待交接按真实任务去重，不将同一执行中任务在七天重复累计。当前工单创建前或任务开始前的日期不计该任务的影响。
- 名单外的当日负责人沿用同一解析逻辑进入本人页面并开工。补齐了工位视图/排班与既有 production.read / production.run 的菜单映射；没有增加权限种类。
- 修复自动派工后仍保留“未分配处理人”阻断，以及实际开工先校验旧执行人字段的问题。负责人在开工事务记录，物料、质检、工位及前序校验保留。
- 保存、开工和交接按周协调事务；接班员工本人只能确认今日交接，交接前实际执行人不变，交接保存原员工及任务范围。
- 手册和页面帮助入口：`docs/OP_MANUAL_PRODUCTION.md`，Vue/Vite 动态手册沿用同一来源。

## TDD 与自动验证

RED 证据目录：`/private/tmp/kferp-pr658-evidence/`

- `api-red.log`：名单外临时员工被旧资格校验拒绝。
- `interaction-red.log` / `vue-red.log`：实时预览和名单外候选契约失败。
- `employee-menu-red.log`：普通生产员工缺少工位入口权限映射。
- `owner-readmodel-red.log`：排班已有负责人但任务 CanStart=false，仍提示未分配处理人。
- `handover-identity-red.log`：他人可以代接班员工提交确认。
- `handover-count-red.log`：同一执行中任务跨天重复统计。
- 另在隔离开工链路直接重现“本任务尚未分配执行人”，修复后与任务页面均可正常开工。

GREEN：

- Go 后端全包验证通过（`scripts/verify_kferp.sh backend`）。
- PostgreSQL 针对性回归通过：PR657/658 排班生命周期、备选名单、临时人、请假清除、批量部分替换/保持出勤/同时休息、事务回滚、版本冲突、幂等、同数量任务范围替换、本人开工和交接、任务数量与 875g+874g 尾差等。
- Vue 测试 1225/1225 通过，包括快速连续修改、预览乱序、失败保留输入、替补选中/排序/保存/重新读取、待交接去重；Vite 构建通过。
- 扩大 PostgreSQL 回归出现 17 项既有失败；在独立未修改的 `origin/develop@cdd6768c` 上重跑得到完全相同的 17 项，无新增失败。主要为历史 BOM/物料夹具与已迁移逐任务人员测试。明细见 `postgres-regression.log` 与 `postgres-baseline.log`；不能将该扩大套件表述为全通过。

## 真实页面验证（隔离数据）

使用本机正常 Go/Vue 应用与真实 PostgreSQL 独立 schema，普通员工通过正常账号密码登录。

1. 4 名隔离员工全周上班，保存 V1；A 的周二、四、六改休息，自动预览 B 接替 3 个工位，保存 V2，刷新一致。
2. 智烘移除 B 后重新选择 B，选中即加入，调整顺序为 B→D；保存、刷新、切换其他工位再返回，替补顺序保持。
3. 智烘周日临时选择名单外 C，保存 V3；C 普通生产账号在“我的今日工位”看到 2 项任务，实际点击“开始本任务”成功，2kg 投料/2kg 目标产出和绿色 WIP 齐套正常。
4. 整日 C→B 并勾选 C 休息，预览显示 1 个工位、1 个未开始任务、1 个待交接任务，保存 V4。
5. B 普通生产账号登录：当前执行任务仍属于 C、下一任务已跟随 B；B 点击“接手工位（1）”后两项任务均展示 B，交接记录保留 C。
6. 保存后的补位清单仍可从 C 的原工位范围继续选择 D，不必把已请假 C 恢复为上班。
7. 430×900 窄屏换人面板和排班卡片可用，操作栏无内容不可达问题。

截图：`/private/tmp/kferp-pr658-evidence/`

- `01-backup-config.png`：保存、刷新后的替补顺序。
- `02-auto-relief.png`：A 周二、四、六休息，B 自动补位。
- `03-owner-candidates.png`：含名单外员工的当日候选。
- `04-bulk-replacement.png` / `04b-replacement-impact.png`：整日换人和真实任务影响。
- `05-outside-owner-start.png`：名单外负责人实际开工成功。
- `06-handover-before.png` / `06b-handover-success.png`：交接前后员工归属。
- `07-narrow-replacement.png` / `07b-narrow-roster.png`：430px 窄屏。

## 部署与业务验收边界

- 实现分支 `codex/production-roster-relief-20260913`；已推送提交 `ae3c240c16bacf03a07d9a7821e2ec3af75b3c9e`。整合最新 develop 后 `scripts/verify_kferp.sh all` 通过；GitHub [PR #120](https://github.com/Backstage2539/KFerp/pull/120) 合并为 `9e143dd5542e9d105e944eada59a51763452b84a`。
- 从干净、与远端一致的 develop 克隆执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`，返回 `Release completed` 和 exit 0。开发环境运行版本为上述 `9e143dd5`；本次后续证据提交仅更新文档，不改变运行应用版本。
- 回滚源码 `/opt/stacks/erp/orderapp.backup.deploy-20260913205817-9e143dd5542e`；镜像 `kferp-orderapp-rollback:development-20260913205817-9e143dd5542e`。
- 技术冒烟：登录页、认证后 Vue 排班页及排班 API 均 HTTP 200；未认证排班 API 401，未认证 `/app/` 按现有机制跳转登录页。PR-658 在需求 API 中为 review / VA。服务端排班、Vue 页面和生产手册 SHA256 与验证源码一致；RELEASE_INFO 版本一致。
- `erp_orderapp` 运行、重启次数 0，开发 PostgreSQL healthy，实际承载入口的共享 `erp_prod_caddy` 运行。旧 `erp_caddy` 为已停止容器，不是当前入口；未改动它。上线后应用日志无新增 error/panic/fatal。生产应用保持原运行状态，没有发布微信。
- 用户当前真实排班、未保存修改和实际生产工单保留；开工、换人、交接写操作均使用隔离数据。真实人员配置只修正已明确核实的智烘关系：工位 id3，主负责人段其晶 id6 保持，替补从空补为刘祎泊 id10；工位价格、适配工序等其他字段相同。
- 修正后 GET 重新读取：9/8、9/10、9/12 的智烘负责人均为刘祎泊，来源 backup；其他日期仍为段其晶。保存前后完整出勤、当日人工安排及周版本 V3 相同。配置操作日志时间 2026-09-13 21:07:08，actor=order，记录 primary=6 / backups=[10]。没有推测或补改其他工位名单。
- 独立打开开发页面只读复核，新增 `08-live-auto-relief.png`；原浏览器未保存页面没有刷新、导航或保存。其他工位仍缺替补或替补未排班时，页面显示具体原因。
- 部署证据：`deploy.log`、`deployment-smoke.json`、`runtime.log`、`live-config-repair.json`、`live-repair-audit.json`，均位于 `/private/tmp/kferp-pr658-evidence/`。截图总览 `screenshots.md`。
- 技术验证与隔离页面场景已完成，Van 的真实业务验收待进行。
