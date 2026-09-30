# PR-676 商品创建器 V5 工艺接线与命名即时预览

## 目标与范围

让 V5 物料 BOM 和商品 BOM 都能连接工艺路线；每个 BOM 最多连接一条，同一工艺节点可以服务多个 BOM。运行时按“本次改选 → 连线工艺 → BOM 默认或规格模板路线”确定结果。商品 BOM 的工艺覆盖作用于新 BOM 的全部规格，不改被引用的规格模板。

命名编辑器在输入固定文字、变量和示例值时直接显示最终拼接结果。示例值只在当前界面生效，不进入模板或运行草稿。

V1–V4 已发布版本和运行记录继续沿用原行为；编辑时只将草稿升级为 V5。开发环境交付后由 Van 完成业务验收；production 与微信发布不在范围内。

## 开发基线与追踪

- 功能分支：`codex/product-creator-v5-process-preview-20260930`
- 开发基线：`origin/develop@54e6432f`
- 需求：PR-676 商品创建器规格模板与 V5 工艺/命名优化
- DEV：DEV-710-PC-V5-ROUTE-CONNECTION、DEV-711-PC-V5-ROUTE-OVERRIDE、DEV-712-PC-NAME-LIVE-PREVIEW、DEV-713-PC-V5-DOCS-DELIVERY
- 操作手册：`docs/OP_MANUAL_PRODUCT_CREATOR.md`；Vue/Vite 入口沿用「商品 → 创建器手册」。

## 自动化验证

- [x] RED/GREEN：前端连接校验与命名预览定向测试在实现前因缺少新 V5 逻辑失败，补齐实现后通过。
- [x] HTTP/API：商品创建器模块目录测试先因目录仍断言 22 个模块而失败；V5 目录核对修正后通过，验证 27 个模块及 V5 BOM 工艺输入口和商品节点不包含规格模板字段。
- [x] Go 图校验：两种 BOM 均接受一条工艺，同一工艺可复用，第二条工艺被拒绝；无效路线节点、历史 V4 规则、运行覆盖优先级均有覆盖。
- [x] PostgreSQL 预览：隔离测试库验证一个工艺节点可同时连接物料和商品 BOM；覆盖时所有规格显示共享路线名，移除连线后恢复各规格模板路线名。
- [x] PostgreSQL BOM 事务：验证覆盖路线写入商品 BOM 的全部规格，未覆盖时保留各规格模板路线，模板本身未改变；同时核对创建操作日志的工艺来源、节点和路线编号。
- [x] Vue 定向：`node --test src/lib/product-creator-graph.test.js src/lib/product-creator-variables.test.js src/lib/product-creator-v5-view.test.js`。
- [x] Vue 全量：`scripts/verify_kferp.sh all`；1,293 项通过。
- [x] Vite 构建：`npm run build` 成功；PDF 预览资源超过 500 KB 的提示为既有构建警告。
- [x] `scripts/verify_kferp.sh changed`、Go 全量/前端全量/Vite 发布门禁通过。
- [x] `git diff --check`。

额外对整个 PostgreSQL BOM 测试包启用数据库后，出现多项旧 PR-598/600/603/617 用例失败，涉及既有物料夹具、结构约束及模板迁移断言；本轮 V5 专项隔离数据库测试通过。该额外整包失败不计作通过，也不包含在未配置可选测试数据库的标准发布门禁中。

## 浏览器与开发环境验收

- [ ] 半成品烘焙 BOM 已有工艺时，再连接第二个工艺，原连线保留并显示带 BOM/工艺名称的提示。
- [ ] 新工艺可连接商品 BOM；商品 BOM 切换为物料产出后仍可连接；删除工艺线后默认路线/规格模板路线立即恢复。
- [ ] 两个 BOM 共用一条工艺；运行时分别单独改选，其他 BOM 保持原路线。
- [ ] 命名固定文字、变量、默认值、示例值和撤销/重做均即时更新预览；示例值没有进入保存的模板或运行。
- [ ] 发布指定 PR-676 验收模板最新草稿，保留已有修改；通过两级 BOM 创建一个验收商品，确认各规格工艺与源规格模板一致性、结果卡片及操作日志。
- [ ] 更新 DEV 记录为完成并提供开发环境版本/健康检查证据；PR-676 保持待 Van 业务验收。

## 交付记录

浏览器检查发现指定 V4 验收模板当前草稿有未保存修订；在新代码进入 development 后继续从最新草稿升级，保存前保留现有修改。部署提交、备份路径、运行号、商品/BOM 编号及 operation log 证据在集成完成后补入此文档与 `ACTIVE_REQUIREMENTS.md`。
