# PR-377 商品配置 KV 入口、公共 SKU 引用与 SKU 列表布局

## 范围
- SKU 特殊属性配置入口可发现，不再只写在计划或隐藏字段里。
- 客户复制公共商品配置模板后，引用公共 SKU 和公共商品分类，不复制公共 SKU 主档。
- SKU 列表横向适配，产品类型和产品子类型保持单行展示。

## 验收步骤
1. 进入 SKU设置 → 商品配置 → 商品配置模板。
2. 确认特殊属性区域标题为“产品信息字段（特殊属性KV）”，说明中明确“SKU列表特殊属性列填写具体值”，勾选项为“展示到价格表/PDF”。
3. 回到 SKU 列表，找到未绑定字段的 SKU，特殊属性列应显示“未配置字段”和“配置字段”入口。
4. 点击“配置字段”，页面应切到商品配置模板编辑区；在这里新增字段并勾选“展示到价格表/PDF”后，SKU 行填写的值会进入产品价格表页面、发布快照和 PDF。
5. 切换到履约客户，在商品配置模板列表中复制一个公共商品配置模板。复制后页面仍停留在该客户视图，并开启公共 SKU 和公共商品分类引用；客户视图展示的是公共产品引用，不新增一份客户 SKU。
6. 在 SKU 列表查看“产品类型/产品子类型/特殊属性”等列，列内容保持一行；页面宽度不足时在 SKU 列表区域左右滚动，不出现产品类型一个字一行。

## 自动化证据
- `node --test src/lib/product-settings.test.js` 覆盖特殊 KV 配置入口、公共 SKU 引用和 SKU 表格横向滚动源码守卫。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'TestDeriveProductConfigTemplate|TestProductSettingsAPIExposesSavesAndDerivesProductConfigTemplates' -count=1` 覆盖复制公共商品配置后保存公共 SKU/分类引用。
- `go test ./internal/interfaces/http/support -run TestDev377 -count=1` 覆盖 PR/DEV/UT/API/REV 种子、源码标记和手册验收文档。

## 浏览器验收
- 打开 SKU设置，确认 SKU 列表横向滚动区域存在，产品类型不再竖排。
- 在 SKU 特殊属性列点击“配置字段”，确认能进入商品配置模板字段配置。
- 在履约客户下复制公共商品配置模板，确认客户页面仍显示公共 SKU 引用，不创建重复客户 SKU。
