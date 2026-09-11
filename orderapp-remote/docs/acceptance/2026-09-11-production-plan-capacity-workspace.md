# PR-651 生产计划产能拆分工作区验收记录

日期：2026-09-11

## 目标

- 产能拆分按本次计划的冻结工艺路线展示真实工序名称和顺序，不写死生产阶段。
- 每个工序按计划任务独立核对，保留商品/物料、冻结规格、数量、客户和来源订单；同商品多张订单不丢失。
- 重量任务和计件任务使用各自业务单位；短缺草稿可保存但不能确认，超排需明确核对。
- 页面采用与生产计划详情一致的完整工作区，保存与确认均不生成工单。

## TDD 与自动验证

- RED（前端）：`node --test src/lib/produce-plan.test.js` 首次因缺少 `buildProductionPlanCapacityGroups` 导出而失败；补 helper 后，组件契约继续因 `ProductionPlanCapacityWorkspace.vue` 不存在而失败。
- RED（后端）：定向 Go 测试首次编译失败，明确指出产能预览覆盖行缺少 `RequiredQty / ArrangedQty / DiffQty / Unit`。
- GREEN（定向）：任务原生单位、真实工序分组、10/8/2 来源任务、短缺草稿和超排确认用例通过；生产接口与 PostgreSQL 生产包定向测试通过。
- GREEN（完整仓库）：`./scripts/verify_kferp.sh all` 通过；Go 全包、前端 `1178/1178`、类型/构建门禁均通过，Vite 生产构建成功。
- GREEN（浏览器）：真实 Vue 组件在 `1488 × 1058` 桌面视口渲染；已切换两个实际工序，确认 `10袋 / 8袋 / 2袋` 来源任务和不足态可见，浏览器控制台无错误或警告。
- GREEN（设计）：项目根目录 `design-qa.md` 将参考图与两个工序状态的浏览器截图在同一轮比较，未发现待处理 P0/P1/P2，结论为 `passed`。
- development 首轮浏览器验收发现：不同计划行的路线工序都从序号 1 开始时，单纯按工序序号会把下游“包装”排到上游“咖啡烘焙+除石”之前。新增回归用例先稳定复现 RED，再按制造计划依赖层级优先、路线序号其次排序；修复后目标顺序为 `咖啡烘焙+除石 → 包装`。
- 合并后完整复验再次通过：Go 全包、前端 `1178/1178`、类型检查和 Vite 构建全部成功。
- development 已部署应用提交 `455b3df8ed1eb1e2b46ad151dd19a52f18963698`；发布脚本返回 `Release completed`，外部登录页 HTTP 200。

## API 与业务边界

- `POST /api/production-plans/:id/operation-splits/preview` 在原有克重兼容字段外返回逐任务 `required_qty / arranged_qty / diff_qty / unit`，接口仍保持只读。
- 计件工序优先使用计划需求冻结的销售单位；重量工序使用所选工位产能的重量单位。
- 既有 `PATCH /api/production-plans/:id/draft` 和 `POST /api/production-plans/:id/operation-splits` 继续负责持久化与操作日志；没有新增数据库表或迁移。
- `确认安排` 只完成拆分保存并返回计划详情，不调用提交计划接口，也不生成工单。

## 开发环境验收

- 功能分支：`codex/production-plan-capacity-ui-20260911@dacad016`；合并提交：`455b3df8ed1eb1e2b46ad151dd19a52f18963698`。
- `./deploy_orderapp.sh development` 成功；回滚源码：`/opt/stacks/erp/orderapp.backup.deploy-20260911211634-455b3df8ed1e`；回滚镜像：`kferp-orderapp-rollback:development-20260911211634-455b3df8ed1e`。
- `erp_orderapp` 正常运行，`erp_postgres` healthy；容器内 `/login` 和外部 `https://dev.qacoohee.com/app/login` 均返回 200；最近应用日志无 panic/fatal/error。
- `PP-0000000109` 只读验收确认工序为 `咖啡烘焙+除石 → 包装`，显示来自计划冻结路线而非页面写死；浏览器控制台无 error/warning。
- 上游任务显示 `初晓 5.64kg`；包装工序保留 `10袋 / 8袋 / 2袋` 三组任务，并显示六张来源订单和客户名称。
- 桌面截图：`/private/tmp/kferp-pr651-acceptance/pp109-capacity-roast.png`、`/private/tmp/kferp-pr651-acceptance/pp109-capacity-package.png`。
- 验收未执行保存、确认或其他业务写入；`PP-0000000109` 保持草稿。

## 产品验收边界

- 技术实现、自动测试、development 部署与浏览器截图由 Codex 完成。
- production 未包含在本需求。
- 产品需求保持待 Van 最终业务验收。
