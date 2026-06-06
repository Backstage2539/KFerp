# PR-431 顶部通知浏览与清空验收记录

## 范围

- ERP 顶部通知仍保留 3 条紧凑展示窗口。
- 未读通知超过 3 条时，通过上下箭头查看所有当前已拉取通知。
- “清空”一键清除当前通知区，并同步后端已读，避免刷新或轮询后回弹。

## 测试证据

- RED：`node --test src/lib/global-notifications.test.js` 在实现前失败，因为 `clampNotificationWindowStart`、`notificationWindow` 和 `notificationBackendIDs` 尚未导出。
- RED：`go test ./internal/interfaces/http/support -run TestDev431NotificationCenterScrollClear -count=1` 在实现前失败，因为 PR-431 种子、App 接线和通知手册缺失。
- GREEN：`node --test src/lib/global-notifications.test.js` 覆盖通知窗口分页、后端通知 ID 去重、清空通知接线。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev431NotificationCenterScrollClear -count=1` 覆盖需求种子、Vue 接线、API 拉取上限和文档。

## 验收口径

- 顶部通知区出现计数，例如 `1-3 / 5`。
- 点击下一条通知箭头后，窗口移动并显示第 4 条以后通知；点击上一条通知箭头可回到前面的通知。
- 点击“清空通知”后，通知区立即消失；刷新或等待轮询后，已清空通知不再回弹。
- 新产生的后端通知或本地 `kferp:notify` 仍可正常显示。

## 实现标记

- `notificationWindow`
- `clampNotificationWindowStart`
- `notificationBackendIDs`
- `clearAllNotifications`
