# Hermes Enterprise Platform (HEP) / Hermes 企业级平台

> **v0.3.3 Control Plane Demo / v0.3.3 控制平面 Demo**
>
> HEP is an enterprise management-plane demonstration for Hermes Agent. It is not a production Hermes runtime, Docker orchestrator, sandbox, or LLM gateway.
>
> HEP 是面向 Hermes Agent 的企业管理控制平面演示；它不是生产 Hermes Runtime、Docker 编排器、Sandbox 或 LLM Gateway。

## English

### Implemented now

- **One login, two protected surfaces:** User Workspace for every account and Admin Console for authorized administrators.
- **Organization & access:** department tree, user lifecycle, CSV validation/import, filter-based export, batch actions, scoped Role Bindings, and Effective Permissions.
- **Agents:** Agent Templates, managed/personal Agent Profiles, assignment sources, effective Model/Skill/Knowledge configuration, and policy-controlled configuration overlays.
- **Governance:** versioned Skill artifacts, ordered Skill review workflows, Knowledge content/version history/import jobs, approvals, executions, audit export, quotas, settings, and notifications.
- **Runtime control plane:** Runtime Templates, User Runtimes, desired/observed state, Runtime Hosts, placement, and inventory.
- **Model control plane:** Model Providers, Provider Models, logical Models, slot policies, and write-only inline credentials.
- **Workspace:** effective user resources, profile-aware Skill installs, policy-scoped Knowledge, Channels, Usage, and persisted Mock Chat conversations/messages/feedback.
- **Localization:** en-US and zh-CN, with browser-persisted language selection.

### Explicitly not implemented

HEP does **not** provision real Hermes containers or profiles, execute SSH/Docker commands, mount or access the host Docker Socket, call a real LLM, index a vector database, run a production Sandbox, or integrate Kubernetes, SSO, Vault, SIEM, or malware scanning. Runtime, host, model, knowledge, notification, and chat adapters are explicit Mock Provider seams.

### Quick start

```bash
cp .env.example .env
docker compose --env-file .env --project-directory . -f deploy/docker-compose.yml up -d --build
```

Open http://localhost:18080. The health endpoint is http://localhost:18081/healthz.

Before starting, set **ALLOWED_ORIGIN** to the browser URL (for example http://192.168.88.11:18080 on a LAN). **HEP_DEMO_MODE=true** is for a disposable local Demo only. **HEP_SECRET_MASTER_KEY** must be exactly 32 bytes, or base64: followed by 32 decoded bytes. GOPROXY defaults to https://mirrors.aliyun.com/goproxy/.

If a Synology Docker engine rejects a build due to its seccomp/BuildKit policy, correct the engine/build policy. Do not work around it by granting HEP privileged access or a host Docker Socket.

### Demo accounts

| Account | Password environment variable | .env.example value |
| --- | --- | --- |
| admin | SEED_ADMIN_PASSWORD | ChangeMe-Admin-2026! |
| user01 | HEP_DEMO_USER_PASSWORD | ChangeMe-User-2026! |
| user02 | HEP_DEMO_USER_PASSWORD | ChangeMe-User-2026! |

The database stores bcrypt hashes only. In Demo mode, seed updates these known Demo accounts to configured credentials without storing plaintext. Never use these example credentials or keys outside a local Demo.

### Main routes

| Surface | Routes |
| --- | --- |
| Login | /login |
| Workspace | /workspace/chat, /workspace/agents, /workspace/models, /workspace/skills, /workspace/knowledge, /workspace/channels, /workspace/usage, /workspace/settings |
| Admin | /admin/overview, /admin/organization, /admin/roles, /admin/agent-profiles, /admin/agent-templates, /admin/models, /admin/skills, /admin/knowledge, /admin/runtime, /admin/runtime-hosts, /admin/runtime-templates, /admin/scheduling, /admin/executions, /admin/approvals, /admin/audit, /admin/usage, /admin/quotas, /admin/settings |

Legacy Admin paths redirect to /admin equivalents. Backend RBAC is authoritative; hidden UI is not authorization.

### Verification

```bash
make test
docker compose --env-file .env --project-directory . -f deploy/docker-compose.yml ps
cd frontend && npm run i18n:audit && npm run build
```

The frontend build includes TypeScript checking. A standalone lint script is not currently defined.

See [documentation index / 文档索引](docs/README.md).

---

## 简体中文

### 当前已实现

- **单一登录、双界面：**所有账号使用用户工作台；有权限的管理员可进入管理控制台。
- **组织与访问：**部门树、用户生命周期、CSV 校验/导入、按筛选导出、批量操作、范围化 Role Binding 和有效权限。
- **Agent：**Agent 行为模板、受管/个人 Agent 配置文件、分配来源、最终生效的模型/Skills/知识库配置和受策略控制的覆盖配置。
- **治理：**版本化 Skill Artifact、有序 Skill 审核流、知识库内容/版本历史/导入任务、审批、执行记录、审计导出、配额、设置和通知。
- **Runtime 控制平面：**运行环境模板、用户运行环境、期望/实际状态、运行主机、调度位置和资源盘点。
- **模型控制平面：**模型供应商、上游模型、逻辑模型、槽位策略和只写入式内联凭据。
- **工作台：**有效用户资源、按 Profile 的 Skill 安装、策略范围内的知识库、Channels、用量，以及持久化的 Mock Chat 会话/消息/反馈。
- **国际化：**en-US 和 zh-CN，浏览器持久化语言选择。

### 明确未实现

HEP **不会**创建真实 Hermes 容器或 Profile、执行 SSH/Docker 命令、挂载/访问宿主机 Docker Socket、调用真实 LLM、建立向量数据库、运行生产 Sandbox，也没有 Kubernetes、SSO、Vault、SIEM 或恶意软件扫描集成。Runtime、主机、模型、知识、通知和聊天均使用明确的 Mock Provider 边界。

### 快速开始

```bash
cp .env.example .env
docker compose --env-file .env --project-directory . -f deploy/docker-compose.yml up -d --build
```

访问 http://localhost:18080；健康检查为 http://localhost:18081/healthz。

启动前请将 **ALLOWED_ORIGIN** 设置为浏览器实际地址（局域网示例：http://192.168.88.11:18080）。**HEP_DEMO_MODE=true** 只适用于可丢弃的本地 Demo；**HEP_SECRET_MASTER_KEY** 必须正好为 32 字节，或为 base64: 加 32 字节解码结果。GOPROXY 默认使用 https://mirrors.aliyun.com/goproxy/。

若群晖 Docker 因 seccomp/BuildKit 策略拒绝构建，应修复 Docker 引擎/构建策略；不要通过为 HEP 授予 privileged 权限或宿主机 Docker Socket 来绕过问题。

### Demo 账号

| 账号 | 密码环境变量 | .env.example 示例 |
| --- | --- | --- |
| admin | SEED_ADMIN_PASSWORD | ChangeMe-Admin-2026! |
| user01 | HEP_DEMO_USER_PASSWORD | ChangeMe-User-2026! |
| user02 | HEP_DEMO_USER_PASSWORD | ChangeMe-User-2026! |

数据库只存储 bcrypt Hash。Demo 模式下，Seed 会将这三个已知 Demo 账号更新为环境变量中的密码，但不会保存明文。示例账号、密码和密钥不得用于非本地 Demo 环境。

### 主要路由

登录页为 /login。用户工作台位于 /workspace/*，管理控制台位于 /admin/*。历史管理路径会重定向到 /admin 对应地址。后端 RBAC 是最终授权依据，隐藏 UI 不构成权限控制。

### 验证

```bash
make test
docker compose --env-file .env --project-directory . -f deploy/docker-compose.yml ps
cd frontend && npm run i18n:audit && npm run build
```

前端 build 包含 TypeScript 检查；当前没有单独定义 lint script。详细资料见 [文档索引](docs/README.md)。
