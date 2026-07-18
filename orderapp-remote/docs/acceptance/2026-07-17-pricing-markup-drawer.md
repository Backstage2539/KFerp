# PR-539 价格计算模板统一加价率与右侧编辑抽屉验收

## 目标与边界
- 价格计算模板只按 `税前价 = 成本基数 × (1 + 加价率)` 计算；`0.8` 表示 80%，成本 100 时税前价为 180。
- 实际毛利率仍作为试算结果和最低毛利预警，不再反推售价。
- 历史毛利率或缺少方式的模板规范为加价率；历史整数百分数先除以 100；固定加价或不支持方式保留原参数说明并置为失效。隔离模板不能复制或直接保存，必须新建加价率模板后人工录入确认参数。
- 点击模板名称、新建或复制成功后，在右侧 `价格计算模板编辑` 抽屉编辑；列表下方不再展开表单。
- 只部署开发环境。本需求不授权生产环境部署、数据写入或入口切换。

## RED（实现前）
- Catalog 应用层定向测试失败：历史 `gross_margin`、整数百分数和缺少方式的数据未规范，`fixed_add` 仍可保存。
- Costing 应用层定向测试失败：成本 100、历史 `gross_margin=0.8` 试算为 500，而非加价口径的 180；固定加价未被拒绝。
- HTTP API 定向测试失败：保存返回仍是 `gross_margin`，整数百分数 `80` 未规范为 `0.8`。
- PostgreSQL 迁移合同测试失败：缺少 PR-539 安全、幂等的模板规范化块。
- 前端 `node --test src/lib/product-settings.test.js` 在实现前为 157/160：旧方式仍写入 payload、旧/缺失方式未规范、编辑表单仍在列表下方。
- 隔离绕过回归测试失败：去掉请求体中的迁移标记后，对既有隔离模板 ID 发送 clean markup PUT 仍返回 200，并覆盖原迁移证据。

## GREEN（实现后定向验证）
- [x] Catalog 模板保存/读取规范化、Costing 加价率试算、HTTP API 和 PostgreSQL 迁移定向 Go 测试通过。
- [x] 成本 100、加价率 0.8 返回税前价 180、实际毛利率约 44.44%；历史毛利/缺失方式与整数百分数得到相同税前价。
- [x] 历史固定加价/不支持模板被隔离失效，原方式和参数说明可追溯；复制和直接保存均被阻止，新保存和运行时试算不会静默转换。
- [x] 前端定向测试通过，覆盖只提交 `markup`、模板名称/新建/普通模板复制打开右侧抽屉、关闭抽屉和清理过期反馈；抽屉支持对话框语义、Esc 关闭、初始/返回焦点和 Tab 焦点约束。
- [x] Vue/Vite 生产构建、Go 全量测试、需求/验收/手册合同和 `git diff --check` 通过。

## 历史快照与审计
- [x] 迁移只修改价格计算模板配置，不更新已发布价格表、订单行最终价、财务凭证或历史单据。
- [x] 开发环境使用正式 API 创建并立即失效 `PR539-SMOKE` 模板 id 13；`/api/audit` 返回 2 条 `product_pricing_rule / save_product_pricing_rule` 记录，原始操作日志也有对应 POST/PUT 两条成功记录。该模板最终为停用状态，不会成为可用计价来源。

## 开发环境烟测
- [x] 功能分支和部署门禁修复分支均已推送并合并，仓库部署脚本已把 `origin/develop=c1e767231364a86d885723ff25c23087ec3ec720` 部署到开发环境；应用备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260718144532`。
- [x] 鉴权 API 读取价格计算模板与只读试算均为 200；成本基数 100、加价率 0.8、损耗/税率/其他成本为 0 时返回加价金额 80、税前价 180、最终价 180、实际毛利率 0.4444，公式只含加价率。开发环境 Chrome 验证模板名称和“新建价格计算模板”均打开右侧抽屉，Esc 关闭并恢复焦点，控制台错误为 0。
- [x] PR-539 迁移只更新 `product_pricing_rules`，没有更新发布价格、订单或财务表；开发库原始记录均为 `profit_method=markup`，无启用隔离模板。生产环境未部署、未写入、未切换入口。

## 实现验证证据
- `node --test src/lib/product-settings.test.js`：通过，161/161。
- `npm run build`：通过，Vite 构建 401 个模块；仅保留既有大 chunk 提示。
- `./scripts/verify_kferp.sh backend`：通过，完整 Go 测试全部绿色。
- `./scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'TestProductPricingRule.*CleanUpdateOverQuarantined' -count=1`：通过；服务会读取数据库现存模板状态，无法通过删掉请求标记覆盖或重新启用隔离 ID。
- `./scripts/verify_kferp.sh frontend-tests`：711/717；6 个失败与当前干净 `origin/develop` 基线一致，均不在 PR-539 变更文件：Vue shell remount、customer portal refresh、current view selector、BOM customer context、customer workspace menu、workspace mode wiring。
- 本地 PostgreSQL 16 原样执行 PR-539 迁移两遍：首次 `UPDATE 3` + `UPDATE 4`，第二次 `UPDATE 0` + `UPDATE 0`；`gross_margin=80` 规范为 `markup=0.8`，`fixed_add` 以及 JSON null/字符串/数组均安全隔离并保留原 JSON，冻结价格 `88.500000` 未变化。

## 开发部署与烟测证据
- 功能提交 `15d63614`；行为合并 `8e241ff9`；Docker 上下文门禁修复 `db27ab6c`；部署提交 `c1e76723`。
- 第一次部署在容器替换前被支持契约测试阻止，因为短期协调文件 `ACTIVE_REQUIREMENTS.md` 不属于 `orderapp-remote` Docker 构建上下文。修复只移除该临时文件断言，保留 req_store、代码、迁移、需求、验收、手册和本证据文件的 PR-539 持久合同；完整 backend verifier 和第二次容器内 `go test ./...` 均通过。
- 容器状态：`erp_orderapp` 运行、`erp_postgres` healthy、最近日志 fatal/panic/migration failure 为 0；未鉴权 `/app/` 为 303，鉴权 Vue、需求 API、价格模板 API 均为 200，需求 API 可见 PR-539。
- 开发数据库/价格模板 API 在审计 smoke 前为 12 条、2 条启用，非 markup 0、隔离 0、启用隔离 0；审计 smoke 后仅新增 1 条立即停用的 markup 测试模板。
- 生产环境未执行部署脚本、未写业务数据、未切换任何入口。
