# Audit and risk

Audit events are append-only application records. Phase 2 records the actor, action, resource, scope, result, source IP, user agent, request ID, trace ID, metadata, risk level, score, reason and timestamp.

`RiskEvaluator` is a deterministic rule engine in this Demo. Break-glass login, self-elevation, security-role changes, quarantined Skill actions and critical secret operations are critical; role elevation, runtime resize, Skill publication and model security changes are high; runtime lifecycle and Knowledge ACL changes are medium; routine reads are low. Rules are explainable and are stored in `risk_rules` for a future policy service.

`risk_events` links high/critical evaluations to their source audit event and supports `open`, `acknowledged`, `resolved` and `false_positive`. The audit query API applies all filters server-side, and Audit Administrator/Break-glass users can export the current filtered result as CSV or JSON. Export itself creates an audit event.

The Demo intentionally does not claim WORM storage, signatures, SIEM delivery or immutable infrastructure.
