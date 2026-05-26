# PR-380 复制公共分类引用公共 SKU

## 范围
- SKU设置选择履约客户后，公共分类的“复制为客户分类”。
- 只修复引用开关和客户视图范围，不复制公共 SKU 主档。

## 验收步骤
1. 进入 `SKU设置`，在 `SKU归属` 选择一个履约客户。
2. 确认该客户原本未开启 `是否使用公共SKU` 和 `是否使用公共商品分类`。
3. 点击 `分类设置`，找到一个公共产品类型或产品子类型，点击 `复制为客户分类`。
4. 复制完成后刷新或等待页面自动刷新，确认仍在当前客户视图。
5. 确认客户分类树中出现客户自己的可编辑分类，并且该公共分类下的公共 SKU 以只读引用显示。
6. 确认客户 SKU 列表中可见对应公共 SKU；数据库中不新增这些公共 SKU 的客户副本。
7. 若该客户之前开启了公共梯度模板引用，复制后仍开启；若之前关闭，复制后仍关闭。

## 自动化证据
- RED：`go test ./internal/application/catalog -run TestDeriveProductCategoryEnablesPublicSKUReference -count=1` 失败在未保存公共 SKU 引用。
- RED：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIDerivesPublicCategoryAndProductTemplates -count=1` 失败在 `publicUsageSaved=false`。
- GREEN：同两条测试通过；全量测试、构建和浏览器验收见最终交付记录。
