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

针对性 PostgreSQL 与前端测试已通过。合并最新 develop 5adb6d11（已部署验证的 PR-635），复用命名价格表选择并补齐客户与公共表各自当前版本校验。客户分类移动入口原来被禁用，现已支持在客户目录独立调整，不改工厂及其他客户。完整检查通过；开发页面/API结果见下。

## 本地完整检查

Go 全量、Vue 1102/1102、小程序 238/238、类型检查、Vite 与微信开发构建均通过。真实 PostgreSQL 目录复制/删除/移动/恢复/失败回滚、排序稳定、来源冻结和当前公共版本显式选择通过。日志 /tmp/pr636-final-all3.log、pr636-final-postgres.log、pr636-catalog-final.log、pr636-final-mini-tests.log、pr636-final-mini-types.log、pr636-final-mini-build.log。

- 补充客户视图录单范围回归：带 customer_id 的录单表单必须同时保留当前客户引用的客户报价及可显式选择的公共报价；其他客户专属商品仍不可见。RED /tmp/pr637-scoped-form-red.log；相关应用及 HTTP 测试 GREEN。

## 开发环境实际结果

- 订单 `1611 / SO-20260907-0004`：客户 301，商品 1063，227g / BOM 规格 3 / variant 483，数量 2，客户价格表 124 / V3.0.5，单价 37.50，合计 75.00。API 修改订单日期到 2025-01-01 后列表 ID 顺序完全一致；页面再次改为 2025-01-02，最终部署后改为 2025-01-03 并保存成功，单据日期仍 2026-09-07，仍在当日订单位置。
- 订单 `1612 / SO-20260907-0005`：同客户、商品、BOM、数量，明确选择公共表 122 / V3.0.22，单价 33.00，合计 66.00。客户和公共价格均已保存落库。页面也实选公共表后重新选品，显示 33.00、可保存；未覆盖客户表测试订单。
- 订单详情：客户表显示「咖啡豆 / PR634验收-目录客户B / V3.0.5 / 2026-09-07 21:51:40」；公共表显示「咖啡豆 / 工厂公共 / V3.0.22 / 2026-09-07 17:42:03」。BOM V007、咖啡豆包装路线保持一致。
- 客户 300、商品 1084、引用 111：移动分类、删除绑定、重复删除、复制恢复均 200；恢复复用原 ID、名称及分类。混入不存在商品的整批删除返回 400 并回滚。商品主档、BOM、规格、组件和库存指纹在此组操作前后不变。审计 8525 / move_customer_products、8526 / remove_customer_products、8527 / copy_products_to_customer。
- 页面实际将 1084 移到未分类，再移回「咖啡豆 / PR634客户意式」，提示「已调整 1 款客户商品的分类，工厂分类保持不变」。客户价格表读取到咖啡豆 2 款、咖啡生豆 10 款、未分类 73 款，与当前目录调整相符。未修改 NB 的名称、分类、报价或发布。
- 两个客户账号相互请求客户目录、删除及移动接口，各 3 个请求均返回 403。
- 页面删除按钮及「只删除客户绑定，保留原商品和生产配置」确认文案已核对；浏览器工具在确认框处中断，未把该页面点击记为成功写入。删除及恢复已通过真实开发 API 和数据库完成验证，测试绑定保留为有效供复查。

## 验收中补充修复

1. BOM 订单编辑接口漏传 parent_product_id，页面回填退回旧规格并清空价格。新增真实序列化 RED 后修复；页面复验回填 227g / 37.50 / 75.00，并成功保存仅订单日期变化。日志 pr637-edit-parent-red.log / green.log。
2. 编辑保存后的列表刷新覆盖已加载详情，报价来源变空；现在保存后重新读取详情。公共最新价格表曾与客户默认表比较而误标过期，现按同一归属比较。行为测试 RED / GREEN，前端完整 1104/1104 与 Vite 构建通过；日志 pr637-refresh-red.log / green.log / full.log。

## 证据文件

本机 `outputs/pr637-catalog-order-followup/` 保留页面截图、API/落库摘要、客户越权结果、RED/GREEN 与完整检查日志。原始验收日志在 `/tmp/pr637-*`、`/tmp/pr636-*`；早期日志编号沿用任务登记前缀，正式需求为 PR-637。

## 最终发布与复验（2026-09-08）

- 功能分支代码 `3d8a948a` 已推送；合并后的 `origin/develop@60e0c43a33e4c669ab0791b8e5c3e9785a4fdc84` 由干净 release 工作区执行 `./deploy_orderapp.sh development`，退出 0。发布完成时间本机 2026-09-08 00:09，日志 `pr637-deploy-delivered.log`。后续验收文档补交不改变本次运行代码。
- 部署包含服务器 Go、Vue 1104 项、小程序 238 项、类型检查和构建；最终 `scripts/verify_kferp.sh changed` 通过。56 个微信开发包清单文件校验通过，开发包同步 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`。
- 旧源码 `/opt/stacks/erp/orderapp.backup.deploy-20260908000229-60e0c43a33e4`；回滚镜像 `kferp-orderapp-rollback:development-20260908000229-60e0c43a33e4`。回滚通过开发部署脚本既有恢复流程使用此源码与镜像，不对生产操作。
- `erp_orderapp` running、重启 0；PostgreSQL healthy；外部登录页 200，未认证录单接口 401，认证编辑表单和客户候选接口 200。7 个修改源码的服务器 SHA256 与推送代码一致，服务器 RELEASE_INFO 与本机开发小程序包均为 `60e0c43a`。
- 最终页面实际保存订单 1611 的订单日期为 2025-01-03，单据日期 2026-09-07 不变、列表仍位于 1612 之后；单价 37.50、数量 2、客户表 124 不变。保存提示成功后，详情报价来源、归属、版本、发布日期仍完整显示。订单 1612 公共表 V3.0.22 不再出现“非最新价格表”误报，单价 33.00、数量 2 不变。
- 生产运行记录仍为 `ce58d4ec90df6fe2ea079f650362ccdacf7046ec`；本次没有生产部署、微信上传或提审。小程序测试和开发包构建通过，本次未使用真机进行人工录单验收。
- PR-637 与 4 个 DEV 条目在开发环境可查，保持 review 等待产品复查。验收客户、引用、两张订单保留。删除确认框的浏览器工具限制已在上文单独记录，未夸大为页面完成删除。

最终本机证据目录：`/Users/yiiiple-work/.codex/worktrees/ccd2/KFerp/outputs/pr637-catalog-order-followup/`。重点文件：`final-release-health.json`、`api-database-evidence.json`、`order-customer-source-final.png`、`order-public-source-final.png`、`order-after-save-dom.txt`、`nb-source-categories-dom.txt`、`customer-category-move.png`、`customer-price-categories.png`。
