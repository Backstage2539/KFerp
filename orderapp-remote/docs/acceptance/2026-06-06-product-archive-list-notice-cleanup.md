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

## Pending
- Merge to `develop`, development deploy, and browser acceptance on 商品档案.
