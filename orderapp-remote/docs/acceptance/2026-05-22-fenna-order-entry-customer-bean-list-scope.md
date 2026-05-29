# PR-324-FENNA-ORDER-ENTRY-CUSTOMER-BEAN-LIST-SCOPE

## 验收范围
- 芬纳咖啡这类已有客户商用豆单的客户，在 ERP 录单商品选择器中只看到该客户商用豆单发布快照内、且带 `commercial_wholesale_tiers` 的商品。
- 公共商用商品、未发布到客户商用豆单的客户 SKU 不得出现在该客户录单可选商品中。
- `芬纳定制-红酒日晒-中深烘` 和 `芬纳-曲奇定制（20%乌干达，15%云南厌氧日晒，65%云南水洗）` 自动价来自客户已发布商用豆单快照，不为空。

## 验收证据
- 单元：`TestCommercialOrderPublicationTiersParseTemplatePrices`、`TestApplyCommercialOrderPublicationTiersReplacesCustomerRoastedTiers` 覆盖发布快照价格解析与覆盖。
- API：`TestOrderAPIFormUsesCustomerCommercialBeanListForProductOptions` 覆盖 `/api/order/form?customer_id=...` 返回客户商用豆单商品与价格，并隐藏公共商用商品。
- 前端：`filterProductsForCustomer hides public products when customer commercial bean list owns the scope` 覆盖客户专属商用豆单范围下的商品下拉过滤。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录客户有专属商用豆单时的录单商品范围和价格来源规则。
