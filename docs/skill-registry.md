# Skill registry

Skills are versioned artifacts. `skills` contains marketplace metadata, `skill_versions` is the immutable-oriented version boundary, `skill_artifacts` groups a version’s content, and `skill_artifact_files` stores safe relative paths, text content, content type, size and SHA-256.

Draft versions can create, edit and delete text files such as `SKILL.md`, `scripts/`, `templates/` and `references/`. Submission changes the governance state; publication sets `immutable=TRUE` and later changes must be made as a new version. The marketplace detail API exposes Overview, Files, Versions, Permissions, Reviews, Distribution and Activity data. File viewing supports Markdown, code and raw modes in the React UI.

The Demo does not execute Skill code, clone Git repositories, accept ZIP uploads or perform malware scanning. Artifact import and object storage are future provider seams.


## 简体中文（当前 Skill 注册中心）

Skill 是版本化 Artifact：Skill 保存市场元数据，Skill Version 是发布边界，Artifact/File 保存 SKILL.md、scripts、templates、references 等安全相对路径文本文件。Draft 可编辑；提交会创建绑定组织、Skill Version 和流程快照的审核实例。Timeline 只读，步骤有顺序和角色校验；Mock 自动检查会显式标注来源。

工作台安装必须指定当前用户拥有的 Profile 和已发布 Skill Version。profile_skill_installs 保存实际安装版本、审批请求与 Mock Runtime 应用状态。Demo 不执行 Skill、不会 Git Clone/ZIP 上传/恶意软件扫描，也不会写入 Hermes。
