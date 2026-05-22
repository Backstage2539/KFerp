# 2026-05-22 豆单版本列表下载验收

## 需求

- PR-308-BEAN-LIST-VERSION-DOWNLOAD：产品豆单的“豆单版本列表”每条记录都支持“下载 PDF”。
- 下载必须使用该行版本保存时锁定的 `content` 和 `config` 快照，不能被当前商品实时价格、当前抽屉设置或当前筛选状态影响。
- 公共豆单、客户豆单、草稿、已发布和已撤回历史版本都可按行下载；没有快照内容的异常行禁用下载。

## 验收点

- [x] 版本列表操作列出现“下载 PDF”按钮，位于“生成新版”和“撤回”之前。
- [x] 点击“下载 PDF”时，前端把该行发布记录设为下载来源，并调用已有豆单 PDF 打印/保存流程。
- [x] 下载预览使用 `copyBeanListPublicationContentGroups(row)` 读取行快照，不回到当前成本试算或当前产品筛选重新构造历史豆单。
- [x] 下载主题通过 `beanListPublicationPdfOptions(row, pdfOptions)` 从行 `config` 和版本号恢复，支持生豆 `green` 类型、更新说明和分类编号开关。
- [x] 操作手册说明版本列表下载使用锁定快照。

## 验证证据

- `node --test src/lib/bean-list-pdf.test.js src/lib/costing-bean-list-version-ui.test.js`
- `go test ./internal/interfaces/http/support -run TestDev308BeanListVersionDownload -count=1`
