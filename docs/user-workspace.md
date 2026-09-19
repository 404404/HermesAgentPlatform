# User Workspace (v0.3)

HEP has one login and one session. After login every account opens the Workspace. Canonical UI routes are `/login`, `/workspace` (redirecting to `/workspace/chat`) and `/admin` (redirecting to `/admin/overview`). Accounts with an administrative role may switch to `/admin` from the user menu; ordinary users are denied by the backend and remain in the Workspace. Legacy root Admin paths such as `/organization` redirect to their `/admin/*` equivalent with query parameters preserved.

Workspace resources are always scoped to the authenticated user. The `/me` route family exposes the current user, effective permissions, managed and personal Agent Profiles, selectable logical models, effective Skills and Knowledge, channel connections, notifications and usage. No endpoint accepts a user id for impersonation.

Chat uses `MockChatProvider`. Conversations and messages are stored in `chat_conversations` and `chat_messages`, so the UI demonstrates persistence without calling an LLM. A User owns one User Runtime record and can have multiple Agent Profiles inside it. Managed profiles are consolidated from Department, Role and explicit User template assignments; personal profiles are created by the user when the effective self-service policy permits it.

Self-service policy is resolved by specificity: organization, department, role, then user. Each capability is controlled by `disabled`, `allowed`, `whitelist` or `admin_managed`; the backend evaluates the policy before profile, model, Skill, Knowledge or channel mutations.


## 简体中文（当前用户工作台）

所有账号登录后进入 Workspace；管理员可切换 /admin，普通用户的管理端 API/路由会被后端拒绝。/me/agents 只返回当前用户 Profile；/me/models、/me/skills、/me/knowledge 都是有效资源解析，不是全局目录。

知识权限支持 query_only、read、contribute、manage：无权资源隐藏，query_only 不暴露正文。Skill 安装必须选择目标 Profile 与版本，审批和安装状态会持久化。新建会话、发送消息、重新生成、候选选择与反馈都保存到聊天表，并通过 MockChatProvider 返回 Demo 回复，不调用真实 LLM。
