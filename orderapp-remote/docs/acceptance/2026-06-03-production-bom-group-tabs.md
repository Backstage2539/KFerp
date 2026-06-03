# PR-402 生产 BOM 分组 Tab 与未分类

## 背景
- Van 要求生产 BOM 分组去掉默认分组。
- 生产 BOM 页面不再保留截图中的独立“生产 BOM 档案”列表，只保留“商品 BOM列表”作为主列表。
- 分组交互改成和商品档案/客户商品名一致的 Tab：全部分组、未分类、用户新增分组。

## 需求
- 不再创建或展示 `默认分组` / `默认配方组`；旧默认分组下 BOM 迁回未分类。
- 商品 BOM列表上方显示“全部分组”“未分类”和用户新增分组 Tab。
- 一个 BOM 只能属于一个分组 Tab；全部分组展示所有 BOM，未分类展示 `group_id=0` 的 BOM。
- 勾选商品 BOM 后，通过“移动到分组”卡片移动到未分类或某个分组；移动覆盖旧分组并继续写操作日志。
- 删除分组时，该分组下 BOM 回到未分类，不删除 BOM、不改变商品绑定、版本或配方明细。
- BOM 名称作为编辑入口；操作列只保留“复制”和红色“失效”。

## RED Evidence
- `node --test src/lib/bom.test.js`
  - 初始失败：页面仍显示“生产 BOM 档案”、`group-tree` 和默认分组文案，未显示“全部分组/未分类/移动到分组”。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomGroupsArePureUIFoldersWithDeleteAndSort -count=1`
  - 初始失败：删除分组仍把 BOM 移到默认分组，后端仍依赖默认分组。

## GREEN Evidence
- `node --test src/lib/bom.test.js`
  - 通过。
- `go test ./internal/infrastructure/postgres/bom -run TestProductionBomGroupsArePureUIFoldersWithDeleteAndSort -count=1`
  - 通过。
- `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/application/bom -count=1`
  - 通过。
- `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/application/bom ./internal/interfaces/http/support -count=1`
  - 通过。
- `npm run build`（在 `orderapp-remote/frontend-vue-shell`）
  - 通过；仅有既有 Vite chunk size warning。
- `go test ./...`（在 `orderapp-remote`）
  - 通过。
- `scripts/verify_kferp.sh changed`
  - 通过：命令退出码 0。
- 待补充：development deploy smoke。

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
