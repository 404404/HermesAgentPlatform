# Audit and risk

Audit events are append-only application records. Phase 2 records the actor, action, resource, scope, result, source IP, user agent, request ID, trace ID, metadata, risk level, score, reason and timestamp.

`RiskEvaluator` is a deterministic rule engine in this Demo. Break-glass login, self-elevation, security-role changes, quarantined Skill actions and critical secret operations are critical; role elevation, runtime resize, Skill publication and model security changes are high; runtime lifecycle and Knowledge ACL changes are medium; routine reads are low. Rules are explainable and are stored in `risk_rules` for a future policy service.

`risk_events` links high/critical evaluations to their source audit event and supports `open`, `acknowledged`, `resolved` and `false_positive`. The audit query API applies all filters server-side, and Audit Administrator/Break-glass users can export the current filtered result as CSV or JSON. Export itself creates an audit event.

The Demo intentionally does not claim WORM storage, signatures, SIEM delivery or immutable infrastructure.


## v0.2.1 execution separation

Execution Logs are persisted in executions with model, skills, tools, runtime, tokens, cost, duration, risk and approval fields. High and critical execution requests become pending Approval Requests; approval decisions update the execution state. Execution risk is not a replacement for Control Plane Audit. Audit category and action label are controlled by backend catalog values, filters are applied server-side, and CSV or JSON export uses the current filter set. Export actions are themselves audited and ordinary users cannot export the global audit stream.
## v0.2.2 filter contract

Audit Log remains Control Plane Audit and is not an Execution Log. The v2 query accepts time range, category, action, actor, department, resource type and ID, result, risk, source IP, profile, Runtime, Skill, Model and request ID. Category controls the action options in the UI, while all filtering is performed by the backend. CSV and JSON export uses the exact same query and export is itself audited; ordinary users cannot export the global stream.
