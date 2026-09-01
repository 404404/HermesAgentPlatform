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
