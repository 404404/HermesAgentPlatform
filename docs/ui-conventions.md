# HEP UI conventions

## Tables and actions

Primary resource names open Detail. The rightmost Actions column uses View, Edit and More, stays sticky where horizontal scrolling is needed, and does not compete with row expansion. Relationship values use RelationTags with a small visible set and a +N popover.

## Forms and relations

Create and edit prefer a shared responsive modal. ResponsiveFormGrid uses two columns on desktop and one column on narrow screens. EntityMultiSelect provides search, loading, multiple selection, selected tags and disabled/status metadata for Users, Roles, Departments, Skills, Knowledge, Agent Templates and Models. Dirty forms require an explicit discard confirmation.

## Navigation and state

The header is sticky and owns global Refresh, Notifications, Language and User actions. Sidebar groups are collapsible and independently scrollable; the main content has its own scroll area. Selected departments and list filters should be represented in URL query state when possible so Back and Refresh retain context.

## Safety and feedback

Every main list has loading, error with retry-capable context and empty states with a useful CTA. Delete and high-risk actions use confirmation and show dependencies or impact. Kill Switch requires a reason. Relationship changes are submitted to the backend binding API and are never kept as a second frontend-only copy.

## Responsive targets

Desktop layouts are checked at 1440px, 1280px and 1024px. Settings uses a three-column grid on wide screens, two columns at medium widths and one column when space is constrained. Desktop information density remains the primary design target.
