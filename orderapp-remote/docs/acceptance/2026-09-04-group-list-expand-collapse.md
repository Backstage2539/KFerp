# PR-626 全部展开与全部收缩补充验收

- DEV：`DEV-626-BULK-EXPAND-COLLAPSE`。
- 范围：商品档案、物料档案、生产 BOM、具体仓库库存的共享 Vue 分组工作区；沿用既有分组 helper，不新增 API/数据库/业务写入。
- 发布授权：2026-09-04 Van 明确要求依次合并 develop、部署 development、合并 main、部署 production。微信上传/审核/发布与人工业务验收仍由 Van 完成。

## RED / GREEN

- RED：真实 Vue SFC 组件测试 5 项因缺少“全部展开/全部收缩”按钮失败；support 交付合同因 DEV、手册和组件入口缺失失败。
- GREEN：新增 5 项组件运行时测试通过；合并既有分组 helper/四页合同回归共 29/29 通过。覆盖所有层级、未分类、无模板平铺组、空列表、加载/移动禁用、直接调用防护、搜索快照与移动恢复；业务行、分类页码及勾选不被改写。
- 完整门禁：`scripts/verify_kferp.sh all` 通过，Go 全包通过、前端 1065/1065、Vite 构建通过；仅保留既有大 chunk 提示。
- API：`go test ./internal/interfaces/http/support -run TestDev626GroupListInteractionDeliveryContracts -count=1` 通过，既有 assignment 合同不变。
- 本地浏览器样例：实际共享组件展开/收缩和移动禁用验证通过，390px 与桌面视口按钮成组换行正常；样例不连接业务数据，不能替代 Van 的真实 ERP 验收。
- 手册：`OP_MANUAL_INVENTORY_MATERIALS.md`、`OP_MANUAL_PRODUCTION.md`；既有 Vue 手册页面读取同一 Markdown 源。

## 发布检查点

- [ ] development：合并、预检、部署、容器/页面/需求 API/源码指纹验证。
- [ ] production：从最新 main 合并已验证 develop、预检、部署、容器/页面/需求 API/源码指纹验证。
- [ ] Van 四页面业务验收。
