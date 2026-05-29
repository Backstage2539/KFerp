# PR-380 复制公共分类引用公共 SKU（历史方案，已由 PR-382 取代）

## 范围
- 该方案上线后被新版 SKU 复制模型取代。
- 新版不再通过只读引用公共 SKU 复用产品；客户需要产品时使用 PR-382 “SKU复制”生成客户自己的 SKU 主档。

## 验收步骤
1. 进入 `SKU设置`，在 `SKU归属` 选择一个履约客户。
2. 在 `商品配置 → 商品分类管理` 维护或复制客户分类结构。
3. 回到 SKU 列表，点击 `SKU复制`，从公共 SKU 或其他客户选择分类和 SKU 复制到当前客户。
4. 确认复制结果新增或覆盖客户 SKU，而不是显示公共 SKU 只读引用。

## 自动化证据
- RED：`go test ./internal/application/catalog -run TestDeriveProductCategoryEnablesPublicSKUReference -count=1` 失败在未保存公共 SKU 引用。
- RED：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPIDerivesPublicCategoryAndProductTemplates -count=1` 失败在 `publicUsageSaved=false`。
- SUPERSEDED：PR-382 覆盖新版复制模型；全量测试、构建和浏览器验收见 PR-382 最终交付记录。
