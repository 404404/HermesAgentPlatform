package main

import (
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func seedPhase2DataImpl(db *sql.DB, password string) error {
	permissions := []string{
		"dashboard.read", "role.read", "role.assign", "role.manage", "security.policy.manage",
		"runtime_template.read", "runtime_template.manage", "profile_template.read", "profile_template.manage",
		"model_provider.read", "model_provider.manage", "secret.read", "secret.manage",
		"skill.artifact.read", "skill.artifact.manage", "knowledge.document.read", "knowledge.document.manage",
		"approval.read", "approval.create", "approval.review", "risk.read", "risk.manage", "audit.export",
		"settings.read", "settings.manage", "system.health.read", "notification.read", "quota.read", "quota.manage",
	}
	for _, code := range permissions {
		if _, err := db.Exec("INSERT INTO permissions(code,description) VALUES(?,?) ON DUPLICATE KEY UPDATE description=VALUES(description)", code, code); err != nil {
			return err
		}
	}

	// Preserve the original role id but make the high-risk exception explicit.
	if _, err := db.Exec("UPDATE roles SET name='Break-glass Super Administrator',description='Emergency full access; not for daily administration' WHERE id=1 AND name='Super Admin'"); err != nil {
		return err
	}
	roles := []struct {
		name, description string
	}{
		{"System Administrator", "Users, departments, runtimes, profiles and system configuration"},
		{"Security Administrator", "Roles, permissions, security policy and high-risk approvals"},
		{"Audit Administrator", "Audit logs, risk events, export and administrator activity review"},
	}
	for _, role := range roles {
		if _, err := db.Exec("INSERT INTO roles(organization_id,name,description,is_system) VALUES(1,?,?,TRUE) ON DUPLICATE KEY UPDATE description=VALUES(description)", role.name, role.description); err != nil {
			return err
		}
	}
	roleID := func(name string) int64 {
		var id int64
		_ = db.QueryRow("SELECT id FROM roles WHERE name=? AND (organization_id=1 OR organization_id IS NULL) LIMIT 1", name).Scan(&id)
		return id
	}
	grant := func(roleName string, codes []string) error {
		id := roleID(roleName)
		for _, code := range codes {
			var permissionID int64
			if err := db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&permissionID); err != nil {
				continue
			}
			if _, err := db.Exec("INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES(?,?)", id, permissionID); err != nil {
				return err
			}
		}
		return nil
	}
	if err := grant("System Administrator", []string{"dashboard.read", "user.read", "user.create", "user.update", "user.disable", "department.read", "department.manage", "profile.read", "profile.create", "profile.update", "profile.delete", "runtime.read", "runtime.manage", "runtime_template.read", "runtime_template.manage", "profile_template.read", "profile_template.manage", "settings.read", "settings.manage", "notification.read", "system.health.read", "quota.read"}); err != nil {
		return err
	}
	if err := grant("Security Administrator", []string{"dashboard.read", "role.read", "role.assign", "role.manage", "security.policy.manage", "skill.read", "skill.review", "skill.publish", "skill.submit", "skill.artifact.read", "skill.artifact.manage", "model_provider.read", "model_provider.manage", "secret.read", "secret.manage", "knowledge.read", "knowledge.manage", "knowledge.document.read", "knowledge.document.manage", "runtime.read", "approval.read", "settings.manage", "approval.create", "approval.review", "risk.read", "risk.manage", "settings.read", "notification.read", "system.health.read"}); err != nil {
		return err
	}
	if err := grant("Audit Administrator", []string{"dashboard.read", "audit.read", "audit.export", "settings.manage", "risk.read", "risk.manage", "approval.read", "usage.global.read", "notification.read", "system.health.read"}); err != nil {
		return err
	}
	if err := grant("Break-glass Super Administrator", permissions); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE rp FROM role_permissions rp JOIN roles r ON r.id=rp.role_id JOIN permissions p ON p.id=rp.permission_id WHERE r.name=? AND p.code=?", "Security Administrator", "runtime.manage"); err != nil {
		return err
	}
	// The legacy Super Admin role was renamed above; its original permissions remain.

	adminID, err := phase2EnsureUser(db, password, "admin", "Platform Administrator", "admin@demo.local", 1)
	if err != nil {
		return err
	}
	securityID, err := phase2EnsureUser(db, password, "security-admin", "Security Administrator", "security-admin@demo.local", 6)
	if err != nil {
		return err
	}
	auditID, err := phase2EnsureUser(db, password, "audit-admin", "Audit Administrator", "audit-admin@demo.local", 6)
	if err != nil {
		return err
	}
	if _, err := db.Exec("INSERT IGNORE INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')", roleID("System Administrator"), adminID); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT IGNORE INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')", roleID("Security Administrator"), securityID); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT IGNORE INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')", roleID("Audit Administrator"), auditID); err != nil {
		return err
	}

	templates := []struct {
		name, description, cpu, memory, storage, class string
		profiles, jobs                                 int
		isDefault                                      bool
	}{
		{"Lightweight", "For low-volume assistants", "1 CPU", "1 GB", "10 GB", "lightweight", 3, 2, true},
		{"Standard", "Balanced enterprise default", "2 CPU", "2 GB", "20 GB", "standard", 8, 4, false},
		{"Developer", "Build and tool development workload", "4 CPU", "8 GB", "50 GB", "developer", 12, 8, false},
		{"Heavy", "High-throughput research workload", "8 CPU", "16 GB", "100 GB", "heavy", 20, 12, false},
	}
	for _, t := range templates {
		if _, err := db.Exec(`INSERT INTO runtime_templates(organization_id,name,description,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,is_default,status,created_by)
			VALUES(1,?,?,?,?,?,?,?,'mock-hermes-0.2','mock',?,'restricted',TRUE,FALSE,?,'active',?)
			ON DUPLICATE KEY UPDATE description=VALUES(description),cpu_limit=VALUES(cpu_limit),memory_limit=VALUES(memory_limit),storage_limit=VALUES(storage_limit),profile_limit=VALUES(profile_limit),max_concurrent_jobs=VALUES(max_concurrent_jobs),is_default=VALUES(is_default)`, t.name, t.description, t.cpu, t.memory, t.storage, t.profiles, t.jobs, t.class, t.isDefault, adminID); err != nil {
			return err
		}
	}
	if _, err := db.Exec("UPDATE runtime_templates SET is_default=FALSE WHERE organization_id=1 AND name<>'Lightweight'"); err != nil {
		return err
	}
	var defaultTemplateID int64
	_ = db.QueryRow("SELECT id FROM runtime_templates WHERE organization_id=1 AND is_default=TRUE LIMIT 1").Scan(&defaultTemplateID)
	if defaultTemplateID > 0 {
		_, _ = db.Exec("UPDATE runtimes SET template_id=?,storage_limit='10 GB',profile_limit=3,max_concurrent_jobs=2,image_version='mock-hermes-0.2',runtime_provider='mock',runtime_class='lightweight',network_policy='restricted' WHERE template_id IS NULL", defaultTemplateID)
		_, _ = db.Exec("UPDATE runtimes SET desired_status=CASE WHEN status IN ('running','stopped','suspended') THEN status ELSE 'running' END,observed_status=CASE WHEN status IN ('running','stopped','suspended') THEN status ELSE 'unknown' END WHERE observed_status='unknown'")
	}

	var standardModel int64
	_ = db.QueryRow("SELECT id FROM models WHERE name='standard' LIMIT 1").Scan(&standardModel)
	profileTemplates := []struct{ name, display, description, class, skills, knowledge string }{
		{"developer-assistant", "Developer Assistant", "Managed coding companion", "developer", `["coding-policy","github-review"]`, `["R&D Knowledge Base"]`},
		{"research-assistant", "Research Assistant", "Managed research and synthesis agent", "standard", `["report-generator"]`, `["Corporate Policies"]`},
		{"network-operations", "Network Operations", "Managed network operations helper", "developer", `["network-troubleshooting"]`, `["Network Operations KB"]`},
		{"finance-assistant", "Finance Assistant", "Managed finance reporting helper", "standard", `["report-generator"]`, `["Finance Policies"]`},
	}
	for _, p := range profileTemplates {
		if _, err := db.Exec(`INSERT INTO profile_templates(organization_id,name,display_name,description,default_model_id,runtime_class,default_skills,default_knowledge,skill_policies,managed,status,created_by)
			VALUES(1,?,?,?,?,?,?,? ,JSON_OBJECT(),TRUE,'active',?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),description=VALUES(description)`, p.name, p.display, p.description, nullableID(standardModel), p.class, p.skills, p.knowledge, adminID); err != nil {
			return err
		}
	}
	var devDept, networkDept, financeDept int64
	_ = db.QueryRow("SELECT id FROM departments WHERE name='软件开发部'").Scan(&devDept)
	_ = db.QueryRow("SELECT id FROM departments WHERE name='网络部'").Scan(&networkDept)
	_ = db.QueryRow("SELECT id FROM departments WHERE name='财务部'").Scan(&financeDept)
	for _, b := range []struct {
		name string
		dept int64
	}{{"developer-assistant", devDept}, {"network-operations", networkDept}, {"finance-assistant", financeDept}} {
		var tid int64
		_ = db.QueryRow("SELECT id FROM profile_templates WHERE name=?", b.name).Scan(&tid)
		if tid > 0 {
			_, _ = db.Exec("INSERT IGNORE INTO profile_template_bindings(template_id,scope,organization_id,department_id,created_by) VALUES(?,'department',1,?,?)", tid, nullableID(b.dept), adminID)
		}
	}

	secretID, err := phase2EnsureSecret(db, "openai-api-key", "api_key", "organization", "not_configured")
	if err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,secret_reference_id,status,description,created_by) VALUES(1,'OpenAI Native','openai','hermes_native','https://api.openai.com/v1','secret_reference',?,'active','Hermes native provider reference',?) ON DUPLICATE KEY UPDATE description=VALUES(description)`, secretID, adminID); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,status,description,created_by) VALUES(1,'Enterprise Gateway','enterprise_gateway','enterprise_gateway','http://gateway.internal/v1','secret_reference','active','Internal gateway boundary; no real gateway in Phase 2',?) ON DUPLICATE KEY UPDATE description=VALUES(description)`, adminID); err != nil {
		return err
	}

	settings := map[string]string{
		"organization_name": "Demo Corporation", "default_language": "en-US", "timezone": "UTC", "local_account_enabled": "true", "sso_reserved_status": "reserved",
		"session_ttl": "1440", "password_minimum_length": "12", "login_failure_lockout": "5", "audit_retention_days": "365", "high_risk_threshold": "70",
		"runtime_provisioning": "Automatic", "default_runtime_template": "Lightweight", "default_runtime_provider": "mock", "default_hermes_image_version": "mock-hermes-0.2",
		"model_access_mode": "Hermes Native", "default_model": "standard", "skill_submission_enabled": "true", "review_required": "true", "default_risk_policy": "standard",
		"default_document_status": "draft", "knowledge_approval_required": "false", "default_export_format": "CSV", "critical_risk_notifications": "true", "approval_notifications": "true", "runtime_failure_notifications": "true",
	}
	for key, value := range settings {
		typ := "string"
		if value == "true" || value == "false" {
			typ = "boolean"
		}
		if key == "session_ttl" || key == "password_minimum_length" || key == "login_failure_lockout" || key == "audit_retention_days" || key == "high_risk_threshold" {
			typ = "number"
		}
		if _, err := db.Exec(`INSERT INTO system_settings(organization_id,setting_key,setting_value,value_type,updated_by) VALUES(1,?,?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),value_type=VALUES(value_type)`, key, value, typ, adminID); err != nil {
			return err
		}
	}

	if _, err := db.Exec(`INSERT INTO risk_rules(code,name,event_pattern,risk_level,risk_score) VALUES
		('break_glass_login','Break-glass login','auth.break_glass.login','critical',100),('role_elevation','Role elevation','role.assign','high',85),('runtime_resize','Runtime resource increase','runtime.resize','high',75),('skill_publish','Skill publication','skill.publish','high',70),('knowledge_acl','Knowledge ACL change','knowledge.binding.create','medium',40)
		ON DUPLICATE KEY UPDATE risk_level=VALUES(risk_level),risk_score=VALUES(risk_score),enabled=TRUE`); err != nil {
		return err
	}
	if _, err := db.Exec(`INSERT INTO quota_policies(organization_id,scope,monthly_token_limit,monthly_cost_limit,max_profiles,max_runtime_cpu,max_runtime_memory,max_concurrent_executions,created_by) SELECT 1,'organization',1000000,500,50,'8 CPU','16 GB',12,? WHERE NOT EXISTS (SELECT 1 FROM quota_policies WHERE organization_id=1 AND scope='organization')`, adminID); err != nil {
		return err
	}
	if _, err := db.Exec("INSERT INTO user_preferences(user_id,language,timezone) VALUES(?, 'en-US','UTC') ON DUPLICATE KEY UPDATE language=language", adminID); err != nil {
		return err
	}

	if err := seedSkillArtifacts(db, adminID); err != nil {
		return err
	}
	if err := seedKnowledgeDocuments(db, adminID); err != nil {
		return err
	}
	var approvalCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM approval_requests").Scan(&approvalCount)
	if approvalCount == 0 {
		var securityReviewer int64
		_ = db.QueryRow("SELECT id FROM users WHERE username='security-admin'").Scan(&securityReviewer)
		res, _ := db.Exec("INSERT INTO approval_requests(type,requester,resource_type,resource_id,status,risk_level,current_reviewer,reason,metadata) VALUES('role_elevation',?,'role',?,'pending','high',?,'Seeded example: independent review for protected role elevation','{}')", adminID, roleID("Audit Administrator"), securityReviewer)
		approvalID, _ := res.LastInsertId()
		_, _ = db.Exec("INSERT INTO approval_steps(approval_request_id,step_order,reviewer_id,status) VALUES(?,1,?,'pending')", approvalID, securityReviewer)
	}
	var riskCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM risk_events").Scan(&riskCount)
	if riskCount == 0 {
		_, _ = db.Exec(`INSERT INTO risk_events(actor_user_id,action,resource_type,risk_level,risk_score,risk_reason,status) VALUES(?, 'runtime.resize','runtime','high',75,'Seeded example: high privilege runtime resource change','open')`, adminID)
	}
	var notificationCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&notificationCount)
	if notificationCount == 0 {
		_, _ = db.Exec("INSERT INTO notifications(user_id,type,title,body,status) VALUES(?,'risk_event','Critical Risk Event','Review the seeded governance event in Risk Events.','unread')", adminID)
	}
	_ = auditID
	return nil
}

func phase2EnsureUser(db *sql.DB, password, username, display, email string, department int) (int64, error) {
	var id int64
	err := db.QueryRow("SELECT id FROM users WHERE username=?", username).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		hash, e := bcryptHash(password)
		if e != nil {
			return 0, e
		}
		res, e := db.Exec("INSERT INTO users(organization_id,department_id,username,display_name,email,password_hash,status) VALUES(1,?,?,?,?,?,'active')", department, username, display, email, hash)
		if e != nil {
			return 0, e
		}
		id, e = res.LastInsertId()
		if e != nil {
			return 0, e
		}
		_, _ = db.Exec("INSERT IGNORE INTO auth_identities(user_id,provider_type,provider_id,external_subject) VALUES(?,'local','local',?)", id, username)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return id, nil
}

func bcryptHash(password string) (string, error) {
	// Keep password hashing in one seed-only helper while using the same bcrypt policy as login.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func phase2EnsureSecret(db *sql.DB, name, typ, scope, status string) (int64, error) {
	var id int64
	if err := db.QueryRow("SELECT id FROM secrets WHERE organization_id=1 AND name=?", name).Scan(&id); err == nil {
		return id, nil
	}
	res, err := db.Exec("INSERT INTO secrets(organization_id,name,type,scope,status) VALUES(1,?,?,?,?)", name, typ, scope, status)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func seedSkillArtifacts(db *sql.DB, actor int64) error {
	rows, err := db.Query("SELECT id,latest_version,risk_level FROM skills")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var skillID int64
		var version, risk string
		if rows.Scan(&skillID, &version, &risk) != nil {
			continue
		}
		var versionID int64
		if db.QueryRow("SELECT id FROM skill_versions WHERE skill_id=? AND version=?", skillID, version).Scan(&versionID) != nil {
			res, e := db.Exec("INSERT INTO skill_versions(skill_id,version,artifact_hash,status,required_tools,required_network,required_secrets,immutable,risk_level) VALUES(?,?,?,'published','[]','[]','[]',TRUE,?)", skillID, version, sha256Text("# "+version), risk)
			if e != nil {
				return e
			}
			versionID, _ = res.LastInsertId()
		}
		artifact, e := ensureArtifactForSeed(db, versionID)
		if e != nil {
			return e
		}
		files := []struct {
			path, content, typ string
			dir                bool
		}{{"SKILL.md", fmt.Sprintf("# %s\n\nManaged Phase 2 Skill artifact.\n", version), "text/markdown", false}, {"scripts", "", "inode/directory", true}, {"templates", "", "inode/directory", true}, {"references", "", "inode/directory", true}}
		for _, f := range files {
			_, e = db.Exec(`INSERT INTO skill_artifact_files(artifact_id,path,content,content_type,size_bytes,sha256,is_directory) VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE content=VALUES(content),sha256=VALUES(sha256)`, artifact, f.path, f.content, f.typ, len(f.content), sha256Text(f.content), f.dir)
			if e != nil {
				return e
			}
		}
		_, _ = db.Exec("UPDATE skill_versions SET immutable=TRUE,risk_level=? WHERE id=?", risk, versionID)
		_, _ = db.Exec("UPDATE skill_artifacts SET status='published',artifact_hash=?,size_bytes=(SELECT COALESCE(SUM(size_bytes),0) FROM skill_artifact_files WHERE artifact_id=?) WHERE id=?", sha256Text("# "+version), artifact, artifact)
		_ = actor
	}
	return rows.Err()
}

func ensureArtifactForSeed(db *sql.DB, versionID int64) (int64, error) {
	var id int64
	if err := db.QueryRow("SELECT id FROM skill_artifacts WHERE skill_version_id=?", versionID).Scan(&id); err == nil {
		return id, nil
	}
	res, err := db.Exec("INSERT INTO skill_artifacts(skill_version_id,status) VALUES(?,'published')", versionID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func seedKnowledgeDocuments(db *sql.DB, actor int64) error {
	rows, err := db.Query("SELECT id,name FROM knowledge_bases")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var kbID int64
		var name string
		if rows.Scan(&kbID, &name) != nil {
			continue
		}
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM knowledge_documents WHERE knowledge_base_id=?", kbID).Scan(&count)
		if count > 0 {
			continue
		}
		title := name + " Handbook"
		content := "# " + title + "\n\nPhase 2 demo knowledge content.\n"
		res, e := db.Exec("INSERT INTO knowledge_documents(knowledge_base_id,title,type,status,owner_user_id,index_status) VALUES(?,?, 'markdown','published',?,'indexed')", kbID, title, actor)
		if e != nil {
			return e
		}
		id, _ := res.LastInsertId()
		_, e = db.Exec("INSERT INTO knowledge_document_versions(document_id,version,content,content_hash,created_by,status) VALUES(?,1,?,?,?,'published')", id, content, sha256Text(content), actor)
		if e != nil {
			return e
		}
	}
	return rows.Err()
}
