# PR-669 商品归属过滤视图客户改名 验收证据（2026-09-20）

## 背景
复制商品到客户后按归属过滤，配置抽屉“商品名”编辑的是替换后的客户名但保存走 PUT /api/products/:id——客户名被写回工厂商品 products.name，影响工厂与所有客户。

## 修复（方案A：引用体系内补全）
1. 列表：客户名行附带小字“工厂名：xxx”（canonical_name 标注）。
2. 抽屉：productReferenceRenameContext（归属过滤=客户X + 公共商品 + 存在 active 引用）激活时，“商品名”切换为“客户商品名”，工厂商品名只读提示。
3. 保存：先 PUT /api/product-customer-references/:id（customer_display_name，回填货号/备注/active），PUT /api/products/:id 的 basics payload 剔除 name——工厂名在该视图不可达。
4. 客户名留空时校验“请填写客户商品名”。

## 兼容边界
- 客户自有商品（customer_id>0）：不受影响。
- 客户 SKU 上下文：公共引用行禁编辑保护（canEditSkuRow）不变。
- 工厂/全部过滤：正常编辑工厂商品名。
- 订单录单 CustomerProductDisplayName：改名后自动生效。

## 自动化证据
- 前端：node --test src/lib/*.test.js 1227/1227 全绿（新增合同：canonical_name 标注 / productReferenceRenameContext / 客户商品名+工厂商品名 / 引用 PUT + delete basicsPayload.name）
- 构建：vite build ✓
- Go：internal/interfaces/http/support 全绿（req_store PR-669 种子）
