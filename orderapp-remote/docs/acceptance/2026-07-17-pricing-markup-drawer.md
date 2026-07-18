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
- [ ] 使用正式 API 保存和失效模板后，操作日志能看到对应动作。

## 开发环境烟测
- [ ] 功能分支已推送并合并到当前 `origin/develop`，使用仓库部署脚本部署开发环境。
- [ ] 鉴权 API 读取价格计算模板和执行加价率试算成功；开发环境页面完成模板名称、新建、复制和右侧抽屉交互验收。
- [ ] 已发布价格表快照和历史订单价格未被重算；生产环境未部署、未写入、未切换入口。

## 实现验证证据
- `node --test src/lib/product-settings.test.js`：通过，161/161。
- `npm run build`：通过，Vite 构建 401 个模块；仅保留既有大 chunk 提示。
- `./scripts/verify_kferp.sh backend`：通过，完整 Go 测试全部绿色。
- `./scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'TestProductPricingRule.*CleanUpdateOverQuarantined' -count=1`：通过；服务会读取数据库现存模板状态，无法通过删掉请求标记覆盖或重新启用隔离 ID。
- `./scripts/verify_kferp.sh frontend-tests`：711/717；6 个失败与当前干净 `origin/develop` 基线一致，均不在 PR-539 变更文件：Vue shell remount、customer portal refresh、current view selector、BOM customer context、customer workspace menu、workspace mode wiring。
- 本地 PostgreSQL 16 原样执行 PR-539 迁移两遍：首次 `UPDATE 3` + `UPDATE 4`，第二次 `UPDATE 0` + `UPDATE 0`；`gross_margin=80` 规范为 `markup=0.8`，`fixed_add` 以及 JSON null/字符串/数组均安全隔离并保留原 JSON，冻结价格 `88.500000` 未变化。

## 待补证据
- 合并提交、开发部署提交、备份路径、操作日志和 API/UI 烟测：部署完成后补充。
