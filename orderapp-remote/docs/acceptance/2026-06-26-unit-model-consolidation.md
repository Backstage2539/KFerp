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

## 2026-06-27 Follow-up：商品档案库存单位驱动 BOM 产出单位

### 范围
- 创建新商品档案和商品档案配置抽屉补齐 `库存单位` 与 `整数库存`。
- 商品库存单位写入 `products.unit_rule_override_json.inventory_unit`，读取兼容历史商品配置/分类和 `integer_unit`。
- 生产 BOM 新建/编辑只读展示产出单位，后端保存时按产出商品有效库存单位写入，不信任前端 `output_unit`。
- `/api/bom/products` 返回 `inventory_unit_explicit`，商品缺少显式库存单位时 BOM 页面提示先到商品档案设置。
- 历史已发布 BOM 版本不回改；详情中提示当前版本单位和商品档案库存单位差异。

### RED
- `go test ./internal/interfaces/http/catalog ./internal/application/bom -run 'TestProductInventoryUnitAPIContract|TestUpdateProductionBomDerivesOutputUnitFromProductInventoryUnit' -count=1`：`CreateProductCommand` 缺少 `UnitRuleOverrideJSON`，`UpdateProductionBomCommand` 缺少 `OutputUnit`。
- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`：商品新增/配置 payload 缺少 `inventory_unit`/`integer_inventory_unit`，BOM 表单缺少来源文案和历史单位差异提示。

### GREEN
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1`
- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`
- `npm ci`
- `npm run build`：通过，保留既有 large chunk 警告。
- `scripts/verify_kferp.sh changed`
- `git diff --check`

### 部署后验收
- Development deployed from `91658fca0e16d12d14ff7c3ba69ac4dc55ed9823`; Docker build ran `go test ./...` and passed. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260627010939`.
- Authenticated smoke: `/app/api/product-settings?limit=1`、`/app/api/bom/products`、`/app/api/production-boms?status=all&limit=1` all returned 200. BOM detail includes version `output_unit`; BOM products include `inventory_unit` and `inventory_unit_explicit`.
- Write smoke created product `558` with `inventory_unit=盒` and `integer_inventory_unit=true`; creating production BOM `5736` with request `output_unit=kg` still saved the draft version `output_unit=盒`.
- Remaining manual UI click-through: browser plugin was not available in this session, so the final UI click path should be spot-checked from 商品档案 -> 生产 BOM if a visual browser sign-off is required.
