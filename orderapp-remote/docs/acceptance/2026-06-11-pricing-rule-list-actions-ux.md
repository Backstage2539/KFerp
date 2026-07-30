# PR-468 商品价格管理模板列表操作调整

## 需求
- `PR-468-PRICING-RULE-LIST-ACTIONS-UX`
- 商品价格管理的价格试算从模板行操作移到列表顶部，位于 `新建价格计算模板` 左侧。
- 点击 `价格试算` 后在抽屉内选择启用的价格计算模板，再选择商品试算。
- 列表不再单独做 `编辑模板` 按钮，点击模板名称进入编辑。
- 普通停用模板置灰，但仍可通过 `复制` 生成启用的新模板；PR-539 后历史 `fixed_add` 等隔离模板不得复制或直接保存，需新建加价率模板。

## RED
- `node --test src/lib/product-settings.test.js`：实现前失败，`product-settings.js` 尚未导出 `buildPricingRuleCopyPayload`。
- `go test ./internal/interfaces/http/support -run TestDev468PricingRuleListActionsUXContracts -count=1`：实现前失败，缺少 `PR-468` 需求/验收文档和前端标记。

## GREEN
- 通过：`node --test src/lib/product-settings.test.js`，130/130 通过。
- 通过：`go test ./internal/interfaces/http/support -run TestDev468PricingRuleListActionsUXContracts -count=1`。
- 通过：`go test ./internal/interfaces/http/support -count=1`，并将既有 PR-444 支持契约从旧行内 `编辑模板` 文案更新为模板名称编辑入口标记。
- 通过：`go test ./internal/interfaces/http/catalog -run TestProductPricingRuleAPICopyCreateActivatesCopiedTemplate -count=1`。
- 通过：`go test ./...`。
- 通过：`npm run build`，仅保留既有 Vite chunk-size warning。
- 通过：`scripts/verify_kferp.sh changed`。
- 通过：`git diff --check`。
- 通过：本地生产构建 + mock API + Chrome DevTools Protocol smoke。断言包括：顶部 `价格试算` 和 `新建价格计算模板` 可见；行内 `试算` / `编辑模板` 不存在；`复制` 可见；停用行 opacity 为 `0.42`；停用行复制按钮 opacity 为 `1`；试算模板下拉包含启用模板且不包含停用模板。

## 验收口径
- 商品价格管理顶部动作顺序为 `价格试算`、`新建价格计算模板`。
- 模板列表行内不再显示 `试算` 或 `编辑模板`。
- 试算抽屉包含 `试算模板` 下拉，只列出启用模板。
- 点击模板名称进入编辑表单。
- 普通停用模板置灰，`复制` 按钮保持正常可点击；历史 `fixed_add` 等隔离模板的复制按钮禁用。
- 复制普通停用模板时走 `POST /api/product-pricing-rules` 生成启用模板，保存路径继续写操作日志；隔离模板必须新建加价率模板。
