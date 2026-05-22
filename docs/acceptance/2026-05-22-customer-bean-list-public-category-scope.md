# 客户豆单公共分类范围验收

## 需求
- `PR-318-CUSTOMER-BEAN-LIST-PUBLIC-CATEGORY-SCOPE`
- 产品豆单在客户范围下必须遵守 SKU设置 的“是否使用公共商品分类”开关。

## 验收步骤
1. 在 SKU设置 选择芬纳咖啡，关闭“是否使用公共商品分类”。
2. 进入客户账户模式的产品豆单，确认豆单范围为芬纳咖啡。
3. 检查商用批发、零售、生豆、挂耳豆单预览和“生成豆单”抽屉的产品候选。
4. 再回到 SKU设置 开启“是否使用公共商品分类”，刷新产品豆单。

## 通过标准
- 关闭公共商品分类时，产品豆单只展示芬纳咖啡自己的 SKU。
- 关闭公共商品分类时，公共分类、公共 SKU 和公共豆单内容不进入客户豆单预览或生成候选。
- 开启公共商品分类后，公共分类下公共 SKU 才作为只读引用进入客户豆单范围。
- 客户豆单发布仍保存到当前客户归属，不影响公共豆单。

## 验证证据
- `node --test src/lib/bean-list-pdf.test.js src/lib/product-bean-list-split.test.js`
- `go test ./internal/interfaces/http/support -run TestDev318CustomerBeanListPublicCategoryScope`
