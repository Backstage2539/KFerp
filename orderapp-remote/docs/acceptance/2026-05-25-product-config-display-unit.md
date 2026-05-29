# PR-372 商品配置移除价格表展示单位验收记录

## 范围
- SKU设置 → 商品配置 → 商品配置模板。
- 移除固定“展示方式”和商品配置里的独立“价格表展示单位”。

## 验收点
- 商品配置模板不再出现“盒装/箱装展示”“按重量展示”等场景化展示方式。
- 商品配置模板不再出现“价格表展示单位”字段。
- 产品价格表展示单位继承阶梯价模板的展示单位；需要不同单位时复制或新建阶梯价模板。
- 保存后的价格表规则不再写入新版 `display_unit`；旧 `display_mode/display_unit` 读取时只做兼容。
- 盒装、箱装、重量展示都通过阶梯价模板、单位模板和 BOM 用量表达，不再由商品配置写死业务场景。

## 证据
- 单元：`node --test src/lib/product-settings.test.js`
- API：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates -count=1`
- 支持/种子：`go test ./internal/interfaces/http/support -run TestDev372 -count=1`
- 前端构建：`npm run build`（在 `orderapp-remote/frontend-vue-shell`）
- 手册：`orderapp-remote/docs/OP_MANUAL_COSTING.md`
