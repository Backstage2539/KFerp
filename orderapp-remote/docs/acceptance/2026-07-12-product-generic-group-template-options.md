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

## 验收结果

- 开发环境：部署 `origin/develop=eca87ee6a341b5b800cf2c8dd5e3118ca25b3b90`，productMaster、product-settings、PR-534 API 和近期错误日志检查通过；回滚备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260712224356`。
- 生产环境：部署 `origin/main=65eaf66694ff64992c2e693eed831272bb233fad`，容器正常、数据库健康、productMaster HTTP 200、近期错误日志为 0；回滚备份为 `/opt/stacks/erp-production/orderapp.backup.deploy-20260712225746`。
- 生产浏览器：商品档案“选择分组模板”包含并选中 `商品-咖啡豆`；`金色山脉` 仍为 1 个商品并展示 4 个规格 SKU；控制台错误为 0。
- 数据：未迁移、未补写用途、未修改模板、分类或商品归类数据。
