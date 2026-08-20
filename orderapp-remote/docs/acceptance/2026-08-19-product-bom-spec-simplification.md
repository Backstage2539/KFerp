# PR-603 商品规格归一 BOM 与物料损耗恢复 验收证据（2026-08-19）

## 范围
1. 商品档案不再选择规格模板，直接引用默认制造 BOM 的规格（只读展示），删除逐规格 SKU 维护面
2. 物料产出 BOM 恢复原料损耗配置（商品规格组保持纯固定用量）
3. BOM 产出从商品改为物料时删除规格组

## 自动化证据
- 前端：`node --test src/lib/*.test.js` 1000/1000 全绿（bom.test.js 45 项含 PR-603 新合同 3 项；product-settings/product-spec-cutover 合同更新）
- Go：`go test ./...` 全部通过（含 TestDev603ProductBomSpecSimplificationContracts；dev_601/dev_366 合同按 PR-603 口径同步）
- 真实 PostgreSQL：TestUpdateProductionBomToMaterialOutputClearsDraftSpecGroupPostgres RED（variants=2 残留）-> GREEN（转换清空 + 平铺保存 + 发布成功）；bom/costing 包全绿
- 构建：`vite build` ✓ built in 1.71s

## 关键实现
- BomView：isMaterialOutputBom 范围恢复损耗 UI/状态/校验；saveProductionBomRecord 在 outputChangedToMaterial 时先 PUT 再发空 variants 清规格组；syncBomOutputType 切物料时提示
- ProductSettingsView：删除批量模板设置/模板下拉/SKU 明细/默认规格按钮等 7 类遗留面；productProductionBomSpecs 不再限定 cutover；新增 productProductionBomSpecsSummary 汇总展示
- 后端 UpdateProductionBom：identityChangedToMaterial 时事务内对草稿版本调用 saveProductionBomDraftVariantsTx(nil) 清规格组（含 remove_from_draft 审计，规格身份保留）
- lib payload：unit_template_id 保留原值冻结传递（后端沿用 existing，不再经 UI 修改）

## 兼容口径
- 旧商品销售规格模板面板（迁移期）经菜单"单位模板"仍可维护；cutover 迁移面板不变
- 已发布 BOM 产出身份仍不可变（原有守卫）
- 历史比例配方与损耗只读兼容不变（PR-601 口径）
