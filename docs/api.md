# API surface

All routes below are under `/api/v1`, require the local session except login, and return `{ "data": ... }` on success.

| Area | Routes |
| --- | --- |
| Auth | `POST /auth/login`, `GET /auth/demo-info`, `GET /auth/me`, `POST /auth/logout` |
| Dashboard | `GET /dashboard` |
| Users | `GET/POST /users`, `PUT /users/:id`, `POST /users/:id/status` |
| Departments | `GET /departments/tree`, `POST/PUT/DELETE /departments/:id` |
| RBAC | `GET /roles` |
| Profiles | `GET/POST /profiles`, `PUT/DELETE /profiles/:id`, `POST /profiles/:id/status` |
| Runtimes | `GET /runtimes`, `POST /runtimes/:id/action` |
| Models | `GET/POST /models`, `PUT /models/:id` |
| Skills | `GET/POST /skills`, `POST /skills/:id/submit`, `GET /skill-submissions`, `POST /skill-submissions/:id/review`, `POST /skill-submissions/:id/publish` |
| Knowledge | `GET/POST /knowledge-bases`, `POST /knowledge-bases/:id/bindings` |
| Analytics | `GET /usage/overview`, `GET /audit-logs` |

Mutating requests must include the `hep_csrf` cookie value in the `X-CSRF-Token` header. The frontend API client handles this automatically.


## Phase 2 routes

Additional route groups cover managed RBAC and effective permissions, Runtime and Profile Templates, model providers and secret references, Skill versions and files, Knowledge documents and bindings, server-side Audit query/export, Risk Events, Approval Center, persistent Settings, Health, Notifications and Quotas. Mutations continue to require the CSRF cookie/header pair. Errors from new endpoints use `error_code` and `message_params` so the UI owns localization.


## v0.2.1 consolidated routes

| Area | Routes |
| --- | --- |
| Agent Templates | GET/POST/PUT /agent-templates, GET /agent-templates/:id, POST status and assignments, DELETE assignments, GET instances |
| Profiles | GET /profiles/:id/effective-configuration, GET assignment-sources |
| Users | GET summary/activity/effective-permissions and POST reconcile; existing role assign/remove remains backend checked |
| Departments | POST /departments/manage, GET/PUT /departments/:id/detail, POST status, DELETE /departments/:id/managed |
| Runtime | GET detail/effective-skills/executions/events, POST control and kill-switch; existing PUT /runtimes/:id edits resources and may create approval |
| Knowledge | GET/POST/PUT items, publish, versions, consumers, bindings and DELETE binding |
| Execution | GET/POST /executions and GET /executions/:id; high or critical requests create Approval Requests |
| Audit | GET /audit-logs/v2, GET /audit-catalog and backend CSV/JSON export using the same server-side filters |
| Dashboard | GET /dashboard/v3 |

The routes continue to use the session and CSRF middleware. Mutations return structured error codes where implemented, allowing the single frontend i18n layer to localize user-facing text.
## v0.2.2 management routes

The consolidated UI uses `GET /users` with q, department_id, role_id, status, runtime_status and template_id filters. User management also provides `POST /users/import/validate`, `POST /users/import/confirm`, `POST /users/batch` and `GET /users/export`. Role membership uses `GET/POST /roles/:id/members` and `DELETE /roles/:id/members/:binding_id`. Runtime policy uses `GET/POST/PUT/DELETE /runtime-templates/:id/bindings...`. Knowledge Binding responses include target IDs for dependency navigation.

`GET /audit-logs/v2` and `/audit-logs/export` share server-side filters including time range and Runtime, Skill and Model dimensions. `GET /audit-catalog` supplies category and action choices. Mutating routes continue to require session authorization and CSRF protection.


## v0.3 Workspace and infrastructure routes

`GET /me` returns the authenticated identity, department, enterprise roles, effective RBAC permissions, self-service capabilities, Admin eligibility and Runtime summary. The remaining session-scoped me endpoints expose the user-owned Agents, effective Models, Skills, Knowledge, Channels, Usage, notifications and persisted MockChatProvider conversations. Admin-only additions include admin/access, Provider Models, provider test/sync, model slot policies, self-service and channel policies, Runtime Hosts, runtime placement and resource usage. Mutating routes retain session and CSRF protection.

### v0.3.2 infrastructure API

`GET/POST /runtime-hosts`, `GET/PUT/DELETE /runtime-hosts/:id`, `POST /runtime-hosts/:id/test`, `POST /runtime-hosts/:id/inventory`, and `POST /runtime-hosts/:id/status` manage physical/VM inventory behind `runtime.manage`. Credential payload fields are write-only. `GET /runtimes-v2` and `GET /runtimes/:id/detail` expose `runtime_host_id`; API consumers must not construct a second host/runtime relationship. Model Provider create/update accepts write-only `credential` in the existing `/model-providers` API while responses expose only configuration status.


## 简体中文（当前 API）

所有接口均以 /api/v1 为前缀。登录后通过 HttpOnly Session 鉴权；所有变更请求还需携带 hep_csrf Cookie 对应的 X-CSRF-Token。成功响应使用 { "data": ... }，较新的接口使用 error_code 和 message_params 供前端本地化。

- 身份：/auth/login、/auth/logout、/auth/me、/admin/access。
- 管理端：组织用户、RBAC、Agent Profile/Template、Runtime/Runtime Host/Template、模型供应商、Skills、知识库、执行、审批、审计、配额和设置。
- 工作台：/me、/me/agents、/me/models、/me/skills、/me/knowledge、/me/channels、/me/usage，以及 /me/conversations 和消息候选/反馈接口。
- Runtime Host 和 Model Provider 的 credential 为只写入字段；读取结果仅显示配置状态。
- /me 路由只表示当前登录用户，不能通过参数冒充其他用户。
