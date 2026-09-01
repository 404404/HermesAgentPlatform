# Runtime lifecycle

One **User Runtime** represents one user-level Hermes runtime resource. Multiple Agent Profiles can run inside that runtime; a Profile is not a user isolation boundary. `MockRuntimeProvider` owns only the Demo state and no container has a host Docker Socket or privileged mode.

Runtime records include CPU, memory, storage, profile and concurrency limits, image version, provider/class, network policy, auto-start and auto-suspend. Runtime Templates provide Lightweight, Standard, Developer and Heavy presets. Settings select `Automatic` or `Manual` provisioning:

- Automatic applies the department/default template when a user becomes active and assigns matching managed Profile Templates.
- Manual leaves the resource `not_provisioned` until an administrator selects a user/template and provisions it.

Lifecycle orchestration is kept in the service layer. Suspension, disablement and archival stop the runtime and disable managed Profiles. High-risk resource increases create approval requests for non-break-glass actors.

Production Hermes integration should be asynchronous, idempotent and reconciled through an adapter/outbox; this Demo deliberately stops at a resource record.
