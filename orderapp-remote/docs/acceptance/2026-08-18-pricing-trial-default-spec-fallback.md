# PR-602 价格试算默认规格回退 验收证据（2026-08-18）

## 现象
商品价格管理 -> 价格试算选择"初晓"报错：`BOM项目 初晓 成本无法解析：BOM组件成本单位无法换算`

## 根因（dev 库只读 SQL 确认）
1. **RC2（主因）**：初晓（product 619）默认 BOM 版本 V007(1841) 带规格组（2 变体）。PR-600 成本图对带规格组版本删除 `product:619` 节点只留 `product_spec:<id>`；试算未携带 BomSpecID -> 图查不到 -> 全行退回逐行兜底。
2. **RC1（表现）**：兜底路径中"初晓"行是半成品物料（materials.71, is_semi_finished=t），PR-600 强制试算兜底单价=0 -> `productionBomItemCost` 零价 fail-closed -> 报误导性统一错误。
3. **数据缺口（次因，需 Van 处理）**：半成品制造 BOM(18306/V001) 组件"孟连水洗5T批次"（materials.1）采购价 0 且批次加权成本 0，递归解析在材料层 fail-closed。

## 修复
- `production_bom_cost.go`：`versionDefaultSpecKeys` 图别名——带规格组版本的默认规格解析结果以商品键暴露；无 spec 上下文调用按默认规格解析。
- `resolveProductionBomTrialItemCost`：报错细分零单价与单位不匹配（含具体单位名）。
- 半成品 fail-closed 守卫（TestResolvedBomCostDoesNotFallbackToSemiFinishedPurchaseOrBatchCostPostgres）不变，仍全绿。

## 自动化证据
- RED：新增 `TestResolvedBomCostsExposeDefaultSpecificationAsProductFallbackPostgres`（修复前 costs[600] 缺失）+ `TestResolveProductionBomTrialItemCostReportsSpecificReasons`（修复前报笼统错误）均失败。
- GREEN：两项新测试通过；costing 包 95/95 全绿（含全部 PR-600 守卫）；Go 全量无新增失败（catalog/contracts 两项为干净 develop 上同环境预置失败，与本次无关）。

## 数据操作指引（Van）
在物料档案给"孟连水洗5T批次"维护采购价，或通过原料入库/库存调整产生带成本的批次；之后"初晓"试算即可按默认规格（0.227kg 半成品 + 1 袋 0.72 元）算出。
