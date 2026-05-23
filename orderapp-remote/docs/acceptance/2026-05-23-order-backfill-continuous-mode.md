# PR-348-ORDER-BACKFILL-CONTINUOUS-MODE 验收证据

## 范围

- 录单页“订单补录”从说明文字升级为可点击的连续补录模式。
- 单张补录仍可关闭连续补录，只编辑 `订单日期` 和 `单据日期` 后普通保存。
- 连续补录保存后保留客户、双日期、物流、运费、取整、收款方式和豆单版本，清空商品、单号、收款凭证、收款金额、订单备注、优惠和外协费用。

## 验证命令

- `node --test src/lib/order-entry.test.js`
- `go test ./internal/interfaces/http/support -run TestDev348 -count=1`
- `go test ./...`
- `node --test src/lib/*.test.js src/api/*.test.js`
- `npm run build`

## 结果

- 待最终验证后更新。

