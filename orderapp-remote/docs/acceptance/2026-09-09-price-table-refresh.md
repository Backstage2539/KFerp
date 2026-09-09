# PR-646 商品价格表局部刷新验收证据

- 需求：PR-646-PRICE-TABLE-REFRESH；DEV-646-SELECTION-REFRESH / DEV-646-PRICE-REFRESH。
- 分支：`codex/price-table-refresh-20260909`；起点及合并前同步的 `origin/develop`：`4c3de3196b39bbf140df8aef274b27ee65de060d`。
- 状态：实现及自动验证完成，Van 业务验收待进行。交付目标为 develop；本次不部署应用、不发布业务价格表、不修改开发或生产业务数据。

## 实现范围

- Vue `CostingView` 选品区提供“刷新商品和规格”，平铺价格行提供“刷新价格”；各自显示进行中及结果。全部依赖读取完成才更新页面，不调用会恢复整个草稿的 `loadBeanList()` / `loadPriceListTemplateOptions()`。
- 保留当前归属、类型和命名表、有效选择、计价来源及手工价格。新 BOM 规格可手选；已选规格失效或默认 BOM 变化时提示确认，重复刷新不消除未处理提示。
- 价格刷新读取最新阶梯/价格计算模板和商品数据，清空试算缓存后等待新试算。旧请求通过代次校验失效，不能覆盖新价格；初始模板加载的迟到响应也不能覆盖主动刷新结果。
- 同一行手工价保留，包括“手工输入恰好等于旧模板价格”的情况；固定价和沿用客户报价继续遵守已有优先级。读取错误保留原内容，试算错误保留最新配置并显示失败行数。
- 刷新期间禁用重复操作、保存和发布；切换范围/命名表或离开页面后丢弃原刷新结果。只维护当前编辑草稿及预览，既有发布快照和订单不改写。
- 复用现有读取 API 和只读的批量试算 POST；没有新增持久业务写接口，原有草稿保存/发布继续沿用既有操作日志。

## TDD 与自动检查

| 检查 | 结果 / 证据 |
| --- | --- |
| 初始 RED | `price-list-refresh.test.js` 新增 6 个交互测试全部失败，原因是缺少刷新行为；`/tmp/kferp-pr646-red.log` |
| 手工价 RED | 原实现将显式手工价 40 改成新试算 66；`/tmp/kferp-pr646-manual-red.log` |
| 重复刷新 RED | 第二次刷新错误消除 `default_bom_changed` 提示；`/tmp/kferp-pr646-repeat-red.log` |
| 聚焦 GREEN | `price-list-refresh.test.js` 12 个测试通过，0 跳过；覆盖等待、防重复、读取失败、部分试算失败、范围切换后晚到响应、已成功/进行中缓存失效、客户 API 范围、停用模板过滤、手工价和规格确认 |
| 相邻 Vue 回归 | 92 个断言通过，覆盖原有平铺价格、规格、撤销手工修改、发布等合同；`/tmp/kferp-pr646-ui-regression.log` |
| HTTP API | `go test -json ./internal/interfaces/http/costing -run 'Test(BeanListAPI\|PricingRuleTrial)' -count=1`：13 个测试通过、0 失败、0 跳过；覆盖规格元数据、具体规格/单位请求、批量部分错误、未发布 BOM 拒绝。`/tmp/kferp-pr646-api-focused.jsonl` |
| 后端全量 | `scripts/verify_kferp.sh backend` 退出 0；`/tmp/kferp-pr646-backend-final.log` |
| 前端全量及构建 | `scripts/verify_kferp.sh frontend`：1148/1148 通过、0 跳过，Vite 构建通过；`/tmp/kferp-pr646-frontend-final.log` |
| 仓库检查 | `git diff --check`、`scripts/verify_kferp.sh changed` 通过；合并前同步 origin/develop 无新变更 |

此次没有修改数据库结构或后端业务查询；HTTP 验证为现有 handler 级测试，不将未运行的数据库集成测试计为通过。

## 本地真实 Vue 页面交互

在独立本地 Vite 页面挂载完整 `CostingView`，读取和试算接口使用受控虚拟数据；没有访问实际业务环境。通过浏览器点击验证：

1. 旧自动价 40，模拟价格模板更新后点击“刷新价格”，按钮立即禁用并显示“正在刷新价格…”，新结果为 66，区域提示“价格已刷新”。
2. 模拟发布替换原 227g 规格的新 BOM，刷新后出现 1kg / 60kg 两个新选项；保留旧选择并提示切换默认规格，不静默换成新规格。点击“切换当前默认规格”后价格行使用 1kg。
3. 手工填写 80，再刷新价格，最终价仍为 80，模板试算为 66，并显示“人工调整”和“撤销人工修改”；刷新期间发布按钮禁用。
4. 模拟 bean-list 读取返回 503，选品区域显示“测试：网络读取失败”，原规格选择和手工价 80 均保留；重试成功。
5. 390px 窄屏下 `documentElement.scrollWidth === innerWidth === 390`，两个刷新按钮均在视口内，可点击重试；已查看截图，并恢复默认视口。

这些属于本地组件交互验证，不代表开发/生产业务验收。

## 手册及验收边界

- 单一来源手册：`docs/OP_MANUAL_COSTING.md` → “刷新商品、规格和价格（PR-646）”。
- Vue 页面两个操作区已同步说明 BOM 发布前置、价格刷新范围、手工/沿用报价规则及新价格发布后生效。
- PR/DEV 种子及 `docs/REQUIREMENTS.md`、`docs/ACCEPTANCE_TESTS.md`、`ACTIVE_REQUIREMENTS.md` 已维护；产品状态为 review，等待 Van 验收。
