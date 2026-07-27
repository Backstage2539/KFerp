# PR-558 Production Plan Preview Parent BOM No Loss

## 业务口径

- 生产计划预览只计算本次选择形成的精确 `unplanned` 需求。相同具体 SKU、父商品和规格的已排产旧订单不会再次进入当前物料汇总。
- 具体 SKU 完全没有 BOM 时继续继承订单冻结父商品当前已发布 BOM。BOM 有有效物料但缺工艺路线时，预览可以读取配方与损耗并给出路线警告；正式创建草稿仍必须拒绝。
- BOM 原料损耗为0时，投入等于订单冻结成品需求。初晓14件454g按 `14 × 0.454Kg = 6.356Kg`，真实组件预计消耗合计为6356g，不读取旧 `yield_rate=0.8` 或商品 legacy 损耗。

## RED

- 点击待生产需求 `如目达摩454g / SO-20260727-0001` 时，预览返回 `product BOM not configured: 如目达摩 454g`，没有正确使用父商品已发布 BOM。
- 初晓14件454g的冻结需求为6356g、BOM没有配置原料损耗，但旧预览显示预计消耗9932g。
- `MaterialPlan` 重新读取同 SKU 汇总后会把已经进入旧生产计划的订单与本次待计划新订单混合，导致物料数量重复。
- 第一版修复会把完全没有 formal BOM 的历史 `product_bom` 物料库存建议跳过；修正后又发现，显式绑定未发布 formal BOM 时可能错误展示另一个 published 版本的配方。两项均先由回归测试复现。

## GREEN 验收目标

- `TestProducePlanSummaryAPIIgnoresInProductionSiblingDemandForParentBomMaterialPlan`：同 SKU 已提交旧订单与新待计划订单同时存在时，计划行只保留新订单，物料需求不包含旧订单数量。
- `TestProducePlanSummaryAPIPreviewsNoLossParentBomMaterialsWhenRouteMissing`：初晓具体 SKU 继承父商品已发布无损耗 BOM；即使路线缺失，预览仍返回三条真实组件、合计6356g、损耗0和明确路线警告。
- 同一 API 用例继续调用 `POST /api/production-plans`，确认缺路线返回400且没有生产计划、计划行、工单、工序卡、WIP或库存写入。
- `TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials`：完全没有 formal BOM、确有历史 `product_bom` 的商品继续合并 WIP、原料仓、建议领料与采购建议。
- `TestProducePlanSummaryAPIDoesNotReplaceInvalidFormalBomWithAnotherRecipe`：显式绑定 draft formal BOM，同时存在另一 published 版本和历史配方时，只显示配置错误，物料与比例预览不展示任何替代配方。
- 原有父 BOM 18%损耗、同 SKU 不同冻结父商品、损耗只应用一次和生产计划草稿冻结回归继续通过。

## 验证命令

```bash
go test ./internal/interfaces/http/production -run '^(TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials|TestProducePlanSummaryAPI(IgnoresInProductionSiblingDemandForParentBomMaterialPlan|PreviewsNoLossParentBomMaterialsWhenRouteMissing|DoesNotReplaceInvalidFormalBomWithAnotherRecipe))$' -count=1
go test ./internal/interfaces/http/support -run '^TestDev558ProductionPlanPreviewParentBomNoLossContracts$' -count=1
scripts/verify_kferp.sh changed
```

## 数据影响与发布边界

- 不新增数据库字段，不修改订单、BOM、历史计划、工单、库存或生产日志。
- 预览的路线警告不代表可以生成草稿；正式创建继续执行严格路线校验。
- 合并目标为 `develop`，部署目标仅为 development；production 不部署。

## 当前状态

- PR/DEV、需求、验收、生产操作手册和支持合同已登记。
- 业务 RED/GREEN已完成；生产相关临时 PostgreSQL 套件、支持合同、`scripts/verify_kferp.sh changed` 和全仓后端 verifier 通过。
- 独立复审最终结论：P0/P1/P2/P3均无；确认 legacy兼容、无效 formal BOM隔离、缺路线预览/正式创建边界与0%损耗口径。
- 功能提交 `48af54f5`，develop集成提交及实际部署提交 `c1c61ccde7a233951b73d2e372a12c8663c7ebe8`。
- development部署成功；服务器备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260727214503`。容器内全仓Go测试、Vue/Vite和小程序构建通过，orderapp容器健康且近5分钟日志无错误。
- 页面HTTP冒烟：`/app/`返回303、`/app/vue-shell`返回200、`assets/index-Bkc2703T.js`返回200（2135839 bytes）；PR-558 requirements API返回200。
- 如目达摩只读预览 `selected=789-454` 返回200，只包含 `SO-20260727-0001`，`input_g=6356`、克重物料合计6356g且不再报 `product BOM not configured`。
- 初晓只读预览 `selected=765-454` 返回200，`input_g=6356`、三条克重物料合计6356g，并保留 `最新可用 BOM 版本未配置工艺路线: 初晓 生产 BOM/V002/初晓 454g`。
- 交互式浏览器页面冒烟被 development 证书链 `ERR_CERT_AUTHORITY_INVALID` 阻止；遵守浏览器安全边界未绕过证书提示。服务端、页面资源和业务API均已完成只读验证；production未部署、未写入真实订单/BOM/计划/库存。
