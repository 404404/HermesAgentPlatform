# User Workspace (v0.3)

HEP has one login and one session. After login every account opens the Workspace. Accounts with an administrative role may switch to `/admin` from the user menu; ordinary users are denied by the backend and remain in the Workspace.

Workspace resources are always scoped to the authenticated user. The `/me` route family exposes the current user, effective permissions, managed and personal Agent Profiles, selectable logical models, effective Skills and Knowledge, channel connections, notifications and usage. No endpoint accepts a user id for impersonation.

Chat uses `MockChatProvider`. Conversations and messages are stored in `chat_conversations` and `chat_messages`, so the UI demonstrates persistence without calling an LLM. A User owns one User Runtime record and can have multiple Agent Profiles inside it. Managed profiles are consolidated from Department, Role and explicit User template assignments; personal profiles are created by the user when the effective self-service policy permits it.

Self-service policy is resolved by specificity: organization, department, role, then user. Each capability is controlled by `disabled`, `allowed`, `whitelist` or `admin_managed`; the backend evaluates the policy before profile, model, Skill, Knowledge or channel mutations.
