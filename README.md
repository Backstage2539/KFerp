# KFerp

轻量的「订单 → 生产」业务系统仓库（中文业务场景），目标是逐步替代 Excel 流程，提供可追溯、可验收、可部署的交付方式。

## 项目简介

KFerp 当前包含两部分：

- **业务与交付文档**：需求、验收、流程、部署说明
- **应用代码**：`orderapp-remote`（Go 后端 + 前端页面）

核心能力围绕：录单、订单管理、客户/商品档案、审计留痕、生产相关页面。

## 环境要求

建议本地准备：

- Git 2.30+
- Docker / Docker Compose（部署相关）
- Node.js 18+（前端开发）
- npm 9+
- Go 1.22+（后端开发与测试）

> 说明：后端运行依赖 PostgreSQL（通过 `DATABASE_URL` 提供连接串）和应用鉴权变量（`APP_PASS` 等）。

## 快速开始（Quick Start）

下面是一套可复制执行的最小流程，用于 5 分钟内理解并进入开发状态：

```bash
git clone https://github.com/Backstage2539/KFerp.git
cd KFerp

# 1) 先看核心文档（建议顺序）
ls REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md

# 2) 进入主应用目录
cd orderapp-remote

# 3) 安装前端依赖（可选：需要改前端时）
cd frontend && npm install && cd ..

# 4) 查看后端测试命令（需要本机已安装 Go）
go test ./...
```

如果要启动后端（本地开发）：

```bash
cd orderapp-remote
export DATABASE_URL='postgresql://<user>:<pass>@<host>:5432/<db>?sslmode=disable'
export APP_PASS='<your-password>'
# 可选：export APP_USER='order'
# 可选：export LISTEN=':8080'
go run .
```

## 常见命令

- 前端本地开发（Vite）

```bash
cd orderapp-remote/frontend
npm install
npm run dev
```

- 后端测试

```bash
cd orderapp-remote
go test ./...
```

- 一键部署（按仓库脚本）

```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh
```

## 目录结构

- `orderapp-remote/`：主应用代码（Go 服务 + 页面/模板 + 前端目录）
- `orderapp-remote/frontend/`：React + Vite 前端工程
- `REQUIREMENTS.md`：业务需求定义
- `ACCEPTANCE_TESTS.md`：验收清单
- `HOW_TO_WORKFLOW.md`：PR/DEV/UT/API/REV 协作流程
- `DEPLOYMENT.md`：部署教程与排障
- `roastery-system/`：相关部署与基础设施目录（独立子系统）
- `scripts/`：辅助脚本

## FAQ

1) **Q: 为什么 `go test ./...` 报 `go: command not found`？**
- A: 本机未安装 Go。请先安装 Go 1.22+，再执行测试命令。

2) **Q: 后端启动时报 `DATABASE_URL is required`？**
- A: 需先设置 `DATABASE_URL` 环境变量，指向可访问的 PostgreSQL。

3) **Q: 后端启动时报 `APP_PASS is required`？**
- A: 需设置 `APP_PASS`（BasicAuth 密码）；可选设置 `APP_USER`，默认 `order`。

## 协作约定（简版）

- 需求先落文档：PR → DEV → UT/API → REV
- 交付必须有证据（命令输出/测试结果/日志）
- 业务口径以 `REQUIREMENTS.md` 与 `ACCEPTANCE_TESTS.md` 为准
