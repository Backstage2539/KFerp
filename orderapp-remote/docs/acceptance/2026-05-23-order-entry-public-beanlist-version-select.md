# PR-342-ORDER-ENTRY-PUBLIC-BEANLIST-VERSION-SELECT

## 范围
- 客户没有某类型专属豆单时，录单/编辑订单仍能选择公共豆单最新和历史发布版本。
- 默认选中最新公共豆单；用户改选公共历史版本后，商品候选和价格梯度按该版本快照生效。
- 保存订单时行级 `bean_list_publication_id`、`bean_list_version_no` 和价格来源保留所选公共豆单版本。

## 验收证据
- 单元测试：`node --test src/lib/order-entry.test.js` 覆盖 `beanListVersionOptionsForCustomer` 保留公共兜底版本，并覆盖商品携带多版本梯度时按所选 `bean_list_publication_id` 取价。
- 静态/API 测试：`go test ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/orderbeans -run 'TestOrderFormBeanListVersionOptionsIncludeHistoricalPublishedSnapshots|TestOrderSaveExplicitBeanListPublicationAcceptsHistoricalSnapshots|TestExplicitPublicationSelectionAcceptsHistoricalPublishedSnapshots' -count=1` 覆盖公共/客户历史发布快照参与表单选项和显式保存。
- 数据库 API 测试：`go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsHistoricalPublicBeanListVersionsForFallbackCustomer|TestOrderAPISavesHistoricalPublicBeanListPublicationVersion' -count=1` 在有测试数据库时覆盖无专属豆单客户可看到公共历史版本并保存公共历史版本。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录无专属豆单客户也可切换公共豆单版本。

## 验收点
- 选择没有专属熟豆、生豆或挂耳豆单的客户时，对应豆单下拉显示公共豆单最新和历史版本，默认最新。
- 改选公共历史版本后选商品，价格梯度按钮和行内豆单版本来自该历史公共豆单。
- 保存后订单明细的 `bean_list_publication_id`、`bean_list_version_no`、单价和小计不被最新公共豆单覆盖。
