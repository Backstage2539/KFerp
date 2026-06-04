# PR-412 分类模板商品配置模板继承

## 范围
- 分类模板和分类项不再直接引用阶梯价模板、单位模板。
- 分类模板和分类项改为引用商品配置模板。
- 成本和商品价格表读取顺序为：客户商品模板 > 商品档案模板 > 分类项模板 > 分类模板模板 > 旧兼容字段。

## 验收点
- 商品配置和分类模板页面中，分类模板编辑区显示“模板默认商品配置模板”。
- 分类项编辑区显示“分类项商品配置模板”，并提示商品单独选择商品配置模板时会覆盖分类配置。
- 保存分类模板、保存分类项时 API 接受并返回 `product_config_template_id`，操作日志保留该字段。
- 商品价格表和成本输入能从分类项或分类模板引用的商品配置模板读取计价方式、阶梯价模板、固定单价、成本加成和单位模板。
- 旧分类直接阶梯价模板、单位模板字段只作为兼容读取，不作为新 UI 写入入口。

## RED
- `node --test src/lib/product-settings.test.js` 先失败于旧分类模板仍展示直接阶梯价/单位模板，以及覆盖提示 helper 不存在。
- 目标 Go 测试先失败于 catalog 命令缺少 `ProductConfigTemplateID`、schema/repository 未持久化该字段、costing SQL 未读取分类配置模板、support seed 缺少 PR-412。

## GREEN
- `node --test src/lib/product-settings.test.js`：通过 108/108。
- `go test ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -run 'TestProductClassificationTemplateAPIsSaveCategoriesAndAssignments|TestProductClassificationTemplatesPersistProductConfigTemplateReferences|TestLoadProductInputsUsesClassificationConfigTemplatesBeforeLegacyFallback|TestDev412ClassificationConfigTemplateInheritanceSeeds' -count=1`：通过。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/view-routing.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js`：通过 178/178。
- `go test ./...`：通过。
- `npm run build`：通过，保留既有 Vite chunk-size warning。
- `scripts/verify_kferp.sh changed`：退出码 0。
