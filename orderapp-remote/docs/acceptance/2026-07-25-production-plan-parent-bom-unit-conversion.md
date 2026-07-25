# PR-553 生产计划按规格换算并继承父商品 BOM 验收记录

## 范围

- 订单冻结销售规格到库存单位的生产数量换算。
- 生产计划不再从规格显示名称提取数字。
- 具体 SKU BOM 优先；具体 SKU 完全无 BOM 时继承父商品 BOM。
- 计划和工单冻结具体 SKU、父商品、规格换算、计划库存数量、BOM 来源和 BOM 版本。
- 开发环境受控纠正“如目达摩”错误的历史回填损耗；生产环境不部署、不写入。

## 核心场景

- 订单：SO-20260725-0001 同结构回归数据。
- 商品：如目达摩具体 454g SKU，父商品库存单位 Kg。
- 冻结换算：`1件 = 0.454Kg`。
- 需求：`4件 × 0.454Kg/件 = 1.816Kg`。
- BOM：父商品基准产出 1Kg；无整体预期损耗、无物料损耗时按 1.816 倍展开，计划原料合计 1.816Kg。

## 自动化证据

- RED（前端）：`node --test src/lib/produce-plan.test.js` 因缺少 `productionPlanItemQuantitySummary` 和 `productionPlanItemBomSourceLabel` 导出失败。
- GREEN（前端定向）：`node --test src/lib/produce-plan.test.js`，40/40 通过。
- RED（支持合同）：`go test ./internal/interfaces/http/support -run '^TestDev553' -count=1` 同时证明文档/需求种子缺失且 `unprod_summary.go` 仍从 `oi.spec` 正则取数。
- GREEN（订单写入）：`go test ./internal/infrastructure/postgres/orderbeans ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/customerportal ./internal/infrastructure/postgres/customerfulfillment -count=1` 通过。
- GREEN（生产单元）：`go test ./internal/domain/production ./internal/application/production ./internal/infrastructure/postgres/production -count=1` 通过。
- GREEN（真实 PostgreSQL API）：设置临时 `ORDERAPP_TEST_DATABASE_URL` 后执行 `go test ./internal/interfaces/http/production -count=1`，全包通过；覆盖订单结构数据、生产计划、提交和工单冻结闭环。
- GREEN（历史快照并存）：同一具体 SKU 构造两个冻结父商品/换算快照，真实 PostgreSQL API 验证分别生成计划行与工单，并验证共享成品库存只分配一次；订单状态请求按 `商品 + 冻结规格 + 订单号` 三元组绑定。
- GREEN（并发）：`TestProductionPlanAPIConcurrentCreatePlansDemandOnlyOnce -count=10` 通过；每轮一个请求成功、一个请求在锁后重读返回无可计划需求，只生成一个计划和一个计划行。
- GREEN（入口边界）：orderbeans、sales、customerportal、customerfulfillment 四包通过；商城价与零价直发使用当前 SKU 权威换算，真实价格表订单保留发布快照。
- GREEN（历史兼容）：flat/nested `inventory_conversion_json`、重量单位别名及大小写定向测试通过；BOM 产出单位为 lb 时固定用量按 `453.59237g/lb` 折算。
- GREEN（迁移幂等）：同一临时 PostgreSQL schema 上执行生产 schema 定向测试 `-count=2`，通过。
- GREEN（完整后端）：`scripts/verify_kferp.sh backend` 通过。
- GREEN（构建）：`npm run build` 通过。
- 基线说明（完整前端）：功能分支 804/811，7 个失败与同一提交的干净 `origin/develop` 基线完全一致；本次新增定向测试全部通过。
- development 部署和安全冒烟证据：集成完成后补充。

## 必测异常

- 订单冻结换算缺失，且同一具体 SKU 当前权威换算也不可用。
- 销售规格与库存单位维度不兼容。
- 具体 SKU 存在 BOM 配置，但配置失效、无已发布版本、无组件或无工艺路线。
- 父商品失效、父商品多个有效 BOM 且未设置默认、父 BOM 无明细或无工艺路线。
- 同一 SKU 在换算或父商品归属调整前后的合法冻结快照并存；并发请求同时选择同一需求。
- 任一失败必须回滚整个生产计划创建事务，不产生计划行、工单、库存或日志残留。

## 数据纠错边界

- 不修改 V002、现有草稿、历史生产计划、工单、库存、生产日志和历史价格快照。
- 新建纠错 BOM 版本时复制当前已发布配方与工艺路线，只把整体产出率设为 100%，再作为新版本发布。
- 父商品和 454g 子 SKU 只有在当前值仍为 20% 且来源仍为 `legacy-backfill` 时才清零；前置值不同则停止并输出待确认数据。
- 所有开发数据写入必须通过正式 API/服务并进入操作日志。
- 部署和自动化不得为真实 SO-20260725-0001 创建生产计划；由 Van 手工触发。

## 交付状态

- 功能分支：`codex/pr553-production-plan-parent-bom-unit-conversion`
- development：待部署。
- 开发数据纠错：预检发现当前 V002 未绑定工艺路线；纠错版本必须复制有效路线，但现状没有可复制值。获得明确路线选择前不写入。
- production：明确不部署。
