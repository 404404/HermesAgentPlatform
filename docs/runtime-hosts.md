# Runtime hosts and placement (v0.3)

`runtime_hosts` is the infrastructure inventory boundary for a future Runtime Provider. It stores host identity, SSH port, authentication type, credential reference, capacity and observed inventory. Credentials are references only; the API never accepts or returns a plaintext secret. The Demo endpoint is a Mock Runtime Host Provider and deliberately does not expose a Docker Socket, privileged container or Docker-in-Docker path.

`User Runtime` remains the per-user Hermes runtime resource. A runtime may be placed on a healthy host by `MockScheduler`, which selects the least-used host for the Demo. Placement is recorded on the runtime (`host_id`, `placement_status`, actual resources and observed image) and can later be reconciled asynchronously by a real adapter.

The admin Runtime Management screen keeps User Runtimes and Runtime Hosts in separate tabs. Runtime Template fields remain infrastructure-only: CPU, memory, storage, profile limit, concurrency, provider, class and network policy. Model, Skill, Knowledge and Agent behavior stay in Agent Template/Profile domains.


## v0.3.2 Runtime Infrastructure

Settings → Runtime Infrastructure is the physical/VM host onboarding surface. A Runtime Host holds address, SSH port and username, Docker local socket path, inventory and a credential reference. The write-only SSH password is encrypted in `secrets.ciphertext` with AES-256-GCM and `HEP_SECRET_MASTER_KEY`; no Runtime Host response exposes a password, raw secret fields or credential identifier. Existing legacy bcrypt operational secrets are explicitly marked `requires_reentry` because they cannot be recovered. The `MockRuntimeHostProvider` returns deterministic SSH/Docker/resource checks in this Demo and never opens SSH or Docker.

`runtimes.host_id` is retained as the migration-compatible physical column; APIs use `runtime_host_id`. A host owns zero or more User Runtimes, and User Runtime details return that same relationship. Runtime Host delete is blocked while runtimes remain placed on it.
