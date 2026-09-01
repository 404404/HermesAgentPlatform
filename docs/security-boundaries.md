# Security boundaries

- **User Runtime** is the future isolation boundary between user A and user B. Each user gets a separate runtime identity and lifecycle. Phase 1 represents it with mock state, not a production container boundary.
- **Agent Profile** is an internal logical configuration owned by a user. It is not a strong isolation boundary between profiles belonging to the same user.
- **Execution Sandbox** is the planned boundary around terminal/code execution. Phase 1 uses no privileged container, Docker-in-Docker, `hostPath`, host network or Docker Socket mount.
- **Knowledge ACL** is represented by explicit Knowledge Base bindings to departments, roles and profiles. Enterprise Knowledge Base data is not written into Hermes personal memory.
- **RBAC** is enforced in the backend through internal user IDs and scoped RoleBindings. External provider identities will authenticate only.

Passwords use bcrypt. The login session is an opaque, in-memory token in an HttpOnly `hep_session` cookie; a deployment restart logs users out. A separate `hep_csrf` cookie must match `X-CSRF-Token` on mutations. CORS allows only the configured frontend origin. Password hashes, secrets and tokens are not returned by API responses or written to audit metadata.

The demo password is configurable through `SEED_ADMIN_PASSWORD`; replace it before sharing the service. Production hardening should move sessions to a durable encrypted store, enable Secure cookies behind HTTPS, add rate limits, email verification and session rotation.


## Phase 2 governance boundary

The backend RiskEvaluator, role checks and lifecycle orchestration are control-plane services. Break-glass login is critical and high-risk changes can become Approval Requests. Secret records contain references/status only; no plaintext model key is returned. The Demo still has in-memory sessions and Mock providers, and intentionally does not mount a Docker Socket, use privileged containers or perform real Hermes execution.
