# PR-545 录单价格表分类作用域与旧商品隔离

## 问题与目标

- 开发环境 V3.0.19 咖啡豆价格表只有四个父商品，但录单商品候选同时出现 V3.0.18 的旧多规格商品。
- 根因是录单版本接口未返回价格表已保存的分类模板身份，前端按可变名称拆组并自动启用了每个组；后端商品作用域又只考虑客户自有发布，无法正确组合客户专属分类与其他公共分类。
- 修复目标是让新订单只使用当前分类价格表；历史发布和历史订单快照继续可追溯。

## TDD 证据

### RED

- 前端：`node --test src/lib/order-entry.test.js` 因缺少 `activeBeanListPublicationIDsByType` 失败，证明当前没有“权威分类自动启用、旧表显式启用”的候选边界。
- 后端：`go test ./internal/infrastructure/postgres/sales -run TestOrderFormBeanListVersionsUseClassificationIdentityForDefaultsAndFallback -count=1` 因查询缺少 `classification_template_id` 失败。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev545OrderPriceListCatalogScopeContracts -count=1` 因缺少 PR-545 需求种子失败。
- 终审边界：历史编辑冻结标记曾被规格 patch 覆盖；旧草稿恢复后只重新试价；客户自有/公共 publication scope 混用；零售和挂耳曾按 `commercial/green` 二分错误过滤。相应用例先失败，证明这些路径可复现。
- 订单头：唯一商业行 publication 为 9951、陈旧订单头为 999 时，`TestEditDataForAPISingleCommercialLineOverridesStaleHeader` 首次得到 999/999；挂耳快照标明 `list_type=commercial` 的多发布场景也首先复现错误订单头。

### GREEN

- 前端定向：`node --test src/lib/order-entry.test.js`，116/116 通过，覆盖分类稳定分组、V3.0.19 四个父商品、旧表显式启停、历史编辑冻结、复制订单、草稿恢复、客户自有/公共 scope、commercial/retail/drip/green 精确过滤和分类内版本过期提示。
- 后端定向：sales application/repository/API/support 包全部通过；客户可见发布范围、客户分类只覆盖同分类公共版本、旧全局兼容和多分类订单头均有回归测试。
- 真实 PostgreSQL：在开发服务器临时测试目录和临时 schema 中运行四个订单 HTTP/API 用例，分类默认、客户/公共兜底、唯一商业行纠正订单头、多分类订单头清零且订单行各自冻结 publication/版本全部通过；临时 schema 由测试自动清理，未写业务数据。
- 订单头定向：`go test ./internal/interfaces/http/sales -run '^TestEditDataForAPI' -count=1` 通过；唯一商业行会覆盖陈旧订单头和版本，多商业 publication 清零订单头，行 publication 不变，价格来源快照 `list_type` 优先且旧快照仍按商品类型回退。
- 完整验证：`scripts/verify_kferp.sh backend` 全部通过；`scripts/verify_kferp.sh changed` 通过；Vue/Vite 构建通过（401 modules）。完整前端为 793/799，六个失败均为工作区上下文既有断言；干净 `origin/develop` 同样是六个失败（783/789），失败名称一致，本需求没有新增全量前端回归。
- 独立终审：前端和后端分别完成修复后二次只读审查，历史/草稿/权限/多类型过滤及订单头边界均关闭，未发现剩余 P0/P1。
- 开发部署与现场 API/浏览器证据在集成后补充。

## 兼容边界

- 不删除 V3.0.18 或其他历史 publication，不改写历史 `content_json`、订单行、PDF 或财务快照。
- 同一 `list_type` 已有分类价格表时，新订单及复制订单默认不启用无分类 ID 的历史全局价格表；用户明确选择旧版本时仍可补录。仅存在旧全局价格表的类型继续按历史规则自动使用。
- 历史订单编辑保留冻结 SKU、规格、publication、版本和成交价。
- 不自动撤回、保存或重新发布现有价格表；生产环境不部署、不写入。
