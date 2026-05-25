# PR-372 商品配置价格表展示单位验收记录

## 范围
- SKU设置 → 商品配置 → 商品配置模板。
- 将固定“展示方式”改为通用“价格表展示单位”。

## 验收点
- 商品配置模板不再出现“盒装/箱装展示”“按重量展示”等场景化展示方式。
- “价格表展示单位”默认使用“继承报价单位”。
- 覆盖展示单位时，选项来自全局单位字典，例如 kg、g、盒、箱、袋、件。
- 保存后的价格表规则写入 `display_unit`；旧 `display_mode=boxed/weight/by_quote_unit` 读取时兼容为继承报价单位。
- 盒装、箱装、重量展示都通过单位模板和单位换算表达，不再由商品配置写死业务场景。

## 证据
- 单元：`node --test src/lib/product-settings.test.js`
- API：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates -count=1`
- 支持/种子：`go test ./internal/interfaces/http/support -run TestDev372 -count=1`
- 前端构建：`npm run build`（在 `orderapp-remote/frontend-vue-shell`）
- 手册：`orderapp-remote/docs/OP_MANUAL_COSTING.md`
