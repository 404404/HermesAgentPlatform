# RBAC and separation of duties

HEP uses role bindings rather than a `users.role` column. A binding carries a scope (`global`, `organization`, `department`, `user`, `profile`) and can therefore explain both the permission and its source.

The seeded administration roles are deliberately separated:

- **System Administrator** manages organization users, departments, runtimes, runtime/profile templates and general configuration.
- **Security Administrator** manages roles, permissions, security policy, Skill risk, model policy, Knowledge ACLs and high-risk approvals. It does not receive ordinary runtime operations.
- **Audit Administrator** reviews audit/risk activity and exports filtered audit data. It cannot modify users, runtimes, roles or security policy.
- **Break-glass Super Administrator** is an emergency exception account. Its login is a critical risk event and it is not a daily administrator role.

Every protected mutating endpoint checks the effective backend permission. The UI only reflects those capabilities; hiding a menu is not an authorization control. Self-assignment of a protected administrator role is denied. Protected changes from a non-break-glass actor create an Approval Request and are independently reviewable.

User detail exposes effective permissions with `Permission`, `Source Role`, `Scope` and `Source Binding` so administrators can explain access.
