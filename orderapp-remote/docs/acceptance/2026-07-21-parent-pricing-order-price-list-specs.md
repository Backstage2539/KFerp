# PR-544 父商品共享计价与录单价格表规格验收

## 范围

- 商品价格表按父商品只设置一次计价类型及阶梯/价格计算模板，所选规格共同继承；固定价金额继续按 SKU 隔离。
- 录单商品按父商品展示，规格仅来自当前已选已发布价格表中的具体 SKU；保存时服务端校验商品、规格与发布版本一致。
- 历史草稿、历史发布和历史订单保持兼容；不自动重新发布现有价格表，生产环境未部署。

## TDD 证据

- Costing 后端 RED：`go test ./internal/application/costing -run TestBeanListSharedParentPricingRejectsPerSpecModesAndTemplates -count=1` 稳定复现保存草稿仍接受同父规格混用计价类型、阶梯模板、价格计算模板及 SKU 级模板覆盖。
- Costing 后端 GREEN：加入 `product_pricing_scope=parent_product_shared` 权威校验后，草稿和发布共同拒绝不一致设置，按 SKU 分开的固定价金额继续通过。
- Costing 绕过复核 RED：同一测试中的 `shared pricing marker requires concrete spec selections` 首次得到 `SaveBeanListDraft() error = <nil>`，证明声明父商品共享计价却省略 `product_spec_selections` 时会跳过校验；补强后保存草稿与发布均先拒绝缺失数组，Costing 包完整测试通过。
- 价格表前端 GREEN：`node --test src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/product-price-list-draft.test.js src/lib/product-settings.test.js` 共 252 项通过，覆盖父商品单次计价、继承固定价模式、SKU 金额隔离、草稿提升与发布快照标记。
- 支持合同 RED：`go test ./internal/interfaces/http/support -run TestDev544ParentPricingOrderPriceListSpecsContracts -count=1` 因 PR/DEV/REV 种子和需求、手册、验收资料缺失失败。
- 独立兼容复核 RED：纯旧版重量阶梯被 API `product_families` 误标成具体 SKU，且同商品新旧混合版本会丢失旧版入口。补充纯旧版、同商品混合版本、无关旧商品及双向切换回归后再进入全量验证。
- 兼容性二次 RED：`node --test src/lib/order-entry.test.js` 共 106 项时有 2 项失败，分别证明纯旧版同父商品 SKU 被聚合成父 ID，以及缺少 legacy/concrete 发布模式判定；修复后 106/106 通过。切换价格表的静态契约同时验证 legacy 行遇到 concrete 版本时清空规格并显示 `已切换到具体规格价格表，请重新选择价格表中的规格。`。
- 发布合并 GREEN：`go test ./internal/infrastructure/postgres/sales -run 'TestMergeLatestCommercialOrderPublicationTierMaps' -count=1` 通过，覆盖所有 concrete 版本保留、最新 legacy 与 concrete 并存、纯 legacy 仅取最新，以及 concrete + 最新空 legacy 阻断更旧 legacy 回退。
- 跨模块 GREEN：`go test ./internal/application/costing ./internal/application/sales ./internal/infrastructure/postgres/orderbeans ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales ./internal/interfaces/http/support -count=1` 全部通过；七个价格表/录单前端测试文件共 395 项全部通过。
- 全量 GREEN：`./scripts/verify_kferp.sh changed` 通过；`./scripts/verify_kferp.sh backend` 全部通过；`./scripts/verify_kferp.sh frontend-build` 完成 401 modules 的 Vite 生产构建。全量前端为 783/789，通过项覆盖本需求；失败的 6 个 workspace-context 旧断言与干净 `origin/develop` 同名，基线另多 1 个 `contract-stamp` 失败，本需求未触碰这些文件。
- 独立最终复核 GREEN：复核发现并验证修复 concrete/legacy 发布合并覆盖、纯 legacy 同父 SKU 身份与 Vue key、legacy 切 concrete 清空旧规格、公共 concrete 父商品过滤以及显式无效/无权/不含商品发布版本的手工价绕过；复核确认父商品共享计价后端拒绝 SKU 级模式/模板分叉。最新代码无剩余 P0/P1，复核命令为前端 395/395、Go 定向包全部通过及 `git diff --check` 通过。
- develop 合并、开发部署和只读烟测结果在部署完成后补录。

## 验收场景

- 在“选择分类和产品”选择乌拉嘎多个规格，父商品只有一个“商品计价”入口；修改计价模式或模板后全部规格同步继承。
- 固定价模式在同一面板分别保存 18g、100g 等规格金额，草稿往返后不串价；伪造规格级模式或模板被后端拒绝。
- 录单商品只出现父商品乌拉嘎，规格只出现当前价格表已发布的 SKU。选择规格会同步具体商品 ID、价格单位、档位和发布版本。
- 切换价格表版本导致规格失效时，保留父商品、清空规格与自动价并阻止保存，直到人工重选。
- 历史发布、历史订单和缺少共享计价标记的历史草稿不迁移、不重写。

## 交付边界

- 目标为合并到 `develop` 并部署开发环境；开发部署不自动发布价格表。
- 生产环境未部署、未写入，现有已发布价格表未自动撤回或重新发布。
