# MEMORY.md

## Van (profile)
- Name: Van (Asia/Shanghai)
- Prefers bullet points; wants a crisp assistant.
- Coding: Go + VB in VS Code; ~8 years backend.
- Runs a coffee roastery (ToB/wholesale to coffee shops).

## Current direction (2026-02)
- 目标不变：替代 Excel，做“订单→生产”的轻量协作系统（中文数据）。
- 部署方案已更新：不再走 Synology NAS/DSM，改为云服务器部署。
- 现网：1.12.242.58（SSH 22，root），已为 JJ 加入公钥，允许直接登录做验收与排查。
- 现网运行形态：Docker（erp_orderapp + erp_caddy + erp_postgres）。Postgres 用户/库：nocodb/nocodb；业务 schema：p2rms15pepb5ciz。
- 交付流程约束（Van 要求，后续每个需求点都必须执行）：
  1) 单元测试（unit tests）
  2) API 层测试（接口级验证，尽量不走 UI）
  3) 需求验收（按 REQUIREMENTS/ACCEPTANCE_TESTS 对照打勾，产出通过/不通过与证据）
- 需求管理产出物（Van 14:05）：给到“产品需求”后，JJ 必须在 UI 中维护 5 张表，供 Van 随时查看：
  1) 产品需求表：从原始描述抽象/结构化后的产品需求
  2) 开发需求表：从产品需求拆解出的开发任务/技术实现点
  3) 单元测试表：每个开发需求对应的单测清单/结果/证据
  4) API 测试表：每个开发需求对应的接口测试清单/结果/证据
  5) 需求审核表：需求完成后逐条审核是否实现（通过/不通过/证据）
  - 工作流：产品需求 →（抽象入表）→ 拆解开发需求 →（实现）→ 单元测试/ API 测试入表 → 完成后做需求审核入表
  - 测试数据：验收/测试阶段允许 JJ 自造数据（如烘焙机器、载量区间、库存/批次）。

- Van 在 2026-02-17 新确认的“强制执行开发流程”（必须严格按顺序）：
  1) 收到产品需求后，先拆解为开发需求，并写入开发需求表（DEV）。
  2) 按开发需求逐条开发：
     - 2.1 先编写单元测试代码，并在单元测试表（UT）新增条目；同时编写 API 测试代码，并在 API 测试表（API）新增条目。
     - 2.2 再进行代码开发实现。
     - 2.3 单元测试通过后，更新 UT 表状态（含证据）。
     - 2.4 API 测试通过后，更新 API 表状态（含证据）。
  3) 完成后通知 Van 进行产品需求验收。
  4) Van 在需求审核表（REV）验收通过后，产品需求表（PR）状态自动更新为 done。

## 技术栈（2026-02-18 更新）

### React 前端（BOM 配方维护页面）
- **框架**：React 18 + TypeScript + Vite
- **状态管理**：TanStack Query (React Query)
- **HTTP 客户端**：Axios
- **样式**：Tailwind CSS
- **访问路径**：`/app/bom-react`
- **源码位置**：`orderapp-remote/frontend/`
