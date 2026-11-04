# PR-714 订单详情布局修正

状态：实现、开发集成、开发及正式环境发布核验通过；Van 业务验收待完成。

## 范围与结果

- 寄件人：列表刷新原先删除未勾选订单的寄件人状态，导致已打开抽屉的下拉框空白。现在保留当前抽屉状态；默认跟随批次寄件人，手动选择在刷新及重开后保留。寄件人仍为批次快递/导出选择，不新增订单持久化写入。
- 保存：删除付款与交付区重复按钮，只保留底部完整订单保存。保存、失败保留输入、并发校验与操作日志沿用原接口。
- 费用：A 区域显示唯一一份明细，下方保留地区/调整/核算操作。编辑组件将当前费用和净货款同步到 A 区域；核算后更新，关闭重开恢复已存快照。独立编辑页保留自己的费用展示。
- 普通订单与样品手册、编辑器内帮助同步改为底部保存。无 schema、权限或业务事务变更；不改 templates HTML。

## RED / GREEN

- 定向真实 Vue 父子组件 DOM 回归实现前 4 项失败：寄件人刷新后 value 为空、两个保存按钮、重复费用及 A 区域未跟随核算更新。RED：outputs/order-drawer-cleanup-20261022/red.log。
- 实现后同一文件 8/8 通过；包含保存失败重试、双击仅一次、原规格与净折扣金额、地址恢复、关闭重开。GREEN：green.log。
- 全量前端 1498/1498 通过；生产构建通过（既有 chunk 大小提示）。frontend.log、build.log。
- 既有 HTTP 层 TestStaffFulfillmentEditRoutesRequireInternalOrderWrite、TestStaffFulfillmentEditRoutesStrictCommandsAndDispatch、TestStaffRecipientUsesFullFulfillmentTransaction 通过；全量 go test ./... 通过。api.log、go.log。PostgreSQL 条件集成测试沿用环境跳过规则，本次未单独启动数据库，也未对生产保存订单。
- 隔离本机浏览器真实 OrdersView + OrderEntryView：寄件人显示“跟随本次寄件人：营业部”；费用仅在 A 区域出现；点击核算后 A 区域从 14 元变为 16 元；下方没有重复卡片，只有一个底部保存按钮。summary.png、bottom-save.png。使用合成测试数据，未连接生产业务接口。

## 手册与交付边界

- orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md
- orderapp-remote/docs/OP_MANUAL_SAMPLES.md
- Vue OrderEntryView 内录单手册；前端订单销售手册入口沿用 operationManualsByView。
- DEV-ORDER-DRAWER-CLEANUP 归属 PR-714，产品状态保持 review 待 Van 验收。
- 首次交付集成 develop 并发布 development；2026-10-22 Van 随后明确要求将这三处修复合入 main 并部署 production。生产订单仍只读，业务验收另记。

## 开发发布回执

- PR #242：https://github.com/Backstage2539/KFerp/pull/242；实现 63cb7094，合入 develop 05692ae0da240383f1f9fe6cd45d6fca8b1fdbdd。
- 命令：KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development。服务器 Vue1498/1498、miniapp425/425、typecheck/24页产物、Go及镜像内Go全部通过，退出0。
- 2026-10-22 20:35 Asia/Shanghai 开发服务启动，restart_count=0；镜像 sha256:a9606dd940cb8e60651cf640e8167d29b0ee8fb71cd04229d2dbe6553faaf013。
- dev.qacoohee.com 登录/订单shell/已认证需求API均200，未认证保护API401；DEV-ORDER-DRAWER-CLEANUP已出现在开发需求API。
- 公开 main-D1Gc9AtS.js、OrderEntryView-DZcVEYC0.js、OrdersView-CurDEOzO.js 与运行容器字节一致；费用统一展示标记存在，顶部 workflow-save 标记消失。订单销售与样品手册HTTP200且与容器一致。
- 回滚源码：/opt/stacks/erp/orderapp.backup.deploy-20261022202604-05692ae0da24；回滚镜像：kferp-orderapp-rollback:development-20261022202604-05692ae0da24。
- 生产应用启动时间/镜像与发布前相同；入口配置原已一致，未修改。两套数据库健康；未保存真实业务订单。只尝试清理24小时前可重建Docker构建缓存，回收0B，未删除镜像/卷/数据。
- 原始日志：outputs/order-drawer-cleanup-20261022/deploy.log、runtime-smoke.json、production-before.txt。未导出本地小程序包，未上传微信。本文发布后回执为文档提交，运行代码身份以05692ae0为准。

## 正式发布回执

- Van 明确授权将三处修复合入 main 并部署正式环境；发布分支以 main9cfc97c0 为基线，仅带入对应修复、测试和手册，main 既有 BOM/业务保护保留。
- PR #244：https://github.com/Backstage2539/KFerp/pull/244；main/正式运行提交08a352704fcc9f3704345cdc0d312e0e18d638e9。
- KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production，退出0。合并前本机前端1498项、构建和全量Go通过；服务器Vue1498、miniapp425、typecheck、24页产物、全量Go和镜像内Go全部通过。
- 2026-10-22 20:58:46 Asia/Shanghai 正式应用启动，restart_count0；镜像sha256:ec163a32f6f47111ff334bbb17ecc237ac9ee573753067105fccae1864e71a01。
- 正式登录/订单shell/已认证需求API和两份手册HTTP200，未认证保护API401。DEV-ORDER-DRAWER-CLEANUP可见；公开main、OrdersView和OrderEntryView JS与容器字节一致，费用去重标记存在、workflow-save标记消失。
- 正式ERP新标签页只读查看原订单117：寄件人为“跟随本次寄件人：营业部”，A区域仅一份费用明细，服务费3元、预估10.10元，底部唯一“保存订单”。未点击保存、核算、取消、增删或更换商品；原115/117的收件信息、明细、费用与版本摘要哈希在部署前后相同。
- 开发应用启动时间未变，两套数据库健康。服务器仅在发布锁内清理可重建Docker构建缓存，释放2.986GB，未删除业务数据、镜像或卷。
- 回滚源码：/opt/stacks/erp-production/orderapp.backup.deploy-20261022205007-08a352704fcc；回滚镜像：kferp-orderapp-rollback:production-20261022205007-08a352704fcc。
- 证据：production-deploy.log、production-runtime-smoke.json、production-orders-before.json、production-orders-after.json、production-bottom-save.png，均在outputs/order-drawer-cleanup-20261022/。截图和原始运行证据保留本机，不提交业务数据。
- 未导出本地小程序包、未上传/发布微信。本文发布后回执为文档提交，正式运行代码身份以08a35270为准。Van业务验收另记。
