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


## v0.3 surfaces

The single session lands in Workspace. Administrative roles can switch surfaces from the user menu; ordinary users do not see the switch and the backend denies Admin Console routes. Workspace navigation owns Chat, My Agents, Resources, Connections and Account; Platform Health and Notifications remain dashboard/header affordances. Global Refresh lives in the sticky header and dispatches a shared refresh event.

## v0.3.2 conventions

Management tables use a right-sticky Actions column. Primary actions remain in the page header; row actions are compact icon controls in the order status, view, edit, more. Destructive actions belong in More. `ActionCell` is the reusable implementation for new surfaces, while legacy tables are progressively migrated without changing their domain data source.

Credentials are entered inline where their owning resource is created (for example Runtime Host or Model Provider). The UI only renders configured state and never reads a credential value back.


## v0.3.3 action and language standard

ActionCell now renders textual View, Edit and More controls consistently in right-sticky Actions columns; status remains the optional leading switch and destructive commands remain inside More. Browser language selection uses a persisted explicit choice first and otherwise detects a Chinese browser locale before falling back to en-US. New UI strings must be present in both catalogs; npm run i18n:audit verifies parity.
