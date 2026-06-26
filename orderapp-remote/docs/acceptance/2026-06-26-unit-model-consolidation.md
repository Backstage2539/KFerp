# PR-500-UNIT-MODEL-CONSOLIDATION 验收证据

## 范围
- 单位模型统一为 `库存单位 / 销售单位 / 单位转换`。
- 物料单位不作为新业务概念；物料档案的历史 `unit` 字段在 UI 中解释为库存单位。
- 报价单位和录单单位只作为历史兼容字段；新 UI 使用销售单位，API 继续双写旧 `quote_unit/order_unit`。
- 原料入库按库存单位录入，提交 `qty/unit_code`，并兼容旧 `qty_g`；重量单位归一到 `qty_g`，非重量库存单位归一到 `qty_units`。
- BOM 产出单位自动取产出商品库存单位；组件区显示组件库存单位。

## RED
- `node --test src/lib/product-settings.test.js`：新增 sales unit payload 期望后，旧 helper 未返回 `sales_unit`，且未双写历史字段。
- `go test ./internal/interfaces/http/stock -run TestStockAPIRoutes -count=1 -v`：`POST /api/stock/material-receipts` 使用 `qty/unit_code` 返回 `qty_g required`。
- `go test ./internal/interfaces/http/stock -run TestVueMaterialReceiptUsesInventoryUnitQuantity -count=1 -v`：原料入库页仍显示 `数量(g)`，未暴露库存单位选择。
- `go test ./internal/application/stock -run TestReceiveMaterialUsesInventoryUnitsForNonWeightQuantity -count=1`：非重量库存单位还没有 `QtyUnits` 入库字段。

## GREEN
- `go test ./internal/interfaces/http/stock -run 'TestStockAPIRoutes|TestVueMaterialReceiptUsesInventoryUnitQuantity' -count=1 -v`
- `go test ./internal/application/stock ./internal/interfaces/http/stock ./internal/infrastructure/postgres/stock -count=1`
- `go test ./internal/application/bom -run 'TestCreateProductionBom' -count=1 -v`
- `go test ./internal/interfaces/http/catalog -run TestProductSettingsAPISupportsGlobalUnitDefinitionsAndTemplates -count=1 -v`
- `go test ./internal/interfaces/http/support -run TestDev500UnitModelConsolidationContracts -count=1`
- `node --test src/lib/product-settings.test.js`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `go test ./...`
- `git diff --check`
- `scripts/verify_kferp.sh frontend-tests`：仍失败 8 个；最小重跑 `node --test src/lib/workspace-context-pages.test.js src/lib/workspace-mode.test.js` 定位到 3 个既有 workspace/customer-context 断言，`origin/develop` 同样缺少相关标记，不属于本次单位模型改动。

## 人工验收
- 设置 -> 单位模板：只看到库存单位、销售单位和单位转换；保存后刷新列表摘要仍显示库存/销售。
- 库存作业 -> 原料入库：选择物料后入库数量和成本字段显示该物料库存单位，提交成功生成批次。
- 库存作业 -> 库存调整：目标库存数量按库存单位录入。
- 生产管理 -> 生产 BOM：新建 BOM 时产出单位随产出商品库存单位自动显示，不能手工填写；组件表单显示组件库存单位。
