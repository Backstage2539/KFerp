# 2026-05-19 生豆豆单手工价与录单取价验收

## 范围
- 岩师傅“兰卡拼配生豆”录单无价格问题。
- 生豆 SKU 绑定熟豆 BOM 后，生豆豆单按 BOM 原料成本快照生成默认成本参考价。
- 生豆豆单手工修改的阶梯价只保存到本次草稿/发布快照。
- ERP 录单按熟豆、生豆、挂耳分别选择豆单版本，生豆订单只从所选生豆豆单快照取价。

## 验收点
- [x] `product_bom_items` / `bom_version_items` 保存 `unit_cost_snapshot`，生豆豆单默认价按绑定熟豆 BOM 原料成本快照加权计算，不读取原料批次实时成本。
- [x] 生豆豆单选择梯度模板后带出档位区间和展示单位；用户可手工修改每个档位价格，发布内容包含 `green_bean_sale_tiers` 交易快照。
- [x] `/app/api/order/form` 不再给生豆 SKU 返回绑定熟豆梯度来源。
- [x] ERP 录单请求支持 `commercial_bean_list_publication_id`、`green_bean_list_publication_id`、`drip_bean_list_publication_id`，默认各类型最新版本。
- [x] 保存生豆订单时按所选 `green` 豆单版本匹配价格并记录到订单行；缺少生豆豆单价格时拒绝保存并提示缺少生豆豆单价格。

## 验证证据
- `go test ./internal/application/costing -run TestBeanListGreenBeanTemplateTiersDefaultToBomCostWithoutMargin -count=1`
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsUsesBomCostSnapshotForGreenBeanCost -count=1`
- `go test ./internal/infrastructure/postgres/orderbeans -count=1`
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormProductQueryDoesNotExposeBoundRoastedTiersForGreenBeanProducts|TestOrderSaveRejectsMissingGreenBeanListPriceWithoutBoundRoastedFallback|TestOrderFormBeanListVersionOptionsArePartitionedByListType' -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormDoesNotReturnBoundRoastedTiersForGreenBeanProduct|TestOrderAPISavesGreenBeanOrderRejectsMissingGreenBeanListPrice|TestOrderAPISavesGreenBeanOrderUsingSelectedGreenBeanListPublication' -count=1`
- `node --test src/lib/order-entry.test.js src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js`

## 手册
- `OP_MANUAL_GREEN_BEAN_SALES.md`
- `OP_MANUAL_ORDER_SALES.md`
- `OP_MANUAL_COSTING.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
