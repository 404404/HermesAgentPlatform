# Hermes Enterprise Platform (HEP)

Phase 1 control-plane skeleton for an enterprise Hermes Agent platform. HEP is an independent management plane; it does not modify Hermes upstream and does not expose the Docker socket to an Agent runtime.

## Quick start

```bash
cp .env.example .env
DOCKER_BUILDKIT=0 docker compose -f deploy/docker-compose.yml --project-directory . up -d --build
```

Open `http://localhost:18080`. The API health endpoint is `http://localhost:18081/healthz`.

The default demo administrator is `admin` with the password from `SEED_ADMIN_PASSWORD` (example: `ChangeMe-Admin-2026!`). Change it before any shared deployment. The backend hashes it with bcrypt; the password is never stored in plaintext.

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
