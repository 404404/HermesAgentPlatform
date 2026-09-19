# Runtime hosts and placement (v0.3)

`runtime_hosts` is the infrastructure inventory boundary for a future Runtime Provider. It stores host identity, SSH port, authentication type, credential reference, capacity and observed inventory. Credentials are references only; the API never accepts or returns a plaintext secret. The Demo endpoint is a Mock Runtime Host Provider and deliberately does not expose a Docker Socket, privileged container or Docker-in-Docker path.

`User Runtime` remains the per-user Hermes runtime resource. A runtime may be placed on a healthy host by `MockScheduler`, which selects the least-used host for the Demo. Placement is recorded on the runtime (`host_id`, `placement_status`, actual resources and observed image) and can later be reconciled asynchronously by a real adapter.

The admin Runtime Management screen keeps User Runtimes and Runtime Hosts in separate tabs. Runtime Template fields remain infrastructure-only: CPU, memory, storage, profile limit, concurrency, provider, class and network policy. Model, Skill, Knowledge and Agent behavior stay in Agent Template/Profile domains.


## v0.3.2 Runtime Infrastructure

Settings → Runtime Infrastructure is the physical/VM host onboarding surface. A Runtime Host holds address, SSH port and username, Docker local socket path, inventory and a credential reference. The write-only SSH password is encrypted in `secrets.ciphertext` with AES-256-GCM and `HEP_SECRET_MASTER_KEY`; no Runtime Host response exposes a password, raw secret fields or credential identifier. Existing legacy bcrypt operational secrets are explicitly marked `requires_reentry` because they cannot be recovered. The `MockRuntimeHostProvider` returns deterministic SSH/Docker/resource checks in this Demo and never opens SSH or Docker.

`runtimes.host_id` is retained as the migration-compatible physical column; APIs use `runtime_host_id`. A host owns zero or more User Runtimes, and User Runtime details return that same relationship. Runtime Host delete is blocked while runtimes remain placed on it.


## 简体中文（当前运行主机）

运行主机是未来承载用户 Runtime 的物理机/VM 清单，不是 Agent 配置或 Docker 控制端点。可在设置/运行环境管理中新增、编辑、查看、测试和盘点主机，记录主机名/IP、SSH、认证、只写凭据、本地 Docker Socket 路径、Labels、容量和状态。

主机状态为 online、offline、degraded、maintenance、draining、unknown。Runtime 通过 runtimes.host_id（公开名称 runtime_host_id）关联主机；仍有 Runtime 时不能删除主机。MockRuntimeHostProvider 只返回确定性检查结果，MockScheduler 只记录调度决定；二者均不会 SSH、Docker、创建容器或迁移 Runtime。
