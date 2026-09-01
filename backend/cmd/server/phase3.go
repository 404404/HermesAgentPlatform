package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// v0.2.1 keeps the Phase 2 tables compatible, but exposes a consolidated
// domain API. The product calls profile_templates "Agent Templates" while
// the storage name remains unchanged for upgrade compatibility.
func registerPhase3Routes(auth *gin.RouterGroup, s *server) {
	auth.DELETE("/knowledge-bases/:id/bindings/:binding_id", s.deleteKnowledgeBindingV21)
	auth.GET("/agent-templates", s.listAgentTemplates)
	auth.POST("/agent-templates", s.createAgentTemplate)
	auth.GET("/agent-templates/:id", s.agentTemplateDetail)
	auth.PUT("/agent-templates/:id", s.updateAgentTemplate)
	auth.POST("/agent-templates/:id/status", s.setAgentTemplateStatus)
	auth.POST("/agent-templates/:id/assignments", s.assignAgentTemplate)
	auth.DELETE("/agent-templates/:id/assignments/:assignment_id", s.removeAgentTemplateAssignment)
	auth.GET("/agent-templates/:id/instances", s.agentTemplateInstances)

	auth.GET("/profiles/:id/effective-configuration", s.effectiveProfileConfiguration)
	auth.GET("/profiles/:id/assignment-sources", s.profileTemplateAssignmentSources)
	auth.GET("/users/:id/summary", s.userDomainSummary)
	auth.GET("/users/:id/activity", s.userActivity)
	auth.POST("/users/:id/reconcile", s.reconcileUserDomain)

	auth.POST("/departments/manage", s.createDepartmentV21)
	auth.GET("/departments/:id/detail", s.departmentDetailV21)
	auth.PUT("/departments/:id/detail", s.updateDepartmentV21)
	auth.POST("/departments/:id/status", s.setDepartmentStatusV21)
	auth.DELETE("/departments/:id/managed", s.deleteDepartmentV21)

	auth.GET("/runtimes/:id/detail", s.runtimeDetailV21)
	auth.GET("/runtimes/:id/effective-skills", s.runtimeEffectiveSkills)
	auth.GET("/runtimes/:id/executions", s.runtimeExecutions)
	auth.GET("/runtimes/:id/events", s.runtimeEvents)
	auth.POST("/runtimes/:id/control", s.runtimeControlV21)
	auth.POST("/runtimes/:id/kill-switch", s.runtimeKillSwitch)

	auth.GET("/audit-logs/v2", s.auditLogsV21)
	auth.GET("/audit-catalog", s.auditCatalog)
	auth.GET("/dashboard/v3", s.dashboardV21)
	auth.GET("/executions", s.listExecutions)
	auth.POST("/approval-requests/:id/decision-v2", s.decideApprovalV21)
	auth.POST("/executions", s.createExecution)
	auth.GET("/executions/:id", s.executionDetail)
}

func (s *server) auditControlPlane(c *gin.Context, action, label, category, resource string, resourceID int64, result string, metadata gin.H, override *RiskResult) {
	risk := NewRiskEvaluator().Evaluate(action, resource)
	if override != nil {
		risk = *override
	}
	payload := "{}"
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		payload = string(b)
	}
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID, _ = randomToken(12)
	}
	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID = requestID
	}
	var profileID any
	if metadata != nil {
		if v, ok := metadata["profile_id"].(int64); ok {
			profileID = nullableID(v)
		}
	}
	res, err := s.db.Exec(`INSERT INTO audit_logs(actor_user_id,action,action_label,category,resource_type,resource_id,profile_id,scope,result,ip_address,user_agent,request_id,trace_id,metadata,risk_level,risk_score,risk_reason) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, currentUserID(c), action, label, category, resource, nullableID(resourceID), profileID, "global", result, c.ClientIP(), c.GetHeader("User-Agent"), requestID, traceID, payload, risk.Level, risk.Score, risk.Reason)
	if err != nil {
		return
	}
	if risk.Level == "high" || risk.Level == "critical" {
		auditID, _ := res.LastInsertId()
		_, _ = s.db.Exec(`INSERT INTO risk_events(audit_log_id,actor_user_id,action,resource_type,resource_id,risk_level,risk_score,risk_reason,status) VALUES(?,?,?,?,?,?,?,?, 'open')`, auditID, currentUserID(c), action, resource, nullableID(resourceID), risk.Level, risk.Score, risk.Reason)
		s.notifyRole(c, "Risk event requires attention", risk.Reason, resource, resourceID)
	}
}

func (s *server) createDepartmentV21(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	var req struct {
		Name                     string `json:"name"`
		Code                     string `json:"code"`
		Description              string `json:"description"`
		ParentID                 int64  `json:"parent_id"`
		ManagerUserID            int64  `json:"manager_user_id"`
		DefaultRuntimeTemplateID int64  `json:"default_runtime_template_id"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		failCode(c, 400, "department.invalid_request", nil)
		return
	}
	if req.Code == "" {
		req.Code = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}
	res, err := s.db.Exec(`INSERT INTO departments(organization_id,parent_id,name,code,description,manager_user_id,default_runtime_template_id,status) VALUES(?,?,?,?,?,?,?,'active')`, s.currentOrg(c), nullableID(req.ParentID), req.Name, req.Code, req.Description, nullableID(req.ManagerUserID), nullableID(req.DefaultRuntimeTemplateID))
	if err != nil {
		failCode(c, 409, "department.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditControlPlane(c, "department.create", "Department Created", "Organization", "department", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "name": req.Name, "code": req.Code}})
}

func (s *server) departmentDetailV21(c *gin.Context) {
	if !s.requirePermission(c, "department.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var parent, manager, template sql.NullInt64
	var name, code, desc, status, created, updated string
	var members, children, templates, bindings int
	err := s.db.QueryRow(`SELECT d.name,d.code,d.description,d.status,d.parent_id,d.manager_user_id,d.default_runtime_template_id,d.created_at,d.updated_at,(SELECT COUNT(*) FROM users WHERE department_id=d.id),(SELECT COUNT(*) FROM departments WHERE parent_id=d.id),(SELECT COUNT(*) FROM profile_template_bindings WHERE department_id=d.id),(SELECT COUNT(*) FROM knowledge_bindings WHERE department_id=d.id) FROM departments d WHERE d.id=?`, id).Scan(&name, &code, &desc, &status, &parent, &manager, &template, &created, &updated, &members, &children, &templates, &bindings)
	if err != nil {
		failCode(c, 404, "department.not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "code": code, "description": desc, "status": status, "parent_id": nullableSQLID(parent), "manager_user_id": nullableSQLID(manager), "default_runtime_template_id": nullableSQLID(template), "created_at": created, "updated_at": updated, "member_count": members, "child_count": children, "agent_template_assignment_count": templates, "knowledge_binding_count": bindings, "tabs": []string{"Overview", "Members", "Roles", "Agent Templates", "Knowledge", "Runtime Policy", "Activity"}}})
}

func nullableSQLID(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}

func (s *server) updateDepartmentV21(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name                     string `json:"name"`
		Code                     string `json:"code"`
		Description              string `json:"description"`
		ParentID                 int64  `json:"parent_id"`
		ManagerUserID            int64  `json:"manager_user_id"`
		DefaultRuntimeTemplateID int64  `json:"default_runtime_template_id"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		failCode(c, 400, "department.invalid_request", nil)
		return
	}
	before := s.departmentStateV21(id)
	_, err := s.db.Exec(`UPDATE departments SET name=?,code=?,description=?,parent_id=?,manager_user_id=?,default_runtime_template_id=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.Code, req.Description, nullableID(req.ParentID), nullableID(req.ManagerUserID), nullableID(req.DefaultRuntimeTemplateID), id)
	if err != nil {
		failCode(c, 400, "department.update_failed", nil)
		return
	}
	after := s.departmentStateV21(id)
	s.recordChange("department", id, before, after, currentUserID(c))
	s.auditControlPlane(c, "department.update", "Department Updated", "Organization", "department", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": after})
}

func (s *server) departmentStateV21(id int64) gin.H {
	var name, code, desc, status string
	var parent, manager, template sql.NullInt64
	_ = s.db.QueryRow("SELECT name,code,description,status,parent_id,manager_user_id,default_runtime_template_id FROM departments WHERE id=?", id).Scan(&name, &code, &desc, &status, &parent, &manager, &template)
	return gin.H{"id": id, "name": name, "code": code, "description": desc, "status": status, "parent_id": nullableSQLID(parent), "manager_user_id": nullableSQLID(manager), "default_runtime_template_id": nullableSQLID(template)}
}

func (s *server) setDepartmentStatusV21(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Status != "active" && req.Status != "disabled") {
		failCode(c, 400, "department.invalid_status", nil)
		return
	}
	_, err := s.db.Exec("UPDATE departments SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.Status, id)
	if err != nil {
		failCode(c, 400, "department.status_failed", nil)
		return
	}
	s.auditControlPlane(c, "department.status", "Department Status Changed", "Organization", "department", id, "success", gin.H{"status": req.Status}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

func (s *server) deleteDepartmentV21(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var children, members, templates, bindings int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM departments WHERE parent_id=?", id).Scan(&children)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE department_id=?", id).Scan(&members)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM profile_template_bindings WHERE department_id=?", id).Scan(&templates)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM knowledge_bindings WHERE department_id=?", id).Scan(&bindings)
	if children+members+templates+bindings > 0 {
		failCode(c, 409, "department.delete_blocked", gin.H{"children": children, "members": members, "agent_template_assignments": templates, "knowledge_bindings": bindings})
		return
	}
	if _, err := s.db.Exec("DELETE FROM departments WHERE id=?", id); err != nil {
		failCode(c, 400, "department.delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "department.delete", "Department Deleted", "Organization", "department", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) userDomainSummary(c *gin.Context) {
	if !s.requirePermission(c, "user.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var username, email, display, status, dept string
	var deptID, orgID int64
	var runtimeID sql.NullInt64
	var runtimeStatus sql.NullString
	var profiles int
	err := s.db.QueryRow(`SELECT u.username,u.email,u.display_name,u.status,u.organization_id,COALESCE(d.id,0),COALESCE(d.name,''),r.id,r.status,(SELECT COUNT(*) FROM profiles p WHERE p.user_id=u.id) FROM users u LEFT JOIN departments d ON d.id=u.department_id LEFT JOIN runtimes r ON r.user_id=u.id WHERE u.id=?`, id).Scan(&username, &email, &display, &status, &orgID, &deptID, &dept, &runtimeID, &runtimeStatus, &profiles)
	if err != nil {
		failCode(c, 404, "user.not_found", nil)
		return
	}
	roles := s.userRolesData(id)
	templates := s.userTemplatesData(id)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "username": username, "email": email, "display_name": display, "status": status, "organization_id": orgID, "department_id": deptID, "department": dept, "runtime_id": nullableSQLID(runtimeID), "runtime_status": runtimeStatus.String, "profile_count": profiles, "roles": roles, "effective_agent_templates": templates, "profiles": s.userProfilesData(id), "tabs": []string{"Overview", "Organization", "Roles", "Agent Profiles", "Runtime", "Effective Permissions", "Activity"}}})
}

func (s *server) userRolesData(id int64) []gin.H {
	rows, _ := s.db.Query(`SELECT r.id,r.name,rb.scope,COALESCE(d.name,''),rb.created_at FROM role_bindings rb JOIN roles r ON r.id=rb.role_id LEFT JOIN departments d ON d.id=rb.department_id WHERE rb.user_id=? ORDER BY r.name`, id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var rid int64
		var n, scope, d, created string
		if rows.Scan(&rid, &n, &scope, &d, &created) == nil {
			out = append(out, gin.H{"id": rid, "name": n, "scope": scope, "department": d, "created_at": created})
		}
	}
	return out
}
func (s *server) userTemplatesData(id int64) []gin.H {
	rows, _ := s.db.Query(`SELECT DISTINCT pt.id,pt.display_name,pt.template_version FROM profile_templates pt JOIN profile_template_bindings b ON b.template_id=pt.id JOIN users u ON u.id=? WHERE pt.status='active' AND ((b.scope='organization' AND b.organization_id=u.organization_id) OR (b.scope='department' AND b.department_id=u.department_id) OR (b.scope='user' AND b.user_id=u.id) OR (b.scope='role' AND b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=u.id))) ORDER BY pt.display_name`, id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var tid, version int64
		var n string
		if rows.Scan(&tid, &n, &version) == nil {
			out = append(out, gin.H{"id": tid, "display_name": n, "template_version": version})
		}
	}
	return out
}

func (s *server) userActivity(c *gin.Context) {
	if !s.requirePermission(c, "user.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT action,resource_type,COALESCE(resource_id,0),result,created_at FROM audit_logs WHERE actor_user_id=? OR (resource_type='user' AND resource_id=?) ORDER BY created_at DESC LIMIT 100`, id, id)
	if err != nil {
		failCode(c, 500, "user.activity_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var a, r, res, created string
		var rid int64
		if rows.Scan(&a, &r, &rid, &res, &created) == nil {
			out = append(out, gin.H{"action": a, "resource_type": r, "resource_id": rid, "result": res, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) reconcileUserDomain(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	s.consolidateManagedAgents(id)
	c.JSON(200, gin.H{"data": gin.H{"user_id": id, "profiles": s.userProfilesData(id)}})
}
func (s *server) userProfilesData(id int64) []gin.H {
	rows, _ := s.db.Query(`SELECT p.id,p.display_name,p.profile_type,p.status,COALESCE(p.source_template_id,0),COALESCE(p.source_template_version,0) FROM profiles p WHERE p.user_id=? ORDER BY p.display_name`, id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var pid, tid, v int64
		var n, t, st string
		if rows.Scan(&pid, &n, &t, &st, &tid, &v) == nil {
			out = append(out, gin.H{"id": pid, "display_name": n, "profile_type": t, "status": st, "source_template_id": tid, "template_version": v})
		}
	}
	return out
}

func (s *server) runtimeDetailV21(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var uid int64
	var user, runtimeID, status, desired, observed, provider, version, cpu, mem, storage, network, created string
	var profileLimit, jobs int
	var template sql.NullInt64
	var kill bool
	var reason string
	err := s.db.QueryRow(`SELECT r.user_id,u.display_name,r.runtime_id,r.status,r.desired_status,r.observed_status,r.provider,r.hermes_version,r.cpu_limit,r.memory_limit,r.storage_limit,r.profile_limit,r.max_concurrent_jobs,r.network_policy,r.template_id,r.kill_switch_enabled,r.kill_switch_reason,r.created_at FROM runtimes r JOIN users u ON u.id=r.user_id WHERE r.id=?`, id).Scan(&uid, &user, &runtimeID, &status, &desired, &observed, &provider, &version, &cpu, &mem, &storage, &profileLimit, &jobs, &network, &template, &kill, &reason, &created)
	if err != nil {
		failCode(c, 404, "runtime.not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "user_id": uid, "user": user, "runtime_id": runtimeID, "status": status, "desired_status": desired, "observed_status": observed, "provider": provider, "hermes_version": version, "cpu_limit": cpu, "memory_limit": mem, "storage_limit": storage, "profile_limit": profileLimit, "max_concurrent_jobs": jobs, "network_policy": network, "template_id": nullableSQLID(template), "kill_switch_enabled": kill, "kill_switch_reason": reason, "created_at": created, "profiles": s.userProfilesData(uid), "tabs": []string{"Overview", "Profiles", "Effective Skills", "Executions", "Resources", "Controls", "Events"}}})
}

func (s *server) runtimeEffectiveSkills(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var uid int64
	if s.db.QueryRow("SELECT user_id FROM runtimes WHERE id=?", id).Scan(&uid) != nil {
		failCode(c, 404, "runtime.not_found", nil)
		return
	}
	profiles := s.userProfilesData(uid)
	bySkill := map[string]gin.H{}
	for _, p := range profiles {
		pid, _ := p["id"].(int64)
		cfg := s.effectiveConfigurationData(pid)
		if skills, ok := cfg["skills"].([]gin.H); ok {
			for _, skill := range skills {
				name, _ := skill["name"].(string)
				if existing, found := bySkill[name]; found {
					list, _ := existing["profiles"].([]string)
					display, _ := p["display_name"].(string)
					existing["profiles"] = append(list, display)
				} else {
					display, _ := p["display_name"].(string)
					skill["profiles"] = []string{display}
					bySkill[name] = skill
				}
			}
		}
	}
	out := []gin.H{}
	for _, v := range bySkill {
		out = append(out, v)
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) runtimeExecutions(c *gin.Context) {
	if !s.requirePermission(c, "execution.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT e.id,e.execution_id,COALESCE(p.display_name,''),COALESCE(m.display_name,''),e.status,e.risk_level,e.requires_approval,e.started_at,e.finished_at,e.input_tokens,e.output_tokens FROM executions e LEFT JOIN profiles p ON p.id=e.profile_id LEFT JOIN models m ON m.id=e.model_id WHERE e.runtime_id=? ORDER BY e.created_at DESC`, id)
	if err != nil {
		failCode(c, 500, "executions.load_failed", nil)
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": scanExecutionRows(rows)})
}
func (s *server) runtimeEvents(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query("SELECT id,event_type,message,created_at FROM runtime_events WHERE runtime_id=? ORDER BY created_at DESC", id)
	if err != nil {
		failCode(c, 500, "runtime.events_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var eid int64
		var typ, msg, created string
		if rows.Scan(&eid, &typ, &msg, &created) == nil {
			out = append(out, gin.H{"id": eid, "event_type": typ, "message": msg, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) runtimeControlV21(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"start": true, "stop": true, "restart": true}[req.Action] {
		failCode(c, 400, "runtime.invalid_action", nil)
		return
	}
	var rid string
	if s.db.QueryRow("SELECT runtime_id FROM runtimes WHERE id=?", id).Scan(&rid) != nil {
		failCode(c, 404, "runtime.not_found", nil)
		return
	}
	var err error
	switch req.Action {
	case "start":
		err = s.runtime.StartRuntime(c, rid)
	case "stop":
		err = s.runtime.StopRuntime(c, rid)
	case "restart":
		err = s.runtime.RestartRuntime(c, rid)
	}
	if err != nil {
		failCode(c, 500, "runtime.provider_failed", nil)
		return
	}
	desired := "running"
	if req.Action == "stop" {
		desired = "stopped"
	}
	_, _ = s.db.Exec("UPDATE runtimes SET status=?,desired_status=?,observed_status=?,last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=?", desired, desired, desired, id)
	_, _ = s.db.Exec("INSERT INTO runtime_events(runtime_id,event_type,message) VALUES(?,?,?)", id, "control", fmt.Sprintf("Runtime %s", req.Action))
	s.auditControlPlane(c, "runtime."+req.Action, "Runtime "+strings.Title(req.Action), "Runtime", "runtime", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": desired, "desired_status": desired, "observed_status": desired}})
}

func runtimeKillSwitchRisk(reason string) RiskResult {
	return RiskResult{Level: "critical", Score: 100, Reason: "Emergency kill switch activated: " + reason}
}

func (s *server) runtimeKillSwitch(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" {
		failCode(c, 400, "runtime.kill_switch_reason_required", nil)
		return
	}
	if !s.hasRole(currentUserID(c), "Security Administrator", "Break-glass Super Administrator", "Super Admin") {
		failCode(c, 403, "runtime.kill_switch_forbidden", nil)
		return
	}
	_, err := s.db.Exec("UPDATE runtimes SET kill_switch_enabled=TRUE,kill_switch_reason=?,kill_switched_at=UTC_TIMESTAMP(),desired_status='stopped',observed_status='stopped',status='stopped',updated_at=UTC_TIMESTAMP() WHERE id=?", req.Reason, id)
	if err != nil {
		failCode(c, 400, "runtime.kill_switch_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE executions SET status='cancelled',finished_at=UTC_TIMESTAMP() WHERE runtime_id=? AND status IN ('queued','running','waiting_approval')", id)
	_, _ = s.db.Exec("INSERT INTO runtime_events(runtime_id,event_type,message) VALUES(?,?,?)", id, "kill_switch", req.Reason)
	risk := runtimeKillSwitchRisk(req.Reason)
	s.auditControlPlane(c, "runtime.kill_switch", "Emergency Kill Switch Activated", "Security", "runtime", id, "success", gin.H{"reason": req.Reason}, &risk)
	s.notifyRole(c, "Critical runtime kill switch", "A runtime was stopped by the emergency kill switch", "runtime", id)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "kill_switch_enabled": true, "status": "stopped"}})
}

func (s *server) effectiveProfileConfiguration(c *gin.Context) {
	if !s.requirePermission(c, "profile.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	cfg := s.effectiveConfigurationData(id)
	if cfg["id"] == nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": cfg})
}

// effectiveConfigurationData is intentionally centralized so Profile and
// Runtime views use the same additive template/ACL calculation.
func (s *server) effectiveConfigurationData(profileID int64) gin.H {
	var uid int64
	var profileName, profileType string
	var templateID, modelID sql.NullInt64
	var templateName string
	err := s.db.QueryRow(`SELECT p.user_id,p.display_name,p.profile_type,p.source_template_id,COALESCE(p.model_id,0),COALESCE(t.display_name,'') FROM profiles p LEFT JOIN profile_templates t ON t.id=p.source_template_id WHERE p.id=?`, profileID).Scan(&uid, &profileName, &profileType, &templateID, &modelID, &templateName)
	if err != nil {
		return gin.H{}
	}
	model := gin.H{"id": nullableSQLID(modelID), "name": "", "source": "Profile"}
	if modelID.Valid {
		var n string
		_ = s.db.QueryRow("SELECT display_name FROM models WHERE id=?", modelID.Int64).Scan(&n)
		model["name"] = n
	}
	if templateID.Valid {
		var tm sql.NullInt64
		_ = tm.Scan(templateID.Int64)
		_ = s.db.QueryRow("SELECT default_model_id FROM profile_templates WHERE id=?", templateID).Scan(&tm)
		if !modelID.Valid && tm.Valid {
			var n string
			_ = s.db.QueryRow("SELECT display_name FROM models WHERE id=?", tm.Int64).Scan(&n)
			model = gin.H{"id": tm.Int64, "name": n, "source": templateName}
		}
	}
	var deptID, orgID int64
	_ = s.db.QueryRow("SELECT organization_id,COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&orgID, &deptID)
	skills := s.effectiveSkills(profileID, uid, orgID, deptID, templateID, templateName)
	knowledge := s.effectiveKnowledgeSources(profileID, uid, orgID, deptID, templateID, templateName)
	return gin.H{"id": profileID, "profile_type": profileType, "profile_name": profileName, "template": gin.H{"id": nullableSQLID(templateID), "name": templateName, "version": s.templateVersion(templateID)}, "model": model, "skills": skills, "knowledge": knowledge}
}
func (s *server) templateVersion(id sql.NullInt64) any {
	if !id.Valid {
		return nil
	}
	var v int
	_ = s.db.QueryRow("SELECT template_version FROM profile_templates WHERE id=?", id.Int64).Scan(&v)
	return v
}

type effectiveSkillValue struct {
	item     gin.H
	priority int
}

func skillPolicyPriority(p string) int {
	switch p {
	case "blocked":
		return 5
	case "mandatory":
		return 4
	case "explicit":
		return 3
	case "default":
		return 2
	case "optional":
		return 1
	}
	return 0
}
func (s *server) effectiveSkills(profileID, uid, orgID, deptID int64, templateID sql.NullInt64, templateName string) []gin.H {
	values := map[int64]effectiveSkillValue{}
	add := func(id int64, policy, source string) {
		var name, display, cat, risk string
		if s.db.QueryRow("SELECT name,display_name,category,risk_level FROM skills WHERE id=?", id).Scan(&name, &display, &cat, &risk) != nil {
			return
		}
		p := skillPolicyPriority(policy)
		if old, ok := values[id]; ok && old.priority > p {
			return
		}
		values[id] = effectiveSkillValue{gin.H{"id": id, "name": name, "display_name": display, "category": cat, "risk": risk, "policy": policy, "source": source, "enabled": policy != "blocked"}, p}
	}
	if templateID.Valid {
		var raw, policies string
		_ = s.db.QueryRow("SELECT default_skills,skill_policies FROM profile_templates WHERE id=?", templateID.Int64).Scan(&raw, &policies)
		for _, name := range phase3StringList(raw) {
			var id int64
			if s.db.QueryRow("SELECT id FROM skills WHERE name=? OR display_name=?", name, name).Scan(&id) == nil {
				policy := phase3StringMap(policies)[name]
				if policy == "" {
					policy = "default"
				}
				add(id, policy, templateName)
			}
		}
	}
	rows, _ := s.db.Query(`SELECT sa.skill_id,sa.policy,sa.scope FROM skill_assignments sa WHERE (sa.organization_id=? OR sa.department_id=? OR sa.user_id=? OR sa.profile_id=? OR sa.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=?))`, orgID, nullableID(deptID), uid, profileID, uid)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var policy, scope string
			if rows.Scan(&id, &policy, &scope) == nil {
				add(id, policy, scope)
			}
		}
	}
	out := []gin.H{}
	for _, v := range values {
		out = append(out, v.item)
	}
	return out
}
func (s *server) effectiveKnowledgeSources(profileID, uid, orgID, deptID int64, templateID sql.NullInt64, templateName string) []gin.H {
	values := map[int64]gin.H{}
	add := func(id int64, source, policy string) {
		var n, owner, status string
		if s.db.QueryRow("SELECT kb.name,COALESCE(d.name,''),kb.status FROM knowledge_bases kb LEFT JOIN departments d ON d.id=kb.owner_department_id WHERE kb.id=?", id).Scan(&n, &owner, &status) != nil {
			return
		}
		if _, ok := values[id]; !ok {
			values[id] = gin.H{"id": id, "name": n, "owner_department": owner, "status": status, "source": source, "policy": policy}
		}
	}
	if templateID.Valid {
		var raw string
		_ = s.db.QueryRow("SELECT default_knowledge FROM profile_templates WHERE id=?", templateID.Int64).Scan(&raw)
		for _, name := range phase3StringList(raw) {
			var id int64
			if s.db.QueryRow("SELECT id FROM knowledge_bases WHERE name=?", name).Scan(&id) == nil {
				add(id, templateName, "default")
			}
		}
	}
	rows, _ := s.db.Query(`SELECT DISTINCT b.knowledge_base_id,b.binding_type,COALESCE(b.policy,'default') FROM knowledge_bindings b WHERE b.organization_id=? OR b.department_id=? OR b.profile_id=? OR b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=?)`, orgID, nullableID(deptID), profileID, uid)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var source, policy string
			if rows.Scan(&id, &source, &policy) == nil {
				add(id, source, policy)
			}
		}
	}
	out := []gin.H{}
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

func (s *server) agentTemplateInstances(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(200, gin.H{"data": s.agentTemplateInstancesData(id)})
}

type auditRowV21 struct {
	ID, ActorID, ResourceID, ProfileID                                                                             int64
	Actor, Category, Action, Label, ResourceType, Result, IP, RequestID, TraceID, RiskLevel, RiskReason, CreatedAt string
	RiskScore                                                                                                      float64
	Metadata                                                                                                       string
}

func (s *server) auditLogsV21(c *gin.Context) {
	if !s.requirePermission(c, "audit.read") {
		return
	}
	query := `SELECT a.id,COALESCE(a.actor_user_id,0),COALESCE(u.display_name,''),a.category,a.action,a.action_label,a.resource_type,COALESCE(a.resource_id,0),COALESCE(a.profile_id,0),a.result,a.ip_address,a.request_id,a.trace_id,a.metadata,a.risk_level,a.risk_score,a.risk_reason,a.created_at FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE 1=1`
	args := []any{}
	filters := map[string]string{"category": "a.category=?", "action": "(a.action=? OR a.action_label=?)", "actor_id": "a.actor_user_id=?", "department_id": "u.department_id=?", "resource_type": "a.resource_type=?", "resource_id": "a.resource_id=?", "result": "a.result=?", "risk_level": "a.risk_level=?", "ip_address": "a.ip_address=?", "profile_id": "a.profile_id=?", "request_id": "a.request_id=?"}
	for key, expr := range filters {
		if v := c.Query(key); v != "" {
			query += " AND " + expr
			if key == "action" {
				args = append(args, v, v)
			} else {
				args = append(args, v)
			}
		}
	}
	if v := c.Query("time_from"); v != "" {
		query += " AND a.created_at>=?"
		args = append(args, v)
	}
	if v := c.Query("time_to"); v != "" {
		query += " AND a.created_at<=?"
		args = append(args, v)
	}
	query += " ORDER BY a.created_at DESC,a.id DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "audit.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var r auditRowV21
		if rows.Scan(&r.ID, &r.ActorID, &r.Actor, &r.Category, &r.Action, &r.Label, &r.ResourceType, &r.ResourceID, &r.ProfileID, &r.Result, &r.IP, &r.RequestID, &r.TraceID, &r.Metadata, &r.RiskLevel, &r.RiskScore, &r.RiskReason, &r.CreatedAt) == nil {
			out = append(out, gin.H{"id": r.ID, "actor_user_id": r.ActorID, "actor": r.Actor, "category": r.Category, "action": r.Action, "action_label": r.Label, "resource_type": r.ResourceType, "resource_id": r.ResourceID, "profile_id": r.ProfileID, "result": r.Result, "ip_address": r.IP, "request_id": r.RequestID, "trace_id": r.TraceID, "metadata": json.RawMessage(r.Metadata), "risk_level": r.RiskLevel, "risk_score": r.RiskScore, "risk_reason": r.RiskReason, "created_at": r.CreatedAt})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) auditCatalog(c *gin.Context) {
	if !s.requirePermission(c, "audit.read") {
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"categories": []string{"Identity & Access", "Organization", "Roles & Permissions", "Agent Profiles", "Runtime", "Models", "Skills", "Knowledge", "Approvals", "System Settings", "Security", "Exports"}, "actions": gin.H{"Roles & Permissions": []string{"Role Created", "Role Updated", "Role Deleted", "Role Assigned", "Role Removed", "Permission Added", "Permission Removed"}, "Runtime": []string{"Runtime Created", "Runtime Started", "Runtime Stopped", "Runtime Restarted", "Runtime Resized", "Kill Switch Activated"}, "Knowledge": []string{"Knowledge Created", "Knowledge Updated", "Knowledge Binding Changed", "Knowledge Item Created", "Knowledge Item Updated", "Knowledge Item Deleted"}}}})
}
func (s *server) dashboardV21(c *gin.Context) {
	if !s.requirePermission(c, "dashboard.read") {
		return
	}
	var total, active, high, pending, errors, tokens int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE status='active'").Scan(&active)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM executions WHERE risk_level IN ('high','critical')").Scan(&high)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM approval_requests WHERE status='pending'").Scan(&pending)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM runtimes WHERE observed_status IN ('error','degraded') OR status IN ('error','failed')").Scan(&errors)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(input_tokens+output_tokens),0) FROM executions WHERE created_at>=UTC_DATE()").Scan(&tokens)
	c.JSON(200, gin.H{"data": gin.H{"total_users": total, "active_users": active, "high_critical_executions": high, "pending_approvals": pending, "runtime_errors": errors, "token_today": tokens, "attention_required": gin.H{"critical_executions": s.countExecutionsByRisk("critical"), "high_risk_executions": s.countExecutionsByRisk("high"), "pending_approvals": pending, "runtime_failures": errors}, "platform_health": gin.H{"database": "healthy", "runtime_provider": "healthy", "hermes_adapter": "unknown", "model_gateway": "unknown", "knowledge_provider": "healthy", "secret_provider": "unknown"}}})
}
func (s *server) countExecutionsByRisk(level string) int64 {
	var n int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM executions WHERE risk_level=?", level).Scan(&n)
	return n
}

// Keep a small helper available to tests and future adapters.
func parseInt64(value string) int64 { n, _ := strconv.ParseInt(value, 10, 64); return n }

var _ = time.RFC3339

func (s *server) deleteKnowledgeBindingV21(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	bindingID, ok := paramID(c, "binding_id")
	if !ok {
		return
	}
	if _, err := s.db.Exec("DELETE FROM knowledge_bindings WHERE id=?", bindingID); err != nil {
		failCode(c, 400, "knowledge.binding_delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "knowledge.binding.delete", "Knowledge Binding Deleted", "Knowledge", "knowledge_binding", bindingID, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}
