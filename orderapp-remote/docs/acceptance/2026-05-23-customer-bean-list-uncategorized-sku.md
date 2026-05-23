# 客户豆单未挂分类 SKU 验收

## 需求
- `PR-339-CUSTOMER-BEAN-LIST-UNCATEGORIZED-SKU`
- 客户范围产品豆单中，客户 SKU 未挂到当前客户 SKU 分类时必须显示在“未分类”，不得继承基础公共 SKU 或旧 Excel 豆单分类。

## 背景
- 岩师傅 `曲奇拼配2.0` 是客户 SKU，基础豆单模板来自 `红岩2.0`。
- 该客户 SKU 当前未挂到岩师傅自己的 SKU 分类。
- 修复前，客户豆单仍沿用模板的 `4、精品意式拼配：` 分组，导致岩师傅 SKU 分类里没有该分类但豆单显示了该分类。

## 验收步骤
1. 进入客户账户模式，选择岩师傅。
2. 进入 SKU设置，确认 `曲奇拼配2.0` 没有挂到岩师傅自己的二级分类 `精品意式拼配`。
3. 进入产品豆单客户范围，查看商用批发豆单。
4. 找到 `曲奇拼配2.0`。

## 通过标准
- `曲奇拼配2.0` 显示在“未分类”分组。
- 该条目名称显示为客户 SKU 名 `曲奇拼配2.0`。
- 该条目不显示在 `精品意式拼配` 分组下。
- 后续把该 SKU 挂入岩师傅自己的客户分类后，才按客户分类标题展示。

## 验证证据
- `go test ./internal/domain/costing -run TestCustomerAliasBeanListWithoutSkuCategoryUsesUnclassifiedGroup -count=1`
- `go test ./internal/interfaces/http/costing -run TestCostingCalculateAPICustomerAliasWithoutCategoryUsesUnclassifiedGroup -count=1`
