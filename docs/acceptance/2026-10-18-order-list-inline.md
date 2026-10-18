# PR-700 订单列表完整类型与行内极简详情

## 实现与验收口径

- 后端 GET /api/orders 新增 product_type_labels 数组、is_sample_order 布尔值，保留原有 product_kind_summary。可见订单页批量汇总全部明细，真实样品关联只由 sample_work_orders 判定，无逐行查询。
- 分类读取商品配置，规格商品缺分类时继承主商品；不排除已归档商品/分类。按首个明细位置稳定排序、按名称去重；历史明细按已保存且已知的类型回退，未知显示未分类。空订单返回空数组。
- Vue 列表显示订单类型、样品标记及全部商品类型；普通区域单行展开，链接、按钮和复选框独立，文字选择不触发，三角按钮支持键盘。每次只展开一单。
- 详情复用原 GET /api/orders/:id/detail，保留权限。商品/规格/数量/单价/小计来自订单快照，显示商品备注、运费、优惠/委外费用、后台应收总价与订单备注。零金额为 ¥0.00，缺失值为 —；快递费备注不另加应收。
- 每页按 ID 缓存只读详情；加载失败重试；刷新/分页/筛选/保存清理缓存和请求代次，迟到响应不能串单。展开时暂停 15 秒自动刷新避免阅读中收起；手动刷新仍即时清理。
- 390px 下明细按商品分块，数量/单价/小计三列；保留原列表横向滚动，展开详情宽度适配可见区域。

## 测试证据

| 检查 | 结果 | 证据 |
|---|---|---|
| RED | 新字段缺失、helper 缺失、真实组件未接入而失败 | order-inline-red-api.log / red-vue.log / red-runtime.log |
| 单元与组件 | 类型去重/回退、快照/零元、控制点击、迟到响应、失效重取、重试及真实 OrdersView 接入通过 | order-list-inline*.test.js |
| Go 全量 | 通过 | order-inline-final-all.log |
| Vue 全量与构建 | 1400 测试通过，Vite build 成功 | order-inline-final-all.log |
| 隔离 PostgreSQL API + race | 混合分类、主商品继承、归档分类、未知历史、普通/样品/空订单、履约过滤与权限、预付款回归通过 | order-inline-db.log；本地 kferp_order_inline_test 随机 schema 自动清理 |
| 原样品修复回归 + race | 免费样品/收费转换、收件保存与修复事务通过 | order-inline-samples-db.log |
| 小程序 | 330 测试、类型检查及 development 构建通过 | order-inline-mini.log / mini-build.log |
| 本地浏览器 | 实际 OrdersView + 合成 API 数据：文字展开、勾选独立、切单、键盘收起、免费及收费明细；桌面与 390px 截图核对通过 | order-inline-desktop.png / order-inline-mobile.png |

旧静态测试由固定 kind 字段断言更新为完整类型入口；旧 API 夹具补当前已有 settlement_reason、门户 enabled 字段和避免重复部门 ID。没有为通过测试修改业务权限或生产校验。

证据目录：/Users/yiiiple-work/KFerp-order-inline-delivery-20261018/evidence/；截图在上级目录。本地页面检查使用合成订单，不创建或修改开发业务订单。

## 手册与五表

更新单一源 orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md 的“订单列表类型与极简详情”章节，既有 Vue 订单销售手册入口读取该源；列表同时新增操作提示。

PR-700-ORDER-LIST-INLINE / DEV-789-ORDER-LIST-INLINE / UT-700-ORDER-LIST-INLINE / API-700-ORDER-LIST-INLINE / REV-700-ORDER-LIST-INLINE。开发 UI 五表已登记；部署后已通过只读 API 核实 PR review、DEV/UT/API done、REV todo。真人验收仍待 Van 完成。

## 开发交付

基线 origin/develop 5c0c53ce；功能分支 codex/order-list-inline-details-20261018。功能提交 8f73450df823a4a8ae804c17550d3405b4c9df48 已推送，经 [PR #214](https://github.com/Backstage2539/KFerp/pull/214) 合并为 96091e8e6d08c9326d63a597ce4e3d5cef44d218。

首轮发布期间 develop 合入另一工作流的纯文档回执 d29e61bd，已在应用构建和切换前停止旧发布。功能分支同步最新 develop 后重新通过 Go、Vue 1400 测试及构建，再推送并串行发布。实际部署提交为 **d29e61bd3df906321ab539655090c7e2a63d3ee7**，包含本功能和该文档回执。

- 开发环境：https://dev.qacoohee.com/app/；2026-10-18 发布完成，外网登录页 HTTP 200。
- 服务端 Vue 1400 测试、Go 测试、容器构建、小程序 330 测试、类型检查和 development 构建均通过。
- 发布后只读检查：应用 running、重启 0 次，数据库 healthy；订单列表 200 且新字段类型与兼容字段正确；匿名请求 401；Vue 订单入口和订单销售手册 200；五表状态与上述一致。
- 服务端 RELEASE_INFO 核实 development 及完整部署提交；本地交付包下载在中断后单独补齐，不重复发布服务。
- 回滚源：/opt/stacks/erp/orderapp.backup.deploy-20261018224214-d29e61bd3df9；回滚镜像：kferp-orderapp-rollback:development-20261018224214-d29e61bd3df9。
- 发布日志：evidence/order-inline-deploy-final.log；发布后检查：evidence/order-inline-smoke.log。原取消尝试另存 deploy-attempt-canceled-before-release.log，不计为成功发布。

本回执后续提交仅记录交付事实，不改变实际运行提交。生产发布、真人业务验收及小程序微信上传/发布未执行。
