# PR-523 咖啡销售历史数据清洗与临时库验收记录

## 范围
- 源文件：`/data/orderlist.xlsx`，SHA256 `8274bd2c6c9037be5ae2e8afc72f5000e20db2b4031ee7411cb131f205512d99`。
- 纳入月份：2025-01 至 2026-06，共 18 个工作表；工作簿共 74 个工作表。
- 正式业务库只读，清洗结果写入独立数据库 `kferp_orderlist_staging`。

## RED / GREEN
- RED：来源键、客户归并、商品解析、隔离 DDL 和审核工作簿合同测试在实现前因符号缺失失败。
- GREEN：`go test ./internal/migration/orderliststaging ./cmd/orderlist-staging -count=1` 通过。
- 数据质量 RED：带序号商品行和缺包装单位行测试先失败；修复后列表序号被移除，缺包装单位保持待审核。
- 客户质量 RED：联系人字段中的完整地址曾被自动作为客户名；修复后疑似地址、手机号或配送说明名称进入 `customer_name_needs_review`。
- 商品质量 RED：二维码、联系方式、现货替换等说明曾被识别为父商品；修复后进入 `product_line_needs_review`。

## 本地清洗证据
- 原始业务行 2,109；有效订单候选 2,104；无来源键审核行 5。
- 订单明细 5,150；客户候选 1,321；父商品候选 1,768；SKU 规格 403。
- 重复序号后缀分配 28 行，最大后缀 8；1 行因后缀与真实序号冲突进入审核。
- 使用首次映射执行增量重跑后，`source-key-mapping.json` 字节一致；2,109 条指纹到来源键映射 SHA256 均为 `df384365489ca1a53ebf93938b0712b0f20ed370200f50981a0756b8ff65d9df`。
- `orderlist-review.xlsx` 生成 11 个工作表；artifact_tool 公式错误扫描为 0，并完成所有工作表渲染。

## 临时库与隔离证据
- 独立数据库 `kferp_orderlist_staging` 已创建专用非超级用户角色，并建立 `raw/reference/curated/review` 四层 schema；未重启容器、未部署生产应用。
- 最终装载计数：import_runs=1、sheets=74、raw_orders=2,109、customers=1,321、products=1,768、skus=403、orders=2,104、curated order_items=5,138、issues=3,692、revisions=0。
- 5 个无有效来源键的原始行包含 12 条商品描述，只保留在 `raw.order_rows`、清洗数据集和审核工作簿，不进入有订单外键约束的 `curated.order_items`；因此原始拆分总数 5,150 与有效订单明细 5,138 的差额为 12。
- 相同最终装载 SQL 再次执行前后聚合计数均为 `1|2109|0|1321|1768|403|2104|5138|3692`，证明重复执行不增加候选或修订记录。
- 正式生产装载前基线：customers=6、products=0、orders=0、order_items=0。
- 装载完成后正式生产计数仍为 customers=6、products=0、orders=0、order_items=0，四项完全一致。
- 临时库大小 24 MB；备份位于 `/data/orderlist-staging/orderlist-202501-202606-8274bd2c/backups/kferp_orderlist_staging.dump`，大小 1,145,854 bytes，并生成 SHA256 清单。
- 服务器交付目录共 45 MB；目录权限为 700、文件权限为 600，权限扫描没有发现组或其他用户可读文件。

## 结论
- 清洗、审核工作簿、独立临时库装载、幂等重跑、备份和正式库零写入复核均完成。数据仍是审核候选，未创建正式客户、商品、SKU 或订单。
