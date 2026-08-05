# PR-579 / PR-580 合并验收证据：客户地址解析复用与外部账号工作台隔离

- 日期：2026-08-04
- 分支：`codex/miniapp-address-portal-fix-20260804`
- 范围：`PR-579-MINIAPP-CUSTOMER-ADDRESS-PASTE`、`PR-580-CUSTOMER-PORTAL-EXTERNAL-USER-CAPABILITY-TEMPLATE`
- 环境边界：自动化使用本地进程和隔离 PostgreSQL；development 发布只更新应用与开发小程序构建产物，未为验证创建或修改 development/production 客户、账号或订单。production 不在本次范围；微信上传、审核和发布未获授权，Van 的登录态业务验收仍待完成。

## 业务结论

1. ERP 客户档案和员工小程序不再各自维护地址解析规则，统一调用 `POST /api/customer-recipient/parse`；解析只返回联系人、电话和地址，不保存粘贴原文，也不产生客户写入。
2. 员工小程序的“客户维护”和录单内新增/维护客户共用同一编辑器。解析结果可以修改，只有最终保存客户才沿用既有权限、负责人规则和操作日志。
3. 外部账号与客户的 active 关联用于门户/小程序身份，不等于 ERP 工作台授权。非工作台模板和空模板可维护账号、改密码并登录小程序，但不能通过密码、短信或历史会话进入 ERP 工作台。
4. 显式 ERP 工作台绑定仍要求模板暴露工作台；零售商城、其他非工作台或未知模板继续返回 `ERP workbench unavailable for capability template`。
5. 外部用户 API 使用客户权限：读取为 `customers.read`，创建、重置密码和启停登录为 `customers.write`；其他客户履约库存接口继续使用 `stock.read/write`。
6. 外部账号创建、密码重置和登录启停使用现有 `audit_logs` 写可读业务操作日志，记录客户、外部用户、动作和已认证员工；客户端伪造 `X-User` 不影响操作者，且不保存密码明文或密码哈希。
7. 外部账号写入只允许 active 且已启用客户门户的客户；门户关闭后拒绝创建、重置和重新启用，但仍允许管理员禁用已有账号。

## PR-579-MINIAPP-CUSTOMER-ADDRESS-PASTE

### DEV-579-SHARED-RECIPIENT-PARSE-API

- 单一服务端解析器覆盖 ERP 既有的标注多行、紧凑“姓名 电话 地址”、地址在前、姓名带“收”、数字客户名等输入。
- ERP Vue 和员工小程序均调用同一个 POST 接口；小程序没有复制地址解析正则。
- 接口只读：不创建或修改客户，不缓存或记录原文。ERP actor 和员工 mini token 分别复用 `customers.read` 权限；Basic 登录和客户 mini token 不被误放行。

### DEV-579-MINIAPP-CUSTOMER-PASTE

- 共享客户编辑器提供粘贴文本和“地址解析”，同时覆盖客户维护页与录单内客户维护。
- 解析成功只合并联系人、电话、地址；不会从收件人生成或覆盖客户名称，结果仍可手工修改。
- 解析失败不改表单；解析中禁止重复请求；切换客户、关闭编辑器或修改粘贴原文后的迟到成功/失败响应都被丢弃。请求等待期间若员工手工修改了联系人、电话或地址，响应只更新仍与请求前快照一致的字段，不覆盖手工值。
- 小程序写入 `phone` 的联系电话在 ERP 客户列表和编辑抽屉继续可见；ERP 保存时同步兼容字段，既有 phone-only 客户不会因再次保存丢失电话。

### DEV-579-DOCS-ACCEPTANCE

- 已同步根目录及线上需求/验收、员工小程序手册、订单销售手册、总索引、PR/DEV/REV 种子与本证据。
- 最终客户保存继续使用现有客户 API 和操作日志；解析动作本身不写日志，日志中不保存粘贴原文。

### REV-579-MINIAPP-CUSTOMER-ADDRESS-PASTE

等待 Van 在 development 完成以下手工验收：

- 从“简易 ERP → 客户维护”分别新增、编辑客户并粘贴真实收货信息，核对三字段和可编辑性。
- 从录单页新增或维护客户重复同一输入，确认结果与 ERP 客户档案一致，保存后回到当前订单。
- 断网或输入无效文本时确认原表单保持，单独解析不新增客户或客户变更日志。

## PR-580-CUSTOMER-PORTAL-EXTERNAL-USER-CAPABILITY-TEMPLATE

### DEV-580-EXTERNAL-ACCOUNT-WORKBENCH-SEPARATION

- RED：`CreateExternalUser` 在 `retail_mall` 和空模板客户上均返回 `ERP workbench unavailable for capability template`，错误地把账号关联当成工作台授权；同一轮显式 `UpsertCustomerERPBinding` 拒绝用例正常通过。
- GREEN：隔离 PostgreSQL 中，非工作台模板和空模板均可创建、列表、重置密码并完成密码登录；随后读取 ERP 工作台上下文仍返回 `ErrCustomerERPBindingNotFound`。
- 显式工作台绑定的非工作台/未知模板拒绝用例保持通过，没有放宽 ERP 工作台边界。

### DEV-580-ERP-SESSION-GATE

- 安全复核 RED：放开账号创建后，旧 `/api/auth/login` 的密码和短信分支只校验账号凭据，仍会为非工作台 `channel_customer` 写入 ERP `login_sessions`；旧 Bearer 在模板降级后也不会实时失效。
- GREEN：ERP 密码登录在写 token 前调用工作台资格判定，Bearer 解析再次实时调用同一判定；内部员工和工作台模板外部账号的密码登录放行，零售商城、空模板、未知模板、门户关闭和失效绑定 fail closed。密码重置、登录启停、绑定替换、员工/客户启停、门户启停、账号类型、角色或模板工作台能力变化都会在正式保存事务中永久撤销旧 token，恢复配置后也不能复活；员工姓名、客户地址、门户外观和模板文案修改保持会话。短信兼容登录收窄为既有 active 内部员工，渠道客户统一拒绝且不消费验证码。
- 客户小程序的 `/api/mini/login/password` 保持独立，非工作台客户账号仍可按门户规则登录小程序。

### DEV-580-EXTERNAL-USER-PERMISSION

- RED：`GET /api/customer-fulfillment/{customer_id}/external-users` 被误映射为 `stock.read`。
- GREEN：外部用户列表 GET 映射为 `customers.read`；创建、密码重置和登录启停映射为 `customers.write`。
- 反向断言确认 `/api/customer-fulfillment/{customer_id}/overview` 与 custody 等其他履约接口仍使用 `stock.read/write`。

### DEV-580-EXTERNAL-USER-AUDIT

- 安全复核 RED：三个账号写事务原来直接提交，只有通用请求日志，没有 `audit_logs` 业务对象、动作和前后值，无法在“操作日志”按外部用户追溯。
- 创建、重置密码和启停登录改为在业务事务提交前写 `customer_external_user` 审计；审计失败时账号修改一并回滚。
- 复用已有手机号会标明复用、改名、重置密码和重新启用；自动替换旧 active 账号会显示旧账号姓名与员工 ID。操作日志页面归类到“客户管理 / 客户门户配置”，支持“客户外部用户”类型筛选和中文搜索；所有审计字段和 meta 均不得包含提交密码或生成的密码哈希。
- 审计操作者来自已认证 Echo 上下文；定向 HTTP 用例确认三条写接口均忽略客户端伪造的 `X-User`。

### DEV-580-PORTAL-ACCOUNT-WRITE-GATE

- 最终审查 RED：未开通门户、门户关闭或客户停用时仍可预先创建并启用外部账号；之后开启门户会让预置凭据直接生效。
- GREEN：创建账号、重置密码和启用登录在同一事务校验客户 active 且门户 enabled；失败返回 `customer portal not enabled`。关闭门户后仍允许禁用已有账号，便于安全收口。

### DEV-580-AUTH-BOOTSTRAP-HARDENING

- 最终安全审查 RED：旧公开 `/api/auth/password/set` 可按任意手机号创建员工、写密码并解禁；短信发送会创建员工和验证码记录并在响应回显 code；未知手机号配合预置 code 可由短信登录自动建员；管理员内部账号接口可直接修改渠道客户账号。
- GREEN：旧 password/set 不再注册写路由；无真实短信发送器时 send 返回 503 且零写入、零 code；短信登录只解析既有 active 内部员工并在全部门禁通过后消费有效 code；管理员 reset/state 只允许 active 内部员工，外部账号统一走客户门户配置。

### DEV-580-MINI-SESSION-REVOCATION / DEV-580-ACTIVE-BINDING-MUTATION

- 最终安全审查 RED：外部账号被停用或替换后，已投影的无限期 mini session 仍只依赖历史 approved 绑定访问客户；旧客户的 inactive 绑定仍可重置或启停员工全局账号。
- GREEN：外部账号投影绑定在每次 CurrentContext / SwitchCurrentCustomer 实时复核员工、登录、active ERP 关联、客户和门户；正式安全写事务同时使受影响 mini/ERP session 永久过期，恢复配置必须重新登录。密码与手机号投影固定员工身份，人工 approved 绑定不被投影同步覆盖；普通客户、门户和员工资料修改保持当前会话。外部账号写 helper 只接受当前 active 绑定，inactive 行保留只读，历史失效绑定不会误伤其他当前客户。

### DEV-580-DOCS-ACCEPTANCE

- 已同步客户门户和客户履约手册，明确账号维护、小程序登录、ERP 工作台绑定三个不同检查点。
- 外部账号写路由继续经过通用请求日志，并新增现有 `audit_logs` 上的业务审计记录；本修复没有新增写接口、数据库字段或迁移。

### REV-580-CUSTOMER-PORTAL-EXTERNAL-USER-CAPABILITY-TEMPLATE

等待 Van 在 development 完成以下手工验收：

- 选择非工作台模板和空模板客户，分别创建外部账号、重置密码并确认小程序密码登录成功。
- 用同一账号分别尝试 ERP 密码和短信登录，确认返回 403、不签发 token；把工作台模板降级后确认已登录 ERP 会话立即失效且不展示客户工作台数据。
- 通过显式工作台绑定入口给非工作台模板授权，确认仍显示旧错误且没有工作台授权。
- 使用只有客户权限和只有库存权限的测试角色核对外部用户维护边界，并在操作日志核对创建、重置密码、启停三类记录可读且不含密码。

## TDD 与自动化证据

### 文档合同 RED

```text
go test ./internal/interfaces/http/support -run TestDev579580CustomerAddressPortalExternalUserDocumentationContract -count=1 -v
requirements missing PR-579-MINIAPP-CUSTOMER-ADDRESS-PASTE
FAIL
```

### 已确认 GREEN

- Go 全量：`env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./... -count=1` PASS；support 全包、customer/customerfulfillment application、customerportal/customerfulfillment HTTP 和 appmain 定向包均 PASS。
- 共享解析 API：服务端五类解析样例、ERP/员工 mini token 同合同、权限、空/超长输入、错误遮蔽及不回显原文全部 PASS。
- 隔离 PostgreSQL 门禁矩阵：内部员工密码与预置验证码登录、工作台模板外部账号密码登录成功；所有渠道客户短信登录拒绝且验证码不消费；零售、空模板、未知模板、门户关闭的外部账号密码登录不签 ERP session；模板降级或门户关闭后旧 token 失效；mini 密码登录回归保持成功。
- 隔离 PostgreSQL 外部账号：非工作台/空模板创建、列表、重置密码、启停和 mini 登录 PASS；显式工作台绑定拒绝保持；创建/复用/替换、重置密码、启停审计与审计失败回滚 PASS，日志无密码/哈希。临时数据库已停止并移入废纸篓。
- Vue/Vite：完整 `node --test src/api/*.test.js src/lib/*.test.js` 876/876 PASS；`npm run build` PASS，仅保留既有大 chunk 警告。新增 phone-only 兼容、迟到失败和 inactive 外部账号只读来源校验均覆盖。
- 小程序：22 个测试文件 157/157 PASS；`npm run typecheck`、development `mp-weixin` 构建及 13 个页面清单校验 PASS。
- 仓库验证：`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend`、`git diff --check` 全部 PASS。
- 最终独立审查发现的公开认证、ERP/mini 旧会话复活、普通资料误退登录、门户写入开关、审计身份、phone-only 跨端兼容、inactive 跨客户写入和迟到失败均完成 RED→GREEN；修复后独立复审确认无剩余 P1/P2。
- `customerportal` 数据库全包中的历史夹具/schema 失败在干净 `origin/develop@3eb87b55` 可逐项同样复现；本分支新增的 20 项投影/内部/人工会话与正式安全写路径矩阵全部 PASS，未新增该包数据库回归。
- 数据库版 customerfulfillment 全包剩余 8 个失败与干净 `origin/develop@3eb87b55` 的同名同错误基线一致；干净基线共 10 个失败，本分支修复其中 2 个外部账号用例，未新增数据库回归。

## Development 部署与只读冒烟

- 功能分支 `0bb882f3` 已推送，develop merge `6e3b4daf` 已完成；`origin/develop@49489cbd3a7c205dbb033d4690d1d9672faf149c` 于 2026-08-05 部署到 development。
- 远端 Vue 876/876、小程序 157/157、类型检查、development 构建、13 页/52 文件清单、完整 Go 测试与 Docker 构建均通过。`erp_orderapp` 运行中、重启次数 0，`erp_postgres` healthy，部署后关键错误日志计数 0。
- `https://dev.qacoohee.com/app/login` 返回 200；未认证调用 `POST /app/api/customer-recipient/parse` 返回 401。服务器源码存在共享解析路由且不再注册旧 `/api/auth/password/set`。
- 开发小程序产物的 `RELEASE_INFO` 为同一提交、`environment=development`、`api_base=https://dev.qacoohee.com/app`，已同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；上一产物保留在 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260805002617-49489cbd3a7c`。
- 服务器源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260805002028-49489cbd3a7c`，回滚镜像为 `kferp-orderapp-rollback:development-20260805002028-49489cbd3a7c`。production 未部署；未上传、审核或发布微信小程序。

当前不声明 Van 业务验收已完成。请在 development 手工设置“9.9 COFFEE LAB”的目标密码并验证客户小程序登录；本次自动化没有写入该客户或账号。
