# Knowledge Base

Knowledge Bases are enterprise content collections, separate from Hermes personal memory. Phase 2 adds `knowledge_documents` and immutable historical `knowledge_document_versions` records. Draft editing creates a new version; publishing marks the document/version published and invokes `MockKnowledgeProvider`, which currently records an indexed state without embeddings.

Documents support Markdown and plain text, search, status filtering, owner, last modification, version history and Markdown preview. Knowledge bindings can target an organization, department, role or profile and carry scope/policy/creator metadata. Profile detail can query its effective Knowledge Sources.

Indexing, parsing of PDF/Office files, vector search, retrieval authorization and a production Knowledge Gateway remain future work.


## v0.2.1 Knowledge Items

A Knowledge Base now contains maintainable Knowledge Items and immutable-oriented item versions. Supported item types are background, qa, markdown and procedure. Item edits create a version record, Draft is retained until publish, and Markdown content can be previewed through the UI. Bindings target Organization, Department, Role or Profile and carry mandatory, default or optional policy. The consumers endpoint calculates direct bindings and effective active users and profiles from current database relationships.

MockKnowledgeProvider is called on publish and represents indexing only. There is no embedding, vector database, PDF or Office parser, or Hermes personal-memory coupling in this Demo.
## v0.2.2 dependency navigation

Knowledge Binding supports Organization, Department, Role, Profile and Agent Template targets. Binding responses include target IDs as well as display names, allowing the Detail view to link to Organization & Users, Role Detail, Profile Detail and Agent Template Detail without maintaining duplicate frontend relationships. Content remains Background, Q&A, Markdown and Procedure, with draft, publish, preview and version history behavior. Effective Consumers are calculated from current backend relationships.


## v0.3 Workspace knowledge

Workspace Knowledge is calculated from the current user's effective Agent Profiles and the organization/departments/roles/profile bindings. It is read-only in the user surface; administration continues to manage Knowledge Base content, bindings and versions. This remains separate from Hermes personal memory and uses MockKnowledgeProvider.


## v0.3.3 content workspace

Knowledge management uses a master-detail layout: the selected Knowledge Base is an administrative context, while item editing stays inside that context. Q&A and Markdown imports produce explicit import-job records. Workspace reads the same Knowledge Bases through a per-user access policy (query_only, read, contribute or manage); it does not maintain a duplicate user content store.


## 简体中文（当前知识库）

知识库属于企业内容，与 Hermes Personal Memory 分离。内容类型包括 background、qa、markdown、procedure；编辑产生版本记录，发布仅通过 MockKnowledgeProvider 表示索引状态。

绑定目标支持组织、部门、角色、Profile、Agent Template。访问策略支持 organization、department、role、user 以及 query_only、read、contribute、manage；用户策略优先于其他策略。工作台读取同一套控制平面数据：无权资源不出现，query_only 不返回正文，contribute/manage 才可变更允许的内容。

Q&A CSV 支持 BOM、引号、逗号与换行；Markdown 支持多文件。均采用解析、校验预览、显式确认、事务化创建内容/版本/导入任务的流程，并分别返回重复、部分失败与失败。没有 PDF/Office 解析、Embedding 或向量检索。
