# Model access

HEP models are logical catalog entries. A `model_provider` stores non-secret connection metadata and a secret reference status; it never returns a provider key.

- **Hermes Native** represents a provider configuration that a future `HermesAdapter` can render for a user runtime.
- **Enterprise Gateway** represents an internal model gateway and unified audit/budget boundary.
- **Custom Gateway** is the extension point for another compatible endpoint.

System Settings persists the selected Model Access Mode and Default Model. Phase 2 only uses Mock provider records. A future integration must add credential resolution through `SecretProvider`, health checks, policy evaluation and asynchronous adapter reconciliation without making the Control Plane depend on a real LLM service.


## v0.3 provider catalog

Provider Models are stored separately from logical Models. A Provider Model records an upstream identifier and sync status; a logical Model references its provider and provider-model record. Administrators can test connectivity and sync the catalog through MockModelProvider. The UI displays secret reference status only and never accepts a provider API key as a Model field.

## 简体中文（当前模型访问）

HEP 管理逻辑模型访问而非真实 LLM 流量。逻辑模型可关联模型供应商和上游模型；供应商保存端点、认证类型和只写入凭据状态。Hermes Native、Enterprise Gateway、Custom Gateway 目前都是控制平面模式，为未来 Adapter/Gateway 预留。

新增/编辑供应商时可内联写入 API Key/Token，SecretProvider 加密保存且不会返回。MockModelProvider 用于连接测试和目录同步。工作台模型会综合模型、供应商、Profile 和策略状态；Demo 不会真正调用供应商 API。
