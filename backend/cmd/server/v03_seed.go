package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// seedV03Data is the forward-compatible demo baseline. It intentionally
// removes stale ordinary demo accounts and departments created by the early
// phases while preserving the separate security/audit service identities.
// Re-running the API container is safe and produces the same business users.
func v03SeedCleanupNeeded(db *sql.DB) (bool, error) {
	var value string
	err := db.QueryRow("SELECT setting_value FROM system_settings WHERE organization_id=1 AND setting_key='v03_seed_cleanup_completed'").Scan(&value)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return value != "true", nil
}

func seedV03Data(db *sql.DB, adminPassword, userPassword string) error {
	cleanupNeeded, err := v03SeedCleanupNeeded(db)
	if err != nil {
		return fmt.Errorf("check v0.3 seed marker: %w", err)
	}
	if err := v03RemoveLegacySeedData(db); err != nil {
		return fmt.Errorf("remove legacy seed data: %w", err)
	}
	if cleanupNeeded {
		if _, err := db.Exec("DELETE FROM users WHERE username NOT IN ('admin','user01','user02','security-admin','audit-admin')"); err != nil {
			return fmt.Errorf("remove legacy demo users: %w", err)
		}
	}

	// The three business accounts are the only ordinary users shown by the
	// demo. Security and audit accounts remain internal administrative actors.
	adminID, err := v03EnsureUser(db, "admin", "Platform Administrator", "admin@demo.local", "", adminPassword, false)
	if err != nil {
		return err
	}
	if cleanupNeeded {
		if _, err := db.Exec("UPDATE users SET system_account=FALSE,department_id=NULL,status='active' WHERE id=?", adminID); err != nil {
			return err
		}
	}
	securityID, err := v03EnsureUser(db, "security-admin", "Security Administrator", "security-admin@demo.local", "", userPassword, true)
	if err != nil {
		return err
	}
	auditID, err := v03EnsureUser(db, "audit-admin", "Audit Administrator", "audit-admin@demo.local", "", userPassword, true)
	if err != nil {
		return err
	}
	testDepartmentID, err := v03EnsureTestDepartment(db)
	if err != nil {
		return err
	}
	user01ID, err := v03EnsureUser(db, "user01", "User One", "user01@demo.local", "测试部门", userPassword, false)
	if err != nil {
		return err
	}
	user02ID, err := v03EnsureUser(db, "user02", "User Two", "user02@demo.local", "测试部门", userPassword, false)
	if err != nil {
		return err
	}
	if cleanupNeeded {
		if _, err := db.Exec("UPDATE users SET department_id=?,system_account=FALSE,status='active' WHERE id IN (?,?)", testDepartmentID, user01ID, user02ID); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE users SET department_id=NULL,system_account=TRUE,status='active' WHERE id IN (?,?)", securityID, auditID); err != nil {
			return err
		}
	}

	if cleanupNeeded {
		if err := v03ResetBusinessDepartments(db, testDepartmentID); err != nil {
			return err
		}
	}
	if err := v03SeedRoleBindings(db, adminID, securityID, auditID, user01ID, user02ID, cleanupNeeded); err != nil {
		return err
	}
	if err := v03SeedModels(db, adminID); err != nil {
		return err
	}
	if err := v03SeedRuntimeTemplates(db, adminID, cleanupNeeded); err != nil {
		return err
	}
	if err := v03SeedAgentTemplates(db, adminID, testDepartmentID, user01ID, user02ID, cleanupNeeded); err != nil {
		return err
	}
	if err := v03SeedRuntimeRecords(db, adminID, user01ID, user02ID); err != nil {
		return err
	}
	if err := v03SeedPolicies(db, adminID, testDepartmentID, user01ID, user02ID, cleanupNeeded); err != nil {
		return err
	}
	if err := v03SeedRuntimeHost(db, adminID); err != nil {
		return err
	}
	if err := v03SeedWorkspaceContent(db, adminID, user01ID, user02ID, cleanupNeeded); err != nil {
		return err
	}
	if cleanupNeeded {
		if _, err := db.Exec("INSERT INTO system_settings(organization_id,setting_key,setting_value,value_type,updated_by) VALUES(1,'v03_seed_cleanup_completed','true','string',?) ON DUPLICATE KEY UPDATE setting_value='true',updated_by=VALUES(updated_by)", adminID); err != nil {
			return err
		}
	}
	return nil
}

// v03RemoveLegacySeedData only targets records with stable identifiers from
// the pre-v0.3 demo seed. It is intentionally narrower than a general data
// cleanup so user-created accounts, departments and bindings remain intact.
func v03RemoveLegacySeedData(db *sql.DB) error {
	if _, err := db.Exec("DELETE FROM users WHERE username IN ('developer01','network01','finance01')"); err != nil {
		return err
	}
	const legacyDepartments = "SELECT id FROM departments WHERE id IN (1,2,3,4,5,6) AND code IN ('dept-1','dept-2','dept-3','dept-4','dept-5','dept-6')"
	for _, query := range []string{
		"DELETE FROM profile_template_bindings WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM runtime_template_bindings WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM knowledge_bindings WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM skill_assignments WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM role_bindings WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM quota_policies WHERE department_id IN (" + legacyDepartments + ")",
		"DELETE FROM user_self_service_policies WHERE department_id IN (" + legacyDepartments + ")",
		"UPDATE users SET department_id=NULL WHERE department_id IN (" + legacyDepartments + ")",
		"UPDATE knowledge_bases SET owner_department_id=NULL WHERE owner_department_id IN (" + legacyDepartments + ")",
		"UPDATE usage_events SET department_id=NULL WHERE department_id IN (" + legacyDepartments + ")",
		"UPDATE executions SET department_id=NULL WHERE department_id IN (" + legacyDepartments + ")",
	} {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	_, err := db.Exec("DELETE FROM departments WHERE id IN (1,2,3,4,5,6) AND code IN ('dept-1','dept-2','dept-3','dept-4','dept-5','dept-6')")
	return err
}

func v03EnsureUser(db *sql.DB, username, display, email, departmentName, password string, system bool) (int64, error) {
	var id, departmentID int64
	if departmentName != "" {
		_ = db.QueryRow("SELECT id FROM departments WHERE name=? LIMIT 1", departmentName).Scan(&departmentID)
	}
	hash, err := bcryptHash(password)
	if err != nil {
		return 0, err
	}
	err = db.QueryRow("SELECT id FROM users WHERE username=?", username).Scan(&id)
	if err == sql.ErrNoRows {
		res, insertErr := db.Exec(`INSERT INTO users(organization_id,department_id,username,display_name,email,password_hash,status,system_account) VALUES(1,?,?,?,?,?,'active',?)`, nullableID(departmentID), username, display, email, hash, system)
		if insertErr != nil {
			return 0, insertErr
		}
		id, _ = res.LastInsertId()
		_, _ = db.Exec("INSERT IGNORE INTO auth_identities(user_id,provider_type,provider_id,external_subject) VALUES(?,'local','local',?)", id, username)
	} else if err != nil {
		return 0, err
	} else {
		_, err = db.Exec("UPDATE users SET display_name=?,email=?,password_hash=?,updated_at=UTC_TIMESTAMP() WHERE id=?", display, email, hash, id)
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func v03EnsureTestDepartment(db *sql.DB) (int64, error) {
	var id int64
	err := db.QueryRow("SELECT id FROM departments WHERE code='TEST' LIMIT 1").Scan(&id)
	if err == sql.ErrNoRows {
		res, insertErr := db.Exec("INSERT INTO departments(organization_id,parent_id,name,code,description,status) VALUES(1,NULL,'测试部门','TEST','Demo workspace department','active')")
		if insertErr != nil {
			return 0, insertErr
		}
		id, _ = res.LastInsertId()
		return id, nil
	}
	if err != nil {
		return 0, err
	}
	_, err = db.Exec("UPDATE departments SET name='测试部门',parent_id=NULL,description='Demo workspace department',status='active' WHERE id=?", id)
	return id, err
}

func v03ResetBusinessDepartments(db *sql.DB, keepID int64) error {
	// Clear relationship rows before deleting the stale department tree. This
	// is a seed migration cleanup, not a runtime delete endpoint.
	for _, query := range []string{
		"DELETE FROM profile_template_bindings WHERE department_id IS NOT NULL AND department_id<>?",
		"DELETE FROM runtime_template_bindings WHERE department_id IS NOT NULL AND department_id<>?",
		"DELETE FROM knowledge_bindings WHERE department_id IS NOT NULL AND department_id<>?",
		"UPDATE users SET department_id=NULL WHERE department_id IS NOT NULL AND department_id<>?",
		"UPDATE knowledge_bases SET owner_department_id=NULL WHERE owner_department_id IS NOT NULL AND owner_department_id<>?",
	} {
		if _, err := db.Exec(query, keepID); err != nil {
			return err
		}
	}
	if _, err := db.Exec("DELETE FROM departments WHERE id<>?", keepID); err != nil {
		return err
	}
	_, err := db.Exec("UPDATE departments SET parent_id=NULL,status='active' WHERE id=?", keepID)
	return err
}

func v03RoleID(db *sql.DB, name string) int64 {
	var id int64
	_ = db.QueryRow("SELECT id FROM roles WHERE name=? LIMIT 1", name).Scan(&id)
	return id
}

func v03EnsureRoleBinding(db *sql.DB, roleID, userID int64) error {
	var existing int64
	err := db.QueryRow("SELECT id FROM role_bindings WHERE role_id=? AND organization_id=1 AND user_id=? AND scope='user' LIMIT 1", roleID, userID).Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')", roleID, userID)
	}
	return err
}

func v03SeedRoleBindings(db *sql.DB, adminID, securityID, auditID, user01ID, user02ID int64, reset bool) error {
	if reset {
		for _, id := range []int64{adminID, securityID, auditID, user01ID, user02ID} {
			if _, err := db.Exec("DELETE FROM role_bindings WHERE user_id=?", id); err != nil {
				return err
			}
		}
	}
	bindings := []struct {
		role, user int64
	}{
		{v03RoleID(db, "Break-glass Super Administrator"), adminID},
		{v03RoleID(db, "System Administrator"), adminID},
		{v03RoleID(db, "Security Administrator"), securityID},
		{v03RoleID(db, "Audit Administrator"), auditID},
		{v03RoleID(db, "Standard User"), user01ID},
		{v03RoleID(db, "Standard User"), user02ID},
		{v03RoleID(db, "Developer"), user02ID},
	}
	for _, binding := range bindings {
		if binding.role == 0 {
			continue
		}
		if err := v03EnsureRoleBinding(db, binding.role, binding.user); err != nil {
			return err
		}
	}
	return nil
}

func v03SeedModels(db *sql.DB, adminID int64) error {
	var nativeID, gatewayID int64
	_ = db.QueryRow("SELECT id FROM model_providers WHERE name='OpenAI Native' LIMIT 1").Scan(&nativeID)
	_ = db.QueryRow("SELECT id FROM model_providers WHERE name='Enterprise Gateway' LIMIT 1").Scan(&gatewayID)
	if nativeID == 0 {
		res, err := db.Exec("INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,status,description,created_by) VALUES(1,'OpenAI Native','openai','hermes_native','https://api.openai.com/v1','secret_reference','active','Hermes Native mock provider',?)", adminID)
		if err != nil {
			return err
		}
		nativeID, _ = res.LastInsertId()
	}
	if gatewayID == 0 {
		res, err := db.Exec("INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,status,description,created_by) VALUES(1,'Enterprise Gateway','enterprise_gateway','enterprise_gateway','http://gateway.internal/v1','secret_reference','active','Enterprise Gateway mock boundary',?)", adminID)
		if err != nil {
			return err
		}
		gatewayID, _ = res.LastInsertId()
	}
	_, _ = db.Exec("UPDATE model_providers SET health_status='healthy' WHERE id IN (?,?)", nativeID, gatewayID)
	models := []struct {
		name, display, upstream string
		provider                int64
		purpose                 string
	}{
		{"fast", "Fast", "gpt-4o-mini", nativeID, "main"},
		{"standard", "Standard", "gpt-4o", nativeID, "main"},
		{"reasoning", "Reasoning", "o3", nativeID, "main"},
		{"confidential", "Confidential", "enterprise-confidential", gatewayID, "main"},
	}
	ids := []int64{}
	for _, model := range models {
		var providerModelID int64
		_ = db.QueryRow("SELECT id FROM provider_models WHERE provider_id=? AND upstream_model=?", model.provider, model.upstream).Scan(&providerModelID)
		if providerModelID == 0 {
			res, err := db.Exec("INSERT INTO provider_models(provider_id,upstream_model,display_name,status,sync_status,last_sync_at) VALUES(?,?,?,'active','mock',UTC_TIMESTAMP())", model.provider, model.upstream, model.display)
			if err != nil {
				return err
			}
			providerModelID, _ = res.LastInsertId()
		}
		if _, err := db.Exec("UPDATE models SET provider_id=?,provider_model_id=?,purpose=?,status='active',user_selectable=TRUE,updated_at=UTC_TIMESTAMP() WHERE name=?", model.provider, providerModelID, model.purpose, model.name); err != nil {
			return err
		}
		var modelID int64
		_ = db.QueryRow("SELECT id FROM models WHERE name=?", model.name).Scan(&modelID)
		ids = append(ids, modelID)
	}
	allowed, _ := json.Marshal(ids)
	providers, _ := json.Marshal([]int64{nativeID, gatewayID})
	for _, slot := range append([]string{"main"}, hermesAuxiliarySlots...) {
		defaultID := ids[1]
		_, err := db.Exec(`INSERT INTO model_slot_policies(organization_id,slot,default_model_id,override_mode,allowed_models,allowed_providers,updated_by) VALUES(1,?,?, 'whitelist',?,?,?) ON DUPLICATE KEY UPDATE default_model_id=VALUES(default_model_id),allowed_models=VALUES(allowed_models),allowed_providers=VALUES(allowed_providers),updated_by=VALUES(updated_by)`, slot, defaultID, string(allowed), string(providers), adminID)
		if err != nil {
			return err
		}
	}
	return nil
}

func v03SeedRuntimeTemplates(db *sql.DB, adminID int64, reset bool) error {
	templates := []struct {
		name, class, cpu, memory, storage string
		profiles, jobs                    int
		defaultTemplate                   bool
	}{
		{"Lightweight", "lightweight", "1 CPU", "1 GB", "10 GB", 3, 2, true},
		{"Standard", "standard", "2 CPU", "2 GB", "20 GB", 8, 4, false},
		{"Developer", "developer", "4 CPU", "8 GB", "50 GB", 12, 8, false},
		{"Heavy", "heavy", "8 CPU", "16 GB", "100 GB", 20, 12, false},
	}
	for _, t := range templates {
		if !reset {
			continue
		}
		if _, err := db.Exec(`UPDATE runtime_templates SET cpu_limit=?,memory_limit=?,storage_limit=?,profile_limit=?,max_concurrent_jobs=?,runtime_provider='mock',runtime_class=?,network_policy='restricted',is_default=?,status='active',updated_at=UTC_TIMESTAMP(),created_by=? WHERE organization_id=1 AND name=?`, t.cpu, t.memory, t.storage, t.profiles, t.jobs, t.class, t.defaultTemplate, adminID, t.name); err != nil {
			return err
		}
	}
	return nil
}

func v03EnsureTemplateBinding(db *sql.DB, templateID int64, scope string, departmentID, roleID, userID, createdBy int64) error {
	var existing int64
	err := db.QueryRow(`SELECT id FROM profile_template_bindings WHERE template_id=? AND scope=? AND organization_id=1 AND COALESCE(department_id,0)=COALESCE(?,0) AND COALESCE(role_id,0)=COALESCE(?,0) AND COALESCE(user_id,0)=COALESCE(?,0) LIMIT 1`, templateID, scope, nullableID(departmentID), nullableID(roleID), nullableID(userID)).Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`INSERT INTO profile_template_bindings(template_id,scope,organization_id,department_id,role_id,user_id,created_by) VALUES(?, ?,1,?,?,?,?)`, templateID, scope, nullableID(departmentID), nullableID(roleID), nullableID(userID), createdBy)
	}
	return err
}

func v03SeedAgentTemplates(db *sql.DB, adminID, deptID, user01ID, user02ID int64, reset bool) error {
	var standardModel int64
	_ = db.QueryRow("SELECT id FROM models WHERE name='standard'").Scan(&standardModel)
	items := []struct {
		name, display, description, skills, knowledge string
	}{
		{"research-assistant", "Research Assistant", "Research and synthesis workspace agent", `["report-generator"]`, `["Corporate Policies"]`},
		{"developer-assistant", "Developer Assistant", "Managed software development companion", `["coding-policy","github-review"]`, `["R&D Knowledge Base"]`},
		{"personal-assistant", "Personal Assistant", "Personal workspace assistant", `[]`, `["Corporate Policies"]`},
	}
	for _, item := range items {
		if !reset {
			continue
		}
		_, err := db.Exec(`INSERT INTO profile_templates(organization_id,name,display_name,description,default_model_id,runtime_class,default_skills,default_knowledge,skill_policies,managed,status,created_by)
			VALUES(1,?,?,?,?, 'standard',?,?,JSON_OBJECT(),TRUE,'active',?) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name),description=VALUES(description),default_model_id=VALUES(default_model_id),default_skills=VALUES(default_skills),default_knowledge=VALUES(default_knowledge),status='active',updated_at=UTC_TIMESTAMP()`, item.name, item.display, item.description, nullableID(standardModel), item.skills, item.knowledge, adminID)
		if err != nil {
			return err
		}
	}
	if reset {
		if _, err := db.Exec("DELETE FROM profile_template_bindings"); err != nil {
			return err
		}
	}
	var research, developer, personal, standardRole, developerRole int64
	_ = db.QueryRow("SELECT id FROM profile_templates WHERE name='research-assistant'").Scan(&research)
	_ = db.QueryRow("SELECT id FROM profile_templates WHERE name='developer-assistant'").Scan(&developer)
	_ = db.QueryRow("SELECT id FROM profile_templates WHERE name='personal-assistant'").Scan(&personal)
	standardRole = v03RoleID(db, "Standard User")
	developerRole = v03RoleID(db, "Developer")
	bindings := []struct {
		template         int64
		scope            string
		dept, role, user int64
	}{
		{research, "department", deptID, 0, 0},
		{research, "user", 0, 0, user01ID},
		{research, "user", 0, 0, user02ID},
		{developer, "role", 0, developerRole, 0},
		{personal, "user", 0, 0, user01ID},
	}
	for _, binding := range bindings {
		if binding.template == 0 {
			continue
		}
		if err := v03EnsureTemplateBinding(db, binding.template, binding.scope, binding.dept, binding.role, binding.user, adminID); err != nil {
			return err
		}
	}
	// Keep Standard User as a real assignment source for future departments,
	// while the demo's explicit department binding remains the visible source.
	if research > 0 && standardRole > 0 {
		if err := v03EnsureTemplateBinding(db, research, "role", 0, standardRole, 0, adminID); err != nil {
			return err
		}
	}
	s := &server{db: db}
	s.consolidateAllManagedAgents()
	return nil
}

func v03SeedRuntimeRecords(db *sql.DB, adminID, user01ID, user02ID int64) error {
	var templateID int64
	_ = db.QueryRow("SELECT id FROM runtime_templates WHERE name='Lightweight' LIMIT 1").Scan(&templateID)
	for _, userID := range []int64{user01ID, user02ID} {
		var existing int64
		err := db.QueryRow("SELECT id FROM runtimes WHERE user_id=? LIMIT 1", userID).Scan(&existing)
		if err == sql.ErrNoRows {
			_, err = db.Exec(`INSERT INTO runtimes(user_id,runtime_id,status,provider,hermes_version,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,template_id,desired_status,observed_status,placement_status,container_name,actual_cpu,actual_memory,actual_storage,observed_image_version)
			VALUES(?,?,'running','mock','mock-hermes-0.3','1 CPU','1 GB','10 GB',3,2,'mock-hermes-0.3','mock','lightweight','restricted',TRUE,FALSE,?,'running','running','unplaced', '', '', '', '', '')`, userID, fmt.Sprintf("mock-runtime-%d", userID), nullableID(templateID))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func v03SeedPolicies(db *sql.DB, adminID, deptID, user01ID, user02ID int64, reset bool) error {
	settings := map[string]string{
		"runtime_placement_mode": "Automatic", "runtime_health_check_interval": "60", "runtime_provision_timeout": "300",
		"workspace_default_language": "en-US", "channel_provider": "MockChannelProvider",
	}
	if reset {
		for key, value := range settings {
			if _, err := db.Exec("INSERT INTO system_settings(organization_id,setting_key,setting_value,value_type,updated_by) VALUES(1,?,?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),updated_by=VALUES(updated_by)", key, value, "string", adminID); err != nil {
				return err
			}
		}
		if _, err := db.Exec("DELETE FROM user_self_service_policies WHERE scope='user' AND user_id IN (?,?)", user01ID, user02ID); err != nil {
			return err
		}
		allowedModels := `[` + v03ModelIDsJSON(db) + `]`
		for _, userID := range []int64{user01ID, user02ID} {
			for _, capability := range selfServiceCapabilities {
				mode := "disabled"
				values := "[]"
				switch capability {
				case "create_personal_profile", "configure_channel":
					mode = "allowed"
				case "change_main_model":
					mode = "whitelist"
					values = allowedModels
				}
				if _, err := db.Exec(`INSERT INTO user_self_service_policies(organization_id,scope,user_id,capability,mode,allowed_values,updated_by) VALUES(1,'user',?,?,?,?,?)`, userID, capability, mode, values, adminID); err != nil {
					return err
				}
			}
		}
		// Organization policy is the fallback for users added later.
		for _, capability := range selfServiceCapabilities {
			mode := "disabled"
			if capability == "create_personal_profile" || capability == "configure_channel" {
				mode = "allowed"
			}
			if _, err := db.Exec(`INSERT INTO user_self_service_policies(organization_id,scope,capability,mode,allowed_values,updated_by) VALUES(1,'organization',?,?,?,?) ON DUPLICATE KEY UPDATE mode=VALUES(mode),allowed_values=VALUES(allowed_values),updated_by=VALUES(updated_by)`, capability, mode, `[]`, adminID); err != nil {
				return err
			}
		}
	}
	_ = deptID
	return nil
}

func v03ModelIDsJSON(db *sql.DB) string {
	rows, _ := db.Query("SELECT id FROM models WHERE name IN ('fast','standard','reasoning') ORDER BY FIELD(name,'fast','standard','reasoning')")
	if rows == nil {
		return ""
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			values = append(values, fmt.Sprintf("%d", id))
		}
	}
	return strings.Join(values, ",")
}

func v03SeedRuntimeHost(db *sql.DB, adminID int64) error {
	var hostID int64
	err := db.QueryRow("SELECT id FROM runtime_hosts WHERE name='Demo Runtime Host' LIMIT 1").Scan(&hostID)
	if err == sql.ErrNoRows {
		res, insertErr := db.Exec(`INSERT INTO runtime_hosts(organization_id,name,hostname,address,ssh_port,auth_type,credential_reference_id,docker_endpoint,docker_version,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,status,labels,last_seen,last_inventory_at,created_by)
			VALUES(1,'Demo Runtime Host','rs820-demo','mock://rs820-runtime-host',22,'secret_reference',NULL,'mock://local-runtime-provider','mock-docker-27','16 CPU','32 GB','500 GB','2 CPU','2 GB','20 GB','healthy',JSON_ARRAY('demo','mock','synology'),UTC_TIMESTAMP(),UTC_TIMESTAMP(),?)`, adminID)
		if insertErr != nil {
			return insertErr
		}
		hostID, _ = res.LastInsertId()
	} else if err != nil {
		return err
	}
	if _, err := db.Exec("UPDATE runtime_hosts SET status='healthy',docker_version='mock-docker-27',last_seen=UTC_TIMESTAMP(),last_inventory_at=UTC_TIMESTAMP() WHERE id=?", hostID); err != nil {
		return err
	}
	rows, _ := db.Query("SELECT id FROM runtimes WHERE user_id IN (SELECT id FROM users WHERE username IN ('user01','user02'))")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var runtimeID int64
			if rows.Scan(&runtimeID) == nil {
				_, _ = db.Exec("UPDATE runtimes SET host_id=?,placement_status='placed',actual_cpu=cpu_limit,actual_memory=memory_limit,actual_storage=storage_limit,observed_image_version=image_version WHERE id=?", hostID, runtimeID)
			}
		}
	}
	_, _ = db.Exec("UPDATE runtime_hosts SET runtime_count=(SELECT COUNT(*) FROM runtimes WHERE host_id=?),cpu_allocated=CONCAT((SELECT COUNT(*) FROM runtimes WHERE host_id=?),' CPU'),memory_allocated=CONCAT((SELECT COUNT(*) FROM runtimes WHERE host_id=?)*1,' GB'),storage_allocated=CONCAT((SELECT COUNT(*) FROM runtimes WHERE host_id=?)*10,' GB') WHERE id=?", hostID, hostID, hostID, hostID, hostID)
	return nil
}

func v03SeedWorkspaceContent(db *sql.DB, adminID, user01ID, user02ID int64, reset bool) error {
	var feishuID, slackID int64
	_ = db.QueryRow("SELECT id FROM channel_policies WHERE channel_type='feishu'").Scan(&feishuID)
	if feishuID == 0 {
		res, err := db.Exec("INSERT INTO channel_policies(organization_id,channel_type,enabled,user_self_service,user_credentials_allowed,admin_managed,policy,created_by) VALUES(1,'feishu',TRUE,'allowed',FALSE,TRUE,JSON_OBJECT('provider','MockChannelProvider'),?)", adminID)
		if err != nil {
			return err
		}
		feishuID, _ = res.LastInsertId()
	}
	_ = db.QueryRow("SELECT id FROM channel_policies WHERE channel_type='slack'").Scan(&slackID)
	if slackID == 0 {
		_, err := db.Exec("INSERT INTO channel_policies(organization_id,channel_type,enabled,user_self_service,user_credentials_allowed,admin_managed,policy,created_by) VALUES(1,'slack',FALSE,'disabled',FALSE,TRUE,JSON_OBJECT('provider','MockChannelProvider'),?)", adminID)
		if err != nil {
			return err
		}
	}
	var research01, developer02 int64
	_ = db.QueryRow("SELECT id FROM profiles WHERE user_id=? AND display_name='Research Assistant' LIMIT 1", user01ID).Scan(&research01)
	_ = db.QueryRow("SELECT id FROM profiles WHERE user_id=? AND display_name='Developer Assistant' LIMIT 1", user02ID).Scan(&developer02)
	if reset {
		if research01 > 0 {
			_, _ = db.Exec(`INSERT INTO channel_connections(organization_id,user_id,profile_id,channel_type,status,settings,created_by) VALUES(1,?,?, 'feishu','connected',JSON_OBJECT('provider','MockChannelProvider'),?) ON DUPLICATE KEY UPDATE status='connected'`, user01ID, research01, adminID)
		}
		if developer02 > 0 {
			_, _ = db.Exec(`INSERT INTO channel_connections(organization_id,user_id,profile_id,channel_type,status,settings,created_by) VALUES(1,?,?, 'feishu','connected',JSON_OBJECT('provider','MockChannelProvider'),?) ON DUPLICATE KEY UPDATE status='connected'`, user02ID, developer02, adminID)
		}
		for _, userID := range []int64{user01ID, user02ID} {
			var profileID int64
			_ = db.QueryRow("SELECT id FROM profiles WHERE user_id=? ORDER BY managed DESC, id LIMIT 1", userID).Scan(&profileID)
			if profileID == 0 {
				continue
			}
			var conversationID int64
			_ = db.QueryRow("SELECT id FROM chat_conversations WHERE user_id=? LIMIT 1", userID).Scan(&conversationID)
			if conversationID == 0 {
				res, err := db.Exec("INSERT INTO chat_conversations(organization_id,user_id,profile_id,title) VALUES(1,?,?,?)", userID, profileID, "Welcome to HEP Workspace")
				if err != nil {
					return err
				}
				conversationID, _ = res.LastInsertId()
				_, _ = db.Exec("INSERT INTO chat_messages(conversation_id,role,content,metadata) VALUES(?,?,?,JSON_OBJECT('provider','MockChatProvider'))", conversationID, "assistant", "Welcome to the HEP Workspace. Select an Agent Profile to start a simulated conversation.")
			}
		}
	}
	// The test data is deliberately sparse; future real providers own the
	// execution and embedding lifecycle rather than this seed function.
	_, _ = db.Exec("INSERT IGNORE INTO user_preferences(user_id,language,timezone) VALUES(?,'en-US','UTC'),(?,'en-US','UTC')", user01ID, user02ID)
	return nil
}
