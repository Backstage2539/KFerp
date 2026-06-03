# PR-403 BOM、价格表提示与行业字段模板修正

## 范围
- 商品价格表缺少阶梯价模板时，提示改为“未设置计价方式”，并写清从主菜单到具体下拉项的配置路径。
- 行业字段模板改为左侧模板列表、右侧编辑；模板列表支持搜索模板名和启用/停用/全部过滤。
- 生产 BOM 的新建、状态过滤、搜索、分组 Tab、移动分组卡片放到 BOM 列表与编辑详情共同顶部；BOM 列表独立滚动。
- 修复 `Codex测试豆` 这类旧 BOM 有明细但没有生产 BOM 绑定，导致不能勾选移动分组的问题。

## RED 证据
- `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`
  - 失败原因：测试期望新的“未设置计价方式”提示，旧实现仍返回“未配置阶梯价模板”。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomBackfillRepairsLegacyItemsWithoutBindings -count=1`
  - 失败原因：旧实现没有 legacy BOM 绑定修复标记和修复 SQL。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 失败原因：BOM 页面缺少共同操作区和列表滚动标记；行业字段模板缺少模板搜索、状态过滤和左列表右编辑测试标记。

## GREEN 证据
- `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`
  - 通过。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomBackfillRepairsLegacyItemsWithoutBindings -count=1`
  - 通过。
- `node --test src/lib/bom.test.js src/lib/product-bean-list-split.test.js`
  - 通过。
- `npm run build`（`orderapp-remote/frontend-vue-shell`）
  - 通过；保留既有 chunk-size warning。
- `go test ./internal/domain/costing ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/manufacturing ./internal/interfaces/http/support -count=1`
  - 通过。

## 最终验证
- `go test ./...`（`orderapp-remote`）
  - 通过。
- `scripts/verify_kferp.sh changed`
  - 通过。
- 合并 develop 和 development 部署证据待补。

## 手册
- `OP_MANUAL_COSTING.md`：商品价格表提示改为“未设置计价方式”，并写清 `商品与配方 → 商品配置和分类模板 → 商品配置模板 → 计价方式`。
- `OP_MANUAL_INVENTORY_MATERIALS.md`：补充生产 BOM 共同操作区、BOM 列表滚动窗口、旧 BOM 绑定修复和行业字段模板搜索筛选。
- `OP_MANUAL_PRODUCTION.md`：补充行业字段模板左列表搜索/过滤、右侧编辑的操作口径。
