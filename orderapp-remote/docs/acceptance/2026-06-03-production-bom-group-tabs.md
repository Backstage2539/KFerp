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
- 合并后 `develop` 复验：
  - `node --test src/lib/bom.test.js` 通过。
  - `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/application/bom ./internal/interfaces/http/support -count=1` 通过。
  - `npm run build` 通过；仅有既有 Vite chunk size warning。
  - `scripts/verify_kferp.sh changed` 通过：命令退出码 0。

## Deployment Evidence
- Feature branch: `codex/production-bom-group-tabs-20260603`
- Feature commit: `af9015f4 refine production bom group tabs`
- Develop merge: `88dc3043ba9c0839d3bd7d01ff1c75c4d8c72f4b`
- Evidence sync commit: `68a00a6ebe997d88c4b5be8e0e899104e8c40dcb`
- Deploy command: `./deploy_orderapp.sh development`
- Final deployed commit: `origin/develop=68a00a6ebe997d88c4b5be8e0e899104e8c40dcb`
- Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603204052`
- Docker build: ran `go test ./...` successfully inside image build.
- Smoke:
  - `docker compose ps`: `erp_orderapp` up, `erp_postgres` healthy.
  - Unauthenticated `GET https://erp.qacoohee.com/app/`: `303` to `orders`.
  - Authenticated `GET https://erp.qacoohee.com/app/vue-shell`: `200`.
  - Requirement API exposes `PR-402-PRODUCTION-BOM-GROUP-TABS`.
  - Authenticated `GET /app/api/production-bom-groups?include_inactive=1`: `200`; `默认分组/默认配方组` count `0`.
  - Authenticated `GET /app/api/bom/list`: `200`.

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
