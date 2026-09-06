# Hermes Enterprise Platform (HEP)

Phase 1 control-plane skeleton for an enterprise Hermes Agent platform. HEP is an independent management plane; it does not modify Hermes upstream and does not expose the Docker socket to an Agent runtime.

## Quick start

```bash
cp .env.example .env
DOCKER_BUILDKIT=0 docker compose -f deploy/docker-compose.yml --project-directory . up -d --build
```

Open `http://localhost:18080`. The API health endpoint is `http://localhost:18081/healthz`.

## Demo accounts and passwords

The local Demo seeds these accounts:

- `admin` — password from `SEED_ADMIN_PASSWORD` (example: `ChangeMe-Admin-2026!`)
- `user01` and `user02` — password from `HEP_DEMO_USER_PASSWORD` (example: `ChangeMe-User-2026!`)

Set `HEP_DEMO_MODE=true` only for a local Demo. In that mode, omitted password variables use the documented examples above. With `HEP_DEMO_MODE=false`, both password variables are mandatory and the backend refuses to start if either is missing. The backend stores bcrypt hashes only; on every Demo seed run it re-hashes the configured password for the seeded accounts, so an existing Demo database is reset to the configured credentials without ever storing plaintext.

## Phase 1

- Local Account login with HttpOnly session cookie, double-submit CSRF token, restricted CORS and security headers.
- Dashboard, Users, Departments tree, scoped RBAC display, Agent Profiles, User Runtimes, Model Catalog, Skill Market/review queue, Knowledge Bases, Usage and Audit Logs.
- MySQL 8 migrations and idempotent demo seed data.
- `RuntimeProvider`, `SandboxProvider`, `KnowledgeProvider`, `UsageCollector` and Hermes Adapter seams with safe Mock implementations.

## Development

```bash
make test
make up
make status
make logs
```

Go dependencies use `https://mirrors.aliyun.com/goproxy/` by default. Runtime operations are mock state transitions; Phase 1 intentionally does not start per-user Hermes containers, execute arbitrary code, call an LLM or require a vector database.

See `docs/architecture.md`, `docs/database.md`, `docs/security-boundaries.md` and `docs/future-roadmap.md` for boundary decisions and next phases.
