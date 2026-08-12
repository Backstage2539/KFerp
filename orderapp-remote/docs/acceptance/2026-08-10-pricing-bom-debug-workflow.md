# PR-594 价格试算与 BOM 调试闭环验收记录

日期：2026-08-10
范围：开发、自动化单元测试、合入 `develop`；按用户要求不部署、不做浏览器、API 业务流或人工验证。

## 业务合同

- 单次价格试算可显式选择当前产出商品下含组件的 BOM 草稿，并显示“草稿，仅供试算”。默认选择、批量价格计算、商品价格表生成和正式发布继续只使用已发布 BOM。
- 固定用量包材及其他无损耗组件进入 BOM 物料成本；离散包材只有 `qty_units` 库存数量时，当前成本也按匹配成本单位加权读取。
- 试算页可进入对应 BOM 配置，再通过“返回价格试算”恢复当前会话上下文。上下文只在前端内存保存一次，刷新或离开后丢失。
- “更新参数到价格计算模板”只保存临时加价率、已填写的临时税率和其他成本，复用既有价格计算模板保存接口和操作日志；不保存商品、客户、销售规格、BOM、路线、工序模板或报价单位。
- BOM 编辑区域只改名为“有损耗的配方 / 无损耗的配方”，损耗来源、固定用量和组件计算语义不变。

## TDD 证据

- RED：成本仓储测试证明试算 SQL 仅筛选 published BOM，且物料当前成本只按 `qty_g` 加权，含固定包材的草稿和仅有 `qty_units` 的离散包材均无法进入本次试算。
- RED：成本应用测试证明单次试算不能选择草稿，且批量路径缺少草稿隔离合同。
- RED：Vue 单元测试证明缺少 BOM 往返、一次性恢复、模板参数更新 helper 与新配方区域名称。
- GREEN：成本仓储与应用定向测试已覆盖单次草稿试算、正式批量拒绝草稿、固定包材汇总和离散库存成本。
- GREEN：Vue 定向测试已覆盖配方区域改名、BOM 往返的一次性内存状态、模板参数回写边界和抽屉操作合同。

## 自动验证

- 通过：`go test ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/catalog`。
- 通过：`go test ./internal/interfaces/http/support -count=1`。
- 通过：`node --test src/lib/product-settings.test.js src/lib/bom.test.js`，219 项定向测试全部通过。
- 通过：`node --test --test-reporter=dot src/lib/*.test.js`，前端 `src/lib` 全量单元测试通过。
- 通过：`npm run build`，Vite 成功构建 397 个模块；仅保留项目原有的大 chunk 提示。

## 人工验收边界

本次不部署，因此没有 development 或 production 的页面、接口、容器或业务验证结论。后续由 Van 在另行部署后人工核对实际 BOM、包材成本、页面往返和模板更新。
