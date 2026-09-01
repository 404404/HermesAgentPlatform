# HEP architecture

HEP is an independent Enterprise Control Plane. It owns platform users, departments, scoped RBAC, Agent Profile metadata, logical model catalog, skill governance, Knowledge Base bindings, usage dimensions and audit records.

## Boundaries

- Control Plane owns authentication, authorization and lifecycle commands.
- User Runtime is the future per-user Hermes lifecycle boundary.
- Agent Profile is a logical configuration, not a strong isolation boundary.
- Execution Sandbox is the future code/terminal isolation boundary.
- Knowledge Gateway keeps enterprise Knowledge Bases separate from Hermes personal memory.
- Model Gateway resolves logical models without exposing provider keys.
- Hermes Adapter is the only layer that translates Hermes-specific formats.

Gin was chosen for compact middleware and route grouping. The backend uses `database/sql` with parameterized statements and versioned SQL migrations. Provider interfaces make future Docker/Kubernetes runtimes, sandbox brokers and Knowledge backends replaceable without changing control-plane concepts.


## Phase 2 control-plane increment

Phase 2 adds persistent governance domains for versioned Skill artifacts, Knowledge documents and versions, Runtime and Profile Templates, model provider and secret references, approvals, quotas, notifications, Settings, risk events and configuration change history. The React control plane keeps the existing visual language while adding a single en-US and zh-CN i18n layer. Mock providers remain the runtime, Hermes, Knowledge, notification and secret integration boundaries.


## Phase 2 control-plane increment

Phase 2 adds persistent governance domains for versioned Skill artifacts, Knowledge documents and versions, Runtime and Profile Templates, model provider and secret references, approvals, quotas, notifications, Settings, risk events and configuration change history. The React control plane keeps the existing visual language while adding a single en-US and zh-CN i18n layer. Mock providers remain the runtime, Hermes, Knowledge, notification and secret integration boundaries.


## v0.2.1 consolidated domain

The control plane now exposes one coherent relationship: a User belongs to a Department, receives scoped Role Bindings and explicit Agent Template assignments, owns one User Runtime, and can own multiple Agent Profiles. Departments provide runtime policy and Knowledge access; Roles and Departments can assign Agent Templates. An Agent Template defines model, skills, knowledge and managed policy only. A Runtime Template defines infrastructure only. A User Runtime is the future per-user Hermes isolation boundary, while an Agent Profile is a user-owned Hermes profile configuration inside that runtime.

The service layer computes additive Agent Template matches and Effective Configuration. Runtime Template resolution is explicit user assignment, then department policy, then organization default. Skill policy precedence is blocked, mandatory, explicit, default, optional. Execution Logs represent Agent work; Activity is a business summary; Audit Logs represent Control Plane operations; Approval Requests gate high-risk actions. v0.2.1 keeps these domains separate and continues to use Mock Providers at the Hermes, runtime, knowledge, notification and secret boundaries.