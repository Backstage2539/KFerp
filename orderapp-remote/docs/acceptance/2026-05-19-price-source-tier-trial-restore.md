# 2026-05-19 价格来源临时试算恢复验收

## 范围
- 产品豆单主页面继续不展示独立价格试算工作区。
- 商用豆单每档“来源”抽屉恢复临时阶梯价试算。
- 挂耳价格来源抽屉继续只展示当前公式步骤，不提供临时试算表单。

## 验收点
- [x] 产品豆单源码不再包含页面级“价格试算”工作区、保存试算、发布价格或试算批次入口。
- [x] 商用价格来源抽屉展示“当前试算”“临时试算”，并保留临时生豆成本、出成率、利润率输入和“重新试算”按钮。
- [x] 临时参数只通过 `/api/costing/price-explanation` 的 overrides 参与本次来源解释，不保存成本参数、BOM/产品出成率或梯度模板。
- [x] REQUIREMENTS、ACCEPTANCE_TESTS、操作手册和梯度模板验收记录已同步说明主页面价格试算与来源抽屉临时试算的边界。

## 验证证据
- `node --test src/lib/product-bean-list-split.test.js`
- `node --test src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/gradient-templates.test.js`
- `go test ./internal/interfaces/http/support ./internal/interfaces/http/costing -count=1`
- `node --test src/lib/*.test.js`
- `go test ./...`
- `npm run build`
- `git diff --check`
