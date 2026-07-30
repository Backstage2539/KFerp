# PR-556 Production Plan Draft Split UX

## 目标

- 生产计划预览的 BOM 摘要不再展示旧“预期产出率”。
- 仅当实际解析到的 BOM 版本启用 `material_loss_rate` 时展示“预期损耗”。
- 当前计划预览删除“计划投料(g)”列；计划详情和工单仍保留冻结的计划投料数据。
- 顶部“生成草稿”成功后立即打开同一草稿的拆分产能抽屉。
- 删除当前计划区重复的“创建生产计划”，保留提交工单和撤销草稿。

## 业务口径

- 损耗权威值是具体 SKU BOM 优先、父商品 BOM 回退后实际选中的 `production_bom_versions.material_loss_rate`。
- 不从 `bom_yield_rate` 推导损耗；历史兼容默认 `0.8` 不能被解释成用户配置了 20% 损耗。
- `material_loss_rate=0` 时摘要只显示 `默认 BOM`；`material_loss_rate=0.12`、`0.18`、`0.2` 时分别显示 `默认 BOM / 预期损耗 12.00%`、`18.00%`、`20.00%`。
- BOM 解析失败时返回 `bom_summary_error` 并显示 `BOM 配置待完善`，不把解析异常伪装成有效的 0 损耗。
- 只有明确的 BOM 业务配置错误作为行级提示；数据库、连接、上下文和事务错误直接返回接口错误。
- 自动打开抽屉只改变前端操作衔接，不自动保存拆分、提交工单或写 WIP/库存。
- 自动拆分建议只存在于抽屉中；用户保存前不写入当前步骤状态。

## TDD 证据

- RED：`node --test src/lib/produce-plan.test.js` 因缺少 `productionPlanBomSummary` 导出失败。
- RED：`go test ./internal/infrastructure/postgres/production -run '^TestProductionPlanBomMaterialLossRateUsesResolvedVersionMetadata$' -count=1` 因损耗摘要解析函数缺失而编译失败。
- GREEN：`node --test src/lib/produce-plan.test.js`，46/46 通过。
- GREEN：生产计划损耗摘要后端定向单测通过。
- API：临时 PostgreSQL 16 中，BOM 无损耗与子 SKU 继承父 BOM 20% 损耗两条 `/api/produce/unproduced` 用例通过。

## 验收清单

- [x] 页面源码不含“预期产出率”，BOM 摘要使用 `bom_material_loss_rate`。
- [x] 当前计划预览源码不含 `计划投料(g)` 表头和 `row.input_g` 单元格；详情和工单中的计划投料未删除。
- [x] 无损耗 BOM 显式返回 0，页面只显示 `默认 BOM`。
- [x] 子 SKU 继承父商品 BOM 时返回父 BOM 版本 20% 损耗。
- [x] BOM 解析失败时页面显示 `BOM 配置待完善`，并保留悬停错误原因。
- [x] 非 BOM 配置类底层错误不会进入页面行提示。
- [x] 当前计划区没有重复创建按钮，提交和撤销动作保留。
- [x] 创建草稿后调用现有拆分产能入口并打开同一抽屉。
- [x] 未保存的自动拆分不会让步骤提前跳到提交工单。
- [x] 完整 Go、定向前端 46/46、Vue/Vite 构建和变更检查通过；完整前端的 8 条失败报告（7 个既有断言加套件汇总）与干净 `origin/develop` 基线完全一致。
- [x] 合并到 `develop` 并部署 development；production 不部署。
- [x] 部署后完成只读 API、页面资源和服务健康冒烟，不自动创建真实生产计划。

## 部署证据

- 功能提交：`db9448e8`；`develop` 合并提交：`f29e24fd`。
- 执行 `./deploy_orderapp.sh development`，开发服务器备份：`/opt/stacks/erp/orderapp.backup.deploy-20260727114607`。
- `erp_orderapp` 启动且重启次数为 0；`erp_postgres` 为 `healthy`；部署后日志只有正常监听信息，最近日志中 `panic / fatal / SQLSTATE / conn busy` 计数为 0。
- Caddy 回环检查：`/vue-shell?view=producePlan`、REQ API 和未计划生产需求 API 均为 200；REQ 数据包含 PR-556；只读计划预览返回 `bom_material_loss_rate`，现场首条可选需求按新规则返回 BOM 配置提示。
- 应用内浏览器只读打开生产计划页成功：顶部“生成草稿”步骤唯一存在，页面中“创建生产计划”和“预期产出率”计数均为 0，控制台错误为 0。
- 未点击“生成草稿”、未保存拆分、未提交工单、未写 WIP/库存；production 未部署。
