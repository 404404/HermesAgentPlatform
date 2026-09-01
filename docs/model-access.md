# Model access

HEP models are logical catalog entries. A `model_provider` stores non-secret connection metadata and a secret reference status; it never returns a provider key.

- **Hermes Native** represents a provider configuration that a future `HermesAdapter` can render for a user runtime.
- **Enterprise Gateway** represents an internal model gateway and unified audit/budget boundary.
- **Custom Gateway** is the extension point for another compatible endpoint.

System Settings persists the selected Model Access Mode and Default Model. Phase 2 only uses Mock provider records. A future integration must add credential resolution through `SecretProvider`, health checks, policy evaluation and asynchronous adapter reconciliation without making the Control Plane depend on a real LLM service.
