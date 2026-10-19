# PR-703 / PR-704 生产交付

## 授权与代码范围

- Van 于2026-10-19明确要求核对 develop、合入 main、部署生产并交付生产小程序包。本次授权扩展了此前仅开发交付的范围；微信上传、审核和发布仍单独处理。
- 价格表异常是开发验收绑定遗留，已通过审计接口清理，不需要额外修改业务代码。此前客户库存代发和小程序体验优化已全部合入 `develop@78d830f3`。
- 发布分支 `codex/release-customer-dropship-20261019` 从 `main@6071c848` 合入上述 develop，得到 `a552928c38577df87a34a4fed0bea6d426d9dcc3`；保留 main 原有物料 BOM 回填兼容修复。
- PR [#220](https://github.com/Backstage2539/KFerp/pull/220) 合入 main，实际生产代码与生产包版本均为 `0a12b99831a32bbcad6dfcde702a40c58bcdce16`。发布树与预检树完全一致。

## 发布门禁

- 生产参数预检：Vue1420项、小程序364项、类型检查、两端构建、24页面/96页面文件完整性、完整 Go 测试及隔离 Docker 镜像构建均通过。
- 独立本机 PostgreSQL：重复建表、24袋留厂权益、部分入库与真实批次映射、发货门禁、补购2袋返回、原单生产需求、并发占用、取消释放、客户会话权限、公共价格表绑定与跨客户隔离均通过。
- main 物料产出回填真实数据库回归通过。价格绑定数据库用例首次因未开启专用集成开关被跳过，随后开启 `KF_RUN_POSTGRES_INTEGRATION=1` 并指定独立测试库，两项真实运行通过，不把跳过视为通过。
- 正式部署再次运行同一套服务器测试/构建门禁；镜像内部 Go 测试使用 `set -e`，测试或构建失败不会进入源目录切换。

## 生产部署与恢复核验

- 干净 main 发布副本：`/private/tmp/kferp-customer-dropship-production-20261019`。
- 命令：`KFERP_MINIAPP_EXPORT_DIR=/Users/yiiiple-work/KFerp-customer-dropship-production-20261019/KFerp-miniapp-mp-weixin ./deploy_orderapp.sh production`。
- 开始前磁盘仅约602MB可用；在发布锁下仅清理 Go 编译和 Docker 构建缓存，恢复约5.5GB。业务数据、源码备份、卷、运行镜像和回滚镜像保留。
- 数据库备份：`/opt/stacks/erp-production/backups/pre-customer-stock-20261019T111247Z.dump`，12756501字节，`pg_restore --list` 读取2329行；权限0600。仅确认备份可读，未声明做过完整恢复演练。
- 源码备份：`/opt/stacks/erp-production/orderapp.backup.deploy-20261019191436-0a12b99831a3`。
- 回滚镜像：`kferp-orderapp-rollback:production-20261019191436-0a12b99831a3`，镜像ID与发布前生产镜像一致。
- 客户端 SSH 在镜像构建末段超时，命令返回255；服务端后台继续完成切换。因此没有重复执行部署，而是重新连接检查实际状态，并补取生产包。保留原中断日志，不把该命令记为退出0。
- 实际生产镜像：`sha256:5fc4ac6ea848da5e1323f1a9b4ba0de21642edb7b95fb16cbbe3bd406a3b4529`。容器启动于2026-10-19T11:23:39Z，running、restarts0，PostgreSQL healthy。
- 开发应用启动时间保持2026-10-19T03:48:02.302907591Z；未做开发应用重启。
- 认证网页履约入口、履约设置、物流模板、库存代发手册、PR-703/704记录均200；公网登录200；匿名履约设置及小程序目录均401。
- 生产喜沁价格绑定前后完全一致。客户307订单和订单明细按发布前全部原字段计算指纹，前后完全一致；未复制开发验收配置、订单或库存到生产。
- 账单门户以本轮员工技术检查身份访问，发布前后均403，保留原权限边界；本轮未用客户身份在生产下单、占库存、发货或进行账单业务验收。

## 生产小程序包

- 目录：`/Users/yiiiple-work/KFerp-customer-dropship-production-20261019/KFerp-miniapp-mp-weixin`。
- ZIP：同目录上级 `KFerp-PR703-PR704-miniapp-production-20261019.zip`，285670字节。
- SHA256：`dcf63d691fc4af188d0e7f3cfc982a536f155fb72e3be84b28afd43a95926140`。
- `RELEASE_INFO`：commit为上述0a12b998完整版本，environment=production，api_base=`https://erp.qacoohee.com/app`。
- 24页面、96页面文件完整；227个下载文件与服务器发布产物逐字节哈希相同。ZIP解包校验通过，包含project.config.json、app.json、RELEASE_INFO和PAGE_FILE_MANIFEST，未混入开发API地址。
- 包仅导出和校验，未上传微信、提审或发布；未把技术部署记为 Van 最终业务验收通过。

## 证据与记录

私有证据目录：`/Users/yiiiple-work/KFerp-customer-dropship-production-20261019`。包含预检、数据库集成、正式部署中断原日志、缓存清理、数据库备份目录校验、后台完成恢复证据、运行/API检查、订单指纹、价格绑定快照及产物哈希。凭证和完整业务快照不提交仓库。

本记录是部署后文档补记；生产应用与生产包固定为0a12b998，纯文档补记不重复部署。
