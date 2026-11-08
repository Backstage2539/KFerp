# PR-750 订单微信通知验证记录

日期：2026-11-08。产品验收人：Van。需求状态：doing，真实微信验收未完成，不标记 done。

## 实现边界

- 六个节点：下单、付款、开始生产、生产完成、发货、取消。事务提交时捕获最终状态，多次更新合并、回滚不发、重复状态不发。
- 服务号按当前关注、通知偏好、客户绑定、员工权限与订单当前归属筛选。支持加密回执、有限重试、未知结果暂停，以及发送前再次校验。
- Vue 通知配置提供模板对应、逐项虚拟测试、设备配对、发送记录及人工核对；小程序提供关注确认、通知偏好及准确订单跳转。
- 专用 Mac 发送端包含钥匙串凭据、精确控件检查、当前群核对、发送成功记录读回；没有针对当前微信的已验证控件配置，默认不可发送。更换程序或控件配置会使六类验证失效。
- 操作日志覆盖配置、授权、虚拟测试、配对、撤销、重试和人工确认。旧外部 IM 规则不会被本功能自动激活。

## RED → GREEN

1. 六节点识别和模板接口新增测试先因缺失实现失败，补齐后通过。
2. 同一事务“已付款 → 未付款 → 已付款”最初产生错误付款事件；保留第一份事务前状态并对比最终状态后通过。
3. 回执早于发送响应最初无法确认送达；持久化回执并在响应落库时核对后通过。
4. 通知中具体订单登录回跳最初返回空路径；新增严格数值路径允许列表后通过。虚拟测试不链接到不存在的真实订单。
5. 当前订单已属于其他客户时，旧通知快照最初仍返回授权 true；增加发送前订单当前客户核验后通过。
6. 前端模板辅助模块先出现模块不存在，补齐后通过；小程序新增页的首页顺序、环境标识、分享和底部手势在全量测试中检出并修正。

## 自动验证

仅使用独立本地 PostgreSQL，数据库端口 55483、随机测试 schema；没有写生产业务数据。外部微信 HTTP 在集成测试中由本地测试服务替代。

- `scripts/verify_kferp.sh backend`：全 Go 测试通过；数据库集成另行显式运行。
- `ORDERAPP_TEST_DATABASE_URL=... go test ./internal/appmain ./internal/infrastructure/postgres/messagecenter -run 'TestOrderNotificationsAPI|TestWechatOfficialPostgres|TestLifecycle|TestOrderOutbox' -count=1 -v`：通过。
- `TestOrderNotificationsAPISixDesktopBindingAndGuards`：六节点服务号经真实 handler、PostgreSQL 队列、模拟 HTTP 发送和加密签名回执逐项通过；六节点群任务经领取、完整内容回报、错误设备/内容/重复回报拦截逐项通过；配对撤销、版本变更失效、未知结果核对、审计、当前客户归属以及取消订单查询通过。
- 仓储测试覆盖事务提交与回滚、同事务恢复状态、取消关注/重新关注不恢复旧授权、租约过期未知、早到回执及重复 Tick 不重发。
- `scripts/verify_kferp.sh frontend`：1761 个测试通过，Vue/Vite 构建通过。
- `cd miniapp && npm test`：74 个测试文件、446 个测试通过；类型检查与开发环境小程序构建通过。未上传、提审或发布微信版本。
- `tools/wechat-notifier/build.sh`：Mac 程序编译通过；9 条守卫断言通过。编译工具残留的重复模块声明通过临时编译文件视图处理，没有修改系统文件。
- `scripts/verify_kferp.sh changed`：通过。
- 浏览器隔离预览验证了读取模板、禁用不适用模板、字段对应、保存提示及受理/未知状态区别。页面中的消息均为虚拟数据，不代表外部已送达。

![通知配置隔离预览](assets/2026-11-08-order-notifications.png)

## 真实微信验收矩阵

| 节点 | 业务规则与接口 | 服务号实际收到 | 原微信群实际收到 |
| --- | --- | --- | --- |
| 下单成功 | 通过 | 待验收 | 待验收 |
| 付款成功 | 通过 | 待验收 | 待验收 |
| 开始生产 | 通过 | 待验收 | 待验收 |
| 生产完成 | 通过 | 待验收 | 待验收 |
| 部分/全部发货 | 通过 | 待验收 | 待验收 |
| 订单取消 | 通过 | 待验收 | 待验收 |

现场证据与阻塞：

- 通过服务器读取服务号 API 配置并成功读取私有模板列表；仅返回“订阅模板消息”，内容为 `{{content.DATA}}`，尚无适用的持续订单业务模板。本功能拒绝把该一次性模板当作订单模板。
- 此前桌面微信只暴露窗口/菜单，未暴露可靠的聊天控件。尝试控制时出现重新登录提示；用户后续确认已登录。没有继续盲输、没有绕过安全提示，也没有发送任何真实测试消息。
- 重新登录后的只读检查发现微信停在“Insufficient storage space”弹窗；尝试点击“Do not remind today”返回 `AXError.notImplemented`，弹窗仍在。没有清理聊天记录或继续发送操作，已请求用户手动关闭提示。本机可用空间约 1.6 GiB。
- 尚缺准确测试群名称、服务号身份和适用业务模板；需完成同环境回调、测试账号关注绑定、六类真实送达、内容与具体订单跳转验收。
- 当前没有微信控件适配配置；原微信群自动发送不可启用。发送端成功编译与模拟回报不构成真实微信群送达证明。
- 真实订单流程、断网/锁屏/退出/恢复等桌面场景、微信真机跳转仍待验收。

## 交付状态

工作分支：`codex/order-wechat-notifications-20261108`。产品/开发追踪：PR-750，DEV-PR750-01～04。手册：`docs/OP_MANUAL_NOTIFICATIONS.md`，前端使用现有通知配置手册入口及页面帮助。

功能提交 `2dee342c1` 已推送，合入已验证的最新 `origin/develop@15eeea3e1` 后，开发运行版本为 `184ae0944e34bf8d33e8726ba2ece0cbbcb4e7f1`。原 develop 工作目录存在其他未提交工作，使用独立干净集成目录 `/private/tmp/kferp-notifications-develop-20261108` 完成快进、推送及 `./deploy_orderapp.sh development`，没有改动原目录。

2026-11-08 开发发布回执：

- 服务器前端 1761 测试、构建，小程序 446 测试、类型检查与构建，以及两轮后端测试/镜像构建均通过。
- 源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20261108173725-184ae0944e34`。
- 回滚镜像：`kferp-orderapp-rollback:development-20261108173725-184ae0944e34`。仅开发 orderapp 重建/重启。
- `erp_orderapp` running、重启次数 0；`erp_postgres` running/healthy。启动日志未发现通知 worker 错误。
- 登录页 200；认证后的通知页面、配置、手册、PR/DEV 查询 200；未认证管理通知、小程序偏好、设备心跳均 401。两个自动发送开关关闭，设备数 0、发送记录数 0。
- 原检查脚本把 `/app/` 预期写为 401，初次因自动跟随重定向得到 200 而失败。核对当前源码与 `TestVueShellCanLoadBeforeBearerAPIAuth` 后确认入口实际为 303 → `/orders` → Vue 壳；改为禁止自动重定向并分别验证入口 303 与受保护 API 401，未改变认证实现。
- 服务器通知页面文件 `NotificationSettingsView-BUybJKk_.js` 包含本功能；服务端与本机小程序 `RELEASE_INFO` 均为上述运行提交、`development`、`https://dev.qacoohee.com/app`。
- 本机开发小程序包：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；清单 100 文件、25 页面，包含 `pages/order-notifications/order-notifications`。原包备份：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20261108174902-184ae0944e34`。
- 本机证据：`/private/tmp/kferp-notifications-development-deploy.log`、`/private/tmp/kferp-notifications-runtime-result.json`、`/private/tmp/kferp-notifications-integrated-backend.log`、`/private/tmp/kferp-notifications-integrated-db.log`。PR/DEV 保持 doing，开发发布回执与实际微信验收分别记录。
- 开发 PR 与四条 DEV 的证据说明已在同一事务更新，新增 5 条操作审计；通过需求查询与 `/api/audit` 读回确认，状态仍为 doing。未修改订单、库存、款项或通知配置。

后续回执提交仅更新跟踪说明与证据，不改变运行功能。两个自动发送开关默认关闭；正式环境、小程序平台上传/发布及真实微信发送未执行。完成真实验收前不得把此需求标记完成。
