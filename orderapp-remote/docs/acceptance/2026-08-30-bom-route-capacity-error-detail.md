# PR-618 BOM 发布失效产能档错误定位验收

## 范围

- 生产 BOM 发布、工艺路线发布和 BOM 工序成本快照刷新使用同一套详细失效诊断。
- 不修改产能档、工位、工艺路线、BOM 或历史成本快照，只改错误定位信息。

## RED

- `go test ./internal/infrastructure/postgres -run TestStandardCostCapacityIssueError -count=1 -v`
- 失败原因：`StandardCostCapacityIssue` 不存在，系统只能返回原笼统错误。

## GREEN

- 详细诊断单元测试覆盖路线、工序、产能档、工位、产能档停用、工位停用、工位不适用和未选择产能档。
- BOM 发布 API 测试确认 HTTP 400 原样返回详细诊断。
- `go test ./... -count=1`、`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend` 全部通过。
- `develop` 合入和 development 部署证据由本次交付报告记录。

## 业务边界

- 当前 development 现场样例应定位为：`PR-616 挂耳生产路线` 第 1 道 `咖啡研磨`，产能档 `咖啡研磨 1kg/批` 已停用，且工位 `PR-616 挂耳生产工位` 不适用该工序。
- 系统不自动改成其他产能档；用户必须确认新费率和标准批量后自行选择。
