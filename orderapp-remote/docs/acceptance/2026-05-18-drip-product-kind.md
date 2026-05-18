# 挂耳产品形态验收记录

## 需求覆盖

- 产品设置：已验证，挂耳产品支持每袋熟豆克重、每盒袋数、履约/商城渠道开关。
- 挂耳 BOM：已验证，挂耳 BOM 支持熟豆成品组件和包材组件。
- 挂耳价格模板：已验证，供应价模板按袋生成价格并发布袋/盒交易快照。
- 挂耳豆单：已验证，挂耳豆单独立展示袋价和盒价，不混入熟豆商用/零售豆单。
- ERP 按袋录单：已验证，120 袋 × 2.15 = 258.00。
- ERP 按盒录单：已验证，12 盒 × 10 袋/盒按袋价档换算为 21.50/盒，小计 258.00。
- 履约客户下单：已验证，客户履约和小程序履约支持袋/盒单位快照。
- 商城下单：已验证，商城挂耳只使用显式商城价，不暴露供应价；商城价按袋维护，按盒下单自动乘每盒袋数，并保存单位快照。
- 挂耳生产需求：已验证，挂耳订单进入挂耳生产需求，不直接当熟豆订单。
- 熟豆不足上游烘焙需求：已验证，熟豆成品组件不足时显示 upstream roast shortage。
- 操作日志：已验证，挂耳产品、BOM、价格发布、ERP 订单、履约订单、商城订单和生产消耗路径写入审计 metadata。
- 操作手册：已验证，已更新成本、库存/BOM、订单销售、客户履约、客户门户/商城和生产手册。

## 验证命令

- `cd orderapp-remote && go test ./... -count=1`：通过。
- `cd orderapp-remote && go test ./internal/domain/sales ./internal/application/sales ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales -count=1`：通过。
- `cd orderapp-remote && go test ./internal/application/customerfulfillment ./internal/infrastructure/postgres/customerfulfillment ./internal/interfaces/http/customerfulfillment ./internal/application/customerportal ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/customerportal -count=1`：通过。
- `cd orderapp-remote && go test ./internal/application/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -count=1`：通过。
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/produce-plan.test.js src/lib/order-entry.test.js src/lib/drip-product.test.js`：通过。
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js src/api/*.test.js`：通过，193/193。
- `cd orderapp-remote/frontend-vue-shell && npm run build`：通过，保留 Vite chunk size warning。
- `cd miniapp && npm run test -- --run src/utils/servicePage.test.ts src/utils/mall.test.ts src/api/customerPortal.test.ts`：通过。
- `cd miniapp && npm run test -- --run src/utils/mall.test.ts`：通过，覆盖商城挂耳按袋报价、按盒大货下单和购物车分行。
- `cd miniapp && npm run test -- --run`：通过，11 个文件，56/56。
- `cd miniapp && npm run typecheck`：通过。
- `cd miniapp && npm run build:mp-weixin`：通过。
- `git diff --check`：通过。

## 关键接口响应摘要

- `/api/order/form` 产品 payload：挂耳产品返回 `product_kind=drip_bag`、`sales_units=["bag","box"]`、`drip_bag_grams`、`drip_box_bag_count` 和挂耳价格梯度。
- `/api/order` 保存挂耳明细：`order_items` 写入 `product_kind`、`sales_unit`、`unit_bag_count`、`unit_bean_g`、`matched_price_qty`、`price_source_json`。
- `/api/mini/mall`：挂耳商城商品返回显式 `mall_price`；无商城价不向小程序暴露供应价。
- 生产计划接口：挂耳需求行返回 `product_kind=drip_bag`、`need_bags`、`upstream_product_id`、`upstream_shortage_g`。

## 待上线复核

- 合并 `develop` 后需要重新运行完整后端、ERP 前端、miniapp 验证。
- 部署 development stack 后记录 `origin/develop` SHA 和 smoke test 结果。
