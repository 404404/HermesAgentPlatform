# Security boundaries

- **User Runtime** is the future isolation boundary between user A and user B. Each user gets a separate runtime identity and lifecycle. Phase 1 represents it with mock state, not a production container boundary.
- **Agent Profile** is an internal logical configuration owned by a user. It is not a strong isolation boundary between profiles belonging to the same user.
- **Execution Sandbox** is the planned boundary around terminal/code execution. Phase 1 uses no privileged container, Docker-in-Docker, `hostPath`, host network or Docker Socket mount.
- **Knowledge ACL** is represented by explicit Knowledge Base bindings to departments, roles and profiles. Enterprise Knowledge Base data is not written into Hermes personal memory.
- **RBAC** is enforced in the backend through internal user IDs and scoped RoleBindings. External provider identities will authenticate only.

Passwords use bcrypt. Operational integration credentials (such as Runtime Host SSH passwords and Model Provider API tokens) are stored separately with AES-256-GCM ciphertext, a per-value random nonce, algorithm/key-version metadata and a startup-required `HEP_SECRET_MASTER_KEY`. Legacy bcrypt-protected operational credentials cannot be recovered and are returned only as `requires_reentry`; login password hashes are not affected. The login session is an opaque, in-memory token in an HttpOnly `hep_session` cookie; a deployment restart logs users out. A separate `hep_csrf` cookie must match `X-CSRF-Token` on mutations. CORS allows only the configured frontend origin. Password hashes, secrets and tokens are not returned by API responses or written to audit metadata.

Demo credentials are configuration-only: `SEED_ADMIN_PASSWORD` seeds `admin` and `HEP_DEMO_USER_PASSWORD` seeds `user01` and `user02`. `HEP_DEMO_MODE` must be explicitly true before a local-only documented fallback is allowed; when it is false, a missing password aborts startup. All seeded values are bcrypt hashes, never plaintext. Production hardening should move sessions to a durable encrypted store, enable Secure cookies behind HTTPS, add rate limits, email verification and session rotation.


## Phase 2 governance boundary

The backend RiskEvaluator, role checks and lifecycle orchestration are control-plane services. Break-glass login is critical and high-risk changes can become Approval Requests. Secret records contain references/status only; no plaintext model key is returned. The Demo still has in-memory sessions and Mock providers, and intentionally does not mount a Docker Socket, use privileged containers or perform real Hermes execution.


### v0.3.2 infrastructure boundary

Runtime Host onboarding stores only local Docker socket paths as future SSH/Node-Agent instructions. It never exposes a TCP Docker endpoint and this release does not perform SSH, Docker, Hermes provisioning or container control. `MockRuntimeHostProvider` verifies the intended API contract only.


## 简体中文（当前安全边界）

本地密码使用 bcrypt；Session Cookie 为 HttpOnly；变更使用双提交 CSRF；CORS 仅允许 ALLOWED_ORIGIN；后端 RBAC 与 /me 资源归属均在服务端执行。运行凭据由 SecretProvider 使用 AES-256-GCM 加密，并且只可写入；审计在应用语义上追加保存。

用户 Runtime 是未来用户隔离边界，Agent Profile 不是。Demo 不打开 SSH、不访问 Docker、不挂载宿主机 Socket、不使用 privileged/Docker-in-Docker。它仍使用内存 Session 和 Mock Provider，未实现 MFA、限流、生产 SSO、Vault、SIEM、WORM、生产 Sandbox 或真实 Hermes 执行。
