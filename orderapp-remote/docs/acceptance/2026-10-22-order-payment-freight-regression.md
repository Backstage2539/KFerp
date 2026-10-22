# 订单付款状态与运费保护回归修复

- 分支：`codex/order-payment-freight-regression`。
- 基线：显式 fetch `origin/develop` 后的 `7684c047ea245c033750ce8b764deecd03ff73c7`，不是环境保存时的快照。默认 fetch 配置仅跟踪 main，因此另行执行 `git fetch origin develop:refs/remotes/origin/develop`。
- 范围：初始阶段仅本任务工作区代码、文档及隔离测试；后续已授权提交并集成 develop/main，见末尾授权及集成记录。始终禁止部署、读取或改写真实订单。数据库全部使用本机 `127.0.0.1:55432/kferp_test` 的临时测试 schema。
- 已读：`AGENTS.md`、`HOW_TO_WORKFLOW.md`、`.agents/skills/kferp-bugfix/SKILL.md`、相关销售和运费手册；PR 237 的讨论只作为线索，结论基于当前源码及重现。

## 根因与变更

`SaveOrder` 在整单编辑产生运费快照时调用 `protectFreightReceiptsTx`，即使此次只改变付款信息也会进入保护函数。

1. 旧函数只读 `grand_total` 推导已收。历史 `grand_total=0,total_amount=120,prepayment=0` 的已付订单被视为已收0，增加20元运费后跳过保护，仍全部付清。现在仅在无折扣、无运费、无取整差额的历史缺失形态下回退到 `total_amount`；真实零金额、全额优惠及取整为零保留零。增加应收时，原已收120保持120；原已收0则变为未付款。
2. 旧函数把付款状态变化也视为费用调整，使用旧预付款30覆盖显式结清，保存成功却仍部分付款。现在应收和运费都不变时保留显式付款命令；依照原手册，选择“已付款”表示操作员确认收到尾款，保留原预付款，不自动增加实收字段或生成凭证。
3. 旧函数不论是否仍结清都把已收总额写成预付款。现在只有变成部分付款、需要保存原已收金额时才回填；仍结清时保留原预付款，避免合同显示从“已付金额”错误变成“已支付预付款”。
4. 显式付款变更带凭证时，金额须与结清总额或部分付款金额一致；新凭证须为存在的收款凭证资产。不能清空旧凭证来跳过核对。费用与付款字段同时改变时拒绝混合保存，避免静默覆盖或凭空补收款。

生产逻辑仅修改 `internal/infrastructure/postgres/sales/order_freight.go`。`repository.go` 的调用位置、事务、审计链和冻结限制保持原有路径。独立 `UpdateOrderStatus` 从未走这个保护函数，本次未改其实现。`b1783d3d` 的旧小程序 CREATE 兼容也未改动。

回归文件：

- `internal/infrastructure/postgres/sales/order_freight_test.go`：历史零总额、真实零金额、全额折扣、取整、运费增减、已付/部分付款、显式预付款与凭证变更、金额不一致、混合保存、实收运费下限及预付款保留。
- `internal/infrastructure/postgres/sales/order_status_test.go`：带运费快照的独立状态变更仍可将30/100结清，保留30元原预付款且不造收款字段。
- `internal/interfaces/http/sales/order_freight_payment_test.go`：真实 HTTP → Service → SaveOrder → PostgreSQL 的6组案例，验证持久化结果、成功操作日志、冲突回滚、实际运费确认后的冻结限制，以及按提示分两次保存费用和收款。

手册同步至 `docs/OP_MANUAL_FREIGHT.md` 与 `docs/OP_MANUAL_ORDER_SALES.md`，继续由已有 Vue 手册入口加载。
本地追踪更新根目录 `ACTIVE_REQUIREMENTS.md`；本文件保存验收证据，没有写远端 PR/DEV 进度表。

## 重现与验证

工具链：`source /workspace/.kferp-setup/activate.sh`。

```sh
export ORDERAPP_TEST_DATABASE_URL='postgres://kferp@127.0.0.1:55432/kferp_test?sslmode=disable'
export DATABASE_URL="$ORDERAPP_TEST_DATABASE_URL"
cd orderapp-remote
go test ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales \
  ./internal/interfaces/http/customerportal ./internal/domain/sales \
  -run 'TestFreight|TestOrderFreightPaymentSaveAPI|TestOrderStatusUpdate|TestMiniEmployeeOrderStatus|TestMiniPrepayment|TestPrepaymentValidation|TestPrepaymentSettlement|TestPaymentBalance' -count=1 -v
go test ./internal/infrastructure/postgres/customerfulfillment \
  -run 'TestOrderFreightV2|TestLegacyMiniEmployeeFreight' -count=1 -v
go test ./internal/domain/sales ./internal/infrastructure/pdf \
  -run 'TestPaymentBalance|TestSalesOrderPrepaymentRows|TestSalesOrderPaymentRowsShowPaidAndUnpaidWithoutPrepayment' -count=1 -v
```

- **RED 已确认**：修改生产代码前的测试失败；另用 Go overlay 将生产实现替换为 `origin/develop` 的原始文件，对最终回归重跑。HTTP 重现：120→140后仍已付、30/100结清仍部分付款、全额优惠新增运费仍已付，以及仍结清时错误回填预付款。
- **GREEN 已通过（初轮）**：定向套件15个顶层测试、41个子用例，无跳过或失败；包含28个付款/运费边界和5个 HTTP 保存案例。凭证存在性校验补充后已重跑该套件；用户确认后的最终结果见下文。
- **GREEN 已通过**：原有运费v2、待确认、历史快照、并发实际账单、旧小程序 CREATE 兼容及协议防绕过测试，共5个顶层测试。
- **GREEN 已通过**：付款余额与 PDF 付款行的3个顶层测试；已有整套 PDF 测试也在全量检查中通过。
- **GREEN 已通过**：`bash scripts/verify_kferp.sh changed`；`node --test src/lib/operation-manuals.test.js`（3项）。
- **全量后端检查**：`GOFLAGS=-p=2 bash scripts/verify_kferp.sh backend` 使用本地测试数据库运行；存在旧基线失败，最终对照汇总见下文。
- **未运行**：浏览器验收、前端全量构建、小程序构建、部署烟测；本次未改这些实现。需另设 `KF_RUN_POSTGRES_INTEGRATION` 等显式开关的测试未额外启用，不宣称所有可选测试均执行。

原始本地日志在 `/workspace/.kferp-task-evidence/`：`final-baseline-red.log`、`targeted-green.log`、`freight-integration.log`、`payment-presentation.log`、`backend.log`、`backend-baseline.log`。`pristine-overlay.json` 将所有改动 Go 文件映射为基线原文并排除新增测试，以便在相同工具链和本地数据库上对照；没有改动工作区源文件来跑基线。

## 全量基线对照

全量后端命令及原始 develop 对照均已结束，退出码均为1；两者均有 **200个顶层测试失败、15个包失败**，失败测试名及包集合完全相同（`only_current=[]`、`only_baseline=[]`）。这不是全量通过，也不代表这些既有缺陷已修好。本次新增回归没有出现在失败集合中。全量检查后最后补充的凭证存在性校验已通过上述定向重跑。

失败示例（两边均存在）：

- `TestPrepaymentOrderAPIStoresPartialPaymentAndBalances`：测试表缺少 `delivery_mode`，SQLSTATE 42703；还单独用原始实现复核，日志 `baseline-prepayment-api.log`。
- `TestSalesOrderSettingsRoundTrip` 等旧销售测试：简化测试表缺少 `ship_status_id`，SQLSTATE 42703。
- 旧成本/价格表测试：缺少 `customer_order_price_table_bindings` 或 `estimated_unit_price`。
- 客户负责人旧断言期待英文错误，当前返回“请选择负责人”。

其他失败分布在 appmain、BOM、商品、合同、客户、履约、生产、库存等既有测试，未扩展修复。完整名单为 `/workspace/.kferp-task-evidence/baseline-failures.txt`，集合对照为 `baseline-comparison.json`；原始错误在两份 backend 日志中。

## 用户已确认的业务规则（2026-10-22）

用户消息 `Sentinel_9206b4bb65708191a520edd75e869389` 原话：

> 1、累计收款
> 2、分开保存

对应确认问题：实收金额按累计还是本次收款；运费与付款状态同次修改是否接受分开保存。此确认仅用于本任务两项规则，不扩大到其他业务或发布授权。

- **累计收款已确认**：凭证关联的货款＋运费按订单累计实收核对。应收100元，先付30元、后付70元时，结清累计实收是100元，原预付款仍是30元。金额不一致时拒绝保存，不补造流水或凭证。错误提示已明确“累计实收金额”及“实际累计到账金额”。
- **分开保存已确认**：运费／应收变化同时伴随付款状态、预付款或实收／凭证变化时，返回409并整体回滚。提示为“运费或订单应收与收款变更请分两次保存：先保留原付款信息，只保存费用调整；再重新打开订单，核对新应收后单独登记实际累计收款”。回归验证原30/100同时加20元运费、改已付会被拒绝；按提示先保存费用为30/120，再单独登记已付款后结清，预付款仍30且不造实收字段。拒绝时订单、运费快照及操作日志均不改变。
- 既有手册语义保持：显式选择“已付款”是操作员确认已收到尾款，系统未核验银行到账；无独立凭证时仍按既有规则结清，不把预付款30改成100。
- 两项规则不再待业务确认。本地补丁可以交付审阅；全量基线失败仍明确保留，不宣称全量通过。此阶段未授权发布；后续提交、推送、两分支集成的新授权见末尾，部署和生产数据操作仍被排除。

## 确认后的定稿复核

- 沿用基线 `7684c047ea245c033750ce8b764deecd03ff73c7`；未因远端变化重取或扩大修复。
- 此轮只明确两条错误提示、补充分次保存的 HTTP 测试，并同步两份手册、验收记录及 `ACTIVE_REQUIREMENTS.md`；收款金额计算、保护逻辑和独立状态端点的实现范围未扩大。
- 指导提示测试先 RED：原文未说明“累计实收金额”和“分两次保存”的具体步骤；证据 `confirmed-guidance-red.log`。修改提示后的最终定向套件 **15个顶层测试、42个子用例通过，0失败、0跳过**，含28个付款/运费边界、8个独立状态限制子用例及6个 HTTP 保存案例；日志 `confirmed-targeted-green.log`。
- 确认后重跑运费v2／旧小程序 CREATE 兼容集成测试，**5个顶层测试、14个子用例通过**；日志 `confirmed-freight-integration.log`。付款余额与 PDF 付款行 **3个顶层测试、2个子用例通过**；日志 `confirmed-payment-presentation.log`。
- 手册入口及解析测试 **3项通过**，日志 `confirmed-manuals.log`。`gofmt -l` 对4个变更 Go 文件无输出；`bash scripts/verify_kferp.sh changed` 通过，日志 `confirmed-changed.log`。
- 最终差异审阅：生产改动仍仅在 `order_freight.go`；确认 `SaveOrder` 调用条件和事务／日志路径未改，冲突通过既有业务错误机制返回；独立 `UpdateOrderStatus` 实现及冻结限制未放宽。未修改其他业务模块、迁移或部署配置。
- 本轮未重复运行全量后端套件：确认后仅改提示、测试与文档，适用定向检查已重跑；保留此前与基线相同的200个测试／15个包失败记录，不宣称全量通过。浏览器验收、前端／小程序全量构建、可选集成开关测试及部署烟测仍未运行。
- 本地定稿阶段状态：两项业务规则已确认，本地修复和定向验证完成；无需继续等待该两项业务决定。当时改动未提交、推送、合并或部署。

本地定稿阶段无权限拒绝或生产数据操作。后续集成检查遇到的外部查询限制见下文。


## 两分支集成授权与基线

用户后续消息 `Sentinel_10a4690679888191a07f9aa08221f504`：

> 合并到develop和main，并起一个本地session做验收测试

此授权覆盖本修复的提交、推送及 develop/main 集成；不覆盖其他菜单任务、强制推送、部署或生产数据访问。Mac 本地验收由父会话另开 session。

- 已显式 fetch 两个分支：develop `11139d4b32960b20085ff7a4a154ad81e3639645`，main `e64779b86eb5eb3508dc731bebf8092cf4d1328b`。
- 这两个基线中的付款保护源文件、既有相关测试及修改的手册一致；main 有其他独有提交。采用 develop 基线上独立修复提交，再将该修复单独 cherry-pick 到 main 基线，保留 main 其他内容，不把 develop 的无关差异整体合入。
- 两个远端分支 API 均显示未启用保护、无 required status checks。两个树中没有 Actions 工作流；仓库 Actions 运行记录为0。`deploy_orderapp.sh` 为手动部署入口，本任务未调用。
- `gh api` 读取 `/repos/Backstage2539/KFerp/hooks` 返回 `Forbidden`，一次获准的只读沙箱提升后仍相同；没有绕过或继续重试。无法据此排除外部自动部署，已请求确认，确认前不推送。
- 原始基线的200个历史失败不代表最新分支已验证；集成后的测试结果单独记录。

## Mac 本地验收交接

仅使用新建本地数据库和合成订单，不加载生产 `.env` 或生产数据库快照。Go 版本按 `go.mod`，Node 版本按已锁定 Vite 的 engines 要求（Node22.12+）。以下启动方式按源码配置核对，浏览器验收仍由独立本地 session 完成：

```sh
# 在本地 PostgreSQL 创建空库；替换 LOCAL_PG_USER 为本机数据库角色。
createdb -h 127.0.0.1 -p 5432 -U LOCAL_PG_USER kferp_payment_acceptance
cd orderapp-remote/frontend-vue-shell
npm ci
npm run build
cd ..
export DATABASE_URL='postgres://LOCAL_PG_USER@127.0.0.1:5432/kferp_payment_acceptance?sslmode=disable'
export DB_SCHEMA=payment_acceptance
export ASSET_DIR="$PWD/.local-payment-assets"
export TEMPLATE_DIR="$PWD/templates"
export APP_USER=order
export APP_PASS="$(openssl rand -hex 16)"
export LISTEN=127.0.0.1:18080
mkdir -p "$ASSET_DIR"
go run .
```

后端初始化指定 schema，并从 `frontend-vue-shell/dist` 提供页面。浏览器地址为 `http://127.0.0.1:18080/vue-shell?view=orders`；全新库需建立隔离的员工登录账号、客户及商品/价格配置后再做页面用例，不能将 APP_USER/APP_PASS 当成员工登录账号。APP_USER/APP_PASS 仅用于现有 Basic Auth 管理/API验证。可先用本文件前面的 Go 用例运行独立 HTTP 验收，测试会自行建立临时 schema 和数据并清理；测试库 DSN 同样只指向本机新建空库。

验收案例：

1. 历史已付订单 `grand_total=0,total_amount=120,prepayment=0` 且无独立收款，加运费20后显示应收140、已收120、部分付款。
2. 部分付款30/100，仅确认尾款到账并改已付款后保存，重开仍已付款；原预付款保留30，没有自动生成实收字段或凭证。合同不应把累计100改写成原预付款100。
3. 有凭证时累计货款＋运费需与付款状态一致；结清100却只填本次70或原30应拒绝，累计100可保存。原凭证不可清空以绕过校验。
4. 30/100同时加运费20并改已付，返回409且原单、快照、日志不变；依提示先只保存费用，重开显示30/120，再确认实际尾款90到账并单独登记结清120。
5. 全额折扣／真正零金额已付订单新增20运费后为未付20；仍结清的运费和折扣抵消调整保留原预付款0及合同“已付金额”展示。
6. 费用不能降至累计实收以下或低于已收运费；实际运费已确认的冻结限制仍有效；新小程序独立状态端点仍可单独结清，不误入整单运费保护路径。

具体合成数据及 HTTP 断言见 `internal/interfaces/http/sales/order_freight_payment_test.go`；历史存储形态只能在本地测试 schema 构造。
