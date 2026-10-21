# PR-709 录单运费自动核算与直接调整

2026-10-21；用户已确认：两端同步，手填随本单收取，支持未出账可编辑旧单，合入 develop/main、部署生产并交付包。Van 业务及微信发布验收单独记录。

## 实现与边界

新员工普通录单 order_freight_v2，金额空白自动，0是明确金额，包邮优先且无原因；后台成本缺配置不改变手填收费。新接口按 orders.write，县简称仅供搜索，规范行政代码参与计费。客户自助／旧客户端 v1 和代发费用调整保持原规则。父表单保存状态，恢复草稿重报价，复制不带收款／账单。

已有运费快照及数据库延迟一致性约束；未出账编辑与实际导入同锁顺序，token冲突拒绝旧报价。无关编辑保留旧协议，显式调整升级；既有模板和单位重量冻结。待确认内容包含运费，接受／拒绝同版本处理。真实收款不可随本次调整改写，减费超收阻止，增费保留未收余额。

## 测试证据

- RED：初始 domain order_test.go 因 QuoteOrder、OrderProtocol、ManualAmountCents、ResolveAddress 缺失失败；网页 order-freight.test.js 因共享辅助模块缺失失败。随后实现并 GREEN。
- Go 全套通过；真实隔离 PostgreSQL（postgres:///postgres?host=/tmp，非生产）定向覆盖新单60+20=80、旧单无关编辑不迁移、显式转换、待确认不提前计费、确认生效、数据库拒绝直接修改、实际成本导入／修订／撤销不重复应收、编辑与出账竞争只有一方成功且无死锁、收款减费保护／增费余额。
- 网页全套1461；小程序393及typecheck通过（最终门禁记录见交付补充）。
- 本地真实运费组件交互：自动识别、20元应付80、包邮应付60并禁用金额、取消恢复80、孟连县搜索完整路径。模拟请求仅使用合成数据，没有线上订单写入。
- Mini真实SFC脚本及客户端模板通过自定义Vue host renderer 验证自动地址、无商品等待、商品自动报价、包邮开关及金额保留。
- 证据目录：outputs/pr709-freight-v2（主工作区私有输出）。

## 手册

单一来源 docs/OP_MANUAL_FREIGHT.md、OP_MANUAL_ORDER_SALES.md、OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md；已有Vue手册入口继续读取同一Markdown，新字段说明直接进入录单页面。

## 发布

已通过最终门禁并集成 develop/main。生产运行版本与包一致为 2efe9bf3e1c0825058cc291f5592fa5515a682ea；开发最后收款保护同步发布记录见下方交付补充。

## 确认版本读取隔离补充

发布前真实 PostgreSQL RED 复现：通用 OrderViews 将待确认运费投影到财务读取。拆分 OrderEditorViews 仅用于录单编辑，通用／财务 OrderViews 始终读取已接受快照；编辑可见待确认值、财务在确认前不可见、确认后生效的同一用例 GREEN。未做生产业务写入。

## 旧已付款状态保护补充

真实 PostgreSQL RED 覆盖“已付款／无凭证字段”的旧单，原已付80不能因运费20改15而静默变成已付75。复用销售领域 IsFullyPaid，将原单已付总额纳入保护；增费形成部分收款80与未收余额10。定向与Go回归通过后重新发布。发现时生产仍在构建阶段，已中止该次构建，未提升该版本生产应用。

## 生产交付补充（2026-10-21）

- GitHub PR #232/#234/#236/#238 集成 develop；PR #233/#235/#237 选择性集成 main，避免携带其他开发需求。功能分支 codex/order-freight-v2-20261021 已推送 a03a7268286b53ea7f402f4183bf00b7995f7159；本记录后续提交仅补证据，不改变运行产物。
- `./deploy_orderapp.sh production` 成功，main／生产／小程序均为 2efe9bf3e1c0825058cc291f5592fa5515a682ea。生产容器运行、重启0，PostgreSQL healthy；公开登录200、受保护入口匿名401。镜像 sha256:a719b0678774ab0abc9bae2e34dad6108d087cd3d84f89522cc7f28546a234d8，启动 2026-10-21T06:49:02.785635942Z。
- 数据备份 `/opt/stacks/erp-production/backups/pr709-before-freight-v2-20261021T0556Z.dump`；源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261021144041-2efe9bf3e1c0`；回滚镜像 `kferp-orderapp-rollback:production-20261021144041-2efe9bf3e1c0`。空间不足风险通过清理可再生缓存缓解，未清理业务数据或回滚锚点。
- 生产只读核验：孟连县搜索／规范地址解析、PR709记录、黄艳当前目录、手填20和包邮报价、该客户保存地址自动识别均通过；业务订单写入0。普通销售政策未绑定模板，自动模式返回配置提示；现有模板1个，不擅自选择收费模板或填入费率。手填与包邮不受该缺口影响。
- Go全套通过；真实隔离 PostgreSQL 定向用例通过，包括待确认财务隔离、旧已付款状态、同运费但商品应付变化的已收保护。网页1461；生产分支小程序376／66文件，开发分支393／67文件；两端构建、小程序类型检查通过。小程序差异来自开发分支另有已验证需求。
- 包 `/Users/yiiiple-work/.codex/worktrees/ccd2/KFerp/outputs/pr709-freight-v2/KFerp-miniapp-production-PR709-2efe9bf3.zip`；24页面、96清单文件、238 ZIP文件、283741字节，CRC／逐文件字节一致性通过；SHA256 `6afa2de1fc52743ea6c1b0a0c90a208642b36d066c59fa21ea359defe081a810`。固定导入目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，需关闭旧DevTools项目后重新导入。
- 本地真实组件交互截图 `outputs/pr709-freight-v2/final-freight-ui.png`，仅合成数据。未将隔离测试或接口报价计为生产业务验收；Van业务验收及微信上传／审核／发布仍待单独执行。
- 生产包另外核对编译后325处相对模块引用，无缺失文件；实际环境配置中生产API地址正确。
- `./deploy_orderapp.sh development` 最终成功，develop／开发运行版本 5bd6a43c5c0e44cf3367628c6b344824a1cff2ac；运行／重启0／PGhealthy／登录200；最终只读报价、地区解析及需求API通过，业务订单写入0。开发隔离客户没有地址，未将该项记作开发客户地址验收；生产客户地址识别已实测通过。
- 开发源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20261021145100-5bd6a43c5c0e`；回滚镜像 `kferp-orderapp-rollback:development-20261021145100-5bd6a43c5c0e`；镜像 sha256:35f4d18818f8e1931a390ce95525c52af647d40acb19c0bd7908fca287f162d8，启动 2026-10-21T07:00:46.517731477Z。开发导入目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev` 版本相同，生产包不受其影响。
