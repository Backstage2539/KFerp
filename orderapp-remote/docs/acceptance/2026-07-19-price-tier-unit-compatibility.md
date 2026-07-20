# PR-540 商品销售规格与阶梯模板单位兼容验收

## 目标与边界
- “初晓”当前默认销售规格为 `磅` 时，不得继续使用有效档位数量单位为 `kg` 的“咖啡熟豆”阶梯模板；明确提示模板不可用。
- 单位只按同义身份兼容，不按换算兼容：`kg/公斤/千克` 同类，`lb/lbs/磅` 同类，`kg` 与 `磅` 不同类。
- 商品级配置、继承配置、平铺行、预览/PDF、草稿和发布使用同一规则；历史已发布价格表和订单快照不回算。
- 只部署开发环境。生产环境未部署、未写入、未切换入口。

## RED（实现前）
- 前端单位合同因缺少 `priceTierTemplateUnitCompatibility` 导出直接失败；价格表工作流对不兼容阶梯行返回空错误，仍可能继续试算、手工调整和预览旧价格。
- Costing 应用层发布和草稿测试以“初晓=磅、咖啡熟豆=kg”构造数据时仍返回成功，没有数据库主数据兜底。
- HTTP API 绕过测试预期 400，但发布和草稿都返回 200；客户端可伪造单位、模板名或最终价。
- PR-540 支持合同首次运行因需求/开发/验收种子缺失而失败。
- 独立复核新增 RED：当前商品为 `磅`、旧价格快照为 `kg` 时，前端错误优先读取旧快照而判兼容；同一模板/档位的旧 kg 手工价 key 也会在单位改成 lb 后继续复用。
- 混合单位模板绕过 RED：模板同时包含 `lb` 和 `kg` 时，客户端只提交兼容的 lb 档位，发布和草稿 API 返回 200，没有检查模板全部有效档位。

## GREEN（实现后定向验证）
- [x] `product-settings` 单位帮助函数覆盖 `磅` 对 `kg` 不兼容、`lbs/磅`、`1Kg/千克`、`盒（10袋）/盒` 同义兼容；当前非空销售规格优先于旧价格快照，不兼容模板不生成阶梯价格行。
- [x] 价格表工作流将不兼容作为优先阻断错误，人工调整不能绕过；预览/PDF 清除该商品旧价但保留阻断快照。
- [x] Vue 合同覆盖商品级模板禁用、不可用标签、商品行警告、继承模板具体行阻断、最终价禁用及发布/PDF/草稿入口拦截。
- [x] Costing 应用层、PostgreSQL 仓储和 HTTP API 定向测试覆盖当前显式默认销售规格、阶梯全部有效档位、派生 SKU、客户别名不改变实际商品单位、混合单位、缺失身份、缓存、手工调整与绕过拒绝。
- [x] 支持合同、完整 Go、Vue/Vite 构建和 `git diff --check` 通过；当前全量前端为 731/737，干净 `origin/develop=fe849630` 为 725/731，失败的是同 6 个既有工作区上下文合同；本需求新增 6 个测试及定向 253/253 全绿。

## 历史快照和发布边界
- [x] 新校验只在草稿、重新生成和新发布路径执行；没有 schema 迁移，也没有批量更新已发布价格表、订单行、财务凭证或历史 PDF。
- [x] 兼容的新价格行固化实际商品默认销售规格、阶梯数量单位、模板和档位来源；库存换算和成本单位逻辑未改变。

## 开发部署与烟测
- [x] 功能分支已推送并合并到 `develop=4ac8766ad544d19ba4e40d42221ed06b105412de`，开发环境部署完成；备份路径：`/opt/stacks/erp/orderapp.backup.deploy-20260720103433`。
- [ ] 开发环境打开商品价格表后，“初晓”显示磅规格；直接选择“咖啡熟豆”时选项不可用，继承该模板时商品和平铺行显示 `阶梯模板不可用：商品规格“磅”与阶梯规格“kg”不匹配`，预览不显示旧价格。
- [x] 开发环境应用容器重建并运行，镜像内 `go test ./...` 通过；生产环境未部署、未写入、未切换入口。外部开发域名当前不可达，因此未宣称浏览器控制台烟测通过。

## 实现验证证据
- 前端定向：`node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js`，253/253 通过。
- 后端定向：`go test ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing -count=1`，全部通过。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev540PriceTierUnitCompatibilityContracts -count=1`，首次因 PR-540 种子缺失 RED，补齐后 GREEN。
- 完整后端：`./scripts/verify_kferp.sh backend`，全部通过。
- 完整前端：`./scripts/verify_kferp.sh frontend-tests`，当前 731/737；干净 `origin/develop=fe849630` 为 725/731，失败测试名称和数量完全相同，本需求新增 6 个测试全部通过。
- 构建与静态检查：`./scripts/verify_kferp.sh frontend-build` 通过（401 modules，仅既有大 chunk 提示）；`./scripts/verify_kferp.sh changed` 和 `git diff --check` 通过。
