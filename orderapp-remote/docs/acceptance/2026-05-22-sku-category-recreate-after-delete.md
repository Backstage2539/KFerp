# 2026-05-22 SKU 分类删除后同名重建

## 需求
- PR-311-SKU-CATEGORY-RECREATE-AFTER-DELETE：客户 SKU 归属下删除一级或二级商品分类后，允许重新新增同名分类。
- 软删除历史分类仍保留审计追溯，但唯一约束只约束 `active=true` 的分类。

## 根因
- 分类删除接口把 `product_categories.active` 置为 `false`，没有物理删除历史记录。
- 现网曾创建过同名 `product_categories_customer_parent_name_uniq` 索引；启动迁移使用 `CREATE UNIQUE INDEX IF NOT EXISTS`，不会修正旧索引定义。
- 旧索引如果不是 `WHERE active=true` 的部分唯一索引，就会让软删除分类继续占用同名位置，导致新增同名一级分类时报 duplicate key。

## 修复与证据
- `catalog.EnsureSchema` 启动迁移先 `DROP INDEX IF EXISTS product_categories_customer_parent_name_uniq`，再重建为 `WHERE active=true` 的部分唯一索引。
- 单元测试：`go test ./internal/infrastructure/postgres/catalog -run TestProductCategoryNameUniquenessIgnoresSoftDeletedRows -count=1`
- API/支持验证：`go test ./internal/interfaces/http/support -run TestDev311SkuCategoryRecreateAfterDelete -count=1`

## 验收
- [ ] 在 SKU设置 选择芬纳咖啡客户归属。
- [ ] 新增一级分类，例如“测试分类”。
- [ ] 删除该一级分类，确认分类内商品回到未分类。
- [ ] 再次新增同名“测试分类”，页面应提示保存成功，不再显示 `product_categories_customer_parent_name_uniq` duplicate key。
