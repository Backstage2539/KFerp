# PR-599 物料库存、采购价与成本单价单位统一验收

- 需求：`PR-599-MATERIAL-INVENTORY-PRICE-UNIT-UNIFICATION`
- 日期：2026-08-14
- 范围：物料单位合同、历史重量物料迁移、BOM 克重换算成本、Vue 四个成本录入入口、需求与手册。
- 环境边界：自动化通过后合入 `develop` 并只部署 development；`main` 与 production 不操作。浏览器业务验收由 Van 后续执行。

## 锁定口径

1. 物料主档只维护一个库存单位，采购价和批次/标准成本按同一单位解释；兼容字段 `cost_unit` 必须等于 `unit`。
2. 重量物料主档只允许 kg；g、lb、oz 及全局单位字典中的自定义 t/吨等重量单位只可作为 BOM 等业务用量的换算单位，不能作为新物料库存单位。
3. 历史 `g/kg` 等记录迁移为 `kg/kg` 时，规范克库存、批次成本与历史快照不改写；迁移在锁表单一事务内完成，未知单位不一致或自定义非 kg 重量主档使整次迁移回滚。
4. BOM 的 `227g` 在 kg 物料上换算为 `0.227kg`；按 `288元/kg` 计价应为 `65.376元`。
5. 原料入库和库存调整只能使用物料主档单位；历史运行中/部分完成工单即使冻结为 g/lb/oz，仍按规范克维度继续执行且不改快照。

## RED 证据

- `node --test src/lib/materials-ui.test.js`：第一次 17 / 18，通过项外仅“库存单位也是采购/成本单位”合同失败，页面仍显示独立成本单位。
- 同一命令第二次 17 / 18：重量物料主档固定 kg 的候选与默认值合同失败，证明旧页面仍允许 g/lb/oz。
- 同一命令第三次 17 / 18：采购、原料入库和库存调整仍优先读取 `cost_unit` 的合同失败。
- `go test ./internal/interfaces/http/support -run TestDev599MaterialInventoryPriceUnitUnificationContracts -count=1`：因 PR-599 需求种子尚未登记而失败。

## GREEN 证据

- Vue 定向：`node --test src/lib/materials-ui.test.js` 19 / 19 通过。物料详情删除独立成本单位；重量候选只保留 kg，自定义重量单位不进入候选；采购、原料入库和库存调整直接读取库存单位，原料入库单位只读。
- 支持合同：`go test ./internal/interfaces/http/support -run TestDev599MaterialInventoryPriceUnitUnificationContracts -count=1` 通过，需求种子、Vue、根目录/线上需求验收、四本手册和历史证据边界一致。
- 历史守卫：PR-538、PR-561、PR-593 与 PR-599 四项定向支持合同共同通过；`go test ./internal/interfaces/http/support -count=1` 全包通过。
- frontend 全量：`node --test src/lib/*.test.js` 为 963 / 963；`scripts/verify_kferp.sh frontend-tests` 的完整发现集为 984 / 984；Vite 构建 6589 modules、2.15s 通过，仅保留既有大 chunk 提示。
- 材料、成本、库存与正式 HTTP/API 的真实 PostgreSQL 全包通过；覆盖 `kg/kg` API、操作日志、迁移原子回滚与幂等、`materials_unit_cost_unit_match` 约束、自定义 t/吨 重量拒绝、袋等计件允许、原料入库/库存调整错单位与错数量维度整单回滚。
- `227g → 0.227kg → 65.376元` 成本回归通过；生产仓储与 HTTP 真实 PostgreSQL 全包通过。kg 主档下 `600g/700g` 需求分别保留 `0.6/0.7kg` 精度，不再向上取整为 `1kg`；运行中或部分完成的历史 g 冻结工单/预约可继续领料、消耗和完工，冻结快照不改。
- `scripts/verify_kferp.sh all` 通过，Go、Vue、Vite 和支持合同均为 GREEN。远端 preflight 与正式 development 构建再次通过 Vue 984/984、miniapp 205/205、typecheck、Go 全量、Vite/miniapp/Docker 构建。

## 真实 PostgreSQL 验收边界

- 迁移测试准备历史 `unit=g/cost_unit=kg` 重量物料，记录采购价、规范克库存、警戒线、批次成本及冻结 BOM / 工单快照；另准备可迁移行与未知不一致/自定义重量脏行并存的场景，确认失败后所有数据、默认值和约束状态一起回滚。
- 升级后验证主档为 `kg/kg`，规范克数量和历史快照未变化；第二次执行保持幂等。历史 g/lb/oz 冻结执行按规范克换算，而不是强制改写快照。
- 通过正式材料 API 验证 kg/kg 新建成功、内置/自定义非 kg 重量单位、不一致兼容字段及缺失/停用的新单位被拒绝，并确认成功用户写入仍有操作日志。
- 通过正式库存路径验证原料入库与库存调整锁定主档单位，kg+g、袋+盒及重量/计件数量维度混用均拒绝；旧 `qty_g` 或目标克数且省略单位继续兼容。

## development 交付与 production 边界

- feature `0be4c32d` 已正常合入 `develop`，development deployed `3c632d86`。
- 部署前备份 `/opt/stacks/erp/backups/pr599-pre-unit-unification-20260813T191702Z-698413d9.dump`（13,854,860 bytes）已通过 `pg_restore --list` 和临时数据库完整恢复验证；临时恢复库验证后删除。
- 发布源备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260814031805-3c632d86b53c`，回滚镜像为 `kferp-orderapp-rollback:development-20260814031805-3c632d86b53c`。
- 发布后 `erp_orderapp`、PostgreSQL 和文档转换容器均 running，restart=0，PostgreSQL healthy；开发登录页和物料页 HTTP 200，近 10 分钟迁移/panic/fatal 错误标记为 0。
- 只读数据库核对：物料单位分布为 kg/kg 59、个/个 6、条/条 3，单位不一致 0；`materials_unit_cost_unit_match` 恰有一个且 validated。备份恢复库与部署后库的物料非单位字段、物料批次、BOM 版本/明细、工单指纹全部一致。
- 固定开发小程序目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev` 的 RELEASE_INFO 为 `3c632d86` / development / `https://dev.qacoohee.com/app`，56 文件清单验证通过；本次没有上传微信体验版或正式版。
- 不合入 `main`，不部署 production，不修改 production 业务数据。
- 发布后核对 `main` 仍为 `53cee821`，production 未操作。
- HTTP 200、自动化门禁和部署成功只证明发布健康，不代替 Van 的页面业务验收。
