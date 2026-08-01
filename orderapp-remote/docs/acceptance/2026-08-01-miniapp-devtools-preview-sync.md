# PR-570-MINIAPP-DEVTOOLS-PREVIEW-SYNC 验收记录

## 复现与根因

- 微信开发者工具实际打开 production 固定目录 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。该项目在 production 固定目录替换前已打开，替换前上一包没有 `pages/employee-customers/employee-customers`。
- 替换后的 production 和 development 固定包均声明该页面，12 个页面的 `.js/.json/.wxml/.wxss` 共 48 个文件全部存在；源码注册、构建层级和环境地址正确，不是构建漏页。
- DevTools 最后成功上传清单仍是替换前版本；之后虽然收到新页面文件变化且清除了编译缓存，自动预览仍连续返回 `800059 ... employee-customers.js, file not found`。因此根因是目录替换后当前 DevTools 会话保留旧上传清单。

## 修复

- `DEV-570-ARTIFACT-CLOSURE`：服务器构建后解析主包及分包页面，逐页检查四类生成文件并生成 `PAGE_FILE_MANIFEST`；固定包下载后由不依赖 Node 的 shell 校验器按清单复验全部页面文件。
- `DEV-570-DEVTOOLS-REFRESH`：保留完整目录替换和上一包备份，避免逐文件更新产生半包；部署完成明确提示已打开项目关闭并重新导入固定目录，只清编译缓存不作为恢复步骤。
- `DEV-570-DOCS-DELIVERY`：需求、验收、PR/DEV 种子、发布说明和小程序测试说明同步。没有业务数据或操作日志影响。

## TDD 证据

- RED：实现前测试名为 `TestDev570MiniappArtifactValidationAndWatcherSafeSync`，随后根据独立复核把范围收敛并更名为 `TestDev570MiniappArtifactValidationAndDevToolsRefreshContract`。
  - 当时命令：`go test ./internal/interfaces/http/support -run TestDev570MiniappArtifactValidationAndWatcherSafeSync -count=1`。
  - 当时结果：失败，明确报告缺少 `scripts/verify_mp_weixin_artifact.mjs`，证明发布流程没有逐页产物闭包门禁；测试更名原因和最终 GREEN 命令保留在下一项，避免把当前“无测试匹配”误当作 RED。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev570MiniappArtifactValidationAndDevToolsRefreshContract -count=1`
  - 结果：通过。完整客户维护页产物通过；删除 `pages/employee-customers/employee-customers.js` 后校验失败并明确报告该路径；发布脚本同时包含远程构建后、下载后校验及重开项目提示。

## 发布与人工验收

- 完整 `go test ./... -count=1`、`scripts/verify_kferp.sh changed`、Shell/Node 语法检查通过。故障注入覆盖第二次目录移动失败和第一次移动后收到 TERM，两种情况都恢复旧固定包；远端/本机路径规则和合法连续句点路径均通过。
- 后端、前端/DevTools 根因和需求文档三方独立复核当前无未关闭 P0-P2。
- development/production 远程预检、分支合并、双环境部署提交和固定包备份在发布完成后补充。
- 本次现有 DevTools 会话需要一次性关闭项目并重新导入 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。重新编译/自动预览由 Van 验收；微信上传、审核和正式发布不属于服务器部署步骤。
