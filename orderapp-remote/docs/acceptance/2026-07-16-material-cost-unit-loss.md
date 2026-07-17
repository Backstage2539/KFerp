# PR-538 物料成本计价单位与生产 BOM 损耗口径修正验收

## 范围与环境边界

- 需求：`PR-538-MATERIAL-COST-UNIT-LOSS`
- 开发项：`DEV-538-MATERIAL-COST-UNIT`、`DEV-538-BOM-DEFAULT-LOSS`、`DEV-538-TRIAL-COST-LOSS-CLARITY`、`DEV-538-DOCS-DEPLOY`
- 目标环境：development。
- 生产环境未部署、未写入、未切换入口；未取得 Van 的单独授权前，禁止把本修复部署或修数到生产环境。
- 开发数据修复对象：“榛巧拼配”。修数前开发数据库备份路径、时间和校验值：待部署阶段补录。

## 问题复现与根因证据

- 修复前“榛巧拼配”的发布 BOM 同时包含 `yield_rate=0.8`（整体预期损耗 20%）和 `material_loss_rate=0.2`（原料损耗 20%）。
- 原料净成本为 `54×60% + 78×20% + 82×20% = 64.40元/kg`。原料损耗放大后为 `64.40÷0.8 = 80.50元/kg`；再被整体预期损耗放大后约为 `100.63元/kg`，按行分摊取整显示 `100.64元/kg`；加标准工序 `2.04元/kg` 后得到修复前页面的 `102.68元/kg`。
- 新建生产 BOM 的后端默认 `yield_rate=0.8`，而当前创建页面没有让用户明确配置整体预期损耗，导致隐藏的第二次 20% 放大。
- 物料表此前只有库存单位 `unit` 和采购价，价格试算把库存单位当成成本单价单位，因而库存单位为 `g` 时错误显示 `54/g`；实际历史成本口径为 `54元/kg`。

## TDD 证据

### RED（实现前）

- 物料仓储/物料 API 新增合同测试因 `CostUnit` 字段不存在而编译失败。
- 物料 schema 合同测试因缺少 `cost_unit` 列、默认值和历史回填而失败。
- 商品价格试算仓储测试因 SQL 仍读取 `m.unit`、没有读取 `m.cost_unit` 而失败。
- 价格试算双损耗测试因未返回 `整体预期损耗 20%`、`原料损耗 20%`、`连续放大` 警告而失败。
- 新建生产 BOM 默认值测试因仍使用 `yieldRate := 0.8` 而失败。
- 物料 Vue 源码合同测试因没有 `成本计价单位`、动态采购价单位和锁定说明而失败。

### GREEN（实现后定向验证）

- [x] `go test ./internal/infrastructure/postgres/materials ./internal/interfaces/http/materials -count=1`：2 个包通过。
- [x] `go test ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/costing ./internal/application/costing -count=1`：3 个包通过。
- [x] `go test ./internal/interfaces/http/support -run TestDev538MaterialCostUnitLossContracts -count=1`：通过，PR/DEV/REV 种子、实现标记、根/docs 镜像、三份手册和本验收文件合同均满足。
- [x] `node --test src/lib/materials-ui.test.js`：12/12 通过，覆盖物料成本计价单位、保存后锁定以及入库/调整/采购价格单位展示。
- [x] `npm run build`：Vite build 通过，401 modules transformed；仅保留既有 chunk size warning。
- [x] 独立审查后的边界修正：旧采购单接口只列重量物料，旧批次成本调整只允许重量批次；离散物料继续通过原料入库/数量补录按库存单位处理，避免“数量 g、成本元/个”的错账。
- [x] `git diff --check`：通过。
- [x] `go test ./...`：全部 Go 包通过；`scripts/verify_kferp.sh changed`：空白、冲突标记与 diff 检查通过。
- [ ] 记录最终 commit、development 部署版本和 live API/页面证据。

## 开发环境数据修复与 API 验收（待部署后补录）

- [ ] 先备份 development 数据库，记录备份文件、时间、大小和校验值。
- [ ] 通过正式 API/服务复制“榛巧拼配”当前 BOM 为新草稿，保持原料比例 60%/20%/20%、原料损耗 20% 和标准烘焙工序快照，把整体预期损耗设为 0；发布新版并切换默认版本。
- [ ] 商品生产配置的整体预期损耗修正为 0；BOM 新版本、发布、默认切换和生产配置保存均可在操作日志查询。
- [ ] 物料 API 返回三种原料 `cost_unit=kg`，试算明细分别显示 `54/kg`、`78/kg`、`82/kg`，不显示 `/g`。
- [ ] 商品价格试算三行折算成本为 `40.50元/kg`、`19.50元/kg`、`20.50元/kg`，BOM 物料成本 `80.50元/kg`，标准工序成本 `2.04元/kg`，标准制造成本 `82.54元/kg`，不再显示 `102.68元/kg`。
- [ ] 构造同时显式配置两类 20% 损耗的测试版本，确认计算仍连续放大，并返回包含 `整体预期损耗 20%`、`原料损耗 20%`、`连续放大` 和核对建议的警告。
- [ ] development 页面、认证 API 和价格试算烟测结果（URL、状态码、响应关键字段、截图）补录到本节。

## 手册与需求证据

- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`

## 验收结论

- 当前状态：根因、RED 和定向 GREEN 证据完成；全量校验、development 部署、数据修复和 live 浏览器/API 验收待补录。
- 验收人：Van / 待验收。
