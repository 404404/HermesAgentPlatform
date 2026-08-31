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
