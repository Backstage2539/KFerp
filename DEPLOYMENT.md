# DEPLOYMENT - KFerp 部署流程与教程

> 目标：把“从本地代码到线上可访问”这条链路写成可重复执行的步骤。
> 
> 当前线上环境（截至 2026-02）：
> - 域名：`https://erp.qacoohee.com/`
> - 服务器：`root@1.12.242.58`
> - 部署目录：`/opt/stacks/erp`
> - 容器：`erp_orderapp`、`erp_caddy`、`erp_postgres`

---

## 0. 前置条件

### 0.1 你需要有的东西
- 服务器 SSH 权限（能登录 `root@1.12.242.58`）
- 本地有私钥（本项目默认使用 workspace 内的 key）：
  - `openclaw_jj_ed25519`
- 本地有基础工具：
  - `ssh` / `scp`
  - `docker`（仅服务器需要，本地不需要）

### 0.2 服务器端依赖
服务器 `/opt/stacks/erp` 下使用 `docker compose` 管理服务，需要：
- Docker Engine
- Docker Compose v2（`docker compose` 子命令）

---

## 1. 线上结构说明（你在服务器上会看到什么）

登录服务器：
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58
```

核心目录：
- `/opt/stacks/erp/docker-compose.yml`：compose 定义
- `/opt/stacks/erp/.env`：环境变量（包含数据库密码、ORDERAPP_PASS 等）
- `/opt/stacks/erp/orderapp/`：orderapp 源码（会被脚本覆盖同步）
- `/opt/stacks/erp/orderapp_data/`：orderapp 持久化数据（assets 等）
- `/opt/stacks/erp/postgres_data/`：Postgres 持久化数据

---

## 2. 一键部署（推荐）

仓库根目录有脚本：`deploy_orderapp.sh`

### 2.1 脚本做了什么
1) 同步 docs（用于 `/app/docs` 页面展示）：
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
- `HOW_TO_WORKFLOW.md`
- `DEPLOYMENT.md`（本教程）

2) 同步应用源码：
- 把 `orderapp-remote/*` 拷贝到服务器 `/opt/stacks/erp/orderapp/`

3) 重建并滚动更新容器：
- `docker compose build orderapp`
- `docker compose up -d`

### 2.2 执行部署
在本机（workspace 根目录）执行：
```bash
chmod +x deploy_orderapp.sh
./deploy_orderapp.sh
```

执行完成后，访问：
- `https://erp.qacoohee.com/app/`
- `https://erp.qacoohee.com/app/docs`

---

## 3. 手动部署（用于排障/理解）

### 3.1 同步 docs
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "mkdir -p /opt/stacks/erp/orderapp/docs"
scp -i openclaw_jj_ed25519 REQUIREMENTS.md ACCEPTANCE_TESTS.md HOW_TO_WORKFLOW.md DEPLOYMENT.md root@1.12.242.58:/opt/stacks/erp/orderapp/docs/
```

### 3.2 同步源码
```bash
scp -i openclaw_jj_ed25519 -r orderapp-remote/* root@1.12.242.58:/opt/stacks/erp/orderapp/
```

### 3.3 构建与重启
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d"
```

### 3.4 查看状态/日志
```bash
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "cd /opt/stacks/erp && docker compose ps"
ssh -i openclaw_jj_ed25519 root@1.12.242.58 "docker logs --tail=200 erp_orderapp"
```

---

## 4. 常见问题（FAQ）

### Q1: 页面 401 未授权
orderapp 走 BasicAuth：
- 用户名默认：`order`
- 密码来自服务器 `/opt/stacks/erp/.env` 的 `ORDERAPP_PASS`

### Q2: push 到 GitHub 失败（Repository not found）
通常是 SSH Key 权限问题：
- 如果 `ssh -T git@github.com` 显示 `Hi Backstage2539!` 才是账号级别 key
- 如果显示 `Hi Backstage2539/<repo>!` 说明你加成了 deploy key（只对单仓库生效）

### Q3: 容器启动失败
先看 compose 状态 + 日志：
```bash
docker compose ps
docker logs --tail=200 erp_orderapp
```

---

## 5. 建议的发布节奏
- 小改动：直接 `./deploy_orderapp.sh`
- 大改动：先在本地/测试环境跑一遍（特别是数据库变更），再部署线上
