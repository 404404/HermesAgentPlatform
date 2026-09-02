# Runtime lifecycle

One **User Runtime** represents one user-level Hermes runtime resource. Multiple Agent Profiles can run inside that runtime; a Profile is not a user isolation boundary. `MockRuntimeProvider` owns only the Demo state and no container has a host Docker Socket or privileged mode.

Runtime records include CPU, memory, storage, profile and concurrency limits, image version, provider/class, network policy, auto-start and auto-suspend. Runtime Templates provide Lightweight, Standard, Developer and Heavy presets. Settings select `Automatic` or `Manual` provisioning:

- Automatic applies the department/default template when a user becomes active and assigns matching managed Profile Templates.
- Manual leaves the resource `not_provisioned` until an administrator selects a user/template and provisions it.

Lifecycle orchestration is kept in the service layer. Suspension, disablement and archival stop the runtime and disable managed Profiles. High-risk resource increases create approval requests for non-break-glass actors.

Production Hermes integration should be asynchronous, idempotent and reconciled through an adapter/outbox; this Demo deliberately stops at a resource record.


## v0.2.1 runtime management

Runtime Template contains infrastructure fields only: CPU, memory, storage, profile limit, concurrent jobs, provider, class, network policy and status. Agent behavior belongs to Agent Template and Profile, not Runtime Template. Runtime exposes desired status separately from observed status and records provider events. Resources can be edited from Runtime detail; increases are routed to the approval workflow for ordinary administrators. Start, Stop and Restart are lifecycle controls. Emergency Kill Switch is a separate security action that requires a reason, disables new work, cancels demo executions, creates a Critical Audit event and sends a notification.
## v0.2.2 policy binding and management UX

Runtime Template contains infrastructure fields only. Role and Department policy bindings are stored in `runtime_template_bindings` with `binding_priority`; the editor updates existing bindings through a scoped API. Resolution never silently chooses between equal-priority different templates and reports a conflict for administrator action. User Runtime shows desired and observed status, Profiles, Effective Skills, Executions, Resources and Controls. Emergency Kill Switch remains distinct from Stop and requires a reason, security permission, Critical Audit event and notification.
