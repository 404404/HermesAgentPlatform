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


### v0.3.2 storage evolution

Migration `007_v032_runtime_host_infrastructure.sql` adds Runtime Host connection metadata, actual/allocated mock metrics, container count and description. Migration `008_v032_runtime_host_auth_normalization.sql` maps the historical demo `secret_reference` auth marker to supported `password`. `runtimes.host_id` remains the physical storage column for backward compatibility; `runtime_host_id` is the public API/domain name. Migration `009_v032_operational_secret_encryption.sql` adds `ciphertext`, `nonce`, `algorithm`, `key_version` and `requires_reentry` to `secrets`. New operational credentials use AES-256-GCM; old bcrypt-only operational values are cleared and marked `requires_reentry`. Provider and host tables retain only secret references, never plaintext columns.


## 简体中文（当前存储）

HEP 使用 MySQL 8，Migration 位于 backend/migrations，执行状态保存在 schema_migrations。启动时按文件名字典序应用缺失 Migration，已有 Migration 不会被改写。

核心域包括组织/RBAC、Profile/Runtime/Runtime Host、模型和 Secrets、版本化 Skills、知识库内容/策略/导入任务、执行/审批/审计/用量，以及 Workspace Chat。v0.3.3 的 011–015 Migration 增加了稳定知识策略主体键及备份/冲突记录、Skill 审核快照、Profile Skill 安装、Profile-Runtime 关联、聊天候选选择和反馈评论。供应商与主机表只保存 Secret 引用和元数据，不保存明文凭据。
