# PR-538 物料成本计价单位与生产 BOM 损耗口径修正验收

## 范围与环境边界

- 需求：`PR-538-MATERIAL-COST-UNIT-LOSS`
- 开发项：`DEV-538-MATERIAL-COST-UNIT`、`DEV-538-BOM-DEFAULT-LOSS`、`DEV-538-TRIAL-COST-LOSS-CLARITY`、`DEV-538-DOCS-DEPLOY`
- 目标环境：development。
- 生产环境未部署、未写入、未切换入口；未取得 Van 的单独授权前，禁止把本修复部署或修数到生产环境。
- 开发数据修复对象：“榛巧拼配”。修数前开发数据库备份：`/opt/stacks/erp/backups/kferp-dev-before-pr538-20260717-112234.dump`，大小 `12446065` bytes，SHA-256 `175afc127d534ec14bc808ff56fede03939c427286972a83debeac5bc4423e27`。

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
- [x] 功能提交 `9e5a8c48` 已合并为 development 提交 `05badaa3c7f4ab0eb10be805b28f6a96a7ca79df`，并通过 `./deploy_orderapp.sh development` 部署；Vue、miniapp、Docker 内 `go test ./...` 和容器启动均通过。部署前应用备份：`/opt/stacks/erp/orderapp.backup.deploy-20260717112508`。

## 开发环境数据修复与 API 验收

- [x] 修数前完成 development 数据库备份；文件、大小和校验值见“范围与环境边界”。
- [x] 通过正式 API 从 V003/1396 复制草稿 V004/1400，完整保留原料 7/14/26、比例 60%/20%/20%、原料损耗 20%、产出 `1kg`、标准烘焙路线 4 和商品特殊属性；将整体预期损耗改为 0 后发布。BOM 8375 的最新可用版本和商品 658 默认绑定均为 V004/1400。
- [x] 商品生产配置已保存为 `production_bom_id=8375`、`production_bom_version_id=1400`、`process_route_id=4`、`expected_loss_rate=0`。审计日志 7024–7029 记录 actor `order` 的版本创建、草稿更新、特殊属性更新、发布、默认 BOM 绑定和商品生产配置保存。
- [x] 物料 API/试算返回三种原料 `cost_unit=kg`，明细显示 `54/kg`、`78/kg`、`82/kg`，不再显示 `/g`。
- [x] 认证试算 `POST /api/costing/pricing-rule-trial` 返回 V004/1400：三行折算成本 `40.50元/kg`、`19.50元/kg`、`20.50元/kg`，BOM 物料成本 `80.50元/kg`，标准工序 `2.04元/kg`，标准制造成本及 `base_cost` 均为 `82.54元/kg`。
- [x] 发布 V004 前保存的 V003 默认试算仍为物料 `100.64`、工序 `2.04`、合计 `102.68元/kg`，并返回包含“整体预期损耗 20%”“原料损耗 20%”“连续放大”和修正建议的警告，证明历史显式双损耗口径未被静默改写。发布新版后 V003 按版本规则归档，不再进入可试算候选，显式指定旧版本返回 `production BOM version not found for product`。
- [x] 开发容器内认证 API 烟测通过：BOM、商品生产配置、物料和价格试算响应字段均与上述结果一致；PR API 返回 `PR-538-MATERIAL-COST-UNIT-LOSS` 状态 `review`。
- [ ] 可见页面烟测：Chrome 与 Codex 应用内浏览器访问 `https://dev.erp.qacoohee.com/vue-shell?view=productPriceManagement` 均被开发域名证书拦截（`ERR_CERT_AUTHORITY_INVALID`）。未绕过证书告警，因而本次没有页面截图；这是环境证书阻塞，不影响已通过的容器内认证 API 结果。

## 部署与环境隔离证据

- development：`erp_orderapp` 已由本次部署重建并正常运行，`erp_postgres` 保持 healthy。
- production：未部署、未执行数据写入、未切换公网入口。核对时 `erp_prod_orderapp` 创建于 `2026-07-12T16:23:24Z`、启动于 `2026-07-12T16:23:25Z`，`erp_prod_postgres` 创建于 `2026-05-03T07:39:18Z`，均早于本次开发部署且持续运行。

## 手册与需求证据

- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`

## 验收结论

- 当前状态：代码、全量校验、development 部署、榛巧拼配 V004 修复、认证 API 和审计验收完成；仅开发域名证书导致可见页面烟测阻塞，等待 Van 验收。
- 验收人：Van / 待验收。
