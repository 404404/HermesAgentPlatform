# Roadmap

- **Phase 2**: real Hermes User Runtime provider and runtime health lifecycle.
- **Phase 3**: Hermes usage and skill telemetry ingestion through the Adapter.
- **Phase 4**: Skill Registry and signed artifact storage.
- **Phase 5**: Knowledge Gateway with document indexing, RAG retrieval and ACL enforcement.
- **Phase 6**: Sandbox Broker with policy checks and isolated execution.
- **Phase 7**: OIDC, SAML, LDAP and SCIM providers, while retaining internal users and RBAC principals.
- **Phase 8**: Kubernetes, gVisor and Kata runtime/sandbox providers.

Explicitly out of Phase 1: Kubernetes, gVisor, Kata, Firecracker, real production sandboxing, per-user Hermes deployment, LLM calls, vector DB, SSO/LDAP/SCIM, malware scanning, HA and multi-node scheduling.


## Revised roadmap after Phase 2

Phase 2 establishes the Control Plane contract and Mock Provider seams. Next priorities are durable session and secret providers, asynchronous idempotent Hermes Runtime reconciliation, signed/object-backed Skill artifacts, a Knowledge Gateway with real indexing and ACL enforcement, and then production sandbox and identity integrations. Kubernetes, gVisor, Kata, Firecracker, real LLM/vector services, Vault, SIEM and malware scanning remain out of scope for this Demo.


## 简体中文（后续路线）

下一阶段应保持当前 Control Plane 合同：先建设持久 Session、凭据轮换、MFA、限流和部署加固；随后建设异步幂等 HermesAdapter/RuntimeProvider 调和，以及通过 mTLS 的 HEP Node Agent。之后才是模型网关、知识检索/ACL、签名 Skill 分发、生产 Sandbox、外部身份、SIEM 与 WORM 审计。

Kubernetes、gVisor、Kata、Firecracker、真实 LLM 服务和外部 IAM 明确不属于当前 Demo。
