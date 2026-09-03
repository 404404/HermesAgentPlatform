# API surface

All routes below are under `/api/v1`, require the local session except login, and return `{ "data": ... }` on success.

| Area | Routes |
| --- | --- |
| Auth | `POST /auth/login`, `GET /auth/me`, `POST /auth/logout` |
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

The session-scoped me endpoints expose the authenticated user data: profile permissions, Agents, Models, Skills, Knowledge, channels, usage, notifications and persisted MockChatProvider conversations. Admin-only additions include admin/access, Provider Models, provider test/sync, model slot policies, self-service and channel policies, Runtime Hosts, runtime placement and resource usage. Mutating routes retain session and CSRF protection.