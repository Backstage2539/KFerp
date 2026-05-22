# PR-315-SHELL-MENU-SCROLL-TOUCH 验收记录

## 范围
- 左侧菜单固定在单屏视口内独立滚动。
- 点击任何功能菜单后功能页面回到顶部。
- 手机端右滑呼出功能菜单，左滑隐藏功能菜单。

## 证据
- 前端单测：`node --test src/lib/view-routing.test.js src/lib/form-draft-cache.test.js`
- 支持层测试：`go test ./internal/interfaces/http/support -run 'TestDev31[45]' -count=1`
- 手册：`OP_MANUAL_WORKSPACE_MODE.md`

## 验收步骤
1. 打开 ERP Vue 工作台，滚动左侧菜单到底部。
2. 点击任意功能菜单，确认右侧功能页面立即显示在顶部。
3. 继续上下滚动左侧菜单，确认菜单滚动限制在当前屏幕范围内，右侧内容不被带动。
4. 手机宽度下右滑打开功能菜单，左滑隐藏功能菜单。
