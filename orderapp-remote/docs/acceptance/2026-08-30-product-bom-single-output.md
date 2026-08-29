# PR-619 商品 BOM 单一产出与直接商品身份验收

## 范围

- 商品 BOM 新增 `single` 与 `spec_group` 两种明确产出结构。
- 单一产出贯通商品目录、库存、成本、生产、价格与 ERP 订单的直接商品身份。
- 保留既有多规格 BOM 和历史业务身份，不操作 production。

## RED

- Go 定向测试初始编译失败：BOM 命令、汇总与工作区缺少 `specification_mode`，旧规范化函数无法表达单一产出。
- Vue BOM 测试初始 2 项失败：页面没有“产出结构”选择，商品组件没有直接商品身份分支。

## GREEN

- 应用、HTTP、PostgreSQL BOM、商品身份、库存、成本与销售定向包测试通过。
- Vue BOM 44 项测试通过，覆盖字段显隐、切换确认、请求载荷和组件规格条件校验。
- 完整 Go、Vite、统一验证器和 development preflight/部署证据在交付完成后补记。

## 业务验收

1. 新建盒装挂耳商品 BOM，选择“单一产出”，产出 `1盒`。
2. 配方加入 `10袋袋装挂耳` 与包装材料；袋装挂耳组件不选择 BOM 规格。
3. 发布并设默认，确认商品返回 `spec_identity_mode=product` 且 `bom_spec_authoritative=false`。
4. 验证成本递归、生产计划依赖、库存收发、价格表和 ERP 订单均以商品 ID 和盒/袋库存单位工作，两个 BOM 规格字段为 0。
5. 验证原多规格咖啡豆仍要求精确 BOM 规格，历史订单和库存不变。

## 未完成边界

- development 自动业务样例、合并部署与 Van 页面验收尚未执行。
- `main` 与 production 不在本次范围。
