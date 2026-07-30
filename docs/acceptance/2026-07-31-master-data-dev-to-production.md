# 2026-07-31 开发环境业务主数据迁移到正式环境

## 结果

- 状态：通过
- 迁移方式：五批事务合并，开发同业务键覆盖、开发独有新增、正式独有保留
- 客户账号绑定：未迁移
- 服务重启：无

## 安全证据

- 迁移前完整备份：
  - `/opt/stacks/erp-production/backups/pre-master-data-apply-20260731-002422.dump`
  - 同目录保存 SHA-256 校验文件，且已通过 `pg_restore -l` 可读性校验
- 回滚式预演报告：
  - `/opt/stacks/erp-production/migrations/master-data-20260731/dry-run.json`
- 正式执行报告：
  - `/opt/stacks/erp-production/migrations/master-data-20260731/apply.json`
- 排除表迁移前后对比：
  - `excluded-before.txt` 与 `excluded-after.txt` 无差异
  - 覆盖订单、订单明细、库存批次、出入库、工单、生产计划、财务、员工角色、客户账号绑定和登录会话

## 数据验收

| 检查项 | 结果 |
| --- | ---: |
| 客户 | 390 |
| 客户重名组 | 0 |
| 商品 | 861 |
| 原料 | 98 |
| 商品分类 | 86 |
| 商品阶梯价格 | 469 |
| 生产 BOM | 143 |
| BOM 版本 | 227 |
| BOM 明细 | 270 |
| 商品生产配置 | 507 |
| 工艺路线 | 5 |
| 商品客户孤儿引用 | 0 |
| 价格商品孤儿引用 | 0 |
| BOM 版本孤儿引用 | 0 |
| BOM 明细孤儿引用 | 0 |
| 生产配置商品孤儿引用 | 0 |
| 指定员工管理员+销售角色 | 2 |

## API 验收

以下正式环境接口通过应用 BasicAuth 直接访问，均返回 HTTP 200：

- `/api/customers?limit=1`
- `/api/products?limit=1`
- `/api/materials`
- `/api/production-boms`
- `/api/process-routes`
- `/api/manufacturing-workstations`
- `/api/product-production-configs`
- `/api/order/form`

公网未认证 `/app/` 返回 HTTP 401，符合访问控制预期；正式环境容器均处于运行状态，PostgreSQL 健康检查通过。
