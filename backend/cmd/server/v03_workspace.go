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

// v0.3 deliberately keeps provider implementations behind small interfaces.
// The demo uses these deterministic implementations until Hermes, a model
// gateway, or a runtime host adapter is introduced in a later phase.
type ChatProvider interface {
	Reply(profileName, message string) string
}

type MockChatProvider struct{}

func (MockChatProvider) Reply(profileName, message string) string {
	return fmt.Sprintf("MockChatProvider · %s\n\nI received your request: %s\n\nThis workspace response is simulated. A future Hermes Adapter will execute the selected profile.", profileName, message)
}

var _ ChatProvider = MockChatProvider{}

var hermesAuxiliarySlots = []string{
	"title", "vision", "compression", "approval", "web_extract", "skills_hub",
	"mcp", "triage_specifier", "kanban_decomposer", "profile_describer", "curator",
}

var selfServiceCapabilities = []string{
	"create_personal_profile", "change_main_model", "change_auxiliary_models", "add_model_provider",
	"configure_model_credentials", "install_optional_skill", "configure_channel",
	"configure_channel_credentials", "create_personal_knowledge",
}

func registerV03Routes(auth *gin.RouterGroup, s *server) {
	// Console boundary and user-owned Workspace resources.
	auth.GET("/admin/access", s.adminAccess)
	auth.GET("/me", s.workspaceMe)
	auth.GET("/me/permissions", s.workspacePermissions)
	auth.GET("/me/agents", s.workspaceAgents)
	auth.POST("/me/agents", s.createPersonalAgent)
	auth.GET("/me/agents/:id", s.workspaceAgentDetail)
	auth.PUT("/me/agents/:id", s.updatePersonalAgent)
	auth.DELETE("/me/agents/:id", s.deletePersonalAgent)
	auth.GET("/me/models", s.workspaceModels)
	auth.GET("/me/skills", s.workspaceSkills)
	auth.GET("/me/knowledge", s.workspaceKnowledge)
	auth.GET("/me/channels", s.workspaceChannels)
	auth.POST("/me/channels", s.createWorkspaceChannel)
	auth.PUT("/me/channels/:id", s.updateWorkspaceChannel)
	auth.DELETE("/me/channels/:id", s.deleteWorkspaceChannel)
	auth.GET("/me/usage", s.workspaceUsage)
	auth.GET("/me/notifications", s.workspaceNotifications)
	auth.POST("/me/notifications/:id/read", s.readWorkspaceNotification)
	auth.POST("/me/notifications/read-all", s.readAllWorkspaceNotifications)
	auth.GET("/me/self-service-policy", s.workspaceSelfServicePolicy)
	auth.GET("/me/conversations", s.listConversations)
	auth.POST("/me/conversations", s.createConversation)
	auth.GET("/me/conversations/:id/messages", s.listConversationMessages)
	auth.POST("/me/conversations/:id/messages", s.createConversationMessage)

	// Admin-only v0.3 management surfaces. Existing handlers are protected by
	// requirePermission, which also enforces the admin console boundary.
	auth.GET("/provider-models", s.listProviderModels)
	auth.POST("/model-providers/:id/test", s.testModelProvider)
	auth.POST("/model-providers/:id/sync", s.syncModelProvider)
	auth.GET("/model-slot-policies", s.listModelSlotPolicies)
	auth.PUT("/model-slot-policies/:slot", s.updateModelSlotPolicy)
	auth.GET("/self-service-policies", s.listSelfServicePolicies)
	auth.POST("/self-service-policies", s.createSelfServicePolicy)
	auth.PUT("/self-service-policies/:id", s.updateSelfServicePolicy)
	auth.GET("/channel-policies", s.listChannelPolicies)
	auth.POST("/channel-policies", s.createChannelPolicy)
	auth.PUT("/channel-policies/:id", s.updateChannelPolicy)
	auth.GET("/runtime-hosts", s.listRuntimeHosts)
	auth.POST("/runtime-hosts", s.createRuntimeHost)
	auth.PUT("/runtime-hosts/:id", s.updateRuntimeHost)
	auth.POST("/runtime-hosts/:id/test", s.testRuntimeHost)
	auth.POST("/runtime-hosts/:id/inventory", s.inventoryRuntimeHost)
	auth.POST("/runtimes/:id/place", s.placeRuntime)
	auth.GET("/usage/resources", s.resourceUsage)
}

func (s *server) canAccessAdmin(userID int64) bool {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM role_bindings rb JOIN roles r ON r.id=rb.role_id
		WHERE rb.user_id=? AND r.name IN ('System Administrator','Security Administrator','Audit Administrator','Break-glass Super Administrator','Super Admin','Department Admin')`, userID).Scan(&count)
	return count > 0
}

func (s *server) requireAdminConsole(c *gin.Context) bool {
	if s.canAccessAdmin(currentUserID(c)) {
		return true
	}
	failCode(c, http.StatusForbidden, "admin.console_forbidden", gin.H{"path": "/admin"})
	return false
}

func (s *server) adminAccess(c *gin.Context) {
	if !s.requireAdminConsole(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"allowed": true, "console": "/admin"}})
}

func (s *server) requireWorkspaceUser(c *gin.Context) bool {
	var status string
	if s.db.QueryRow("SELECT status FROM users WHERE id=?", currentUserID(c)).Scan(&status) != nil || status != "active" {
		failCode(c, http.StatusForbidden, "workspace.user_inactive", gin.H{"status": status})
		return false
	}
	return true
}

func (s *server) workspaceMe(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	user, err := s.getUser(currentUserID(c))
	if err != nil {
		failCode(c, http.StatusNotFound, "workspace.user_not_found", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (s *server) workspacePermissions(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	rows, err := s.db.Query(`SELECT DISTINCT p.code,r.name,rb.scope,
		COALESCE(d.name,COALESCE(o.name,''))
		FROM role_bindings rb JOIN role_permissions rp ON rp.role_id=rb.role_id
		JOIN permissions p ON p.id=rp.permission_id JOIN roles r ON r.id=rb.role_id
		LEFT JOIN departments d ON d.id=rb.department_id
		LEFT JOIN organizations o ON o.id=rb.organization_id
		WHERE rb.user_id=? OR (rb.user_id IS NULL AND rb.organization_id=(SELECT organization_id FROM users WHERE id=?))
		ORDER BY p.code,r.name`, uid, uid)
	if err != nil {
		failCode(c, 500, "workspace.permissions_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var permission, role, scope, source string
		if rows.Scan(&permission, &role, &scope, &source) == nil {
			out = append(out, gin.H{"permission": permission, "source_role": role, "scope": scope, "binding_source": source})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) workspaceAgents(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	profiles := s.userProfilesData(uid)
	out := make([]gin.H, 0, len(profiles))
	for _, profile := range profiles {
		id, _ := profile["id"].(int64)
		profile["effective_configuration"] = s.effectiveConfigurationData(id)
		profile["assignment_sources"] = s.profileAssignmentSourcesData(id)
		out = append(out, profile)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) workspaceAgentDetail(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	if s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", id).Scan(&owner) != nil || owner != currentUserID(c) {
		failCode(c, http.StatusNotFound, "workspace.agent_not_found", nil)
		return
	}
	cfg := s.effectiveConfigurationData(id)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "configuration": cfg, "assignment_sources": s.profileAssignmentSourcesData(id), "executions": s.userProfileExecutions(id)}})
}

func (s *server) createPersonalAgent(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	if !s.selfServiceAllowed(currentUserID(c), "create_personal_profile", "") {
		failCode(c, http.StatusForbidden, "workspace.personal_profile_disabled", nil)
		return
	}
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		ModelID     int64  `json:"model_id"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.DisplayName) == "" {
		failCode(c, 400, "workspace.agent_invalid_request", nil)
		return
	}
	if req.ModelID > 0 && !s.selfServiceAllowed(currentUserID(c), "change_main_model", strconv.FormatInt(req.ModelID, 10)) {
		failCode(c, http.StatusForbidden, "workspace.model_change_disabled", nil)
		return
	}
	var orgID int64
	_ = s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", currentUserID(c)).Scan(&orgID)
	res, err := s.db.Exec(`INSERT INTO profiles(user_id,model_id,name,display_name,description,status,runtime_class,profile_type,managed,assignment_sources)
		VALUES(?,?,?,?,?,'active','standard','personal',FALSE,JSON_ARRAY())`, currentUserID(c), nullableID(req.ModelID), req.Name, req.DisplayName, req.Description)
	if err != nil {
		failCode(c, 409, "workspace.agent_create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditControlPlane(c, "profile.create", "Personal Agent Profile Created", "Agent Profiles", "profile", id, "success", nil, nil)
	_ = orgID
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "profile_type": "personal"}})
}

func (s *server) updatePersonalAgent(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	var managed bool
	if s.db.QueryRow("SELECT user_id,managed FROM profiles WHERE id=?", id).Scan(&owner, &managed) != nil || owner != currentUserID(c) {
		failCode(c, http.StatusNotFound, "workspace.agent_not_found", nil)
		return
	}
	if managed {
		failCode(c, http.StatusForbidden, "workspace.managed_agent_locked", nil)
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Status      string `json:"status"`
		ModelID     int64  `json:"model_id"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.DisplayName) == "" {
		failCode(c, 400, "workspace.agent_invalid_request", nil)
		return
	}
	if req.ModelID > 0 && !s.selfServiceAllowed(currentUserID(c), "change_main_model", strconv.FormatInt(req.ModelID, 10)) {
		failCode(c, http.StatusForbidden, "workspace.model_change_disabled", nil)
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Status != "active" && req.Status != "disabled" {
		failCode(c, 400, "workspace.agent_invalid_status", nil)
		return
	}
	if _, err := s.db.Exec("UPDATE profiles SET display_name=?,description=?,status=?,model_id=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.DisplayName, req.Description, req.Status, nullableID(req.ModelID), id); err != nil {
		failCode(c, 400, "workspace.agent_update_failed", nil)
		return
	}
	s.auditControlPlane(c, "profile.update", "Personal Agent Profile Updated", "Agent Profiles", "profile", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) deletePersonalAgent(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	var managed bool
	if s.db.QueryRow("SELECT user_id,managed FROM profiles WHERE id=?", id).Scan(&owner, &managed) != nil || owner != currentUserID(c) {
		failCode(c, http.StatusNotFound, "workspace.agent_not_found", nil)
		return
	}
	if managed {
		failCode(c, http.StatusForbidden, "workspace.managed_agent_locked", nil)
		return
	}
	if _, err := s.db.Exec("DELETE FROM profiles WHERE id=? AND user_id=?", id, owner); err != nil {
		failCode(c, 400, "workspace.agent_delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "profile.delete", "Personal Agent Profile Deleted", "Agent Profiles", "profile", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) workspaceModels(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	rows, err := s.db.Query(`SELECT m.id,m.name,m.display_name,m.provider,m.upstream_model,m.status,m.cost_class,m.data_classification,
		COALESCE(mp.name,''),COALESCE(pm.display_name,''),m.purpose
		FROM models m LEFT JOIN model_providers mp ON mp.id=m.provider_id LEFT JOIN provider_models pm ON pm.id=m.provider_model_id
		WHERE m.status='active' AND m.user_selectable=TRUE ORDER BY m.display_name`)
	if err != nil {
		failCode(c, 500, "workspace.models_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, display, provider, upstream, status, cost, classification, providerName, providerModel, purpose string
		if rows.Scan(&id, &name, &display, &provider, &upstream, &status, &cost, &classification, &providerName, &providerModel, &purpose) == nil {
			out = append(out, gin.H{"id": id, "name": name, "display_name": display, "provider": provider, "provider_name": providerName, "provider_model": providerModel, "upstream_model": upstream, "status": status, "cost_class": cost, "data_classification": classification, "purpose": purpose})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"models": out, "slots": hermesAuxiliarySlots, "policy": s.effectiveModelPolicy(currentUserID(c))}})
}

func (s *server) workspaceSkills(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	rows, err := s.db.Query(`SELECT DISTINCT sk.id,sk.name,sk.display_name,sk.category,sk.risk_level,sk.status
		FROM skills sk LEFT JOIN skill_assignments sa ON sa.skill_id=sk.id
		LEFT JOIN profiles p ON p.user_id=?
		WHERE sk.status IN ('published','active') AND (sa.id IS NULL OR sa.organization_id=(SELECT organization_id FROM users WHERE id=?) OR sa.department_id=(SELECT department_id FROM users WHERE id=?) OR sa.user_id=? OR sa.profile_id=p.id)
		ORDER BY sk.display_name`, currentUserID(c), currentUserID(c), currentUserID(c), currentUserID(c))
	if err != nil {
		failCode(c, 500, "workspace.skills_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, display, category, risk, status string
		if rows.Scan(&id, &name, &display, &category, &risk, &status) == nil {
			out = append(out, gin.H{"id": id, "name": name, "display_name": display, "category": category, "risk_level": risk, "status": status})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) workspaceKnowledge(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	values := map[int64]gin.H{}
	for _, profile := range s.userProfilesData(currentUserID(c)) {
		pid, _ := profile["id"].(int64)
		cfg := s.effectiveConfigurationData(pid)
		if sources, ok := cfg["knowledge"].([]gin.H); ok {
			for _, source := range sources {
				id, _ := source["id"].(int64)
				values[id] = source
			}
		}
	}
	out := []gin.H{}
	for _, source := range values {
		out = append(out, source)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) workspaceUsage(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	var input, output, requests, executions int64
	_ = s.db.QueryRow("SELECT COALESCE(SUM(token_input),0),COALESCE(SUM(token_output),0),COALESCE(SUM(requests),0),COALESCE(SUM(executions),0) FROM usage_events WHERE user_id=?", uid).Scan(&input, &output, &requests, &executions)
	var cost float64
	_ = s.db.QueryRow("SELECT COALESCE(SUM(cost),0) FROM executions WHERE user_id=?", uid).Scan(&cost)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"token_input": input, "token_output": output, "tokens": input + output, "requests": requests, "executions": executions, "cost": cost, "runtime": s.workspaceRuntimeData(uid)}})
}

func (s *server) workspaceNotifications(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	rows, err := s.db.Query("SELECT id,type,title,body,status,created_at,read_at FROM notifications WHERE user_id=? ORDER BY created_at DESC LIMIT 50", currentUserID(c))
	if err != nil {
		failCode(c, 500, "workspace.notifications_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var typ, title, body, status, created string
		var readAt sql.NullTime
		if rows.Scan(&id, &typ, &title, &body, &status, &created, &readAt) == nil {
			out = append(out, gin.H{"id": id, "type": typ, "title": title, "body": body, "status": status, "created_at": created, "read_at": nullableTime(readAt)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) readWorkspaceNotification(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	_, _ = s.db.Exec("UPDATE notifications SET status='read',read_at=UTC_TIMESTAMP() WHERE id=? AND user_id=?", id, currentUserID(c))
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) readAllWorkspaceNotifications(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	_, _ = s.db.Exec("UPDATE notifications SET status='read',read_at=UTC_TIMESTAMP() WHERE user_id=? AND status='unread'", currentUserID(c))
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) workspaceRuntimeData(uid int64) gin.H {
	var id int64
	var runtimeID, status, desired, observed, host string
	var hostID sql.NullInt64
	if s.db.QueryRow(`SELECT r.id,r.runtime_id,r.status,r.desired_status,r.observed_status,COALESCE(h.name,''),r.host_id FROM runtimes r LEFT JOIN runtime_hosts h ON h.id=r.host_id WHERE r.user_id=?`, uid).Scan(&id, &runtimeID, &status, &desired, &observed, &host, &hostID) != nil {
		return gin.H{"status": "not_provisioned"}
	}
	return gin.H{"id": id, "runtime_id": runtimeID, "status": status, "desired_status": desired, "observed_status": observed, "host": host, "host_id": nullableSQLID(hostID)}
}

func (s *server) workspaceSelfServicePolicy(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	result := gin.H{}
	for _, capability := range selfServiceCapabilities {
		mode, values := s.effectiveSelfServicePolicy(currentUserID(c), capability)
		result[capability] = gin.H{"mode": mode, "allowed_values": values}
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (s *server) effectiveSelfServicePolicy(uid int64, capability string) (string, []string) {
	var deptID, orgID int64
	_ = s.db.QueryRow("SELECT organization_id,COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&orgID, &deptID)
	rows, _ := s.db.Query(`SELECT scope,COALESCE(department_id,0),COALESCE(role_id,0),COALESCE(user_id,0),mode,allowed_values
		FROM user_self_service_policies WHERE organization_id=? AND capability=? AND (scope='organization' OR department_id=? OR role_id IN (SELECT role_id FROM role_bindings WHERE user_id=?) OR user_id=?)`, orgID, capability, nullableID(deptID), uid, uid)
	if rows == nil {
		return "disabled", []string{}
	}
	defer rows.Close()
	best := -1
	mode := "disabled"
	values := []string{}
	for rows.Next() {
		var scope, rowMode, raw string
		var rowDept, rowRole, rowUser int64
		if rows.Scan(&scope, &rowDept, &rowRole, &rowUser, &rowMode, &raw) != nil {
			continue
		}
		score := map[string]int{"organization": 1, "department": 2, "role": 3, "user": 4}[scope]
		if scope == "department" && rowDept != deptID || scope == "role" && rowRole == 0 || scope == "user" && rowUser != uid {
			continue
		}
		if score < best {
			continue
		}
		best = score
		mode = rowMode
		_ = json.Unmarshal([]byte(raw), &values)
	}
	return mode, values
}

func (s *server) selfServiceAllowed(uid int64, capability, value string) bool {
	mode, allowed := s.effectiveSelfServicePolicy(uid, capability)
	switch mode {
	case "allowed":
		return true
	case "whitelist":
		for _, candidate := range allowed {
			if candidate == value {
				return true
			}
		}
	}
	return false
}

func (s *server) profileAssignmentSourcesData(profileID int64) []gin.H {
	rows, _ := s.db.Query(`SELECT pas.source_type,pas.source_id,pas.source_label,pt.display_name,pas.created_at
		FROM profile_assignment_sources pas JOIN profile_templates pt ON pt.id=pas.template_id WHERE pas.profile_id=? ORDER BY pas.source_type,pas.source_label`, profileID)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var source, label, template, created string
		var id sql.NullInt64
		if rows.Scan(&source, &id, &label, &template, &created) == nil {
			out = append(out, gin.H{"source_type": source, "source_id": nullableSQLID(id), "source_label": label, "template": template, "created_at": created})
		}
	}
	return out
}

func (s *server) userProfileExecutions(profileID int64) []gin.H {
	rows, _ := s.db.Query(`SELECT e.execution_id,e.status,e.risk_level,e.risk_reason,e.created_at FROM executions e WHERE e.profile_id=? ORDER BY e.created_at DESC LIMIT 20`, profileID)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, status, risk, reason, created string
		if rows.Scan(&id, &status, &risk, &reason, &created) == nil {
			out = append(out, gin.H{"execution_id": id, "status": status, "risk_level": risk, "risk_reason": reason, "created_at": created})
		}
	}
	return out
}

func (s *server) listConversations(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	rows, err := s.db.Query(`SELECT cc.id,cc.profile_id,p.display_name,cc.title,cc.status,cc.created_at,cc.updated_at,
		(SELECT COUNT(*) FROM chat_messages cm WHERE cm.conversation_id=cc.id)
		FROM chat_conversations cc JOIN profiles p ON p.id=cc.profile_id WHERE cc.user_id=? ORDER BY cc.updated_at DESC`, currentUserID(c))
	if err != nil {
		failCode(c, 500, "workspace.conversations_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, profileID, count int64
		var profile, title, status, created, updated string
		if rows.Scan(&id, &profileID, &profile, &title, &status, &created, &updated, &count) == nil {
			out = append(out, gin.H{"id": id, "profile_id": profileID, "profile": profile, "title": title, "status": status, "message_count": count, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) createConversation(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	var req struct {
		ProfileID int64  `json:"profile_id"`
		Title     string `json:"title"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ProfileID == 0 {
		failCode(c, 400, "workspace.conversation_invalid_request", nil)
		return
	}
	var orgID int64
	if s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", currentUserID(c)).Scan(&orgID) != nil || s.profileOwnedBy(req.ProfileID, currentUserID(c)) == false {
		failCode(c, 404, "workspace.agent_not_found", nil)
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = "New conversation"
	}
	res, err := s.db.Exec("INSERT INTO chat_conversations(organization_id,user_id,profile_id,title) VALUES(?,?,?,?)", orgID, currentUserID(c), req.ProfileID, req.Title)
	if err != nil {
		failCode(c, 400, "workspace.conversation_create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "profile_id": req.ProfileID, "title": req.Title, "messages": []gin.H{}}})
}

func (s *server) profileOwnedBy(profileID, uid int64) bool {
	var owner int64
	return s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", profileID).Scan(&owner) == nil && owner == uid
}

func (s *server) conversationOwnedBy(conversationID, uid int64) bool {
	var owner int64
	return s.db.QueryRow("SELECT user_id FROM chat_conversations WHERE id=?", conversationID).Scan(&owner) == nil && owner == uid
}

func (s *server) listConversationMessages(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok || !s.conversationOwnedBy(id, currentUserID(c)) {
		failCode(c, 404, "workspace.conversation_not_found", nil)
		return
	}
	rows, err := s.db.Query("SELECT id,role,content,metadata,created_at FROM chat_messages WHERE conversation_id=? ORDER BY created_at,id", id)
	if err != nil {
		failCode(c, 500, "workspace.messages_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var messageID int64
		var role, content, metadata, created string
		if rows.Scan(&messageID, &role, &content, &metadata, &created) == nil {
			out = append(out, gin.H{"id": messageID, "role": role, "content": content, "metadata": phase3JSON(metadata), "created_at": created})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) createConversationMessage(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok || !s.conversationOwnedBy(id, currentUserID(c)) {
		failCode(c, 404, "workspace.conversation_not_found", nil)
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Content) == "" {
		failCode(c, 400, "workspace.message_required", nil)
		return
	}
	var profileName string
	if s.db.QueryRow("SELECT p.display_name FROM chat_conversations cc JOIN profiles p ON p.id=cc.profile_id WHERE cc.id=?", id).Scan(&profileName) != nil {
		failCode(c, 404, "workspace.conversation_not_found", nil)
		return
	}
	if _, err := s.db.Exec("INSERT INTO chat_messages(conversation_id,role,content,metadata) VALUES(?,?,?,JSON_OBJECT('provider','MockChatProvider'))", id, "user", req.Content); err != nil {
		failCode(c, 400, "workspace.message_create_failed", nil)
		return
	}
	reply := MockChatProvider{}.Reply(profileName, req.Content)
	_, _ = s.db.Exec("INSERT INTO chat_messages(conversation_id,role,content,metadata) VALUES(?,?,?,JSON_OBJECT('provider','MockChatProvider','simulated',true))", id, "assistant", reply)
	_, _ = s.db.Exec("UPDATE chat_conversations SET updated_at=UTC_TIMESTAMP() WHERE id=?", id)
	s.audit(c, currentUserID(c), "workspace.chat.message", "chat_conversation", id, "user", "success", nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"role": "assistant", "content": reply, "provider": "MockChatProvider"}})
}

func (s *server) workspaceChannels(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	var orgID int64
	_ = s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", currentUserID(c)).Scan(&orgID)
	rows, err := s.db.Query(`SELECT cp.channel_type,cp.enabled,cp.user_self_service,cp.user_credentials_allowed,
		COALESCE(cc.id,0),COALESCE(cc.profile_id,0),COALESCE(p.display_name,''),COALESCE(cc.status,'not_configured'),COALESCE(cc.credential_reference_id,0),cp.policy
		FROM channel_policies cp LEFT JOIN channel_connections cc ON cc.channel_type=cp.channel_type AND cc.organization_id=cp.organization_id AND cc.user_id=?
		LEFT JOIN profiles p ON p.id=cc.profile_id WHERE cp.organization_id=? ORDER BY cp.channel_type`, currentUserID(c), orgID)
	if err != nil {
		failCode(c, 500, "workspace.channels_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var channel, selfMode, status, policy string
		var profile string
		var enabled, credentials bool
		var connectionID, profileID, credentialID int64
		if rows.Scan(&channel, &enabled, &selfMode, &credentials, &connectionID, &profileID, &profile, &status, &credentialID, &policy) == nil {
			out = append(out, gin.H{"channel_type": channel, "enabled": enabled, "user_self_service": selfMode, "user_credentials_allowed": credentials, "connection_id": nullableID(connectionID), "profile_id": nullableID(profileID), "profile": profile, "status": status, "credential_reference_configured": credentialID > 0, "policy": phase3JSON(policy)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) createWorkspaceChannel(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	if !s.selfServiceAllowed(currentUserID(c), "configure_channel", "") {
		failCode(c, http.StatusForbidden, "workspace.channel_disabled", nil)
		return
	}
	var req struct {
		ChannelType           string `json:"channel_type"`
		ProfileID             int64  `json:"profile_id"`
		CredentialReferenceID int64  `json:"credential_reference_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ChannelType == "" {
		failCode(c, 400, "workspace.channel_invalid_request", nil)
		return
	}
	var orgID int64
	_ = s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", currentUserID(c)).Scan(&orgID)
	var enabled, credentials bool
	if s.db.QueryRow("SELECT enabled,user_credentials_allowed FROM channel_policies WHERE organization_id=? AND channel_type=?", orgID, req.ChannelType).Scan(&enabled, &credentials) != nil || !enabled || (req.CredentialReferenceID > 0 && !credentials) || (req.ProfileID > 0 && !s.profileOwnedBy(req.ProfileID, currentUserID(c))) {
		failCode(c, http.StatusForbidden, "workspace.channel_policy_denied", nil)
		return
	}
	res, err := s.db.Exec(`INSERT INTO channel_connections(organization_id,user_id,profile_id,channel_type,credential_reference_id,status,settings,created_by)
		VALUES(?,?,?,?,?,'connected',JSON_OBJECT('provider','MockChannelProvider'),?)
		ON DUPLICATE KEY UPDATE credential_reference_id=VALUES(credential_reference_id),status='connected',updated_at=UTC_TIMESTAMP()`, orgID, currentUserID(c), nullableID(req.ProfileID), req.ChannelType, nullableID(req.CredentialReferenceID), currentUserID(c))
	if err != nil {
		failCode(c, 409, "workspace.channel_save_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditControlPlane(c, "channel.connection.create", "Channel Connection Configured", "Models", "channel_connection", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "status": "connected", "provider": "MockChannelProvider"}})
}

func (s *server) updateWorkspaceChannel(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	if s.db.QueryRow("SELECT user_id FROM channel_connections WHERE id=?", id).Scan(&owner) != nil || owner != currentUserID(c) {
		failCode(c, 404, "workspace.channel_not_found", nil)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Status != "connected" && req.Status != "disconnected") {
		failCode(c, 400, "workspace.channel_invalid_status", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE channel_connections SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.Status, id)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) deleteWorkspaceChannel(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	if s.db.QueryRow("SELECT user_id FROM channel_connections WHERE id=?", id).Scan(&owner) != nil || owner != currentUserID(c) {
		failCode(c, 404, "workspace.channel_not_found", nil)
		return
	}
	_, _ = s.db.Exec("DELETE FROM channel_connections WHERE id=?", id)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

// effectiveModelPolicy is a compact representation used by Workspace model
// selectors. The full slot policy remains admin-managed in the database.
func (s *server) effectiveModelPolicy(uid int64) []gin.H {
	var orgID int64
	_ = s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", uid).Scan(&orgID)
	rows, _ := s.db.Query("SELECT slot,COALESCE(default_model_id,0),override_mode,allowed_models,allowed_providers FROM model_slot_policies WHERE organization_id=? ORDER BY slot", orgID)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var slot, mode, models, providers string
		var id int64
		if rows.Scan(&slot, &id, &mode, &models, &providers) == nil {
			out = append(out, gin.H{"slot": slot, "default_model_id": nullableID(id), "override_mode": mode, "allowed_models": phase3JSON(models), "allowed_providers": phase3JSON(providers)})
		}
	}
	return out
}

func (s *server) listProviderModels(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.read") {
		return
	}
	query := `SELECT pm.id,pm.provider_id,mp.name,pm.upstream_model,pm.display_name,pm.status,pm.sync_status,pm.last_sync_at FROM provider_models pm JOIN model_providers mp ON mp.id=pm.provider_id WHERE mp.organization_id=?`
	args := []any{s.currentOrg(c)}
	if provider := c.Query("provider_id"); provider != "" {
		query += " AND pm.provider_id=?"
		args = append(args, provider)
	}
	query += " ORDER BY mp.name,pm.display_name"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "provider_models.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, providerID int64
		var provider, upstream, display, status, syncStatus string
		var synced sql.NullTime
		if rows.Scan(&id, &providerID, &provider, &upstream, &display, &status, &syncStatus, &synced) == nil {
			out = append(out, gin.H{"id": id, "provider_id": providerID, "provider": provider, "upstream_model": upstream, "display_name": display, "status": status, "sync_status": syncStatus, "last_sync_at": nullableTime(synced)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) testModelProvider(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec("UPDATE model_providers SET health_status='healthy',last_tested_at=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?", id, s.currentOrg(c)); err != nil {
		failCode(c, 400, "provider.test_failed", nil)
		return
	}
	s.auditControlPlane(c, "model_provider.test", "Model Provider Tested", "Models", "model_provider", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "healthy", "provider": "MockModelProvider"}})
}

func (s *server) syncModelProvider(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var name string
	if s.db.QueryRow("SELECT name FROM model_providers WHERE id=? AND organization_id=?", id, s.currentOrg(c)).Scan(&name) != nil {
		failCode(c, 404, "provider.not_found", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE provider_models SET sync_status='mock',last_sync_at=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE provider_id=?", id)
	_, _ = s.db.Exec("UPDATE model_providers SET health_status='healthy',last_sync_at=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=?", id)
	s.auditControlPlane(c, "model_provider.sync", "Provider Models Synced", "Models", "model_provider", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"provider": name, "status": "synced", "provider_type": "MockModelProvider"}})
}

func (s *server) listModelSlotPolicies(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	rows, err := s.db.Query(`SELECT p.slot,COALESCE(p.default_model_id,0),COALESCE(m.display_name,''),p.override_mode,p.allowed_models,p.allowed_providers,p.updated_at
		FROM model_slot_policies p LEFT JOIN models m ON m.id=p.default_model_id WHERE p.organization_id=? ORDER BY p.slot`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "model_slot_policies.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var slot, model, mode, models, providers, updated string
		var id int64
		if rows.Scan(&slot, &id, &model, &mode, &models, &providers, &updated) == nil {
			out = append(out, gin.H{"slot": slot, "default_model_id": nullableID(id), "default_model": model, "override_mode": mode, "allowed_models": phase3JSON(models), "allowed_providers": phase3JSON(providers), "updated_at": updated})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "slots": hermesAuxiliarySlots})
}

func (s *server) updateModelSlotPolicy(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	slot := c.Param("slot")
	valid := slot == "main"
	for _, candidate := range hermesAuxiliarySlots {
		valid = valid || slot == candidate
	}
	if !valid {
		failCode(c, 400, "model_slot_policies.invalid_slot", gin.H{"slot": slot})
		return
	}
	var req struct {
		DefaultModelID   int64   `json:"default_model_id"`
		OverrideMode     string  `json:"override_mode"`
		AllowedModels    []int64 `json:"allowed_models"`
		AllowedProviders []int64 `json:"allowed_providers"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"admin_managed": true, "allowed": true, "whitelist": true}[req.OverrideMode] {
		failCode(c, 400, "model_slot_policies.invalid_request", nil)
		return
	}
	models, _ := json.Marshal(req.AllowedModels)
	providers, _ := json.Marshal(req.AllowedProviders)
	_, err := s.db.Exec(`INSERT INTO model_slot_policies(organization_id,slot,default_model_id,override_mode,allowed_models,allowed_providers,updated_by)
		VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE default_model_id=VALUES(default_model_id),override_mode=VALUES(override_mode),allowed_models=VALUES(allowed_models),allowed_providers=VALUES(allowed_providers),updated_by=VALUES(updated_by),updated_at=UTC_TIMESTAMP()`, s.currentOrg(c), slot, nullableID(req.DefaultModelID), req.OverrideMode, string(models), string(providers), currentUserID(c))
	if err != nil {
		failCode(c, 400, "model_slot_policies.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "model.policy.change", "Model Slot Policy Changed", "Models", "model_slot_policy", 0, "success", gin.H{"slot": slot}, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) listSelfServicePolicies(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	rows, err := s.db.Query(`SELECT p.id,p.scope,p.capability,p.mode,p.allowed_values,COALESCE(d.name,''),COALESCE(r.name,''),COALESCE(u.display_name,''),p.updated_at
		FROM user_self_service_policies p LEFT JOIN departments d ON d.id=p.department_id LEFT JOIN roles r ON r.id=p.role_id LEFT JOIN users u ON u.id=p.user_id WHERE p.organization_id=? ORDER BY p.capability,p.scope`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "self_service.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var scope, capability, mode, values, department, role, user, updated string
		if rows.Scan(&id, &scope, &capability, &mode, &values, &department, &role, &user, &updated) == nil {
			out = append(out, gin.H{"id": id, "scope": scope, "capability": capability, "mode": mode, "allowed_values": phase3JSON(values), "department": department, "role": role, "user": user, "updated_at": updated})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "capabilities": selfServiceCapabilities})
}

type selfServicePolicyRequest struct {
	Scope         string   `json:"scope"`
	DepartmentID  int64    `json:"department_id"`
	RoleID        int64    `json:"role_id"`
	UserID        int64    `json:"user_id"`
	Capability    string   `json:"capability"`
	Mode          string   `json:"mode"`
	AllowedValues []string `json:"allowed_values"`
}

func (s *server) saveSelfServicePolicy(c *gin.Context, id int64, req selfServicePolicyRequest) (int64, error) {
	validScope := map[string]bool{"organization": true, "department": true, "role": true, "user": true}[req.Scope]
	validMode := map[string]bool{"disabled": true, "allowed": true, "whitelist": true, "admin_managed": true}[req.Mode]
	validCapability := false
	for _, capability := range selfServiceCapabilities {
		validCapability = validCapability || capability == req.Capability
	}
	if !validScope || !validMode || !validCapability {
		return 0, fmt.Errorf("invalid self-service policy")
	}
	values, _ := json.Marshal(req.AllowedValues)
	if id == 0 {
		res, err := s.db.Exec(`INSERT INTO user_self_service_policies(organization_id,scope,department_id,role_id,user_id,capability,mode,allowed_values,updated_by) VALUES(?,?,?,?,?,?,?,?,?)`, s.currentOrg(c), req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.UserID), req.Capability, req.Mode, string(values), currentUserID(c))
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else {
		_, err := s.db.Exec(`UPDATE user_self_service_policies SET scope=?,department_id=?,role_id=?,user_id=?,capability=?,mode=?,allowed_values=?,updated_by=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.UserID), req.Capability, req.Mode, string(values), currentUserID(c), id, s.currentOrg(c))
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *server) createSelfServicePolicy(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	var req selfServicePolicyRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "self_service.invalid_request", nil)
		return
	}
	id, err := s.saveSelfServicePolicy(c, 0, req)
	if err != nil {
		failCode(c, 409, "self_service.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "security.self_service_policy.change", "Self-Service Policy Created", "Security", "self_service_policy", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

func (s *server) updateSelfServicePolicy(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req selfServicePolicyRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "self_service.invalid_request", nil)
		return
	}
	if _, err := s.saveSelfServicePolicy(c, id, req); err != nil {
		failCode(c, 400, "self_service.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "security.self_service_policy.change", "Self-Service Policy Updated", "Security", "self_service_policy", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) listChannelPolicies(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	rows, err := s.db.Query("SELECT id,channel_type,enabled,user_self_service,user_credentials_allowed,admin_managed,policy,updated_at FROM channel_policies WHERE organization_id=? ORDER BY channel_type", s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "channel_policies.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var channel, mode, policy, updated string
		var enabled, credentials, adminManaged bool
		if rows.Scan(&id, &channel, &enabled, &mode, &credentials, &adminManaged, &policy, &updated) == nil {
			out = append(out, gin.H{"id": id, "channel_type": channel, "enabled": enabled, "user_self_service": mode, "user_credentials_allowed": credentials, "admin_managed": adminManaged, "policy": phase3JSON(policy), "updated_at": updated})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

type channelPolicyRequest struct {
	ChannelType            string `json:"channel_type"`
	Enabled                bool   `json:"enabled"`
	UserSelfService        string `json:"user_self_service"`
	UserCredentialsAllowed bool   `json:"user_credentials_allowed"`
	AdminManaged           bool   `json:"admin_managed"`
}

func (s *server) saveChannelPolicy(c *gin.Context, id int64, req channelPolicyRequest) (int64, error) {
	if req.ChannelType == "" || !map[string]bool{"disabled": true, "allowed": true, "whitelist": true, "admin_managed": true}[req.UserSelfService] {
		return 0, fmt.Errorf("invalid channel policy")
	}
	if id == 0 {
		res, err := s.db.Exec(`INSERT INTO channel_policies(organization_id,channel_type,enabled,user_self_service,user_credentials_allowed,admin_managed,policy,created_by) VALUES(?,?,?,?,?,?,JSON_OBJECT('provider','MockChannelProvider'),?)`, s.currentOrg(c), req.ChannelType, req.Enabled, req.UserSelfService, req.UserCredentialsAllowed, req.AdminManaged, currentUserID(c))
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else {
		_, err := s.db.Exec("UPDATE channel_policies SET channel_type=?,enabled=?,user_self_service=?,user_credentials_allowed=?,admin_managed=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?", req.ChannelType, req.Enabled, req.UserSelfService, req.UserCredentialsAllowed, req.AdminManaged, id, s.currentOrg(c))
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *server) createChannelPolicy(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	var req channelPolicyRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "channel_policies.invalid_request", nil)
		return
	}
	id, err := s.saveChannelPolicy(c, 0, req)
	if err != nil {
		failCode(c, 409, "channel_policies.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "channel.policy.change", "Channel Policy Created", "Models", "channel_policy", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id}})
}

func (s *server) updateChannelPolicy(c *gin.Context) {
	if !s.requirePermission(c, "security.policy.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req channelPolicyRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "channel_policies.invalid_request", nil)
		return
	}
	if _, err := s.saveChannelPolicy(c, id, req); err != nil {
		failCode(c, 400, "channel_policies.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "channel.policy.change", "Channel Policy Updated", "Models", "channel_policy", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

type runtimeHostRequest struct {
	Name           string   `json:"name"`
	Hostname       string   `json:"hostname"`
	Address        string   `json:"address"`
	SSHPort        int      `json:"ssh_port"`
	AuthType       string   `json:"auth_type"`
	CredentialID   int64    `json:"credential_reference_id"`
	DockerEndpoint string   `json:"docker_endpoint"`
	CPUTotal       string   `json:"cpu_total"`
	MemoryTotal    string   `json:"memory_total"`
	StorageTotal   string   `json:"storage_total"`
	Labels         []string `json:"labels"`
}

func (s *server) listRuntimeHosts(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	rows, err := s.db.Query(`SELECT id,name,hostname,address,ssh_port,auth_type,credential_reference_id,docker_endpoint,docker_version,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,runtime_count,status,labels,last_seen,last_inventory_at FROM runtime_hosts WHERE organization_id=? ORDER BY name`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "runtime_hosts.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, port, count int64
		var name, hostname, address, authType, endpoint, dockerVersion, cpu, mem, storage, allocatedCPU, allocatedMem, allocatedStorage, status, labels string
		var credential sql.NullInt64
		var last, inventory sql.NullTime
		if rows.Scan(&id, &name, &hostname, &address, &port, &authType, &credential, &endpoint, &dockerVersion, &cpu, &mem, &storage, &allocatedCPU, &allocatedMem, &allocatedStorage, &count, &status, &labels, &last, &inventory) == nil {
			out = append(out, gin.H{"id": id, "name": name, "hostname": hostname, "address": address, "ssh_port": port, "auth_type": authType, "credential_reference_configured": credential.Valid, "docker_endpoint": endpoint, "docker_version": dockerVersion, "cpu_total": cpu, "memory_total": mem, "storage_total": storage, "cpu_allocated": allocatedCPU, "memory_allocated": allocatedMem, "storage_allocated": allocatedStorage, "runtime_count": count, "status": status, "labels": phase3JSON(labels), "last_seen": nullableTime(last), "last_inventory_at": nullableTime(inventory)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "provider": "MockRuntimeHostProvider", "security": gin.H{"docker_socket_exposed": false, "auth_type": "secret_reference"}})
}

func (s *server) saveRuntimeHost(c *gin.Context, id int64, req runtimeHostRequest) (int64, error) {
	if req.Name == "" || req.Hostname == "" || req.Address == "" {
		return 0, fmt.Errorf("name, hostname and address are required")
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.AuthType == "" {
		req.AuthType = "secret_reference"
	}
	if req.AuthType != "secret_reference" && req.AuthType != "reserved" {
		return 0, fmt.Errorf("unsupported auth type")
	}
	if req.DockerEndpoint == "" {
		req.DockerEndpoint = "mock://local-runtime-provider"
	}
	if !strings.HasPrefix(req.DockerEndpoint, "mock://") {
		return 0, fmt.Errorf("only mock runtime endpoints are supported in the demo")
	}
	if req.CPUTotal == "" {
		req.CPUTotal = "8 CPU"
	}
	if req.MemoryTotal == "" {
		req.MemoryTotal = "16 GB"
	}
	if req.StorageTotal == "" {
		req.StorageTotal = "200 GB"
	}
	labels, _ := json.Marshal(req.Labels)
	if id == 0 {
		res, err := s.db.Exec(`INSERT INTO runtime_hosts(organization_id,name,hostname,address,ssh_port,auth_type,credential_reference_id,docker_endpoint,cpu_total,memory_total,storage_total,labels,status,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'unknown',?)`, s.currentOrg(c), req.Name, req.Hostname, req.Address, req.SSHPort, req.AuthType, nullableID(req.CredentialID), req.DockerEndpoint, req.CPUTotal, req.MemoryTotal, req.StorageTotal, string(labels), currentUserID(c))
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else {
		_, err := s.db.Exec(`UPDATE runtime_hosts SET name=?,hostname=?,address=?,ssh_port=?,auth_type=?,credential_reference_id=?,docker_endpoint=?,cpu_total=?,memory_total=?,storage_total=?,labels=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, req.Name, req.Hostname, req.Address, req.SSHPort, req.AuthType, nullableID(req.CredentialID), req.DockerEndpoint, req.CPUTotal, req.MemoryTotal, req.StorageTotal, string(labels), id, s.currentOrg(c))
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (s *server) createRuntimeHost(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	var req runtimeHostRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime_hosts.invalid_request", nil)
		return
	}
	id, err := s.saveRuntimeHost(c, 0, req)
	if err != nil {
		failCode(c, 409, "runtime_hosts.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.create", "Runtime Host Created", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "status": "unknown"}})
}

func (s *server) updateRuntimeHost(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req runtimeHostRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime_hosts.invalid_request", nil)
		return
	}
	if _, err := s.saveRuntimeHost(c, id, req); err != nil {
		failCode(c, 400, "runtime_hosts.save_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.update", "Runtime Host Updated", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) testRuntimeHost(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec("UPDATE runtime_hosts SET status='healthy',docker_version='mock-docker-27',last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?", id, s.currentOrg(c)); err != nil {
		failCode(c, 400, "runtime_hosts.test_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.test", "Runtime Host Tested", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "healthy", "provider": "MockRuntimeHostProvider"}})
}

func (s *server) inventoryRuntimeHost(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec(`UPDATE runtime_hosts h SET h.runtime_count=(SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id),h.cpu_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id),' CPU'),h.memory_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id)*2,' GB'),h.storage_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id)*10,' GB'),h.last_inventory_at=UTC_TIMESTAMP(),h.last_seen=UTC_TIMESTAMP(),h.status='healthy',h.updated_at=UTC_TIMESTAMP() WHERE h.id=? AND h.organization_id=?`, id, s.currentOrg(c)); err != nil {
		failCode(c, 400, "runtime_hosts.inventory_failed", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "status": "healthy", "provider": "MockRuntimeHostProvider"}})
}

func (s *server) placeRuntime(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var hostID sql.NullInt64
	if s.db.QueryRow(`SELECT id FROM runtime_hosts WHERE organization_id=? AND status='healthy' ORDER BY runtime_count,id LIMIT 1`, s.currentOrg(c)).Scan(&hostID) != nil {
		failCode(c, http.StatusConflict, "runtime.placement_no_capacity", nil)
		return
	}
	if _, err := s.db.Exec(`UPDATE runtimes SET host_id=?,placement_status='placed',actual_cpu=cpu_limit,actual_memory=memory_limit,actual_storage=storage_limit,observed_image_version=image_version,updated_at=UTC_TIMESTAMP() WHERE id=?`, hostID.Int64, id); err != nil {
		failCode(c, 400, "runtime.placement_failed", nil)
		return
	}
	_, _ = s.db.Exec(`UPDATE runtime_hosts SET runtime_count=runtime_count+1,last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=?`, hostID.Int64)
	s.auditControlPlane(c, "runtime.place", "Runtime Placed", "Runtime", "runtime", id, "success", gin.H{"host_id": hostID.Int64, "scheduler": "MockScheduler"}, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"runtime_id": id, "host_id": hostID.Int64, "placement_status": "placed", "scheduler": "MockScheduler"}})
}

func (s *server) resourceUsage(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	var users, profiles, runtimes int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE organization_id=? AND status='active' AND system_account=FALSE", s.currentOrg(c)).Scan(&users)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM profiles p JOIN users u ON u.id=p.user_id WHERE u.organization_id=?", s.currentOrg(c)).Scan(&profiles)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM runtimes r JOIN users u ON u.id=r.user_id WHERE u.organization_id=?", s.currentOrg(c)).Scan(&runtimes)
	rows, _ := s.db.Query(`SELECT name,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,runtime_count,status FROM runtime_hosts WHERE organization_id=? ORDER BY name`, s.currentOrg(c))
	hosts := []gin.H{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, cpu, memory, storage, allocatedCPU, allocatedMemory, allocatedStorage, status string
			var count int
			if rows.Scan(&name, &cpu, &memory, &storage, &allocatedCPU, &allocatedMemory, &allocatedStorage, &count, &status) == nil {
				hosts = append(hosts, gin.H{"name": name, "cpu_total": cpu, "memory_total": memory, "storage_total": storage, "cpu_allocated": allocatedCPU, "memory_allocated": allocatedMemory, "storage_allocated": allocatedStorage, "runtime_count": count, "status": status})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"active_users": users, "profiles": profiles, "runtimes": runtimes, "hosts": hosts, "note": "AI usage and host capacity are separate accounting domains in the demo."}})
}

// Keep the compiler honest when a deployment uses a database driver returning
// nullable timestamps for newly added infrastructure fields.
var _ = time.RFC3339
