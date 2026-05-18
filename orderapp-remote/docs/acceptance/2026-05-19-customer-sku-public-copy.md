# 验收记录：客户 SKU 公共配置复制与产品豆单空页面修复

日期：2026-05-19

## 需求
- 产品豆单界面不能因为豆单类型映射错误而为空，尤其生豆豆单必须读取生豆豆单字段。
- SKU设置顶部选中客户后，隐藏“新增公共产品”。
- “客户专属 SKU”使用顶部选中的客户，不再重复选择客户。
- “商品分类 · 客户SKU”和“客户SKU列表 · 客户 SKU”提供“是否使用公共SKU / 是否使用公共商品分类”开关；开启后复制当前公共 SKU 和公共商品分类给该客户，用于后续生成客户自定义豆单。

## 根因
- 产品豆单页面内部 `green` 类型仍回落到 `commercial_bean_list` / `commercial_wholesale_tiers`，生豆豆单筛选时可能找不到生豆字段。
- SKU设置客户下拉只包含已经有客户 SKU 的客户，导致新客户无法先进入客户上下文初始化 SKU。
- 客户专属 SKU 创建路径仍有独立客户选择，和顶部 SKU 归属上下文割裂。
- 后端没有一次性、可审计、幂等的“复制公共 SKU/商品分类到客户”接口。

## 实现
- 产品豆单补齐 `green` 的元数据、价格档、版本列表和选择状态映射，并修正挂耳类型标签。
- SKU设置顶部客户下拉改为所有启用客户；选中客户时隐藏新增公共产品。
- 客户专属 SKU 创建使用顶部客户上下文。
- 新增 `POST /api/product-settings/customer-public-copy`：
  - `use_public_sku=true` 时复制当前公共 SKU 为客户自己的 SKU，保留基础 SKU 关联、价格档、BOM、挂耳配置、生豆 BOM 绑定和分类位置。
  - `use_public_categories=true` 时复制公共商品分类树，复用客户侧同名分类，避免重复。
  - 已存在同一 `base_product_id` 的客户 SKU 不重复创建。
  - 写入 `customer_product_catalog` 操作日志，动作 `copy_public_catalog`。

## 验证
- `node --test src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js`
- `npm run build`
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/domain/catalog -count=1`

## 结果
- 通过。
