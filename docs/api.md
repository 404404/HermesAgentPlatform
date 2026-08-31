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
