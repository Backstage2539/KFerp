# PR-601 BOM 规格模板 UI 打磨 验收证据（2026-08-18）

## 范围
1. 稳定规格键隐藏 + 内部 spec-N 生成
2. 默认规格复选框缩小
3. 损耗比例删除、只支持用量、主投入用量改名规格用量
4. 物料名去 SKU- 前缀（前端 label + 后端补 materials.code）
5. 规格组选择器与自动回填表单对齐

## 自动化证据
- 前端：`node --test src/lib/*.test.js` 997/997 全绿（bom.test.js 37 项含 4 项 PR-601 新合同）
- Go：`go test ./...` 74 包全绿（含 TestDev601BomSpecUiPolishContracts / TestDev601MaterialOptionLabelNeverFabricatesSkuPrefix）
- 构建：`vite build` ✓ built in 1.90s
- 兼容：PR-593 合同改用新文案 `历史比例配方中所有组件消耗单位必须为比例 %`（校验逻辑保留）

## 遗留兼容口径
- 已发布比例配方与损耗数据只读保留；`materialLossRateDisplay` 仅展示
- 新草稿消耗单位下拉无 `比例 %`；比例模式仅历史草稿/发布版本可进入
- 后端 `ratio_pct` / `material_loss_rate` 字段与成本试算比例逻辑零改动

## 人工验收（待 Van 在 development）
按 docs/ACCEPTANCE_TESTS.md PR-601 清单执行。
