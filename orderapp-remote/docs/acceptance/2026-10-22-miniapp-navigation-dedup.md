# PR-712 客户小程序入口去重与导航整改

## 范围与根因

首页动作、摘要和底栏分别硬编码；底栏去重后摘要重新添加同业务入口，且订单的 source=official 参数未同步。源码片段测试没有组合检查实际整页导航，手册留下互相矛盾的历史描述。

- DEV-829：去掉四类摘要、客户及访客价格表横幅、重复切换用户操作；删除摘要查询。
- DEV-830：共享目的地、权限、子页底栏；正式订单保留筛选及代加工发货入口。
- DEV-831：手册、回归、dev/main/生产与匹配小程序包交付。

## TDD 与自动验证

- 首页实际组合 RED：/tmp/pr712-home-red.log，11项中8失败，分别命中三个主题的重复摘要、摘要请求、客户/访客价格横幅及重复账号操作；修复后相关18项GREEN，/tmp/pr712-home-green.log。
- 导航 RED：/tmp/pr712-nav-red.log，8项实际断言失败，另新共享目的地尚未实现；修复后7文件59项GREEN，/tmp/pr712-nav-green.log。
- 合并最新 origin/develop@7684c047 后：完整小程序71文件425项通过，/tmp/pr712-integrated-mini.log；类型检查及生产mp-weixin构建通过。
- Vue1478项与手册解析3项、生产Vue构建通过；customerportal/support两个Go接口与契约包通过，/tmp/pr712-api-integrated.log。
- 独立代码复审：费用权限、员工隔离、processing库存入口、official关联订单及草稿保存均无阻塞。客户/场景草稿存储与onHide保存未改，未新增业务写入。
- API 沿用后端现有权限和客户隔离，无业务写入或数据库迁移。

## 手册

- docs/OP_MANUAL_CUSTOMER_PORTAL.md
- docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md，Vue/Vite customerPortalManual/customerFulfillmentManual 直接读取此单一来源。

## 交付与边界

- 开发和生产环境已分别交付并通过运行检查，最终小程序包已同步DevTools并完成一致性校验。微信上传/审核/发布与Van业务验收单独记录。

## 发布前观察

- WeChat DevTools 当前项目 KFerp客户中心，吴国宏已登录；只读复现旧首页价格横幅及待处理订单、客户库存、待对账三张卡片。
- 服务器初始空闲约1.1GiB，构建尚未启动。仅清理两个闲置Go缓存后约1.3GiB；为保留完整回滚能力，对2026年8/9月历史源码备份逐份完整压缩，验证gzip、tar目录与逐内容比较、SHA256后保留归档，当前及10月source/DB/镜像不动。服务器归档清单 /opt/stacks/source-backup-archives-20261022.tsv，本地证据 /tmp/pr712-cache-cleanup.log。

## 候选包 DevTools 只读界面复验

- 候选包：f693f2904232ebd5b8395b54643763f4ebf2c191，生产API，24页/96页面文件。完整树校验后替换当前项目，保留原项目目录备份和private config；重开项目后吴国宏会话有效，无需重新登录。此时服务器尚未部署PR-712。
- 吴国宏首页只剩一件代发、无库存代发、我的库存，底栏五项保留；无价格横幅、待处理订单、客户库存、待对账。
- 我的库存显示吴国宏唯一获权仓；点击高亮的底部首页实际回到客户首页。
- 底部我的订单进入正式订单，标题为我的订单；普通客户没有代加工发货按钮。
- 个人中心只显示一个退出登录按钮；未执行退出、下单、发货或账单写操作。
- 用户切换孟也咖啡后已完成实际界面复验：首页只显示生产工单/我的库存，正式我的订单内显示代加工发货按钮；进入原发货查询页后，再点击底栏我的订单成功回到正式订单。现有记录为空，未创建记录或触发取消/发货写入；完整取消/包裹/轨迹组件保留，自动回归已覆盖接线。
- DevTools重新编译刷新模拟器事件状态后导航正常，会话保持有效；最终交付已使用服务器导出的匹配包，详见下方最终产物记录。

## 磁盘恢复完成

- 2026-10-22 11:41:56，161份8/9月旧源码备份（dev85/prod76）经gzip、目录集合、tar逐内容/元数据及SHA256校验后完整保留为tar.gz；原占用8,297,906,176B，归档占用3,214,168,064B。
- 连同闲置Go缓存清理，共释放约4.990GiB；持锁最终空闲6,449,160,192B（6.006GiB）后自动停止并释放部署锁。当前源码、全部10月备份、近期回滚锚点、数据库、业务数据及镜像未改变。
- 清单和恢复说明：/opt/stacks/source-backup-archives-20261022.tsv、/opt/stacks/source-backup-archives-20261022-RESTORE.md；本地交付evidence目录留完整副本。

## 开发环境交付

- GitHub PR #239 已合并；origin/develop、运行与小程序包均为 aca709f6b6312f4d3e9e7cf8ac3d70c0be6b1d50。
- ./deploy_orderapp.sh development 完成全部服务端Vue1478、小程序425/类型/构建、完整Go及Docker内部Go门禁。
- 备份：/opt/stacks/erp/orderapp.backup.deploy-20261022114218-aca709f6b631；回滚镜像 kferp-orderapp-rollback:development-20261022114218-aca709f6b631。
- 运行检查：app running/restarts0、DBhealthy；login200；匿名需求/mini身份/库存401；认证三份手册、PR-712及DEV829-831可见；服务器小程序SHA/API匹配，共享导航与无摘要产物检查通过。
- 日志：/tmp/pr712-development-deploy.log、/tmp/pr712-development-smoke.log；开发包固定目录 /Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev，24页/96页面文件。


## 生产交付与最终产物

- GitHub PR #240 合并main，服务器及正式小程序包 b080846f1f2dc29adef269adfc3cd62ac2f06847；命令 ./deploy_orderapp.sh production 于2026-10-22 12:07完成。本次收据更新仅文档，不重新部署。
- 全部生产门禁：Vue1478、小程序71文件425项、类型检查、生产Vue/小程序构建、完整Go及Docker内部Go全部通过；未新增迁移或业务数据修复。
- 源码备份 /opt/stacks/erp-production/orderapp.backup.deploy-20261022115811-b080846f1f2d；回滚镜像 kferp-orderapp-rollback:production-20261022115811-b080846f1f2d。
- 运行镜像 sha256:e48d8364b3a54b578ab77af0473460a7676a03a76a708e3c3128bf3162ee9559 与部署tag一致；app running/restarts0、DBhealthy、login200、匿名受保护接口401；认证三份手册及PR-712/DEV829-831可见。容器内首页静态入口及客户门户手册hash与实际部署源码一致。
- 只读生产mini API：吴国宏488有效上下文及代发所需商品下单权限；库存仅含其唯一授权成品仓，正常返回4行；伪造其他客户及包材查询不扩大仓库或类别权限。未提交、取消、发货或改客户配置。
- 正式导出 /Users/yiiiple-work/KFerp-miniapp-mp-weixin；当前DevTools项目 /Users/yiiiple-work/Documents/Codex/2026-10-21/xia/outputs/KFerp-miniapp-production。候选与正式包除RELEASE_INFO外全部运行文件相同；原子更新最终版本元信息，私有配置/登录会话保留。
- 交付ZIP /Users/yiiiple-work/KFerp-navigation-dedup-delivery-20261022/KFerp-miniapp-production-b080846f.zip；SHA256 599a4585ae507b00363b3d953e715a1587663a7ceb8194139c83841af17fb605。24页/96页面文件/239运行文件；导出、DevTools、ZIP逐文件一致，CRC、生产API/appid及分享hooks校验通过，无private config进入ZIP。
- 最终包重新编译后，孟也咖啡会话保持：首页仅生产工单/我的库存；我的订单显示代加工发货，打开原发货中心，再点击同栏高亮我的订单返回正式列表。无发货记录，未触发业务写入。
- 日志 /tmp/pr712-production-deploy.log、/tmp/pr712-production-smoke.log、/tmp/pr712-production-runtime-identity.log；最终包校验 /tmp/pr712-final-delivery-verification.json、/tmp/pr712-devtools-final-package.json。完整副本保留于上述交付目录evidence/。

## 验收口径

- 已完成技术交付、自动回归、只读接口和实际导航复验。取消/真实包裹轨迹保留既有组件与权限，本轮无记录可现场验收；草稿安全依据即时保存逻辑、现有store/quote测试和源码复核，未声称在最终包现场编辑恢复过草稿。
- processing-only客户的正式订单仍按原权限返回200空页，代加工发货入口独立显示，原发货列表接口允许processing；本轮没有扩大正式业务数据权限。
- 微信上传、审核、正式发布均未执行；Van业务验收待确认。PR保持review，DEV829-831技术工作done。
