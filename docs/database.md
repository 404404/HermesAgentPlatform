# Database model

The migration in `backend/migrations/001_init.sql` creates:

- Organization hierarchy: `organizations`, `departments`, `users`, `auth_identities`.
- Authorization: `roles`, `permissions`, `role_permissions`, `role_bindings`. A binding can scope to `global`, `organization`, `department`, `user` or `profile`.
- Agent layer: `profiles`, `runtimes`, `models`.
- Skill governance: `skills`, `skill_versions`, `skill_submissions`, `skill_reviews`, `skill_assignments`.
- Enterprise knowledge: `knowledge_bases`, `knowledge_bindings`.
- Observability: `usage_events`, `audit_logs`.

Every usage event carries organization, department, user, profile, session, execution, model, skill and runtime dimensions (nullable where the event type does not have that dimension), plus token, request, execution and latency measurements.

`audit_logs` is append-only from the application perspective. Foreign keys use nullable references where deletion should preserve the historical record. Migration state is tracked in `schema_migrations`; the backend applies missing SQL files in lexical/version order before seeding the demo data.


## Phase 2 migration

`backend/migrations/002_phase2_control_plane.sql` evolves the initial schema without rewriting migration 001. It adds `skill_artifacts`, `skill_artifact_files`, `knowledge_documents`, `knowledge_document_versions`, `runtime_templates`, `profile_templates`, `profile_template_bindings`, `model_providers`, `secrets`, `approval_requests`, `approval_steps`, `risk_rules`, `risk_events`, `system_settings`, `resource_change_history`, `quota_policies`, `notifications` and `user_preferences`, and extends runtime, profile, binding and audit records.


## Phase 3 domain migration

Migration 004_domain_consolidation.sql evolves the existing Demo schema without rewriting earlier migrations. It adds department codes and runtime policy references, Agent Template version and user bindings, profile template version and assignment sources, desired and observed runtime status, kill-switch metadata, audit category and action labels, profile_assignment_sources, knowledge_items and knowledge_item_versions, executions, execution_events and runtime_events. The storage name profile_templates is retained for compatibility, while the API and product name is Agent Template. Empty databases apply all migrations in lexical order; existing Demo databases apply only the missing migration marker.
## v0.2.2 migration

Migration `005_v022_relationship_management.sql` evolves the schema without editing earlier migrations. It adds `runtime_template_bindings` for infrastructure policy relationships and adds `knowledge_bindings.agent_template_id` for Knowledge to Agent Template associations. Foreign keys and indexes preserve upgrade safety. The backend applies migration 005 to existing Demo databases and the same ordered migrations work on an empty database.
