# PR-726 完工入库纠错与数量一致性

## 口径与范围

Van 确认截图中的14袋是误报，选择从入库页直接更正。正确数量由现场填写；不自动将生产记录改成计划数，不开放超计划入库，不改写已入库历史。PR-725 已提供合并入口，但其失败 fixture 只验证错误反馈，并未解决本次数量口径问题。

## RED / GREEN

- RED：`TestReceiptCorrectionMisreportAndAudit` 在修复前返回 `available_qty:14`，计划为1，未返回报工阻塞原因（2026-10-24）。
- RED：前端同货主不同销售客户仍拆组，异常来源被计入可入库17而非3；新增部分模式与固定重试测试先失败。
- GREEN：针对性 Go API/数据库测试已通过，使用本机 PostgreSQL 的 `test_prod_flow_*` 隔离 schema，退出后清理；不连接生产写接口。
- GREEN：前端 helper 9项、全量前端1,573项通过，Vue生产构建通过；`go test ./...` 全包通过。
- 验证项：误报更正必填原因/版本、整数、拒绝超计划、原值新值审计、不改变库存、旧入口门禁、已入库拒绝更正、40+60袋分次入库保留累计100、相同/不同请求并发、更正与入库竞争、请求重放。
- 数据断言：累计成品100袋/22700g、预留耗用22700g及包装100个、两张增量入库单；重复请求不增加单据。

## 扩展数据库回归

开启本机数据库运行完整生产 API 包出现17个历史失败，集中于旧计划供给、工位人员和规格转换 fixture；同一组用例在未修改的 `origin/develop`（20c728c1）归档副本上全部复现，新增失败集合为空。未删除或跳过失败用例。本次相关入库、纠错、并发、原子过账与部分入库定向数据库测试通过。完整标准 Go 门禁不启用数据库变量，数据库证据以上述实际运行结果单列。

## 隔离浏览器

用实际 Vue 组件和完全本地模拟接口验证：4张工单按227g/454g归2组；227g含不同销售客户、部分入库和误报来源；14→1更正后合计可入7袋。填写4袋显示来源3+1，提交结果4袋、2张单据，剩余3袋继续待办。窄屏表单可见袋数、冻结规格、仓名和来源分配。本地 fixture 没有连接业务写接口。

## 手册与进度

- `docs/OP_MANUAL_PRODUCTION.md`、`docs/OP_MANUAL_STOCK.md`。
- 页面“操作帮助”进入生产手册，带返回完工入库入口。
- PR-726；DEV-862～865。产品验收保留给 Van。

## 发布

### 开发交付

- 功能提交 `182e22d4a3d9d7dae2bb8550db7a05c7ae0ab118`；PR #287 合入 develop `7615ce4e85c6fa65856173a8ddef86d194cc04b2`。
- 2026-10-24 正式运行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`。Vue 1,573项、小程序432项、类型检查、构建、宿主及镜像内 Go 全包门禁通过；开发登录外部检查 HTTP 200。
- 在已部署开发源码上连接 `erp_postgres`，运行 `^TestReceiptCorrection`，使用独立 `test_prod_flow_*` schema；更正审计、40+60增量入库、同请求/不同请求/更正与入库并发全部通过（35.845s），测试结束清理隔离 schema。
- 开发受保护入库接口 HTTP 200，页面 HTTP 200；主入口与入库组件公共资源哈希均与运行容器一致。入库组件 `ProductionAcceptanceView-wzf24MzA.js` SHA-256：`1b0d9185f125b53fcee9d2e0ee3b8f0a559d32d20bac5f11892fdd71efe49ced`，包含更正接口、数量框及提交版本标识。
- 开发镜像 `sha256:8d139d99532f9d86f656c05deb134caea050e2f09c2adce7ed8f1dfc891edb8b`；源码回滚 `/opt/stacks/erp/orderapp.backup.deploy-20261024102306-7615ce4e85c6`；镜像回滚 `kferp-orderapp-rollback:development-20261024102306-7615ce4e85c6`。

### 生产集成

- 在最新 main `46387a92` 上单独集成已验证变更，保留生产分支既有发布记录；候选 `9c0e41ae` 的全包 Go 与纠错数据库测试通过。PR #288 合入 main `f4303055f4c28c7e9ab2362375da98211a51f86e`，合并树与已测试候选完全一致。
- 发布前确认部署锁空闲；按已有授权清理 Docker builder、Go 编译/模块、npm、dnf 可重建缓存，保留数据库、卷、镜像与源码回滚锚点。正式生产构建前可用空间 `5,969,403,904` bytes；两套 PostgreSQL 均健康。
- 2026-10-24 正式运行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`，六阶段门禁及外部登录 HTTP 200 全部通过；运行版本 `f4303055f4c28c7e9ab2362375da98211a51f86e`，镜像 `sha256:f01a1c99d231cf73bc6de3f6efb6f4f68ed4ec9c57faafa3ad9423c22e438041`。
- 回滚镜像 `kferp-orderapp-rollback:production-20261024103714-f4303055f4c2`，对应发布前镜像 `sha256:cc4e9622ae2f4490d974660783df17504a221111beb5593c3398d040d961542f` / 运行版本 `7883f1aa3beb0f5ab76646758f8b88075484d168`。源码回滚目录 `/opt/stacks/erp-production/orderapp.backup.deploy-20261024103714-f4303055f4c2`。
- 受保护页面、入库接口、生产和库存手册均 HTTP 200；公共主入口及 `ProductionAcceptanceView-wzf24MzA.js` 与运行容器哈希一致，更正接口、提交版本和袋数输入标识齐全。
- 生产浏览器仅打开页面：待办8张工单归7组，包含跨销售客户同货主的两张227g商品工单；冻结227g/454g规格和成品仓/客户仓名称可见。目标误报工单显示“报工14袋、计划1袋、需更正”、可入库0，来源末工序卡可更正。未点击更正或入库控件。
- 发布前后只读数据库快照一致：已提交完工入库2张、更正审计0条；目标卡投入14/产出14/状态completed，目标工单入库0张。比较忽略 psql 的 BEGIN/SET/COMMIT 输出标签，仅比较业务值。
- DEV-865 已在开发及生产进度表更新为 done，并分别记录进度操作日志；PR-726 保持 review / Van。仓库中的后续交付记录仅补充证据和进度种子，不代表应用运行版本再次发布。
- 发布结束、部署锁释放后再清理 Docker builder / Go 编译 / npm 缓存；根盘最终可用 `5,557,522,432` bytes（约5.2 GiB），两套应用运行、两套 PostgreSQL healthy，回滚镜像保留并核对一致。
- 真实报工更正及入库由现场执行，Van 业务验收待确认。小程序构建通过，本次未上传或发布微信版本。
