# HEP domain model

## Core relationship

A User is the ownership boundary for Agent Profiles and one User Runtime. A user belongs to a Department and receives scoped Role Bindings. A user can also receive explicit Agent Template assignments.

Department provides organization membership, runtime policy and Knowledge access. Role provides permissions and can assign Agent Templates. Organization provides defaults.

Agent Template is the behavior and capability definition for a job, role, department or person. It contains a default logical Model, Skills, Knowledge and managed policy. It never contains CPU, memory, storage or container settings. The compatible storage table remains profile_templates for migration compatibility, while the API and UI use Agent Template.

Agent Profile is a concrete user-owned profile corresponding to a future Hermes Profile. It is either managed or personal. A managed Profile points to an Agent Template and template version, records all assignment sources, and exposes Effective Configuration. A personal Profile is created by the user and is not an enterprise template instance.

Runtime Template is an infrastructure preset. It contains CPU, memory, storage, profile limit, concurrent jobs, provider, runtime class, network policy and lifecycle defaults. It contains no Skill, Knowledge, Model or agent behavior.

User Runtime is the future per-user Hermes runtime or container record. It is the important user isolation boundary and contains one to many Agent Profiles. This Demo only calls MockRuntimeProvider and does not use Docker Socket, privileged containers or real Hermes provisioning.

## Effective configuration

Agent Template matches are additive: explicit user assignments plus Role assignments plus Department assignments. Duplicate matches result in one managed Agent Profile, with all sources stored for explanation. Runtime Template selection is Explicit User, then Department, then Organization Default. Skill policy precedence is blocked, mandatory, explicit, default, optional. Knowledge is the union of allowed Organization, Department, Role and Profile bindings.

## Operational domains

Activity is a compact business history such as login or profile creation. Execution Log records an Agent task attempt and its model, skills, tools, runtime, tokens, cost, status and risk. Approval Request is the governance gate for high-risk actions. Audit Log is the append-only Control Plane record for identity, organization, RBAC, templates, runtime, model, skill, knowledge, settings, approvals and exports.

## Future adapter boundary

Hermes Adapter will translate the effective Profile configuration into Hermes-native provider and profile configuration. Runtime Provider will provision and reconcile User Runtime resources. Knowledge Provider will index enterprise Knowledge separately from Hermes personal memory. Model Gateway and Secret Provider remain interfaces. All are Mock Providers in v0.2.1.
