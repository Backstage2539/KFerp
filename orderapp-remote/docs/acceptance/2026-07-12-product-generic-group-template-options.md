# PR-534 商品档案通用分组模板候选兼容验收

## 验收目标

- 没有旧用途绑定的通用分组模板可在商品档案和商品价格表选择。
- 带明确其他用途的历史专用模板继续按用途隔离。
- 不修改模板、分类、商品归类或历史快照数据。

## 自动验证

- `node --test src/lib/business-grouping.test.js src/lib/product-settings.test.js`
- `go test ./internal/interfaces/http/support -count=1`
- `scripts/verify_kferp.sh changed`
- `scripts/verify_kferp.sh frontend-build`

## 浏览器验收

1. 打开 `业务设置 / 分组模板`，确认 `商品-咖啡豆` 及其大类、小类仍存在。
2. 打开 `商品 / 商品档案`，确认“选择分组模板”可选择 `商品-咖啡豆`。
3. 选择模板后确认大类、小类和未分类正常显示，现有商品数量和归类不发生自动变化。
