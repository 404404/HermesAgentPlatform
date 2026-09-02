# Knowledge Base

Knowledge Bases are enterprise content collections, separate from Hermes personal memory. Phase 2 adds `knowledge_documents` and immutable historical `knowledge_document_versions` records. Draft editing creates a new version; publishing marks the document/version published and invokes `MockKnowledgeProvider`, which currently records an indexed state without embeddings.

Documents support Markdown and plain text, search, status filtering, owner, last modification, version history and Markdown preview. Knowledge bindings can target an organization, department, role or profile and carry scope/policy/creator metadata. Profile detail can query its effective Knowledge Sources.

Indexing, parsing of PDF/Office files, vector search, retrieval authorization and a production Knowledge Gateway remain future work.


## v0.2.1 Knowledge Items

A Knowledge Base now contains maintainable Knowledge Items and immutable-oriented item versions. Supported item types are background, qa, markdown and procedure. Item edits create a version record, Draft is retained until publish, and Markdown content can be previewed through the UI. Bindings target Organization, Department, Role or Profile and carry mandatory, default or optional policy. The consumers endpoint calculates direct bindings and effective active users and profiles from current database relationships.

MockKnowledgeProvider is called on publish and represents indexing only. There is no embedding, vector database, PDF or Office parser, or Hermes personal-memory coupling in this Demo.
## v0.2.2 dependency navigation

Knowledge Binding supports Organization, Department, Role, Profile and Agent Template targets. Binding responses include target IDs as well as display names, allowing the Detail view to link to Organization & Users, Role Detail, Profile Detail and Agent Template Detail without maintaining duplicate frontend relationships. Content remains Background, Q&A, Markdown and Procedure, with draft, publish, preview and version history behavior. Effective Consumers are calculated from current backend relationships.
