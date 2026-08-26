# 价格表运行态规格身份与同单位发布验收（PR-612）

## 现场问题

- development：发布当前“初晓-商品 / 454g袋”价格表时，运行态行保存 `product_id=1063,parent_product_id=1063,sku_id=7`，已选规格实际为 `bom_spec_id=7,bom_variant_id=422`，服务端报“第1行 SKU 7 未在规格选择中”。
- production：发布“曜石2.0 / 1Kg”价格表时，试算已经输出 `price_unit=袋,inventory_unit=袋`，服务端仍要求旧商品档案提供“袋→袋”换算。

## TDD 证据

- RED：Go 精确复现“分组商品项 SKU 7 未在规格选择中”和“价格单位：袋，库存单位：袋但缺少换算”；Vue 精确复现 BOM 规格 7 留在 `sku_id` 未被归一。
- GREEN：前后端均按父商品下的精确 BOM 规格 ID 归一；相同价格/库存单位固化 1:1 快照并保留 legacy SKU 身份。
- 防回归：父商品占位且多规格无法唯一判断时仍阻断；不同价格/库存单位且缺少商品档案换算时仍阻断。

## 环境验收

- [x] 全量 Go/Vue/Vite 和 `scripts/verify_kferp.sh all` 通过（Vue 1040/1040；Vite 6594 modules）。
- [ ] 合入 development 并部署后，在已登录开发页面重新生成预览并实际发布，确认不再出现“SKU 7 未在规格选择中”，发布版本和操作日志可追溯。
- [ ] 合入 main 并部署 production 后，登录页、受保护 API 和价格表只读接口健康；生产不由自动化点击发布，不改生产价格表业务数据。
