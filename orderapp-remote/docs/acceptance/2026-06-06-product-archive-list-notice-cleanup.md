# 2026-06-06 商品档案列表与顶部通知清理

## Scope
- PR-429-PRODUCT-ARCHIVE-LIST-NOTICE-CLEANUP
- 商品档案列表删除独立 `BOM 使用` 列、行内 `查看使用关系` 按钮、挂耳 `每袋克重` 和 `每盒袋数` 行内编辑。
- 顶部新订单通知关闭后增加当前浏览器本地已关闭兜底，避免轮询重新弹出同一通知。

## RED
- `node --test src/lib/global-notifications.test.js src/lib/product-settings.test.js` failed before implementation because `filterDismissedNotifications` was not exported, product basics/custom SKU payload still emitted `drip_bag_grams` / `drip_box_bag_count`, and the 商品档案 table still rendered `BOM 使用` / `查看使用关系`.

## GREEN
- `node --test src/lib/global-notifications.test.js src/lib/product-settings.test.js` passed 115/115 after implementation.
- `go test ./internal/interfaces/http/support -run 'TestProductArchiveListNoticeCleanup' -count=1` passed after requirement seed and manual updates.
- `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size/plugin timing warning.
- `go test ./internal/interfaces/http/support -count=1` passed after updating historical source guards.
- `go test ./...` in `orderapp-remote` passed.
- `scripts/verify_kferp.sh changed` passed.
- `git diff --check` passed.

## Deploy
- Merged and pushed `develop` at `7b4e113ec910cb2ce41fed860e7842717899ee17`.
- `./deploy_orderapp.sh` deployed development successfully.
- Previous app backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260606131523`.
- Smoke: `/vue-shell?view=productMaster&view_context=customer&workspace=customer&customer_id=169` returned 200; `erp_orderapp` restarted and logs showed `orderapp listening on :8080`.

## Browser Acceptance
- 商品档案表头为 `商品名 / 商品编号 / 当前归类 / 行业字段 / 归属 / 新增动作 / 预期损耗率 / 利润率覆盖 / 商品状态 / 处理 / 备注`，不再有 `BOM 使用`。
- 商品档案页面不再出现 `查看使用关系`、`每袋克重`、`每袋克数` 或 `每盒袋数`。
- 点击商品名仍能打开“商品档案配置”抽屉，并显示“被哪些 BOM 使用”。
- 关闭 `SO-20260606-0013` 顶部新订单通知并等待超过一次 15 秒轮询后，该通知未重新出现；当前可见通知补位为 `SO-20260606-0012`、`SO-20260606-0011`、`SO-20260606-0010`。
