# 验收记录：客户 SKU 公共引用与产品豆单空页面修复

日期：2026-05-19

## 需求
- 产品豆单界面不能因为豆单类型映射错误而为空，尤其生豆豆单必须读取生豆豆单字段。
- SKU设置顶部选中客户后，隐藏“新增公共产品”。
- 默认公共SKU时不展示“客户专属 SKU”；选中履约客户后，“客户专属 SKU”使用顶部选中的客户，不再重复选择客户。
- “商品分类 · 客户SKU”只提供“是否使用公共商品分类”开关；“客户SKU列表 · 客户 SKU”只提供“是否使用公共SKU”开关；开启后分别引用当前公共商品分类和公共 SKU，用于后续生成客户自定义豆单。
- 公共商品分类和公共 SKU 只是引用，不能复制成客户自己的分类、SKU、BOM 或价格梯度；关闭开关后必须从当前客户视图移除。

## 根因
- 产品豆单页面内部 `green` 类型仍回落到 `commercial_bean_list` / `commercial_wholesale_tiers`，生豆豆单筛选时可能找不到生豆字段。
- SKU设置客户下拉原来不是履约客户候选，且曾只包含已经有客户 SKU 的客户，导致新履约客户无法先进入客户上下文初始化 SKU。
- 客户专属 SKU 创建路径仍有独立客户选择，和顶部 SKU 归属上下文割裂。
- 前一版后端把公共 SKU/公共分类复制成客户自己的数据，关闭开关不会隐藏这些复制行，也违背“公共内容只是引用”的数据模型。

## 实现
- 产品豆单补齐 `green` 的元数据、价格档、版本列表和选择状态映射，并修正挂耳类型标签。
- SKU设置顶部客户下拉改为 `/api/customer-fulfillment/customers` 的履约客户候选；选中客户时隐藏新增公共产品。
- 默认公共SKU时隐藏客户专属 SKU 模块；客户专属 SKU 创建使用顶部客户上下文。
- 商品分类区和客户SKU列表区拆成两个独立开关，勾选即分别保存公共分类/公共 SKU 引用状态。
- 新增 `POST /api/product-settings/customer-public-usage`：
  - `use_public_sku=true` 时只读展示公共 SKU 引用；`false` 时从当前客户 SKU 列表移除公共 SKU。
  - `use_public_categories=true` 时只读展示公共商品分类引用；`false` 时从当前客户商品分类移除公共分类。
  - 保存开关时写入 `customer_sku_public_usage`，不插入客户产品、客户分类、BOM 或价格梯度复制数据。
  - 兼容清理旧逻辑生成的未改名公共 SKU 复制行和空的同名公共分类复制行。
  - 写入 `customer_product_catalog` 操作日志，动作 `update_public_usage`。

## 验证
- `node --test src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js`
- `npm run build`
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/domain/catalog -count=1`

## 结果
- 通过。
