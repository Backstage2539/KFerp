# PR-382 SKU 统一新增与批量复制

## 范围
- SKU 新增统一为一个入口：当前是公共视图就创建公共 SKU，当前是客户视图就创建该客户 SKU。
- SKU 复制替代公共 SKU 只读引用：从公共 SKU 或其他客户复制成目标客户自己的 SKU。
- 商品分类维护移到 `商品配置 → 商品分类管理`，SKU 列表只保留新增、复制、编辑、启用/停用等日常操作。

## 验收步骤
1. 进入 `SKU设置`，默认公共 SKU 视图，点击 `新增SKU`。确认表单只有 SKU 名称、备注、产品类型、产品子类型；没有产品形态、定制类型、基础产品、复制 BOM 或复制价格梯度。
2. 保存一个公共 SKU，再切到某个履约客户，点击同一个 `新增SKU`。确认表单完全一致，保存后归属为当前客户。
3. 在客户视图点击 `SKU复制`，来源选择 `公共SKU` 或其他客户。确认抽屉顶部显示 `选择分类和产品 X/Y 款`，支持总复选框、`全选` 和 `清空`。
4. 按产品类型、产品子类型和 SKU 行勾选一批 SKU 后点击 `复制SKU`。确认目标客户没有对应分类时会自动补分类结构，完成后提示新增和覆盖数量。
5. 再次复制同一批 SKU，确认同名同分类不重复创建，而是覆盖资料并保留目标 SKU ID；历史订单和已发布价格表快照不被回改。
6. 复制含成品组件 BOM 或生豆绑定熟豆的 SKU，确认目标客户里的 BOM 组件和生豆绑定都指向目标客户同批复制或同名同分类 SKU；若依赖 SKU 未选择且目标客户没有对应 SKU，复制接口必须报错并回滚。
7. 停用某个来源 SKU 后重新打开复制抽屉，确认该 SKU 显示已停用且不可选。
8. 切到 `商品配置`，确认 `商品分类管理` 是分类维护入口；SKU 列表不再显示旧 `分类设置`、公共 SKU 引用开关或行内 `复制为客户SKU`。

## 自动化证据
- Unit：`node --test src/lib/product-settings.test.js` 覆盖统一新增 SKU payload、不带 `product_kind/custom_type/base_product_id`，以及 SKU复制抽屉源码守卫。
- Unit/API：`go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'TestCreateSKU|TestCopySKUs|TestProductSettingsAPICreatesUnifiedSKU|TestProductSettingsAPISKUCopy' -count=1` 覆盖统一创建、复制去重、同源同目标拒绝和 API 路由。
- Support：`go test ./internal/interfaces/http/support -run TestDev382 -count=1` 覆盖 PR/DEV/UT/API/REV 种子、源码标记、操作手册和验收文档。
- Full：最终交付需包含 `go test ./... -count=1`、完整 Vue Node 测试、`npm run build`、浏览器验收和 development 部署 smoke。

## 浏览器验收
- 用测试客户在 SKU设置 客户视图打开 `SKU复制`，从公共 SKU 全选复制，确认新增数量。
- 再次复制同一批，确认显示覆盖数量且客户 SKU 列表不出现重复同名行。
- 新增一个不选产品子类型的 SKU，确认进入停车场；选择子类型后可进入对应分类并参与后续产品价格表生成。
