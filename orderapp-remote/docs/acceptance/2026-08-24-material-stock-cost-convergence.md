# PR-605 物料档案、仓库盘点与成本入口收敛验收记录

## 范围

- 物料列表复选框与行业字段展示。
- 来源仓实时库存余额。
- 按仓库盘点、FIFO 批次分配及离散物料成本调整。
- 采购单与采购收货分离、采购收货原子过账、普通原料入库退役。
- 历史采购、库存单据、批次、流水和订单只读兼容。

## RED（实现前）

- Vue 定向合同 4/4 失败：缺少标准复选框、行业字段列、来源仓余额、按仓盘点、离散采购和收货确认。
- Go 定向合同编译失败：采购命令缺少 `qty/unit_code/qty_units/target_warehouse`。
- 普通 `POST /api/stock/material-receipts` 仍返回 200，可绕过采购收货直接建立库存和成本。

## GREEN（实现后）

- PR-605 与关联 Vue 定向测试：28/28；frontend 完整发现集：1030/1030。
- `go test ./...`：全包通过；PR-605 support 合同通过。
- 真实 PostgreSQL：purchase 2.422s、stock 18.149s、materials 1.840s，通过；覆盖单仓盘点不覆盖其他仓、FIFO 调整分配、包材仓离散数量、离散批次价值变化、采购收货成功一致性与强制失败完整回滚。
- `npm run build`：6594 modules，通过（仅保留既有 chunk size warning）。
- `scripts/verify_kferp.sh all`：exit 0。
- 最终功能提交 `a392d3bf` 的 development preflight 通过 Vue 1030/1030、小程序 217/217、typecheck、两个 Go 全包门禁及隔离镜像构建。
- 合并提交 `776d8d1f` 已部署 development；发布门禁全绿，登录页 HTTP 200，固定开发小程序包已同步。源码备份与回滚镜像均已生成；`main` 和 production 未操作。

## 人工验收

- Van 在 development 验收物料档案、库存作业、盘点调整和采购收货页面。
- 本次不操作 `main` 或 production，不重写历史业务记录。
