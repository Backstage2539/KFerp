# PR-629 商品归属交互与仓库领料统一开发环境验收

## 范围与发布

- 目标环境：development；生产环境未发布、未改数据。
- 主实现和连续修复经 PR [#63](https://github.com/Backstage2539/KFerp/pull/63)、[#64](https://github.com/Backstage2539/KFerp/pull/64)、[#65](https://github.com/Backstage2539/KFerp/pull/65)、[#66](https://github.com/Backstage2539/KFerp/pull/66)、[#67](https://github.com/Backstage2539/KFerp/pull/67)、[#68](https://github.com/Backstage2539/KFerp/pull/68)、[#69](https://github.com/Backstage2539/KFerp/pull/69) 合并到 `develop`。
- 最终业务代码提交：`04ef34b0ad5b92ae34a9d4ad3c9493ad4494d4eb`。
- development 发布命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`。
- 发布回滚源：`/opt/stacks/erp/orderapp.backup.deploy-20260905221431-04ef34b0ad5b`。
- 发布回滚镜像：`kferp-orderapp-rollback:development-20260905221431-04ef34b0ad5b`。
- `erp_orderapp` 为 running、restart count 0；`https://dev.qacoohee.com/app/login` 返回 HTTP 200；线上源代码包含 `p.sku_code` 修复。

## TDD 与自动验证

- RED：先补生产计划组件来源 API 与请求映射测试；首次运行 2 项失败，分别指出来源仓保存 endpoint 和逐组件 payload 尚不存在。
- RED：履约工作台真实 PostgreSQL 回归用例先稳定复现 `ERROR: column p.code does not exist (SQLSTATE 42703)`。
- GREEN：客户引用商品查询改读 `products.sku_code`；真实 PostgreSQL 用例验证商品档案编码、客户商品名和客户货号均正确返回。
- GREEN：`scripts/verify_kferp.sh all`、全部 Go 包、Vue 1077/1077、Vite 6597 modules、miniapp 220/220、类型检查和 development 微信包构建通过。
- 数据库模块全包在独立空库中仍有既有夹具缺少 `product_bom_spec_authorities`、`finished_inventory` 的失败；本次新增的真实 PostgreSQL 用例单独通过，服务器发布门禁及全量非数据库测试通过。该夹具限制不计作业务通过证据。

## 商品与物料页面

- 商品档案截图：[商品归属筛选、商品来源和商品归属](pr629-product-ownership-development.png)。页面显示“商品归属”可搜索筛选、“商品来源”“商品归属”两列；商品行操作顺序为“复制到客户”后“复制”，其中“复制”为最后一个操作。
- 物料档案截图：[物料归属筛选和复制到客户](pr629-material-ownership-development.png)。页面用“物料归属”筛选，关联操作为“复制到客户”，成功提示为“已保存物料归属，请在「物料归属」中搜索客户名称查看”。
- 商品 `1086 / SKU-001086 / PR629验收复制商品-0905` 复制给客户 `74 / 芬纳咖啡` 后，客户商品名为 `PR629芬纳复制商品`、客户货号为 `PR629-FN-001`；重复保存后有效引用仍为 1 条。
- 物料 `76 / MAT-859771260668 / PR629验收物料-0905` 复制给客户 74 和 298；重复保存不新增重复引用，未复制的客户 299 不在其归属范围。
- 商品与客户引用接口不再返回或处理供料方式；历史 `order_items.material_source_mode` 只保留审计值，新订单不写入也不参与计算。

## 旧数据迁移

- 执行前备份：`/opt/stacks/erp/backups/pr629-warehouse-source-20260905161635/development-full.dump`，15,366,618 bytes，SHA-256 `4f772af328d3e94252f3f2cc80e765349f244e93307f02f79df4af8e7c4c3164`。
- 预览清单 `pr629-warehouse-source-v1-r0-o687-d0-p1-w0`：687 条历史订单行、4 条已迁移客户需求、草稿计划 101、无客户专用活动工单、无阻断记录。
- apply 取消旧草稿计划 101，未重新扣库；重复 apply 和最终 verify 清单均为 `pr629-warehouse-source-v1-r0-o687-d0-p0-w0`。
- 预览、apply、重复 apply、verify 均保持：物料库位 151,180,586g / 1,801 units，成品批次 231,508g / 189 units，预留 8,055g / 21 units；未迁移客户需求为 0。
- 开发服务器保留 `/var/tmp/pr629-cutover-preview-773a3d9e.json`、`apply`、`repeat`、`verify` 和 `final-verify` 五份结果。

## 生产计划、领料、取消与完工

### 代加工客户 A

- 客户：`296 / 代加工客户A-PR627-0905`；商品：`1069 / PR627-A2-客户来料`；BOM `2470`、规格 `393`、变体 `486`。
- 计划 `103 / PP-0000000103`、计划行 80 选择 `customer_296_raw / owner 296`，需求 23,000g。选择时客户库存 5,000g，即使工厂同物料有 4,000g，提交仍返回缺少 18,000g，没有跨仓或跨货主借料。
- 客户补录 `30 / SE-0000000030` 入库 25,000g；相同幂等键重复提交未重复入库。
- 工单 `55 / WO-PP-0000000103-0000000080` 完成；预留 60 从客户两个批次冻结 5,000g + 18,000g，领料、耗用均为 23,000g，WIP 货主为 296。
- 领料单 `31 / SE-0000000031`，完工单 `32 / SE-0000000032`。客户原料仓余 7,000g；客户成品仓 `customer_296_finished / owner 296` 的商品 1069 为 25 units；工厂原料没有被此工单扣减。

### 批发客户 B

- 客户：`297 / 批发客户B-PR627-0905`，仅开通成品仓；商品：`1070 / PR627-B2-工厂供料`；BOM `2473`、规格 `396`、变体 `487`。
- 计划 `104 / PP-0000000104` 明确选择 `raw_materials / owner 0` 后生成工单 56 和预留 61；取消计划后工单为 cancelled，1,000g 预留全部释放，工厂库存未减少。
- 重建计划 `105 / PP-0000000105`，工单 `57 / WO-PP-0000000105-0000000082` 完成；预留 62 从工厂批次领用、耗用 1,000g。
- 领料单 `33 / SE-0000000033`，完工单 `34 / SE-0000000034`。工厂原料从 4,000g 变为 3,000g；客户成品仓 `customer_297_finished / owner 297` 的商品 1070 为 1 unit；不要求客户原料仓。
- 计划 103、104、105 的组件来源均保存来源仓、来源货主和选择时库存快照；相同计划重复提交被状态门禁拒绝，没有重复预留。

## 履约工作台和客户隔离

- 为员工 27/28 建立的临时开发验收会话完成后已删除；脱敏结果保留在开发服务器 `/var/tmp/pr629-portal-acceptance-final.json`。
- 员工 27 绑定客户 296：`/api/customer-processing/portal/options` 和 `/overview` 均为 200；候选含商品 1069，不含客户 297 的商品 1070。客户商品 B 显示为“客户商品B”，商品 C 显示为“PR627客户商品C”。
- 员工 28 绑定客户 297：两个接口均为 200；候选含商品 1070，不含客户 296 的商品 1069；公共商品 B 仍显示工厂名称“晚香玉”。
- 客户 296 工作台显示已发布价格表 `121 / PR627-A-V6` 等版本，只显示其客户库存：白月光 7,000g、挂耳滤袋 100 units、客户成品 1069 共 25 units。
- 客户 297 工作台显示已发布价格表 `117 / PR627-B-V2`、`115 / PR627-B-V1`，无原料或包材余额，只显示 `customer_297_finished` 中商品 1070 共 1 unit。
- 两个接口都从登录账号绑定客户派生范围；请求未携带可切换的客户参数，另一客户专属商品、价格表和仓库库存均未返回。

## 结论

- PR-629 开发、迁移、development 发布和实际业务验收完成。
- 商品/物料归属交互、无商品级供料方式、来源仓冻结、缺料不借仓、客户/工厂领料、取消释放、完工入库、价格表和库存隔离均通过。
- `REV-629-PRODUCT-CATALOG-WAREHOUSE-SOURCE` 保留给 Van 做页面复查；这不是开发环境遗留开发项。
