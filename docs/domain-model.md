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

## v0.2.2 relationship consolidation

The product domain uses one vocabulary: Agent Template defines model, Skills, Knowledge and managed policy; Agent Profile is a concrete User-owned Hermes Profile; Runtime Template defines infrastructure policy only; User Runtime is the per-user runtime boundary that contains one or more Profiles. Profile Templates remains only as the compatible storage table name.

The source of truth for relationships is the backend binding data. A User receives scoped Role Bindings, Department membership, additive Agent Template assignments and one Runtime. A managed Profile is deduplicated by user and template, while profile_assignment_sources explains every Department, Role or explicit User source. Runtime policy bindings are separate from Agent Template bindings.

Effective rules are centralized in the service layer: Runtime Template resolution is Explicit User > Role > Department > Organization Default, with binding_priority and an explicit conflict result for equal-priority different templates. Agent Template matches are additive and deduplicated. Skill policy precedence is blocked > mandatory > explicit > default > optional. Knowledge access is the union of allowed Organization, Department, Role, Profile and Agent Template bindings.

Activity is a business summary, Execution Log records Agent work, Approval Request is a governance gate, and Audit Log records Control Plane operations. These are separate APIs and tables even when they are linked by metadata.


## v0.3 unified workspace

HEP uses one authenticated session and two presentation surfaces: Workspace for every user and Admin Console for administrative roles. Workspace APIs are scoped to the session user and do not support impersonation. Chat persistence is represented by conversations and messages and uses MockChatProvider until a Hermes adapter is introduced.

The model catalog separates logical Models, Model Providers and Provider Models. Model Slot Policies reserve the Hermes auxiliary slots without coupling the Control Plane to provider secrets or real model calls. Runtime Hosts and MockScheduler are infrastructure foundations and do not change the User Runtime/Profile boundary.