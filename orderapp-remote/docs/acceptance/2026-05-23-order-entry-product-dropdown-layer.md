# 录单商品下拉层级验收

## 需求
- `PR-340-ORDER-ENTRY-PRODUCT-DROPDOWN-LAYER`
- 录单和修改订单的商品下拉列表必须浮在后续商品行、规格输入和价格梯度按钮之上。

## 背景
- 录单存在多条商品明细时，打开第一行或上方商品行的商品下拉，候选列表可能被后续商品行的商品选择框盖住。
- 根因是商品 combobox 没有打开态层级，普通商品行都使用相同 `product-cell` 层级，后续商品行会盖住前一行下拉。

## 验收步骤
1. 进入录单页面。
2. 新增至少两条商品明细。
3. 在上方商品明细的“商品”输入框中输入关键词，让商品下拉展开。
4. 观察下拉列表与下一条商品明细、规格输入和价格梯度按钮的遮挡关系。

## 通过标准
- 商品下拉显示在后续商品行上方。
- 商品下拉不被后续商品选择框、规格输入框或价格梯度按钮遮挡。
- 选择商品后下拉关闭，商品名、规格和价格梯度仍正常联动。

## 验证证据
- `node --test src/lib/order-entry.test.js --test-name-pattern 'order entry raises the active combobox above following fields'`
- `go test ./internal/interfaces/http/support -run TestDev340 -count=1`
