# Skill registry

Skills are versioned artifacts. `skills` contains marketplace metadata, `skill_versions` is the immutable-oriented version boundary, `skill_artifacts` groups a version’s content, and `skill_artifact_files` stores safe relative paths, text content, content type, size and SHA-256.

Draft versions can create, edit and delete text files such as `SKILL.md`, `scripts/`, `templates/` and `references/`. Submission changes the governance state; publication sets `immutable=TRUE` and later changes must be made as a new version. The marketplace detail API exposes Overview, Files, Versions, Permissions, Reviews, Distribution and Activity data. File viewing supports Markdown, code and raw modes in the React UI.

The Demo does not execute Skill code, clone Git repositories, accept ZIP uploads or perform malware scanning. Artifact import and object storage are future provider seams.
