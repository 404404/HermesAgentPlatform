# RBAC and separation of duties

HEP uses role bindings rather than a `users.role` column. A binding carries a scope (`global`, `organization`, `department`, `user`, `profile`) and can therefore explain both the permission and its source.

The seeded administration roles are deliberately separated:

- **System Administrator** manages organization users, departments, runtimes, runtime/profile templates and general configuration.
- **Security Administrator** manages roles, permissions, security policy, Skill risk, model policy, Knowledge ACLs and high-risk approvals. It does not receive ordinary runtime operations.
- **Audit Administrator** reviews audit/risk activity and exports filtered audit data. It cannot modify users, runtimes, roles or security policy.
- **Break-glass Super Administrator** is an emergency exception account. Its login is a critical risk event and it is not a daily administrator role.

Every protected mutating endpoint checks the effective backend permission. The UI only reflects those capabilities; hiding a menu is not an authorization control. Self-assignment of a protected administrator role is denied. Protected changes from a non-break-glass actor create an Approval Request and are independently reviewable.

User detail exposes effective permissions with `Permission`, `Source Role`, `Scope` and `Source Binding` so administrators can explain access.


## v0.2.1 binding and protection rules

Role assignment remains a role_bindings record, never users.role. User detail shows Role, Scope, Source and Assigned At, and Effective Permissions show the source role and binding. Protected administrator role self-elevation is rejected in the backend. System Administrator, Security Administrator and Audit Administrator retain separate permission sets; Break-glass remains an emergency exception and is audited as critical.

Agent Template assignments are additive across Department, Role and explicit User sources. Matching sources are consolidated into one managed Profile and stored in profile_assignment_sources. Managed profiles retain enterprise configuration; personal profiles are user-created. All protected mutations are authorized by backend permissions even when a UI control is hidden.
## v0.2.2 relationship management

Role membership is always stored in `role_bindings`; there is no user role field. Role Detail and User Detail read and write the same binding domain. Binding scope remains global, organization, department, user or profile and is displayed with source and assignment time. Protected administrator role changes are backend checked, self elevation is denied and high-risk changes can create Approval Requests.

The three administrative roles remain separate: System Administrator handles identity, organization, Runtime and basic configuration; Security Administrator handles permissions, policy and high-risk approval; Audit Administrator handles Audit and export. Break-glass is an emergency exception and is audited at critical risk.
