# PR-637 订单与客户目录跟进验收

## 范围

从 origin/develop 56ea7d3b 建立 codex/customer-catalog-order-followup-20260907，仅 development 发布。生产环境与微信发布不在本次范围。原工作区未提交修改保持不动，PR-635/PR-636 属于独立任务。

## 复现与 RED

- 订单查询排序为 order_date；真实 PostgreSQL 日期交错测试返回 [2,3,1]，预期按单据日期 [3,1,2]。
- 客户目录行停用调用全局商品失效接口；新增事务删除绑定测试 RED。
- 前端客户版本过滤存在客户表时隐藏公共选项；新增客户默认优先、公共表仍可显式选择测试 RED。
- 原订单来源只返回版本号；新增发布归属、发布日期冻结及小程序公开表选择测试 RED。
- RED 日志 /tmp/pr636-sort-red.log、pr636-catalog-red.log、pr636-frontend-red.log、pr636-trace-red.log、pr636-mini-red.log。

## NB咖啡分类核对

开发客户 102，共 85 个有效商品引用。源商品有效分类：1 个咖啡豆、10 个咖啡生豆，74 个处于系统默认分组的空分类；客户目录一致。API /api/product-settings/customer-catalog?customer_id=102、/api/business-group-feature-selections/product_catalog、/api/costing/bean-list?customer_id=102 均 200。实际页面显示“咖啡豆（1款）”“咖啡生豆（10款）”“未分类（74款）”。源头没有有效分类，未执行猜测性数据修正；保留 NB 的“NB的初晓”和价格表 126 / V3.0.5。

## 验证状态

针对性 PostgreSQL 与前端测试已通过。合并最新 develop 5adb6d11（已部署验证的 PR-635），复用命名价格表选择并补齐客户与公共表各自当前版本校验。客户分类移动入口原来被禁用，现已支持在客户目录独立调整，不改工厂及其他客户。完整检查及开发页面/API验证进行中。

## 本地完整检查

Go 全量、Vue 1102/1102、小程序 238/238、类型检查、Vite 与微信开发构建均通过。真实 PostgreSQL 目录复制/删除/移动/恢复/失败回滚、排序稳定、来源冻结和当前公共版本显式选择通过。日志 /tmp/pr636-final-all3.log、pr636-final-postgres.log、pr636-catalog-final.log、pr636-final-mini-tests.log、pr636-final-mini-types.log、pr636-final-mini-build.log。
