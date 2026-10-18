# PR-697 录单恢复价格表商品规格范围

## 范围与保护边界

- 用户要求撤回 PR-694 的全部有效 BOM 规格录单，并撤回后续默认规格特殊选档（PR-695）及旧模板现场报价确认（PR-696）。
- ERP 和员工小程序恢复为当前客户所选已发布价格表中的商品/规格，继续使用普通自动阶梯、手动选档及授权改价。临时换规格入口和报价接口关闭；旧客户端无法创建转换价订单。
- 原换算组配置及所有历史订单保留，无数据库迁移，无线上数据修复，无价格表重发布。原订单只改数量时由服务器读取冻结单价；复制、更换规格或重新报价不得绕过表内规格限制。
- `fa98f276` 明确保留：价格表中心、样品申请、分配、确认转订单、跟进提醒、权限及样品订单编辑逻辑没有撤回。两个录单页面以该提交为基准恢复，补充必要的历史转换价兼容。

## TDD 与接口

- RED：`/tmp/kferp-scope-red-api.log` 旧规格目录返回全部 3 个 BOM 规格；Vue 和 miniapp RED 分别确认旧入口及复制保留转换模式。
- GREEN：`TestPriceTableOnlyOrderScopePostgresHTTP` 覆盖 ERP/员工小程序退役入口 410、自动/手动表外规格拒绝、表内订单服务器价格复核、历史 BOM 模板订单改量冻结、伪造快照/复制拒绝、价格表内容不变、订单审计。
- `TestConversionPostgresHTTPAndFrozenOrder` 覆盖配置换算旧订单冻结、退役创建/重新报价拒绝、下线价格表历史修改、配置修订/停用及审计。
- `TestRetiredMiniConversionQuotesKeepEmployeePermissionChecks` 保留员工/客户权限隔离。
- `TestSamplesPostgres*` 四个真实 PostgreSQL 流程回归通过，包括确认转单事务、权限、提交并发和提醒；以上相关数据库测试加 race 均通过。

## 客户端与构建

- Go 全仓非数据库测试通过；Vue 1367 tests；miniapp 329 tests、类型检查、开发构建通过；Vue 构建通过。
- 本地真实 Vue 页面使用模拟目录验证：BOM 含 227g/500g/250g，价格表仅 227g，规格菜单只显示 227g；2 件单价 48，改成 14 件单价 40。482px 窄屏显示正常，临时换规格/来源档位/当前模板确认入口已撤回。没有保存真实业务订单。
- UI 证据 `/Users/yiiiple-work/KFerp-price-table-scope-delivery-20261018/ERP-scope-narrow.png`。
- 扩展数据库套件并非全绿：在回退前开发版本 `a892ab5e` 与本分支运行相同四个包，均有 91 个相同失败项（旧夹具缺少规格迁移/价格绑定表等，含既有 V8 断言失败），新增失败为 0。记录 `/tmp/kferp-scope-db{-baseline,}-failures.txt`；这些既有问题不在本回退范围。

## 手册与交付

手册：OP_MANUAL_ORDER_SALES.md、OP_MANUAL_COSTING.md、OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md、OP_MANUAL_BOM_ORDER_PRICING.md、OP_MANUAL_SPEC_CONVERSION.md 与总索引、前端帮助同步。

功能分支 `codex/restore-price-table-order-scope-20261018`，基线 `43de729d`。合并及 development 部署记录待补；生产、微信上传和 Van 真实业务验收不在本次执行中。

## 发布路径补充回退

移除仅用于跨规格报价的发布时可执行模板冻结要求。已经生成的价格行按原规则发布，不因关联模板后来停用而被新增检查阻断；不会重新计算或改写价格。`TestPriceTableOnlyOrderScopePostgresHTTP` 增加停用关联模板后仍能发布已定价行、最终售价不变的 RED/GREEN；RED 为“价格档位的计算模板已停用”。
