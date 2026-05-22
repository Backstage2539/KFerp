# 2026-05-22 SKU 分类删除后同名重建

## 需求
- PR-312-SKU-CATEGORY-RECREATE-AFTER-DELETE：客户 SKU 归属下删除一级或二级商品分类后，允许重新新增同名分类。
- 软删除历史分类仍保留审计追溯，但唯一约束只约束 `active=true` 的分类。

## 根因
- 分类删除接口把 `product_categories.active` 置为 `false`，没有物理删除历史记录。
- 现网曾创建过同名 `product_categories_customer_parent_name_uniq` 索引；启动迁移使用 `CREATE UNIQUE INDEX IF NOT EXISTS`，不会修正旧索引定义。
- 旧索引如果不是 `WHERE active=true` 的部分唯一索引，就会让软删除分类继续占用同名位置，导致新增同名一级分类时报 duplicate key。
- 索引修复后，芬纳咖啡又出现一个 active 的客户自有空分类“咖啡豆”；前端分类树把空的客户自有分类误判为重复公共分类并隐藏，导致页面显示 0 个分类，操作员再次新增同名分类时仍会撞 active-only 唯一约束。

## 修复与证据
- `catalog.EnsureSchema` 启动迁移先 `DROP INDEX IF EXISTS product_categories_customer_parent_name_uniq`，再重建为 `WHERE active=true` 的部分唯一索引。
- `buildSkuContextCategoryTree` 保留空的客户自有分类，即使分类名与公共分类同名；只有实际带公共 SKU 别名/子分类内容的 legacy 公共复制分类继续按重复公共分类隐藏。
- 单元测试：`go test ./internal/infrastructure/postgres/catalog -run TestProductCategoryNameUniquenessIgnoresSoftDeletedRows -count=1`
- 前端单元测试：`node --test src/lib/product-settings.test.js`
- API/支持验证：`go test ./internal/interfaces/http/support -run TestDev312SkuCategoryRecreateAfterDelete -count=1`

## 验收
- [ ] 在 SKU设置 选择芬纳咖啡客户归属。
- [ ] 新增一级分类，例如“测试分类”。
- [ ] 删除该一级分类，确认分类内商品回到未分类。
- [ ] 再次新增同名“测试分类”，页面应提示保存成功，不再显示 `product_categories_customer_parent_name_uniq` duplicate key。
- [ ] 如果同名 active 空分类已经存在，刷新 SKU设置 后分类树应显示该分类，而不是继续显示“商品分类 0”。
