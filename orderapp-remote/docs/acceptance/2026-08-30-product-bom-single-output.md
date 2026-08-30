# PR-619 商品 BOM 单一产出与直接商品身份验收

## 范围

- 商品 BOM 新增 `single` 与 `spec_group` 两种明确产出结构。
- 单一产出贯通商品目录、库存、成本、生产、价格与 ERP 订单的直接商品身份。
- 保留既有多规格 BOM 和历史业务身份，不操作 production。

## RED

- Go 定向测试初始编译失败：BOM 命令、汇总与工作区缺少 `specification_mode`，旧规范化函数无法表达单一产出。
- Vue BOM 测试初始 2 项失败：页面没有“产出结构”选择，商品组件没有直接商品身份分支。

## GREEN

- 应用、HTTP、PostgreSQL BOM、商品身份、库存、成本、生产与销售定向包测试通过。
- Vue BOM 44 项测试通过，覆盖字段显隐、切换确认、请求载荷和组件规格条件校验。
- `scripts/verify_kferp.sh all` 通过：完整 Go、Vue 1044 项测试与 Vite 构建均为绿色。
- development preflight 通过；`origin/develop@4da005b95254a6cceee6c691c3b358fede4f0e94` 已部署，外网登录烟测 HTTP 200。

## 业务验收

1. development 创建商品 `1065 PR619验收-袋装挂耳` 与 `1066 PR619验收-盒装挂耳`，发布并设默认单一产出 BOM `22001/V001`、`22002/V001`；页面显示盒装产出 `1盒`，配方为 `10袋袋装挂耳 + 1个挂耳盒子`，商品组件不选择 BOM 规格。
2. 商品 `1065/1066` 返回 `spec_identity_mode=product`、`bom_spec_authoritative=false`；BOM、库存、价格、计划与订单当前业务行的 `bom_spec_id/bom_variant_id` 均为 0。
3. 成本试算完成且无未解析组件：盒装挂耳 BOM 成本 `10.18/盒`，其中袋装挂耳 `10袋` 为 `9.98`，挂耳盒子为 `0.20`。
4. 订单 `1590 / SO-20260830-0001` 使用商品 `1066`、`2盒`、发布价格表 `112 / V3.0.32`；订单行保持直接商品身份。
5. 生产计划 `92 / PP-0000000092` 形成 `2盒商品1066 -> 20袋商品1065 -> 0.2kg物料71`，依赖分别保存 `required_units=20` 与 `required_g=200`。
6. 正式库存接口把商品 `1066` 调整为 `1盒`，读取结果为规格 ID 0、库存单位盒、`spec_identity_mode=product`，并产生 `0盒 -> 1盒` 操作日志。
7. 原多规格商品 `594 初晓 挂耳`、`1063 初晓-商品` 仍为 `bom_spec`，默认 BOM `8316/18587` 仍为 `spec_group`。
8. BOM 创建、草稿修改、发布、设默认，订单创建和成品库存调整均可在操作日志检索到。

## 交付边界

- 自动化与 development 业务样例已完成；Van 页面验收仍由 `REV-619-PRODUCT-BOM-SINGLE-OUTPUT` 跟踪，不在 Codex 自动验收中代签。
- 回滚点：源码 `/opt/stacks/erp/orderapp.backup.deploy-20260830024922-4da005b95254`，镜像 `kferp-orderapp-rollback:development-20260830024922-4da005b95254`。
- `main` 与 production 不在本次范围。
