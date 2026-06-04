# PR-415-PRODUCT-PRICE-LIST-NO-DRIP-TEMPLATE 商品价格表下线挂耳专用模板

## 范围
- 商品价格表不再根据“挂耳/drip”分类名或 `product_kind=drip_bag` 切换到专用 `drip` 价格表类型。
- 挂耳商品按普通商品配置模板读取计价方式、阶梯价模板、固定单价或成本加成。
- `/api/drip-price-templates` 和 `/api/costing/drip-price-explanation` 不再注册，成本 schema 不再 seed 默认挂耳供应价。
- 旧 `drip` 已发布价格表、旧订单和旧 PDF 快照保留兼容读取。

## 验证命令
- `node --test src/lib/product-bean-list-split.test.js src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js`
- `go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
- `go test ./internal/interfaces/http/support -run TestDev415 -count=1`
- `npm run build`
- `scripts/verify_kferp.sh changed`

## 手册证据
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 结果
- `node --test src/lib/product-bean-list-split.test.js src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js`：通过，51/51。
- `go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`：通过。
- `go test ./internal/interfaces/http/support -run TestDev415 -count=1`：通过。
- `npm run build`：通过；保留既有 chunk size warning。
- `scripts/verify_kferp.sh changed`：通过，exit code 0。
