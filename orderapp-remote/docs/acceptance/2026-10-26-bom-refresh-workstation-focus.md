# PR-733 BOM 补齐后继续生产与工位任务直达验收

日期：2026-10-26  
分支：`codex/bom-refresh-workstation-task-resolution`  
基线：`44672f1dc`（开始实施时）；提交前已重放最新 `origin/develop@1fef1b577`。

## 已完成验证

- **RED（隔离 PostgreSQL/API）**：在开发数据库的独立测试 schema 上运行基线版本，`TestOperationCompletionRefreshesPublishedRoastingBOMAtomically` 失败。预览仍显示冻结配方 70%（50% + 20%），`bom_refresh_pending=false`，没有切换到同 BOM 已发布的完整版本；日志留存在开发主机 `/tmp/codex-pr733-red.log`。
- **GREEN（隔离 PostgreSQL/API）**：相同集成测试在当前实现通过，运行时间 7.09 秒。除核对 V002 完整配方、3kg 本批扣减 2.1kg/0.45kg/0.45kg、旧配料未用预留释放、重复请求幂等和后续批次固定版本外，还覆盖预览后发布新完整版本：旧预览被拒绝且不产生耗料/入库，再刷新后按新版本成功。测试在远端开发数据库创建并清理 `test_prod_flow_*` schema，未修改业务工单。
- **Go**：重放最新 develop 后，`./scripts/verify_kferp.sh backend` 全量通过；目标 PostgreSQL/API 用例另在隔离 schema 中通过。
- **Vue/Vite**：`./scripts/verify_kferp.sh frontend` 通过，前端测试 1,622/1,622，生产构建通过（保留既有大 chunk 提示）。
- **工作区检查**：`git diff --check` 通过。

## 尚未验收与发布门槛

- [ ] 部署到开发环境后，在浏览器从 BOM 返回原工位，确认工位显示 100% 有效配方，WO `…0115` 每批默认投料 3kg、参考产出 2.497kg；在隔离工单上点击完成并核对耗料、成本、批次、入库及操作记录。
- [ ] 使用隔离未排班工序卡从工单直达工位，确认真实待执行状态、无人值班说明、排班入口及完成按钮禁用；安排测试负责人后刷新，确认任务可以继续。
- [ ] 覆盖 BOM 发布版本变化、真实缺料、已有业务耗料时拒绝换配方、并发完成和库存失败回滚。
- [ ] 仅在上述开发浏览器验收全部通过后，才执行干净 `main` 集成和生产发布。生产业务工单检查只读；真实工序完成由现场操作。

## 现场数据边界

尚未对开发或生产的真实工作订单执行 BOM 刷新、耗料、工序完成、入库或排班写入。开发环境的 API 验证仅操作自动清理的隔离 schema。WO `…0115` 与工单 `…0124` 的浏览器验收及业务数量核对待开发部署后进行。
