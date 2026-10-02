# PR-681 商品创建器分类大类选择与归属

日期：2026-10-03  
范围：商品创建器运行表单分类抽屉、分类归属保存、通用分类列表展示与价格表分类筛选。  
环境：本地定向验证已完成；development / production 发布及线上 smoke 待完成。

## 验收口径

- 分类抽屉同时列出当前功能启用的分类模板名称（例如“烘焙咖啡豆”“咖啡生豆”）和模板内的全部有效分类项。
- 大类名称可直接选择并搜索；选择后新对象写入该模板根级（`group_item_id=0`），提交成功后显示在相应大类标题下。
- 选择下级分类、搜索完整路径及已有对象保留原分类的行为不变。
- 创建器提交校验活动模板和当前用途；失效模板、负分类项 ID、对象权限及事务保护仍然生效。
- 已根级归属的商品在模板级价格表中可筛选，在具体子分类价格表中不可错误出现。

## RED / GREEN 与测试证据

- RED：新增模板根分类选项测试时，分类选项辅助函数尚不存在；根级归属服务测试被 `group_item_id <= 0` 拒绝；根级对象列表测试将对象归入“未分类”。修复后的价格表回归测试还捕获了一次缺陷：模板级范围过滤器原先会丢掉 `group_item_id=0`。
- GREEN：
  - Vue 定向：`node --test src/lib/business-grouping.test.js src/lib/product-creator-v6-view.test.js src/lib/unified-category-move-ui.test.js src/lib/product-price-list-types.test.js`，43 项通过。
  - Go：`go test ./internal/interfaces/http/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/productcreator -count=1`，通过。
  - 全量前端：`scripts/verify_kferp.sh frontend`，全部 Vue shell 单测和 Vite 构建通过；构建仅报告既有大 chunk 警告。
  - 全量后端：`scripts/verify_kferp.sh backend`，`go test ./...` 通过。
  - 仓库检查：`scripts/verify_kferp.sh changed` 通过。
  - 后端独立数据库集成测试 `TestCreatorAssignsBusinessGroupRoot` 已增加；当前本地未设置 `ORDERAPP_TEST_DATABASE_URL`，因此该隔离数据库测试会跳过，不能将其记为已执行。
  - `git diff --check` 通过。
- 尚待：浏览器及两环境发布 smoke。

## 变更与留痕

- 抽屉为每个当前启用的分类模板加上可选的大类项，模板内子分类保留原路径和模糊搜索。
- 创建器执行器与普通分类归属服务允许 `group_item_id=0`，仍要求模板启用且包含当前业务用途；既有 PostgreSQL 审计写入不变。
- 归组列表将根级对象展示在对应模板标题下；价格表只将根级对象放入模板级范围。
- 用户操作说明：`orderapp-remote/docs/OP_MANUAL_PRODUCT_CREATOR.md`。
- PR/DEV 跟踪：PR-681 / DEV-732-PC-CATEGORY-ROOT-OPTIONS。

## 发布状态

- development：待发布。
- production：用户已明确授权在 development 发布后合入 `main` 并部署；待发布。
- Van 产品业务验收：待确认。
