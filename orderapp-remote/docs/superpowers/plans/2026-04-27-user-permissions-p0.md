# 用户权限 P0 实施计划

1. 新增 `internal/application/authz`，定义角色、Actor、权限合并、页面可见性和员工角色分配命令。
2. 新增 `internal/infrastructure/postgres/authz`，创建角色、权限、角色权限、员工角色、页面权限表并写入默认种子。
3. 调整 schema 初始化顺序，让 `company` 先于依赖员工表的 `support` 和 `authz`。
4. 在 support HTTP 模块新增 `/api/auth/me`、角色列表、员工角色读取和保存接口。
5. 在 Echo 路由组合中接入权限中间件，对 `/api/*` 的业务接口映射到权限 code。
6. 在 Vue/Vite 前端新增权限 API、菜单过滤 helper 和用户权限页面。
7. 更新 5 张需求表种子，记录 PR/DEV/UT/API/REV。
8. 验证顺序：Go 单元/API 测试、Node 前端单测、Vite build、部署后 curl 和浏览器 smoke。
