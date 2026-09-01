# Knowledge Base

Knowledge Bases are enterprise content collections, separate from Hermes personal memory. Phase 2 adds `knowledge_documents` and immutable historical `knowledge_document_versions` records. Draft editing creates a new version; publishing marks the document/version published and invokes `MockKnowledgeProvider`, which currently records an indexed state without embeddings.

Documents support Markdown and plain text, search, status filtering, owner, last modification, version history and Markdown preview. Knowledge bindings can target an organization, department, role or profile and carry scope/policy/creator metadata. Profile detail can query its effective Knowledge Sources.

Indexing, parsing of PDF/Office files, vector search, retrieval authorization and a production Knowledge Gateway remain future work.
