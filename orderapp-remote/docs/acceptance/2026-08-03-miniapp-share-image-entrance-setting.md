# PR-575 员工小程序图片分享入口系统开关验收记录

## 当前状态

- 产品需求：`review`。
- 开发需求：`done`。
- Van 业务验收：`todo`。
- 代码、全量门禁和文档已完成；功能提交 `23ad8cce` 已推送并随本记录合入 `develop`。
- Van 已授权将当前 develop 合入 main 并部署 production；release merge `a06aa95e` 已部署，production 小程序固定包已同步。微信 DevTools 上传、审核或正式发布未执行，不宣称已在微信现网生效。

## 用户需求

员工小程序把销售单或发货单图片发送给客户后，微信图片消息下方可能显示小程序入口。系统需要提供全局开关，并在员工个人中心只向管理员开放，用来决定所有员工后续直接分享的图片是否携带该入口。

## DEV 合同

### DEV-575-SHARE-ENTRANCE-SETTING-API-AUDIT

- 配置键固定为 `miniapp.share_image.need_show_entrance`，保存在既有 `app_config`，不新增数据库表或迁移。
- 没有保存记录时返回 `true`，保持升级前微信图片分享行为；管理员可明确保存 `true` 或 `false`。
- `GET /api/mini/employee/share-settings` 允许合法销售和管理员读取当前全局值；`PUT` 同时要求员工身份、`admin` 角色和 `settings.write` 权限。
- 客户、普通销售、无令牌、过期令牌、缺权限和缺少布尔字段的请求不能写入配置。
- 配置写入按键加事务锁，并与 `audit_logs` 的操作者、配置项、旧值和新值在同一事务提交；审计失败时配置回滚。
- 操作日志把 `ui_setting` 显示为“系统设置”，菜单显示“系统 / 小程序设置”，字段显示“分享图片携带小程序入口”。

### DEV-575-ADMIN-PROFILE-SETTING

- 个人中心仅在 `account_type=employee`、角色含 `admin` 且权限含 `settings.write` 时显示开关。
- 设置区域明确说明这是对所有员工之后分享生效的全局开关，不是本机偏好。
- 加载期间或普通读取失败时开关禁用；失败可重试，不清空有效员工登录，也不把未知值误保存为关闭；认证失效时清除失效会话并返回登录页。
- 保存成功以服务端回包为准；失败恢复上一次已保存值并提示。

### DEV-575-IMAGE-SHARE-RUNTIME

- 销售单和发货单 PNG 每次分享前调用设置读取接口，避免把全局值长期缓存到某个员工会话。
- 微信直接图片分享显式传入 `needShowEntrance=true/false`，不依赖微信版本默认值。
- 设置读取失败时提示并安全回落为 `false`；登录失效仍按原流程清除会话并重新登录。
- PDF 不读取、不使用该设置；PNG 像素内容、单据快照、版本和下载鉴权不变化。
- `wx.showShareImageMenu` 不可用或调用失败时继续回退图片预览。预览接口没有 `needShowEntrance` 参数，该路径的入口由微信客户端控制。

### DEV-575-DOCS-ACCEPTANCE

- 同步根目录与线上需求/验收、PR/DEV 种子、小程序员工 ERP、设置审计和总索引。
- 已发送给客户的历史微信图片消息不能被追溯修改；开关只影响保存后新发出的直接图片消息。
- 合入代码、服务器部署、development/production 小程序构建固定包、微信上传/审核/正式发布分别记录，不能互相替代。

## TDD RED 证据

- 图片分享测试首次加入关闭和开启双态后，开启用例仍收到硬编码 `false`。
- 订单详情测试首次要求分享前读取设置并传 `needShowEntrance` 时，页面没有设置 API 或参数传递。
- 个人中心测试首次要求员工管理员条件、开关和读取/保存接口时，页面没有相应内容。
- Go API 测试首次编译时不存在 `registerMiniEmployeeShareSettingsAPI` 和配置键，证明后端尚无小程序设置边界。
- 原子审计测试首次失败于缺少 `pool.Begin`，证明既有 `app_config` 更新与审计分两次提交。

## 自动化验收矩阵

- [x] 合法销售 GET 无配置时返回 `image_need_show_entrance=true` 且 `can_manage=false`。
- [x] 管理员 PUT 可保存 `false`，回包返回 `can_manage=true`，Store 收到小程序员工 actor、固定 key 和明确布尔值。
- [x] 普通销售、客户、无令牌和缺字段 PUT 分别拒绝且 Store 写调用为零。
- [x] `app_config` 更新、审计插入和提交使用同一事务，并按配置键加事务级串行锁。
- [x] 操作日志对象、菜单、功能和字段使用中文可读名称，摘要包含管理员及旧值/新值。
- [x] Miniapp API wrapper 使用稳定 GET/PUT 路径、Bearer token 和明确请求体。
- [x] 个人中心只在员工管理员且有设置权限时显示开关，加载/保存失败逻辑已受页面合同保护。
- [x] 图片分享工具在关闭和开启时分别把 `false/true` 传给微信；失败仍回退图片预览，PDF 行为保持。
- [x] 订单详情每次 PNG 分享读取设置并传给下载分享链，读取失败回落不携带入口。
- [x] 完整 Go 套件、miniapp 全量测试、类型检查、development 构建、ERP Vue/Vite 构建、支持合同和变更校验通过。

## GREEN 证据

- 定向 Go：customerportal/support 的设置 API、权限、缺省值、提交后二次读取、非法持久化值、同事务审计、中文日志筛选与搜索测试通过。
- 定向 Miniapp：customerPortal、fileOutput、employeeOrderDetailPage、mainTabs 共 39 项测试通过。
- 完整 Go：`go test ./... -count=1` 及受影响包 `go vet` 通过。
- Miniapp：21 个测试文件、152 项测试通过；`vue-tsc --noEmit` 与 development `mp-weixin` 构建通过，编译产物包含管理员条件开关、实时设置读取和 `needShowEntrance` 双态透传。
- ERP 前端：Vue/Vite production build 通过；操作日志新增“系统设置”类型筛选。
- 后端和小程序前端两轮独立复核均无开放 P0-P2；功能分支 `codex/miniapp-share-image-no-entrance-20260803` 的 `23ad8cce` 已推送，集成提交合入 `develop`。
- Development 发布：受控发布流程完成服务端 Vue、小程序、Go 与 Docker 构建门禁；`erp_orderapp` 和数据库容器正常，外部 `/app/login` 返回 200，需求 API 命中 PR-575，新设置路由无 mini token 返回 401，近 10 分钟无 panic/fatal/error/SQLSTATE。
- Development 小程序固定包：`RELEASE_INFO` 为 `ee77f730` / `development` / `https://dev.qacoohee.com/app`，52 个 `PAGE_FILE_MANIFEST` 文件和 PR-575 管理员开关、设置 API、`needShowEntrance` 编译标记复验通过，未混入 production API。

## Van 验收清单

- [ ] 使用员工管理员登录小程序，进入个人中心，确认显示“分享图片时携带小程序入口”；普通销售和客户账号均不显示。
- [ ] 关闭开关并刷新个人中心，确认仍显示“不携带入口”；使用另一名员工分享一张销售单图片和一张已发货订单的发货单图片，客户收到的新图片消息下方不显示小程序入口。
- [ ] 重新开启开关，另一名员工无需重新登录，再分享销售单/发货单图片，确认新图片消息下方出现小程序入口且可进入当前小程序。
- [ ] 在 ERP 操作日志按管理员姓名或“小程序设置”搜索，确认两次修改分别显示字段、旧值和新值；普通销售伪造保存没有成功设置日志。
- [ ] 抽查 PDF 分享、图片像素内容和单据版本，确认开关没有改变文件内容或多生成版本。
- [ ] 抽查一条修改前已发送的微信图片消息，确认历史消息保持原样，理解系统不能追溯修改客户聊天记录。
- [ ] 若设备回退到图片预览，单独记录微信版本和回退结果，不把该客户端菜单结果当作 `needShowEntrance` 直接分享能力的缺陷。

## 部署与发布边界

- development 服务器：已部署 `ee77f730`（包含 PR-575 `53de06fb`）；源码回滚点 `/opt/stacks/erp/orderapp.backup.deploy-20260803223935-ee77f730f9e3`，镜像回滚点 `kferp-orderapp-rollback:development-20260803223935-ee77f730f9e3`。
- production 服务器：已部署 `a06aa95ebe38d7b91806cd234032c0cc3bb62a7e`；数据库发布前备份 `/opt/stacks/erp-production/backups/pre-deploy-20260805232757-a06aa95ebe38.dump` 已通过清单和隔离恢复验证，源码回滚点 `/opt/stacks/erp-production/orderapp.backup.deploy-20260805233037-a06aa95ebe38`，镜像回滚点 `kferp-orderapp-rollback:production-20260805233037-a06aa95ebe38`。
- development 固定小程序目录：已同步 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，上一包备份 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260803224730-ee77f730f9e3`；production 固定目录已同步 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，上一包备份 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260805233618-a06aa95ebe38`。
- 微信开发者工具：未上传、未提交审核、未发布。
- 旧已发布小程序版本不会因为服务器部署或固定包同步自动获得该设置；需关闭旧项目并重新导入 production 固定目录，再执行 DevTools 上传、微信审核和发布。
