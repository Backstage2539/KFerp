# PR-378 商品配置 KV 下拉选项编辑验收

## 范围
- 修复商品配置模板特殊 KV 的下拉选项保存问题。
- 修复字段类型从下拉改为文本后，旧下拉选项继续影响 SKU 设置的问题。

## 操作场景
1. 进入 `SKU设置 → 商品配置 → 商品配置模板`。
2. 选择或新增一个字段，例如 `roast_level / 烘焙度`。
3. 类型选 `下拉`，下拉选项填写 `浅烘，深烘，意式`，或每行一个选项后保存。
4. 回到 SKU 列表，绑定该商品配置的 SKU 在特殊属性列应展示新下拉选项，包含 `深烘` 和 `意式`。
5. 再回到商品配置模板，把 `烘焙度` 类型改为 `文本`。
6. 页面应立即清空下拉选项；保存后回到 SKU 列表，该字段应显示文本输入，不再显示旧下拉。

## 验证证据
- RED：`node --test src/lib/product-settings.test.js` 曾失败，证明旧逻辑会优先保存旧 `options`，且没有类型切换清空处理。
- GREEN：`node --test src/lib/product-settings.test.js` 通过，覆盖编辑后的 `options_text` 写入 schema，以及非下拉类型清空旧选项。
- 支撑测试：`go test ./internal/interfaces/http/support -run TestDev378 -count=1` 覆盖 PR/DEV/UT/API/REV 种子、源码标记和手册标记。
- 浏览器验收：在开发环境使用测试数据编辑 `烘焙度` 下拉选项并切换为文本，确认 SKU 设置里的字段同步刷新。
