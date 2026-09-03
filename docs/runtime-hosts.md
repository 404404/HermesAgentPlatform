# Runtime hosts and placement (v0.3)

`runtime_hosts` is the infrastructure inventory boundary for a future Runtime Provider. It stores host identity, SSH port, authentication type, credential reference, capacity and observed inventory. Credentials are references only; the API never accepts or returns a plaintext secret. The Demo endpoint is a Mock Runtime Host Provider and deliberately does not expose a Docker Socket, privileged container or Docker-in-Docker path.

`User Runtime` remains the per-user Hermes runtime resource. A runtime may be placed on a healthy host by `MockScheduler`, which selects the least-used host for the Demo. Placement is recorded on the runtime (`host_id`, `placement_status`, actual resources and observed image) and can later be reconciled asynchronously by a real adapter.

The admin Runtime Management screen keeps User Runtimes and Runtime Hosts in separate tabs. Runtime Template fields remain infrastructure-only: CPU, memory, storage, profile limit, concurrency, provider, class and network policy. Model, Skill, Knowledge and Agent behavior stay in Agent Template/Profile domains.
