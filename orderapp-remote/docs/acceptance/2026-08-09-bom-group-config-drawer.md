# PR-586 生产 BOM 分组模板配置抽屉化

## 背景
生产 BOM 列表顶部直接平铺"分组模板选择"（checkbox + 保存）和分组动作控件，占用空间大、视觉杂乱。仓库库存页已改为"点击设置分组模板打开抽屉选择"的紧凑模式，生产 BOM 应与之一致。

## 改动
- `BomView.vue`：删除列表顶部内联的 `feature-group-selection` 模板选择块，移入 `productionBomGroupFeatureDrawerOpen` 抽屉。
- `BusinessGroupControls` 增加 `#extra-actions` 插槽放"设置分组模板"入口；未选模板时空状态提供"设置分组模板"+"维护分组模板"两个按钮。
- `saveProductionBomFeatureSelection` 保存成功后关闭抽屉并重置选择/折叠状态。
- 新增 `.bom-group-empty-actions` flex 布局，空状态按钮单行排列。
- 手册 `OP_MANUAL_PRODUCTION.md` 同步"点击设置分组模板打开抽屉"交互。

## 验证
- 前端契约测试 `bom.test.js` 新增"opens from a drawer like warehouse inventory, not inline"：RED（无抽屉状态）-> GREEN。
- `feature-group-selection-ui.test.js` 仍通过（模板选择标记移入抽屉，源码仍含 `data-feature-key="production_bom"` 等）。
- 全量前端 908/908 通过；`npm run build` 通过。
- Go 契约 TestDev450/TestDev453/TestDev458 通过（BOM 分组标记未变）。

## 影响
仅前端交互，无 API/数据/权限变更。用户配置 BOM 分组模板的入口从列表顶部平铺改为抽屉，低频操作收起，列表顶部更紧凑。
