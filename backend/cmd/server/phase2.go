package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/hermes-enterprise-platform/backend/internal/providers"
	"github.com/gin-gonic/gin"
)

// RiskRule and RiskEvaluator deliberately remain deterministic in Phase 2. The
// evaluator is a seam for a future policy service, while keeping governance
// behavior explainable in the demo.
type RiskRule struct {
	Code   string
	Event  string
	Level  string
	Score  float64
	Reason string
}

type RiskResult struct {
	Level  string  `json:"risk_level"`
	Score  float64 `json:"risk_score"`
	Reason string  `json:"risk_reason"`
}

type RiskEvaluator struct{ rules []RiskRule }

func NewRiskEvaluator() RiskEvaluator {
	return RiskEvaluator{rules: []RiskRule{
		{Code: "break_glass_login", Event: "auth.break_glass.login", Level: "critical", Score: 100, Reason: "Break-glass administrator activity"},
		{Code: "self_elevation", Event: "role.self_elevate", Level: "critical", Score: 100, Reason: "Self elevation attempt"},
		{Code: "security_admin_change", Event: "security.role.change", Level: "critical", Score: 95, Reason: "Security administrator permission change"},
		{Code: "audit_policy_change", Event: "audit.policy.change", Level: "critical", Score: 95, Reason: "Audit policy change"},
		{Code: "quarantined_skill", Event: "skill.quarantined.install", Level: "critical", Score: 95, Reason: "Quarantined skill installation"},
		{Code: "critical_secret", Event: "secret.critical", Level: "critical", Score: 95, Reason: "Critical secret operation"},
		{Code: "role_elevation", Event: "role.assign", Level: "high", Score: 85, Reason: "High privilege role assignment"},
		{Code: "runtime_resize", Event: "runtime.resize", Level: "high", Score: 75, Reason: "High privilege runtime resource change"},
		{Code: "skill_publish", Event: "skill.publish", Level: "high", Score: 70, Reason: "Skill publication changes executable capability"},
		{Code: "model_security", Event: "model.security.change", Level: "high", Score: 70, Reason: "Model gateway security policy change"},
		{Code: "runtime_restart", Event: "runtime.restart", Level: "medium", Score: 45, Reason: "Runtime lifecycle change"},
		{Code: "knowledge_acl", Event: "knowledge.binding.create", Level: "medium", Score: 40, Reason: "Knowledge access policy change"},
		{Code: "profile_owner", Event: "profile.owner.change", Level: "medium", Score: 35, Reason: "Profile ownership change"},
	}}
}

func (e RiskEvaluator) Evaluate(action, resourceType string) RiskResult {
	for _, rule := range e.rules {
		if rule.Event == action {
			return RiskResult{Level: rule.Level, Score: rule.Score, Reason: rule.Reason}
		}
	}
	if strings.Contains(action, "delete") || strings.Contains(resourceType, "security") {
		return RiskResult{Level: "medium", Score: 40, Reason: "Destructive or security-sensitive operation"}
	}
	return RiskResult{Level: "low", Score: 10, Reason: "Read or routine control-plane operation"}
}

func registerPhase2Routes(auth *gin.RouterGroup, s *server) {
	// Organization and RBAC
	auth.GET("/users/:id/roles", s.userRoles)
	auth.POST("/users/:id/roles", s.assignUserRole)
	auth.DELETE("/users/:id/roles/:role_id", s.removeUserRole)
	auth.GET("/users/:id/effective-permissions", s.effectivePermissions)
	auth.GET("/roles/manage", s.manageRoles)
	auth.POST("/roles/manage", s.createRole)
	auth.GET("/roles/:id", s.roleDetail)
	auth.PUT("/roles/:id", s.updateRole)
	auth.PUT("/roles/:id/permissions", s.updateRolePermissions)
	auth.GET("/roles/:id/history", s.roleHistory)

	// Runtime and profile governance
	auth.GET("/runtimes-v2", s.listRuntimesV2)
	auth.GET("/profiles-v2", s.listProfilesV2)
	auth.GET("/runtime-templates", s.listRuntimeTemplates)
	auth.POST("/runtime-templates", s.createRuntimeTemplate)
	auth.PUT("/runtime-templates/:id", s.updateRuntimeTemplate)
	auth.POST("/runtime-templates/:id/status", s.setRuntimeTemplateStatus)
	auth.PUT("/runtimes/:id", s.updateRuntimeSpec)
	auth.POST("/runtimes/provision", s.provisionRuntime)
	auth.GET("/profile-templates", s.listProfileTemplates)
	auth.POST("/profile-templates", s.createProfileTemplate)
	auth.PUT("/profile-templates/:id", s.updateProfileTemplate)
	auth.POST("/profile-templates/:id/bindings", s.bindProfileTemplate)

	// Model access and secret references
	auth.GET("/model-providers", s.listModelProviders)
	auth.POST("/model-providers", s.createModelProvider)
	auth.PUT("/model-providers/:id", s.updateModelProvider)
	auth.GET("/secrets", s.listSecrets)
	auth.POST("/secrets", s.createSecretReference)

	// Versioned Skill Registry
	auth.GET("/skills/:id/detail", s.skillDetail)
	auth.GET("/skills/:id/versions", s.skillVersions)
	auth.POST("/skills/:id/versions", s.createSkillVersion)
	auth.GET("/skill-versions/:id/files", s.skillFiles)
	auth.POST("/skill-versions/:id/files", s.upsertSkillFile)
	auth.PUT("/skill-artifact-files/:id", s.updateSkillFile)
	auth.DELETE("/skill-artifact-files/:id", s.deleteSkillFile)
	auth.POST("/skill-versions/:id/submit", s.submitSkillVersion)
	auth.POST("/skill-versions/:id/publish", s.publishSkillVersion)

	// Knowledge content system
	auth.GET("/knowledge-bases/:id/detail", s.knowledgeDetail)
	auth.GET("/knowledge-bases/:id/documents", s.listDocuments)
	auth.POST("/knowledge-bases/:id/documents", s.createDocument)
	auth.GET("/knowledge-bases/:id/bindings", s.listKnowledgeBindings)
	auth.POST("/knowledge-bases/:id/bindings/v2", s.createKnowledgeBindingV2)
	auth.GET("/knowledge-documents/:id", s.documentDetail)
	auth.PUT("/knowledge-documents/:id", s.updateDocument)
	auth.POST("/knowledge-documents/:id/publish", s.publishDocument)
	auth.DELETE("/knowledge-documents/:id", s.deleteDocument)
	auth.GET("/knowledge-documents/:id/versions", s.documentVersions)
	auth.GET("/knowledge-document-versions/:id", s.documentVersion)
	auth.GET("/profiles/:id/knowledge-sources", s.effectiveKnowledge)

	// Governance, audit, settings and system surfaces
	auth.GET("/audit-logs/query", s.phase2AuditLogs)
	auth.GET("/audit-logs/export", s.exportAuditLogs)
	auth.GET("/risk-events", s.listRiskEvents)
	auth.POST("/risk-events/:id/status", s.setRiskEventStatus)
	auth.GET("/approval-requests", s.listApprovalRequests)
	auth.POST("/approval-requests", s.createApprovalRequest)
	auth.POST("/approval-requests/:id/decision", s.decideApproval)
	auth.GET("/settings", s.getSettings)
	auth.PUT("/settings", s.updateSettings)
	auth.GET("/system-health", s.systemHealth)
	auth.GET("/notifications", s.listNotifications)
	auth.POST("/notifications/:id/read", s.readNotification)
	auth.GET("/quotas", s.listQuotas)
	auth.POST("/quotas", s.createQuota)
	auth.PUT("/quotas/:id", s.updateQuota)
	auth.GET("/dashboard/phase2", s.phase2Dashboard)
}

func failCode(c *gin.Context, status int, code string, params gin.H) {
	c.AbortWithStatusJSON(status, gin.H{"error_code": code, "message_params": params})
}

func intQuery(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return v
}

func validScope(scope string) bool {
	return scope == "global" || scope == "organization" || scope == "department" || scope == "user" || scope == "profile" || scope == "role"
}

func (s *server) hasRole(userID int64, names ...string) bool {
	if len(names) == 0 {
		return false
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(names)), ",")
	args := []any{userID}
	for _, name := range names {
		args = append(args, name)
	}
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM role_bindings rb JOIN roles r ON r.id=rb.role_id WHERE rb.user_id=? AND r.name IN ("+placeholders+")", args...).Scan(&count)
	return count > 0
}

func (s *server) isBreakglass(userID int64) bool {
	return s.hasRole(userID, "Break-glass Super Administrator", "Super Admin")
}

func (s *server) protectedRole(roleID int64) (bool, string) {
	var name string
	if err := s.db.QueryRow("SELECT name FROM roles WHERE id=?", roleID).Scan(&name); err != nil {
		return false, ""
	}
	return name == "System Administrator" || name == "Security Administrator" || name == "Audit Administrator" || name == "Break-glass Super Administrator" || name == "Super Admin", name
}

func (s *server) userRoles(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	uid, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT r.id,r.name,r.description,rb.scope,COALESCE(d.name,''),rb.created_at
		FROM role_bindings rb JOIN roles r ON r.id=rb.role_id LEFT JOIN departments d ON d.id=rb.department_id
		WHERE rb.user_id=? ORDER BY r.name`, uid)
	if err != nil {
		failCode(c, 500, "roles.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, desc, scope, dept, created string
		if err := rows.Scan(&id, &name, &desc, &scope, &dept, &created); err != nil {
			continue
		}
		out = append(out, gin.H{"id": id, "name": name, "description": desc, "scope": scope, "department": dept, "created_at": created})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) assignUserRole(c *gin.Context) {
	if !s.requirePermission(c, "role.assign") {
		return
	}
	uid, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		RoleID       int64  `json:"role_id"`
		Scope        string `json:"scope"`
		DepartmentID int64  `json:"department_id"`
		ProfileID    int64  `json:"profile_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.RoleID == 0 {
		failCode(c, 400, "role.invalid_request", nil)
		return
	}
	if req.Scope == "" {
		req.Scope = "user"
	}
	if !validScope(req.Scope) {
		failCode(c, 400, "role.invalid_scope", gin.H{"scope": req.Scope})
		return
	}
	protected, roleName := s.protectedRole(req.RoleID)
	if uid == currentUserID(c) && protected {
		s.auditPhase2(c, "role.self_elevate", "role", req.RoleID, "user", "denied", gin.H{"role": roleName})
		failCode(c, 403, "role.self_elevation_denied", nil)
		return
	}
	if protected && !s.isBreakglass(currentUserID(c)) {
		requestID, err := s.createApproval(c, "role_elevation", "role", req.RoleID, "high", "Protected role assignment requires independent approval", gin.H{"target_user_id": uid, "role_id": req.RoleID, "scope": req.Scope, "department_id": req.DepartmentID, "profile_id": req.ProfileID})
		if err != nil {
			failCode(c, 500, "approval.create_failed", nil)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "pending", "approval_request_id": requestID}})
		return
	}
	if _, err := s.db.Exec(`INSERT INTO role_bindings(role_id,organization_id,department_id,user_id,profile_id,scope) VALUES (?,1,?,?,?,?,?)`, req.RoleID, nullableID(req.DepartmentID), uid, nullableID(req.ProfileID), req.Scope); err != nil {
		failCode(c, 409, "role.assign_failed", nil)
		return
	}
	s.consolidateManagedAgents(uid)
	s.auditPhase2(c, "role.assign", "user", uid, req.Scope, "success", gin.H{"role_id": req.RoleID, "role": roleName})
	c.JSON(201, gin.H{"data": gin.H{"user_id": uid, "role_id": req.RoleID, "role": roleName, "scope": req.Scope}})
}

func (s *server) removeUserRole(c *gin.Context) {
	if !s.requirePermission(c, "role.assign") {
		return
	}
	uid, ok := paramID(c, "id")
	if !ok {
		return
	}
	rid, ok := paramID(c, "role_id")
	if !ok {
		return
	}
	protected, roleName := s.protectedRole(rid)
	if uid == currentUserID(c) && protected {
		s.auditPhase2(c, "role.self_elevate", "role", rid, "user", "denied", gin.H{"operation": "remove", "role": roleName})
		failCode(c, 403, "role.self_change_denied", nil)
		return
	}
	if _, err := s.db.Exec("DELETE FROM role_bindings WHERE user_id=? AND role_id=?", uid, rid); err != nil {
		failCode(c, 400, "role.remove_failed", nil)
		return
	}
	s.consolidateManagedAgents(uid)
	s.auditPhase2(c, "role.remove", "user", uid, "user", "success", gin.H{"role_id": rid, "role": roleName, "protected": protected})
	c.JSON(200, gin.H{"data": true})
}

func (s *server) effectivePermissions(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	uid, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT DISTINCT p.code,r.name,rb.scope,COALESCE(d.name,''),COALESCE(pr.name,'')
		FROM role_bindings rb JOIN roles r ON r.id=rb.role_id JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id
		LEFT JOIN departments d ON d.id=rb.department_id LEFT JOIN profiles pr ON pr.id=rb.profile_id WHERE rb.user_id=? OR (rb.user_id IS NULL AND rb.organization_id=(SELECT organization_id FROM users WHERE id=?)) ORDER BY p.code,r.name`, uid, uid)
	if err != nil {
		failCode(c, 500, "permissions.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var permission, role, scope, dept, profile string
		if rows.Scan(&permission, &role, &scope, &dept, &profile) == nil {
			source := any(role)
			if dept != "" {
				source = dept
			}
			if profile != "" {
				source = profile
			}
			out = append(out, gin.H{"permission": permission, "source_role": role, "scope": scope, "source_binding": source})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) manageRoles(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	rows, err := s.db.Query(`SELECT r.id,r.name,r.description,r.is_system,COUNT(DISTINCT rb.user_id),COUNT(DISTINCT rp.permission_id),r.created_at FROM roles r LEFT JOIN role_bindings rb ON rb.role_id=r.id LEFT JOIN role_permissions rp ON rp.role_id=r.id GROUP BY r.id ORDER BY r.name`)
	if err != nil {
		failCode(c, 500, "roles.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, members, perms int64
		var name, desc, created string
		var sys bool
		if rows.Scan(&id, &name, &desc, &sys, &members, &perms, &created) == nil {
			out = append(out, gin.H{"id": id, "name": name, "description": desc, "is_system": sys, "members": members, "permission_count": perms, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createRole(c *gin.Context) {
	if !s.requirePermission(c, "role.manage") {
		return
	}
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		failCode(c, 400, "role.invalid_request", nil)
		return
	}
	res, err := s.db.Exec("INSERT INTO roles(organization_id,name,description,is_system) VALUES (1,?,?,0)", req.Name, req.Description)
	if err != nil {
		failCode(c, 409, "role.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	if err := s.setRolePermissions(id, req.Permissions); err != nil {
		failCode(c, 400, "role.permission_update_failed", nil)
		return
	}
	s.auditPhase2(c, "role.create", "role", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "name": req.Name}})
}

func (s *server) roleDetail(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var name, desc, created string
	var sys bool
	if err := s.db.QueryRow("SELECT name,description,is_system,created_at FROM roles WHERE id=?", id).Scan(&name, &desc, &sys, &created); err != nil {
		failCode(c, 404, "role.not_found", nil)
		return
	}
	perms := []string{}
	rows, _ := s.db.Query("SELECT p.code FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.code", id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var x string
			if rows.Scan(&x) == nil {
				perms = append(perms, x)
			}
		}
	}
	members := []gin.H{}
	memberRows, _ := s.db.Query("SELECT u.id,u.display_name,u.username,rb.scope FROM role_bindings rb JOIN users u ON u.id=rb.user_id WHERE rb.role_id=? ORDER BY u.display_name", id)
	if memberRows != nil {
		defer memberRows.Close()
		for memberRows.Next() {
			var memberID int64
			var display, username, scope string
			if memberRows.Scan(&memberID, &display, &username, &scope) == nil {
				members = append(members, gin.H{"id": memberID, "display_name": display, "username": username, "scope": scope})
			}
		}
	}
	scopes := []string{}
	scopeRows, _ := s.db.Query("SELECT DISTINCT scope FROM role_bindings WHERE role_id=? ORDER BY scope", id)
	if scopeRows != nil {
		defer scopeRows.Close()
		for scopeRows.Next() {
			var scope string
			if scopeRows.Scan(&scope) == nil {
				scopes = append(scopes, scope)
			}
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "description": desc, "is_system": sys, "permissions": perms, "members": members, "scopes": scopes, "created_at": created}})
}

func (s *server) updateRole(c *gin.Context) {
	if !s.requirePermission(c, "role.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name, Description string `json:"name"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "role.invalid_request", nil)
		return
	}
	before := s.roleState(id)
	if _, err := s.db.Exec("UPDATE roles SET name=?,description=? WHERE id=?", req.Name, req.Description, id); err != nil {
		failCode(c, 400, "role.update_failed", nil)
		return
	}
	after := s.roleState(id)
	s.recordChange("role", id, before, after, currentUserID(c))
	s.auditPhase2(c, "role.update", "role", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": after})
}

func (s *server) updateRolePermissions(c *gin.Context) {
	if !s.requirePermission(c, "role.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Permissions []string `json:"permissions"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "role.invalid_request", nil)
		return
	}
	protected, _ := s.protectedRole(id)
	if protected && !s.isBreakglass(currentUserID(c)) {
		_, _ = s.createApproval(c, "high_risk_permission_change", "role", id, "critical", "Protected role permission change", gin.H{"permissions": req.Permissions})
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "pending"}})
		return
	}
	before := s.roleState(id)
	if err := s.setRolePermissions(id, req.Permissions); err != nil {
		failCode(c, 400, "role.permission_update_failed", nil)
		return
	}
	after := s.roleState(id)
	s.recordChange("role", id, before, after, currentUserID(c))
	s.auditPhase2(c, "security.role.change", "role", id, "global", "success", nil)
	c.JSON(200, gin.H{"data": after})
}

func (s *server) setRolePermissions(roleID int64, codes []string) error {
	if _, err := s.db.Exec("DELETE FROM role_permissions WHERE role_id=?", roleID); err != nil {
		return err
	}
	for _, code := range codes {
		var pid int64
		if err := s.db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&pid); err != nil {
			continue
		}
		if _, err := s.db.Exec("INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES (?,?)", roleID, pid); err != nil {
			return err
		}
	}
	return nil
}
func (s *server) roleState(id int64) gin.H {
	var n, d string
	var sys bool
	_ = s.db.QueryRow("SELECT name,description,is_system FROM roles WHERE id=?", id).Scan(&n, &d, &sys)
	return gin.H{"name": n, "description": d, "is_system": sys}
}

func (s *server) roleHistory(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	s.changeHistory(c, "role", id)
}

func (s *server) listRuntimeTemplates(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.read") {
		return
	}
	rows, err := s.db.Query(`SELECT id,name,description,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,is_default,status,created_at,updated_at FROM runtime_templates WHERE organization_id=1 ORDER BY name`)
	if err != nil {
		failCode(c, 500, "runtime_templates.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, pl, mj int64
		var name, desc, cpu, mem, storage, image, provider, class, network, status, created, updated string
		var start, suspend, def bool
		if rows.Scan(&id, &name, &desc, &cpu, &mem, &storage, &pl, &mj, &image, &provider, &class, &network, &start, &suspend, &def, &status, &created, &updated) == nil {
			out = append(out, gin.H{"id": id, "name": name, "description": desc, "cpu_limit": cpu, "memory_limit": mem, "storage_limit": storage, "profile_limit": pl, "max_concurrent_jobs": mj, "image_version": image, "runtime_provider": provider, "runtime_class": class, "network_policy": network, "auto_start": start, "auto_suspend": suspend, "is_default": def, "status": status, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

type runtimeTemplateRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	CPULimit          string `json:"cpu_limit"`
	MemoryLimit       string `json:"memory_limit"`
	StorageLimit      string `json:"storage_limit"`
	ImageVersion      string `json:"image_version"`
	RuntimeProvider   string `json:"runtime_provider"`
	RuntimeClass      string `json:"runtime_class"`
	NetworkPolicy     string `json:"network_policy"`
	ProfileLimit      int    `json:"profile_limit"`
	MaxConcurrentJobs int    `json:"max_concurrent_jobs"`
	AutoStart         bool   `json:"auto_start"`
	AutoSuspend       bool   `json:"auto_suspend"`
	IsDefault         bool   `json:"is_default"`
}

func (s *server) createRuntimeTemplate(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
		return
	}
	var req runtimeTemplateRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "runtime_template.invalid_request", nil)
		return
	}
	if req.CPULimit == "" {
		req.CPULimit = "1 CPU"
	}
	if req.MemoryLimit == "" {
		req.MemoryLimit = "1 GB"
	}
	if req.StorageLimit == "" {
		req.StorageLimit = "10 GB"
	}
	if req.ProfileLimit == 0 {
		req.ProfileLimit = 5
	}
	if req.MaxConcurrentJobs == 0 {
		req.MaxConcurrentJobs = 2
	}
	if req.ImageVersion == "" {
		req.ImageVersion = "mock-hermes-0.2"
	}
	if req.RuntimeProvider == "" {
		req.RuntimeProvider = "mock"
	}
	if req.RuntimeClass == "" {
		req.RuntimeClass = "standard"
	}
	if req.NetworkPolicy == "" {
		req.NetworkPolicy = "restricted"
	}
	res, err := s.db.Exec(`INSERT INTO runtime_templates(organization_id,name,description,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,is_default,status,created_by) VALUES(1,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'active',?)`, req.Name, req.Description, req.CPULimit, req.MemoryLimit, req.StorageLimit, req.ProfileLimit, req.MaxConcurrentJobs, req.ImageVersion, req.RuntimeProvider, req.RuntimeClass, req.NetworkPolicy, req.AutoStart, req.AutoSuspend, req.IsDefault, currentUserID(c))
	if err != nil {
		failCode(c, 409, "runtime_template.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	if req.IsDefault {
		_, _ = s.db.Exec("UPDATE runtime_templates SET is_default=FALSE WHERE organization_id=1 AND id<>?", id)
	}
	s.auditPhase2(c, "runtime_template.create", "runtime_template", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}
func (s *server) updateRuntimeTemplate(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req runtimeTemplateRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "runtime_template.invalid_request", nil)
		return
	}
	before := s.runtimeTemplateState(id)
	_, err := s.db.Exec(`UPDATE runtime_templates SET name=?,description=?,cpu_limit=?,memory_limit=?,storage_limit=?,profile_limit=?,max_concurrent_jobs=?,image_version=?,runtime_provider=?,runtime_class=?,network_policy=?,auto_start=?,auto_suspend=?,is_default=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.Description, req.CPULimit, req.MemoryLimit, req.StorageLimit, req.ProfileLimit, req.MaxConcurrentJobs, req.ImageVersion, req.RuntimeProvider, req.RuntimeClass, req.NetworkPolicy, req.AutoStart, req.AutoSuspend, req.IsDefault, id)
	if err != nil {
		failCode(c, 400, "runtime_template.update_failed", nil)
		return
	}
	if req.IsDefault {
		_, _ = s.db.Exec("UPDATE runtime_templates SET is_default=FALSE WHERE organization_id=1 AND id<>?", id)
	}
	after := s.runtimeTemplateState(id)
	s.recordChange("runtime_template", id, before, after, currentUserID(c))
	s.auditPhase2(c, "runtime_template.update", "runtime_template", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": after})
}
func (s *server) setRuntimeTemplateStatus(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
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
		failCode(c, 400, "runtime_template.invalid_status", nil)
		return
	}
	_, err := s.db.Exec("UPDATE runtime_templates SET status=? WHERE id=?", req.Status, id)
	if err != nil {
		failCode(c, 400, "runtime_template.status_failed", nil)
		return
	}
	s.auditPhase2(c, "runtime_template.status", "runtime_template", id, "organization", "success", gin.H{"status": req.Status})
	c.JSON(200, gin.H{"data": true})
}
func (s *server) runtimeTemplateState(id int64) gin.H {
	var n, d, cpu, mem, storage, img, prov, class, net, status string
	var pl, mj int
	var a, b, def bool
	_ = s.db.QueryRow("SELECT name,description,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,is_default,status FROM runtime_templates WHERE id=?", id).Scan(&n, &d, &cpu, &mem, &storage, &pl, &mj, &img, &prov, &class, &net, &a, &b, &def, &status)
	return gin.H{"name": n, "description": d, "cpu_limit": cpu, "memory_limit": mem, "storage_limit": storage, "profile_limit": pl, "max_concurrent_jobs": mj, "image_version": img, "runtime_provider": prov, "runtime_class": class, "network_policy": net, "auto_start": a, "auto_suspend": b, "is_default": def, "status": status}
}

func (s *server) provisionRuntime(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	var req struct {
		UserID       int64  `json:"user_id"`
		TemplateID   int64  `json:"template_id"`
		CPULimit     string `json:"cpu_limit"`
		MemoryLimit  string `json:"memory_limit"`
		StorageLimit string `json:"storage_limit"`
	}
	if c.ShouldBindJSON(&req) != nil || req.UserID == 0 {
		failCode(c, 400, "runtime.invalid_request", nil)
		return
	}
	if req.TemplateID == 0 {
		_ = s.db.QueryRow("SELECT id FROM runtime_templates WHERE organization_id=1 AND is_default=TRUE AND status='active' LIMIT 1").Scan(&req.TemplateID)
	}
	if req.TemplateID > 0 {
		_ = s.db.QueryRow("SELECT cpu_limit,memory_limit,storage_limit FROM runtime_templates WHERE id=?", req.TemplateID).Scan(&req.CPULimit, &req.MemoryLimit, &req.StorageLimit)
	}
	if req.CPULimit == "" {
		req.CPULimit = "1 CPU"
	}
	if req.MemoryLimit == "" {
		req.MemoryLimit = "1 GB"
	}
	if req.StorageLimit == "" {
		req.StorageLimit = "10 GB"
	}
	var id int64
	if err := s.db.QueryRow("SELECT id FROM runtimes WHERE user_id=?", req.UserID).Scan(&id); err != nil {
		res, e := s.db.Exec(`INSERT INTO runtimes(user_id,runtime_id,status,provider,hermes_version,cpu_limit,memory_limit,template_id,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend) VALUES(?,?, 'running','mock','mock-hermes-0.2',?,?,?,?,'5',2,'mock-hermes-0.2','mock','standard','restricted',TRUE,FALSE)`, req.UserID, fmt.Sprintf("mock-runtime-%d", req.UserID), req.CPULimit, req.MemoryLimit, req.TemplateID, req.StorageLimit)
		if e != nil {
			failCode(c, 400, "runtime.provision_failed", nil)
			return
		}
		id, _ = res.LastInsertId()
	} else {
		_, _ = s.db.Exec("UPDATE runtimes SET status='running',template_id=?,cpu_limit=?,memory_limit=?,storage_limit=?,last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=?", nullableID(req.TemplateID), req.CPULimit, req.MemoryLimit, req.StorageLimit, id)
	}
	s.assignProfileTemplates(req.UserID)
	s.auditPhase2(c, "runtime.provision", "runtime", id, "user", "success", gin.H{"user_id": req.UserID, "template_id": req.TemplateID})
	c.JSON(201, gin.H{"data": gin.H{"id": id, "status": "running", "user_id": req.UserID}})
}

func (s *server) updateRuntimeSpec(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		TemplateID        int64  `json:"template_id"`
		CPULimit          string `json:"cpu_limit"`
		MemoryLimit       string `json:"memory_limit"`
		StorageLimit      string `json:"storage_limit"`
		ImageVersion      string `json:"image_version"`
		RuntimeProvider   string `json:"runtime_provider"`
		RuntimeClass      string `json:"runtime_class"`
		NetworkPolicy     string `json:"network_policy"`
		ProfileLimit      int    `json:"profile_limit"`
		MaxConcurrentJobs int    `json:"max_concurrent_jobs"`
		AutoStart         *bool  `json:"auto_start"`
		AutoSuspend       *bool  `json:"auto_suspend"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime.invalid_request", nil)
		return
	}
	before := s.runtimeState(id)
	if req.TemplateID == 0 {
		if v, ok := before["template_id"].(int64); ok {
			req.TemplateID = v
		}
	}
	if req.CPULimit == "" {
		req.CPULimit, _ = before["cpu_limit"].(string)
	}
	if req.MemoryLimit == "" {
		req.MemoryLimit, _ = before["memory_limit"].(string)
	}
	if req.StorageLimit == "" {
		req.StorageLimit, _ = before["storage_limit"].(string)
	}
	if req.ProfileLimit == 0 {
		if v, ok := before["profile_limit"].(int); ok {
			req.ProfileLimit = v
		}
	}
	if req.MaxConcurrentJobs == 0 {
		if v, ok := before["max_concurrent_jobs"].(int); ok {
			req.MaxConcurrentJobs = v
		}
	}
	if req.ImageVersion == "" {
		req.ImageVersion, _ = before["image_version"].(string)
	}
	if req.RuntimeProvider == "" {
		req.RuntimeProvider, _ = before["runtime_provider"].(string)
	}
	if req.RuntimeClass == "" {
		req.RuntimeClass, _ = before["runtime_class"].(string)
	}
	if req.NetworkPolicy == "" {
		req.NetworkPolicy, _ = before["network_policy"].(string)
	}
	autoStart, _ := before["auto_start"].(bool)
	autoSuspend, _ := before["auto_suspend"].(bool)
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}
	if req.AutoSuspend != nil {
		autoSuspend = *req.AutoSuspend
	}
	if before["cpu_limit"] != req.CPULimit || before["memory_limit"] != req.MemoryLimit {
		if !s.isBreakglass(currentUserID(c)) {
			_, _ = s.createApproval(c, "runtime_resource_increase", "runtime", id, "high", "Runtime resource change requires approval", gin.H{"cpu_limit": req.CPULimit, "memory_limit": req.MemoryLimit})
			c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "pending"}})
			return
		}
	}
	_, err := s.db.Exec(`UPDATE runtimes SET template_id=?,cpu_limit=?,memory_limit=?,storage_limit=?,profile_limit=?,max_concurrent_jobs=?,image_version=?,runtime_provider=?,runtime_class=?,network_policy=?,auto_start=?,auto_suspend=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, nullableID(req.TemplateID), req.CPULimit, req.MemoryLimit, req.StorageLimit, req.ProfileLimit, req.MaxConcurrentJobs, req.ImageVersion, req.RuntimeProvider, req.RuntimeClass, req.NetworkPolicy, autoStart, autoSuspend, id)
	if err != nil {
		failCode(c, 400, "runtime.update_failed", nil)
		return
	}
	after := s.runtimeState(id)
	s.recordChange("runtime", id, before, after, currentUserID(c))
	s.auditPhase2(c, "runtime.resize", "runtime", id, "user", "success", nil)
	c.JSON(200, gin.H{"data": after})
}
func (s *server) runtimeState(id int64) gin.H {
	var t int64
	var cpu, mem, storage, img, prov, class, net, status string
	var pl, mj int
	var a, b bool
	_ = s.db.QueryRow("SELECT template_id,cpu_limit,memory_limit,storage_limit,profile_limit,max_concurrent_jobs,image_version,runtime_provider,runtime_class,network_policy,auto_start,auto_suspend,status FROM runtimes WHERE id=?", id).Scan(&t, &cpu, &mem, &storage, &pl, &mj, &img, &prov, &class, &net, &a, &b, &status)
	return gin.H{"template_id": t, "cpu_limit": cpu, "memory_limit": mem, "storage_limit": storage, "profile_limit": pl, "max_concurrent_jobs": mj, "image_version": img, "runtime_provider": prov, "runtime_class": class, "network_policy": net, "auto_start": a, "auto_suspend": b, "status": status}
}

func (s *server) listProfileTemplates(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.read") {
		return
	}
	rows, err := s.db.Query(`SELECT id,name,display_name,description,COALESCE(default_model_id,0),runtime_class,default_skills,default_knowledge,managed,status,created_at,updated_at FROM profile_templates WHERE organization_id=1 ORDER BY name`)
	if err != nil {
		failCode(c, 500, "profile_templates.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, model int64
		var n, d, display, class, skills, knowledge, status, created, updated string
		var managed bool
		if rows.Scan(&id, &n, &display, &d, &model, &class, &skills, &knowledge, &managed, &status, &created, &updated) == nil {
			out = append(out, gin.H{"id": id, "name": n, "display_name": display, "description": d, "default_model_id": model, "runtime_class": class, "default_skills": json.RawMessage(skills), "default_knowledge": json.RawMessage(knowledge), "managed": managed, "status": status, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

type profileTemplateRequest struct {
	Name             string   `json:"name"`
	DisplayName      string   `json:"display_name"`
	Description      string   `json:"description"`
	DefaultModelID   int64    `json:"default_model_id"`
	RuntimeClass     string   `json:"runtime_class"`
	DefaultSkills    []string `json:"default_skills"`
	DefaultKnowledge []string `json:"default_knowledge"`
	Managed          bool     `json:"managed"`
}

func (s *server) createProfileTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	var req profileTemplateRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" || req.DisplayName == "" {
		failCode(c, 400, "profile_template.invalid_request", nil)
		return
	}
	skills, _ := json.Marshal(req.DefaultSkills)
	knowledge, _ := json.Marshal(req.DefaultKnowledge)
	res, err := s.db.Exec(`INSERT INTO profile_templates(organization_id,name,display_name,description,default_model_id,runtime_class,default_skills,default_knowledge,managed,status,created_by) VALUES(1,?,?,?,?,?,?,? ,?,'active',?)`, req.Name, req.DisplayName, req.Description, nullableID(req.DefaultModelID), req.RuntimeClass, string(skills), string(knowledge), req.Managed, currentUserID(c))
	if err != nil {
		failCode(c, 409, "profile_template.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditPhase2(c, "profile_template.create", "profile_template", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}
func (s *server) updateProfileTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req profileTemplateRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "profile_template.invalid_request", nil)
		return
	}
	skills, _ := json.Marshal(req.DefaultSkills)
	knowledge, _ := json.Marshal(req.DefaultKnowledge)
	before := s.profileTemplateState(id)
	_, err := s.db.Exec(`UPDATE profile_templates SET name=?,display_name=?,description=?,default_model_id=?,runtime_class=?,default_skills=?,default_knowledge=?,managed=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.DisplayName, req.Description, nullableID(req.DefaultModelID), req.RuntimeClass, string(skills), string(knowledge), req.Managed, id)
	if err != nil {
		failCode(c, 400, "profile_template.update_failed", nil)
		return
	}
	after := s.profileTemplateState(id)
	s.recordChange("profile_template", id, before, after, currentUserID(c))
	s.auditPhase2(c, "profile_template.update", "profile_template", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": after})
}
func (s *server) bindProfileTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Scope        string `json:"scope"`
		DepartmentID int64  `json:"department_id"`
		RoleID       int64  `json:"role_id"`
	}
	if c.ShouldBindJSON(&req) != nil || !validScope(req.Scope) {
		failCode(c, 400, "profile_template.invalid_scope", nil)
		return
	}
	_, err := s.db.Exec("INSERT INTO profile_template_bindings(template_id,scope,organization_id,department_id,role_id,created_by) VALUES(?,?,1,?,?,?)", id, req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), currentUserID(c))
	if err != nil {
		failCode(c, 400, "profile_template.binding_failed", nil)
		return
	}
	s.auditPhase2(c, "profile_template.binding.create", "profile_template", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": true})
}
func (s *server) profileTemplateState(id int64) gin.H {
	var n, d, display, class, skills, knowledge, status string
	var model int64
	var managed bool
	_ = s.db.QueryRow("SELECT name,display_name,description,default_model_id,runtime_class,default_skills,default_knowledge,managed,status FROM profile_templates WHERE id=?", id).Scan(&n, &display, &d, &model, &class, &skills, &knowledge, &managed, &status)
	return gin.H{"name": n, "display_name": display, "description": d, "default_model_id": model, "runtime_class": class, "default_skills": json.RawMessage(skills), "default_knowledge": json.RawMessage(knowledge), "managed": managed, "status": status}
}
func (s *server) assignProfileTemplates(userID int64) {
	rows, _ := s.db.Query(`SELECT pt.id,pt.name,pt.display_name,pt.description,COALESCE(pt.default_model_id,0),pt.runtime_class FROM profile_templates pt JOIN profile_template_bindings b ON b.template_id=pt.id JOIN users u ON u.organization_id=b.organization_id WHERE u.id=? AND pt.status='active' AND (b.scope='organization' OR (b.scope='department' AND b.department_id=u.department_id))`, userID)
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tid, model int64
		var name, display, desc, class string
		if rows.Scan(&tid, &name, &display, &desc, &model, &class) == nil {
			_, _ = s.db.Exec(`INSERT IGNORE INTO profiles(user_id,model_id,name,display_name,description,status,runtime_class,profile_type,managed,source_template_id) VALUES(?,?,?,?,?,'active',?,'managed',TRUE,?)`, userID, nullableID(model), name, display, desc, class, tid)
		}
	}
}

func (s *server) listModelProviders(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.read") {
		return
	}
	rows, err := s.db.Query(`SELECT mp.id,mp.name,mp.type,mp.mode,mp.base_url,mp.auth_type,CASE WHEN mp.secret_reference_id IS NULL THEN 'not_configured' ELSE 'configured' END,mp.status,mp.description,mp.created_at,mp.updated_at FROM model_providers mp WHERE mp.organization_id=1 ORDER BY mp.name`)
	if err != nil {
		failCode(c, 500, "providers.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var n, t, m, url, auth, secret, status, d, created, updated string
		if rows.Scan(&id, &n, &t, &m, &url, &auth, &secret, &status, &d, &created, &updated) == nil {
			out = append(out, gin.H{"id": id, "name": n, "type": t, "mode": m, "base_url": url, "auth_type": auth, "secret_status": secret, "status": status, "description": d, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

type providerRequest struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	Mode              string `json:"mode"`
	BaseURL           string `json:"base_url"`
	AuthType          string `json:"auth_type"`
	Description       string `json:"description"`
	SecretReferenceID int64  `json:"secret_reference_id"`
}

func (s *server) createModelProvider(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	var req providerRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "provider.invalid_request", nil)
		return
	}
	if req.Type == "" {
		req.Type = "custom"
	}
	if req.Mode == "" {
		req.Mode = "hermes_native"
	}
	if req.AuthType == "" {
		req.AuthType = "secret_reference"
	}
	res, err := s.db.Exec(`INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,secret_reference_id,status,description,created_by) VALUES(1,?,?,?,?,?,?, 'active',?,?)`, req.Name, req.Type, req.Mode, req.BaseURL, req.AuthType, nullableID(req.SecretReferenceID), req.Description, currentUserID(c))
	if err != nil {
		failCode(c, 409, "provider.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditPhase2(c, "model_provider.create", "model_provider", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}
func (s *server) updateModelProvider(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req providerRequest
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "provider.invalid_request", nil)
		return
	}
	_, err := s.db.Exec(`UPDATE model_providers SET name=?,type=?,mode=?,base_url=?,auth_type=?,secret_reference_id=?,description=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.Type, req.Mode, req.BaseURL, req.AuthType, nullableID(req.SecretReferenceID), req.Description, id)
	if err != nil {
		failCode(c, 400, "provider.update_failed", nil)
		return
	}
	s.auditPhase2(c, "model_provider.update", "model_provider", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": true})
}
func (s *server) listSecrets(c *gin.Context) {
	if !s.requirePermission(c, "secret.read") {
		return
	}
	rows, err := s.db.Query("SELECT id,name,type,scope,COALESCE(owner_user_id,0),status,COALESCE(last_updated,''),created_at FROM secrets WHERE organization_id=1 ORDER BY name")
	if err != nil {
		failCode(c, 500, "secrets.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, owner int64
		var n, t, scope, status, updated, created string
		if rows.Scan(&id, &n, &t, &scope, &owner, &status, &updated, &created) == nil {
			out = append(out, gin.H{"id": id, "name": n, "type": t, "scope": scope, "owner_user_id": owner, "status": status, "last_updated": updated, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) createSecretReference(c *gin.Context) {
	if !s.requirePermission(c, "secret.manage") {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Scope       string `json:"scope"`
		Status      string `json:"status"`
		OwnerUserID int64  `json:"owner_user_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Name == "" {
		failCode(c, 400, "secret.invalid_request", nil)
		return
	}
	if req.Type == "" {
		req.Type = "api_key"
	}
	if req.Scope == "" {
		req.Scope = "organization"
	}
	if req.Status == "" {
		req.Status = "not_configured"
	}
	res, err := s.db.Exec("INSERT INTO secrets(organization_id,name,type,scope,owner_user_id,status) VALUES(1,?,?,?,?)", req.Name, req.Type, req.Scope, nullableID(req.OwnerUserID), req.Status)
	if err != nil {
		failCode(c, 409, "secret.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditPhase2(c, "secret.reference.create", "secret", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

func sha256Text(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
func safeArtifactPath(path string) bool {
	return path != "" && !strings.HasPrefix(path, "/") && !strings.Contains(path, "..")
}
func (s *server) skillDetail(c *gin.Context) {
	if !s.requirePermission(c, "skill.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var name, display, desc, cat, status, latest, risk, created string
	var publisher int64
	if err := s.db.QueryRow("SELECT name,display_name,description,category,status,latest_version,risk_level,COALESCE(publisher_id,0),created_at FROM skills WHERE id=?", id).Scan(&name, &display, &desc, &cat, &status, &latest, &risk, &publisher, &created); err != nil {
		failCode(c, 404, "skill.not_found", nil)
		return
	}
	versions := s.skillVersionData(id)
	reviews := []gin.H{}
	rows, _ := s.db.Query(`SELECT sr.id,sr.decision,sr.comment,COALESCE(u.display_name,''),sr.created_at FROM skill_reviews sr JOIN skill_submissions ss ON ss.id=sr.submission_id LEFT JOIN users u ON u.id=sr.reviewer_id JOIN skills sk ON sk.id=ss.skill_id WHERE sk.id=? ORDER BY sr.created_at DESC`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var rid int64
			var dec, comment, reviewer, at string
			if rows.Scan(&rid, &dec, &comment, &reviewer, &at) == nil {
				reviews = append(reviews, gin.H{"id": rid, "decision": dec, "comment": comment, "reviewer": reviewer, "created_at": at})
			}
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "display_name": display, "description": desc, "category": cat, "status": status, "latest_version": latest, "risk_level": risk, "publisher_id": publisher, "created_at": created, "versions": versions, "reviews": reviews, "distribution": gin.H{"install_count": 0, "use_count": 0}, "activity": []gin.H{}}})
}
func (s *server) skillVersionData(skillID int64) []gin.H {
	rows, _ := s.db.Query("SELECT id,version,artifact_hash,status,required_tools,required_network,required_secrets,immutable,risk_level,created_at FROM skill_versions WHERE skill_id=? ORDER BY created_at DESC", skillID)
	out := []gin.H{}
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var v, h, status, tools, network, secrets, risk, created string
		var immutable bool
		if rows.Scan(&id, &v, &h, &status, &tools, &network, &secrets, &immutable, &risk, &created) == nil {
			out = append(out, gin.H{"id": id, "version": v, "artifact_hash": h, "status": status, "required_tools": json.RawMessage(tools), "required_network": json.RawMessage(network), "required_secrets": json.RawMessage(secrets), "immutable": immutable, "risk_level": risk, "created_at": created, "files": s.artifactFiles(id)})
		}
	}
	return out
}
func (s *server) artifactFiles(versionID int64) []gin.H {
	rows, _ := s.db.Query("SELECT f.id,f.path,f.content,f.content_type,f.size_bytes,f.sha256,f.is_directory FROM skill_artifact_files f JOIN skill_artifacts a ON a.id=f.artifact_id WHERE a.skill_version_id=? ORDER BY f.path", versionID)
	out := []gin.H{}
	if rows == nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, size int64
		var path, content, typ, hash string
		var dir bool
		if rows.Scan(&id, &path, &content, &typ, &size, &hash, &dir) == nil {
			out = append(out, gin.H{"id": id, "path": path, "content": content, "content_type": typ, "size_bytes": size, "sha256": hash, "is_directory": dir})
		}
	}
	return out
}
func (s *server) skillVersions(c *gin.Context) {
	if !s.requirePermission(c, "skill.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(200, gin.H{"data": s.skillVersionData(id)})
}
func (s *server) createSkillVersion(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	skillID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Version         string   `json:"version"`
		RequiredTools   []string `json:"required_tools"`
		RequiredSecrets []string `json:"required_secrets"`
		RequiredNetwork []string `json:"required_network"`
		RiskLevel       string   `json:"risk_level"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Version == "" {
		failCode(c, 400, "skill.version_invalid", nil)
		return
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "low"
	}
	a, _ := json.Marshal(req.RequiredTools)
	b, _ := json.Marshal(req.RequiredNetwork)
	d, _ := json.Marshal(req.RequiredSecrets)
	res, err := s.db.Exec("INSERT INTO skill_versions(skill_id,version,status,required_tools,required_network,required_secrets,risk_level,immutable) VALUES(?,?, 'draft',?,?,?, ?,FALSE)", skillID, req.Version, string(a), string(b), string(d), req.RiskLevel)
	if err != nil {
		failCode(c, 409, "skill.version_create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditPhase2(c, "skill.version.create", "skill_version", id, "user", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "version": req.Version}})
}
func (s *server) ensureArtifact(versionID int64) (int64, error) {
	var id int64
	err := s.db.QueryRow("SELECT id FROM skill_artifacts WHERE skill_version_id=?", versionID).Scan(&id)
	if err == nil {
		return id, nil
	}
	res, err := s.db.Exec("INSERT INTO skill_artifacts(skill_version_id,status) VALUES(?,'draft')", versionID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (s *server) skillFiles(c *gin.Context) {
	if !s.requirePermission(c, "skill.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(200, gin.H{"data": s.artifactFiles(id)})
}
func (s *server) upsertSkillFile(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	versionID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Path        string `json:"path"`
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
		IsDirectory bool   `json:"is_directory"`
	}
	if c.ShouldBindJSON(&req) != nil || !safeArtifactPath(req.Path) {
		failCode(c, 400, "skill.file_invalid_path", nil)
		return
	}
	var immutable bool
	_ = s.db.QueryRow("SELECT immutable FROM skill_versions WHERE id=?", versionID).Scan(&immutable)
	if immutable {
		failCode(c, 409, "skill.version_immutable", nil)
		return
	}
	artifact, err := s.ensureArtifact(versionID)
	if err != nil {
		failCode(c, 400, "skill.artifact_failed", nil)
		return
	}
	if req.ContentType == "" {
		req.ContentType = "text/plain"
	}
	hash := sha256Text(req.Content)
	_, err = s.db.Exec(`INSERT INTO skill_artifact_files(artifact_id,path,content,content_type,size_bytes,sha256,is_directory) VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE content=VALUES(content),content_type=VALUES(content_type),size_bytes=VALUES(size_bytes),sha256=VALUES(sha256),is_directory=VALUES(is_directory)`, artifact, req.Path, req.Content, req.ContentType, len(req.Content), hash, req.IsDirectory)
	if err != nil {
		failCode(c, 400, "skill.file_save_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE skill_artifacts SET artifact_hash=?,size_bytes=(SELECT COALESCE(SUM(size_bytes),0) FROM skill_artifact_files WHERE artifact_id=?) WHERE id=?", hash, artifact, artifact)
	s.auditPhase2(c, "skill.file.save", "skill_version", versionID, "user", "success", gin.H{"path": req.Path})
	c.JSON(200, gin.H{"data": gin.H{"path": req.Path, "sha256": hash}})
}
func (s *server) updateSkillFile(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct{ Content, ContentType string }
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "skill.file_invalid", nil)
		return
	}
	var versionID int64
	_ = s.db.QueryRow("SELECT a.skill_version_id FROM skill_artifact_files f JOIN skill_artifacts a ON a.id=f.artifact_id WHERE f.id=?", id).Scan(&versionID)
	var immutable bool
	_ = s.db.QueryRow("SELECT immutable FROM skill_versions WHERE id=?", versionID).Scan(&immutable)
	if immutable {
		failCode(c, 409, "skill.version_immutable", nil)
		return
	}
	_, err := s.db.Exec("UPDATE skill_artifact_files SET content=?,content_type=?,size_bytes=?,sha256=? WHERE id=?", req.Content, req.ContentType, len(req.Content), sha256Text(req.Content), id)
	if err != nil {
		failCode(c, 400, "skill.file_save_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": true})
}
func (s *server) deleteSkillFile(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var versionID int64
	_ = s.db.QueryRow("SELECT a.skill_version_id FROM skill_artifact_files f JOIN skill_artifacts a ON a.id=f.artifact_id WHERE f.id=?", id).Scan(&versionID)
	var immutable bool
	_ = s.db.QueryRow("SELECT immutable FROM skill_versions WHERE id=?", versionID).Scan(&immutable)
	if immutable {
		failCode(c, 409, "skill.version_immutable", nil)
		return
	}
	_, err := s.db.Exec("DELETE FROM skill_artifact_files WHERE id=?", id)
	if err != nil {
		failCode(c, 400, "skill.file_delete_failed", nil)
		return
	}
	s.auditPhase2(c, "skill.file.delete", "skill_version", versionID, "user", "success", nil)
	c.JSON(200, gin.H{"data": true})
}
func (s *server) submitSkillVersion(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var skillID int64
	if s.db.QueryRow("SELECT skill_id FROM skill_versions WHERE id=?", id).Scan(&skillID) != nil {
		failCode(c, 404, "skill.version_not_found", nil)
		return
	}
	_, err := s.db.Exec("UPDATE skill_versions SET status='submitted' WHERE id=? AND immutable=FALSE", id)
	if err != nil {
		failCode(c, 400, "skill.submit_failed", nil)
		return
	}
	if _, err = s.db.Exec("INSERT INTO skill_submissions(skill_id,skill_version_id,submitted_by,status,notes) VALUES(?,?,?,'submitted','Versioned artifact submission')", skillID, id, currentUserID(c)); err != nil {
		failCode(c, 400, "skill.review_queue_failed", nil)
		return
	}
	s.auditPhase2(c, "skill.submit", "skill_version", id, "user", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "skill_id": skillID, "status": "submitted"}})
}
func (s *server) publishSkillVersion(c *gin.Context) {
	if !s.requirePermission(c, "skill.publish") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var skillID int64
	if s.db.QueryRow("SELECT skill_id FROM skill_versions WHERE id=? AND status='approved'", id).Scan(&skillID) != nil {
		failCode(c, 409, "skill.version_not_approved", nil)
		return
	}
	_, err := s.db.Exec("UPDATE skill_versions SET status='published',immutable=TRUE WHERE id=?", id)
	if err != nil {
		failCode(c, 400, "skill.publish_failed", nil)
		return
	}
	var version string
	_ = s.db.QueryRow("SELECT version FROM skill_versions WHERE id=?", id).Scan(&version)
	_, _ = s.db.Exec("UPDATE skills SET status='published',latest_version=? WHERE id=?", version, skillID)
	s.auditPhase2(c, "skill.publish", "skill", skillID, "global", "success", gin.H{"version": version})
	c.JSON(200, gin.H{"data": gin.H{"skill_id": skillID, "version": version, "status": "published"}})
}

func (s *server) knowledgeDetail(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var name, desc, visibility, status string
	var dept string
	var docs int
	if s.db.QueryRow(`SELECT kb.name,kb.description,kb.visibility,kb.status,COALESCE(d.name,''),(SELECT COUNT(*) FROM knowledge_documents kd WHERE kd.knowledge_base_id=kb.id) FROM knowledge_bases kb LEFT JOIN departments d ON d.id=kb.owner_department_id WHERE kb.id=?`, id).Scan(&name, &desc, &visibility, &status, &dept, &docs) != nil {
		failCode(c, 404, "knowledge.not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "description": desc, "visibility": visibility, "status": status, "owner_department": dept, "document_count": docs}})
}
func (s *server) listDocuments(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	q := c.Query("q")
	status := c.Query("status")
	query := `SELECT d.id,d.title,d.type,d.status,d.index_status,COALESCE(u.display_name,''),d.created_at,d.updated_at,COALESCE((SELECT MAX(v.version) FROM knowledge_document_versions v WHERE v.document_id=d.id),0) FROM knowledge_documents d LEFT JOIN users u ON u.id=d.owner_user_id WHERE d.knowledge_base_id=?`
	args := []any{id}
	if q != "" {
		query += " AND d.title LIKE ?"
		args = append(args, "%"+q+"%")
	}
	if status != "" {
		query += " AND d.status=?"
		args = append(args, status)
	}
	query += " ORDER BY d.updated_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "documents.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var did, v int64
		var title, typ, st, index, owner, created, updated string
		if rows.Scan(&did, &title, &typ, &st, &index, &owner, &created, &updated, &v) == nil {
			out = append(out, gin.H{"id": did, "title": title, "type": typ, "status": st, "index_status": index, "owner": owner, "created_at": created, "updated_at": updated, "version": v})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) createDocument(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct{ Title, Type, Content string }
	if c.ShouldBindJSON(&req) != nil || req.Title == "" {
		failCode(c, 400, "document.invalid_request", nil)
		return
	}
	if req.Type == "" {
		req.Type = "markdown"
	}
	if req.Type != "markdown" && req.Type != "plain_text" {
		failCode(c, 400, "document.invalid_type", nil)
		return
	}
	res, err := s.db.Exec("INSERT INTO knowledge_documents(knowledge_base_id,title,type,status,owner_user_id,index_status) VALUES(?,?,?,'draft',?,'not_indexed')", kbID, req.Title, req.Type, currentUserID(c))
	if err != nil {
		failCode(c, 400, "document.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	_, err = s.db.Exec("INSERT INTO knowledge_document_versions(document_id,version,content,content_hash,created_by,status) VALUES(?,1,?,?,?,'draft')", id, req.Content, sha256Text(req.Content), currentUserID(c))
	if err != nil {
		failCode(c, 400, "document.version_create_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE knowledge_bases SET document_count=(SELECT COUNT(*) FROM knowledge_documents WHERE knowledge_base_id=?) WHERE id=?", kbID, kbID)
	s.auditPhase2(c, "knowledge.document.create", "knowledge_document", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "version": 1}})
}
func (s *server) documentDetail(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var title, typ, status, index, owner, created, updated string
	var kbID int64
	if s.db.QueryRow("SELECT d.knowledge_base_id,d.title,d.type,d.status,d.index_status,COALESCE(u.display_name,''),d.created_at,d.updated_at FROM knowledge_documents d LEFT JOIN users u ON u.id=d.owner_user_id WHERE d.id=?", id).Scan(&kbID, &title, &typ, &status, &index, &owner, &created, &updated) != nil {
		failCode(c, 404, "document.not_found", nil)
		return
	}
	var vid, ver int64
	var content, hash, vstatus, vcreated string
	if s.db.QueryRow("SELECT id,version,content,content_hash,status,created_at FROM knowledge_document_versions WHERE document_id=? ORDER BY version DESC LIMIT 1", id).Scan(&vid, &ver, &content, &hash, &vstatus, &vcreated) != nil {
		ver = 0
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "knowledge_base_id": kbID, "title": title, "type": typ, "status": status, "index_status": index, "owner": owner, "created_at": created, "updated_at": updated, "current_version": gin.H{"id": vid, "version": ver, "content": content, "content_hash": hash, "status": vstatus, "created_at": vcreated}}})
}
func (s *server) updateDocument(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct{ Title, Type, Content string }
	if c.ShouldBindJSON(&req) != nil || req.Title == "" {
		failCode(c, 400, "document.invalid_request", nil)
		return
	}
	var currentVersion int
	var currentStatus string
	_ = s.db.QueryRow("SELECT COALESCE(MAX(version),0),COALESCE((SELECT status FROM knowledge_document_versions WHERE document_id=? ORDER BY version DESC LIMIT 1),'draft') FROM knowledge_document_versions WHERE document_id=?", id, id).Scan(&currentVersion, &currentStatus)
	if currentStatus == "draft" && currentVersion > 0 {
		_, _ = s.db.Exec("UPDATE knowledge_document_versions SET content=?,content_hash=?,created_by=?,created_at=UTC_TIMESTAMP() WHERE document_id=? AND version=?", req.Content, sha256Text(req.Content), currentUserID(c), id, currentVersion)
	} else {
		currentVersion++
		_, _ = s.db.Exec("INSERT INTO knowledge_document_versions(document_id,version,content,content_hash,created_by,status) VALUES(?,?,?,?,?,'draft')", id, currentVersion, req.Content, sha256Text(req.Content), currentUserID(c))
	}
	_, _ = s.db.Exec("UPDATE knowledge_documents SET title=?,type=?,status='draft',index_status='not_indexed',updated_at=UTC_TIMESTAMP() WHERE id=?", req.Title, req.Type, id)
	s.auditPhase2(c, "knowledge.document.update", "knowledge_document", id, "organization", "success", gin.H{"version": currentVersion})
	c.JSON(200, gin.H{"data": gin.H{"id": id, "version": currentVersion, "status": "draft"}})
}
func (s *server) publishDocument(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var version int
	var content string
	if s.db.QueryRow("SELECT version,content FROM knowledge_document_versions WHERE document_id=? ORDER BY version DESC LIMIT 1", id).Scan(&version, &content) != nil {
		failCode(c, 404, "document.version_not_found", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE knowledge_document_versions SET status='published' WHERE document_id=? AND version=?", id, version)
	_, _ = s.db.Exec("UPDATE knowledge_documents SET status='published',index_status='pending',updated_at=UTC_TIMESTAMP() WHERE id=?", id)
	_ = (providers.MockKnowledgeProvider{}).IndexDocument(context.Background(), id, fmt.Sprintf("document-%d", id), []byte(content))
	_, _ = s.db.Exec("UPDATE knowledge_documents SET index_status='indexed' WHERE id=?", id)
	s.auditPhase2(c, "knowledge.document.publish", "knowledge_document", id, "organization", "success", gin.H{"version": version})
	c.JSON(200, gin.H{"data": gin.H{"id": id, "version": version, "status": "published", "index_status": "indexed"}})
}
func (s *server) deleteDocument(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec("DELETE FROM knowledge_documents WHERE id=?", id); err != nil {
		failCode(c, 400, "document.delete_failed", nil)
		return
	}
	s.auditPhase2(c, "knowledge.document.delete", "knowledge_document", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": true})
}
func (s *server) documentVersions(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query("SELECT id,version,content_hash,created_by,status,created_at FROM knowledge_document_versions WHERE document_id=? ORDER BY version DESC", id)
	if err != nil {
		failCode(c, 500, "document.versions_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var vid, v, creator int64
		var hash, status, created string
		if rows.Scan(&vid, &v, &hash, &creator, &status, &created) == nil {
			out = append(out, gin.H{"id": vid, "version": v, "content_hash": hash, "created_by": creator, "status": status, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) documentVersion(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var docID, version, creator int64
	var content, hash, status, created string
	if s.db.QueryRow("SELECT document_id,version,content,content_hash,created_by,status,created_at FROM knowledge_document_versions WHERE id=?", id).Scan(&docID, &version, &content, &hash, &creator, &status, &created) != nil {
		failCode(c, 404, "document.version_not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "document_id": docID, "version": version, "content": content, "content_hash": hash, "created_by": creator, "status": status, "created_at": created}})
}
func (s *server) listKnowledgeBindings(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT b.id,b.binding_type,b.scope,COALESCE(o.name,''),COALESCE(d.name,''),COALESCE(r.name,''),COALESCE(p.display_name,''),COALESCE(u.display_name,''),b.policy,b.created_at FROM knowledge_bindings b LEFT JOIN organizations o ON o.id=b.organization_id LEFT JOIN departments d ON d.id=b.department_id LEFT JOIN roles r ON r.id=b.role_id LEFT JOIN profiles p ON p.id=b.profile_id LEFT JOIN users u ON u.id=b.created_by WHERE b.knowledge_base_id=? ORDER BY b.created_at DESC`, id)
	if err != nil {
		failCode(c, 500, "knowledge.bindings_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var bid int64
		var typ, scope, org, dept, role, profile, creator, policy, created string
		if rows.Scan(&bid, &typ, &scope, &org, &dept, &role, &profile, &creator, &policy, &created) == nil {
			out = append(out, gin.H{"id": bid, "binding_type": typ, "scope": scope, "organization": org, "department": dept, "role": role, "profile": profile, "created_by": creator, "policy": policy, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) createKnowledgeBindingV2(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		BindingType    string `json:"binding_type"`
		Scope          string `json:"scope"`
		Policy         string `json:"policy"`
		OrganizationID int64  `json:"organization_id"`
		DepartmentID   int64  `json:"department_id"`
		RoleID         int64  `json:"role_id"`
		ProfileID      int64  `json:"profile_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.BindingType == "" {
		failCode(c, 400, "knowledge.binding_invalid", nil)
		return
	}
	if req.Scope == "" {
		req.Scope = req.BindingType
	}
	if req.Policy == "" {
		req.Policy = "allow"
	}
	_, err := s.db.Exec("INSERT INTO knowledge_bindings(knowledge_base_id,binding_type,organization_id,department_id,role_id,profile_id,scope,policy,created_by) VALUES(?,?,?,?,?,?,?,?,?)", id, req.BindingType, nullableID(req.OrganizationID), nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.ProfileID), req.Scope, req.Policy, currentUserID(c))
	if err != nil {
		failCode(c, 400, "knowledge.binding_failed", nil)
		return
	}
	s.auditPhase2(c, "knowledge.binding.create", "knowledge_base", id, "organization", "success", gin.H{"binding_type": req.BindingType, "scope": req.Scope})
	c.JSON(201, gin.H{"data": true})
}
func (s *server) effectiveKnowledge(c *gin.Context) {
	if !s.requirePermission(c, "profile.read") {
		return
	}
	pid, ok := paramID(c, "id")
	if !ok {
		return
	}
	var uid, dept int64
	if s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", pid).Scan(&uid) != nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	_ = s.db.QueryRow("SELECT COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&dept)
	rows, err := s.db.Query(`SELECT DISTINCT kb.id,kb.name,kb.description,kb.visibility FROM knowledge_bases kb LEFT JOIN knowledge_bindings b ON b.knowledge_base_id=kb.id WHERE kb.organization_id=1 AND (kb.visibility='organization' OR b.organization_id=1 OR b.department_id=? OR b.profile_id=? OR b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=?)) ORDER BY kb.name`, dept, pid, uid)
	if err != nil {
		failCode(c, 500, "knowledge.effective_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, desc, vis string
		if rows.Scan(&id, &name, &desc, &vis) == nil {
			out = append(out, gin.H{"id": id, "name": name, "description": desc, "visibility": vis, "source": "effective_binding"})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

type auditQueryRow struct {
	ID                                                                                                                        int64 `json:"id"`
	ActorID                                                                                                                   int64 `json:"actor_user_id"`
	Actor, Action, ResourceType, Scope, Result, IP, UserAgent, RequestID, TraceID, Metadata, CreatedAt, RiskLevel, RiskReason string
	ResourceID                                                                                                                int64
	RiskScore                                                                                                                 float64
}

func (s *server) auditQuery(c *gin.Context) (string, []any) {
	query := `SELECT a.id,COALESCE(a.actor_user_id,0),COALESCE(u.display_name,''),a.action,a.resource_type,COALESCE(a.resource_id,0),a.scope,a.result,a.ip_address,a.user_agent,a.request_id,a.trace_id,a.metadata,a.created_at,a.risk_level,a.risk_score,a.risk_reason FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE 1=1`
	args := []any{}
	filters := []struct{ key, sql string }{{"actor_id", " AND a.actor_user_id=?"}, {"department_id", " AND u.department_id=?"}, {"category", " AND a.category=?"}, {"action", " AND (a.action LIKE ? OR a.action_label LIKE ?)"}, {"resource_type", " AND a.resource_type=?"}, {"resource_id", " AND a.resource_id=?"}, {"result", " AND a.result=?"}, {"risk_level", " AND a.risk_level=?"}, {"ip_address", " AND a.ip_address=?"}, {"target_user_id", " AND ((a.resource_type='user' AND a.resource_id=?) OR JSON_UNQUOTE(JSON_EXTRACT(a.metadata,'$.target_user_id'))=?)"}, {"runtime_id", " AND ((a.resource_type='runtime' AND a.resource_id=?) OR JSON_UNQUOTE(JSON_EXTRACT(a.metadata,'$.runtime_id'))=?)"}, {"skill_id", " AND ((a.resource_type='skill' AND a.resource_id=?) OR JSON_UNQUOTE(JSON_EXTRACT(a.metadata,'$.skill_id'))=?)"}, {"model_id", " AND ((a.resource_type='model' AND a.resource_id=?) OR JSON_UNQUOTE(JSON_EXTRACT(a.metadata,'$.model_id'))=?)"}}
	for _, f := range filters {
		if v := c.Query(f.key); v != "" {
			if f.key == "action" {
				query += f.sql
				args = append(args, "%"+v+"%", "%"+v+"%")
			} else {
				query += f.sql
				if strings.Count(f.sql, "?") == 1 {
					args = append(args, v)
				} else {
					args = append(args, v, v)
				}
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
	return query, args
}
func (s *server) loadAuditRows(c *gin.Context, limit, offset int) ([]auditQueryRow, error) {
	query, args := s.auditQuery(c)
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auditQueryRow{}
	for rows.Next() {
		var r auditQueryRow
		if err := rows.Scan(&r.ID, &r.ActorID, &r.Actor, &r.Action, &r.ResourceType, &r.ResourceID, &r.Scope, &r.Result, &r.IP, &r.UserAgent, &r.RequestID, &r.TraceID, &r.Metadata, &r.CreatedAt, &r.RiskLevel, &r.RiskScore, &r.RiskReason); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
func auditRowJSON(r auditQueryRow) gin.H {
	return gin.H{"id": r.ID, "actor_user_id": r.ActorID, "actor": r.Actor, "action": r.Action, "resource_type": r.ResourceType, "resource_id": r.ResourceID, "scope": r.Scope, "result": r.Result, "ip_address": r.IP, "user_agent": r.UserAgent, "request_id": r.RequestID, "trace_id": r.TraceID, "metadata": json.RawMessage(r.Metadata), "risk_level": r.RiskLevel, "risk_score": r.RiskScore, "risk_reason": r.RiskReason, "created_at": r.CreatedAt}
}
func (s *server) phase2AuditLogs(c *gin.Context) {
	if !s.requirePermission(c, "audit.read") {
		return
	}
	page := int(intQuery(c, "page"))
	if page < 1 {
		page = 1
	}
	size := int(intQuery(c, "page_size"))
	if size < 1 || size > 500 {
		size = 50
	}
	rows, err := s.loadAuditRows(c, size, (page-1)*size)
	if err != nil {
		failCode(c, 500, "audit.load_failed", nil)
		return
	}
	out := []gin.H{}
	for _, r := range rows {
		out = append(out, auditRowJSON(r))
	}
	c.JSON(200, gin.H{"data": out, "meta": gin.H{"page": page, "page_size": size, "count": len(out)}})
}
func (s *server) exportAuditLogs(c *gin.Context) {
	if !s.requirePermission(c, "audit.export") {
		return
	}
	if !s.hasRole(currentUserID(c), "Audit Administrator", "Break-glass Super Administrator", "Super Admin") {
		failCode(c, 403, "audit.export_forbidden", nil)
		return
	}
	rows, err := s.loadAuditRows(c, 0, 0)
	if err != nil {
		failCode(c, 500, "audit.export_failed", nil)
		return
	}
	s.auditPhase2(c, "audit.export", "audit_log", 0, "global", "success", gin.H{"filters": c.Request.URL.Query(), "record_count": len(rows)})
	format := strings.ToLower(c.Query("format"))
	if format == "json" {
		payload := make([]gin.H, 0, len(rows))
		for _, r := range rows {
			payload = append(payload, auditRowJSON(r))
		}
		c.Header("Content-Disposition", "attachment; filename=audit-export.json")
		c.JSON(200, payload)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audit-export.csv")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"id", "actor", "action", "resource_type", "resource_id", "scope", "result", "risk_level", "risk_score", "ip_address", "request_id", "created_at"})
	for _, r := range rows {
		_ = w.Write([]string{strconv.FormatInt(r.ID, 10), r.Actor, r.Action, r.ResourceType, strconv.FormatInt(r.ResourceID, 10), r.Scope, r.Result, r.RiskLevel, fmt.Sprintf("%.2f", r.RiskScore), r.IP, r.RequestID, r.CreatedAt})
	}
	w.Flush()
}

func (s *server) auditPhase2(c *gin.Context, action, resource string, resourceID int64, scope, result string, metadata gin.H) {
	risk := NewRiskEvaluator().Evaluate(action, resource)
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
	res, err := s.db.Exec(`INSERT INTO audit_logs(actor_user_id,action,resource_type,resource_id,scope,result,ip_address,user_agent,request_id,trace_id,metadata,risk_level,risk_score,risk_reason) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, currentUserID(c), action, resource, nullableID(resourceID), scope, result, c.ClientIP(), c.GetHeader("User-Agent"), requestID, traceID, payload, risk.Level, risk.Score, risk.Reason)
	if err != nil {
		return
	}
	if risk.Level == "high" || risk.Level == "critical" {
		auditID, _ := res.LastInsertId()
		_, _ = s.db.Exec(`INSERT INTO risk_events(audit_log_id,actor_user_id,action,resource_type,resource_id,risk_level,risk_score,risk_reason,status) VALUES(?,?,?,?,?,?,?,?, 'open')`, auditID, currentUserID(c), action, resource, nullableID(resourceID), risk.Level, risk.Score, risk.Reason)
		s.notifyRole(c, "Critical Risk Event", "A "+risk.Level+" risk event needs attention", resource, resourceID)
	}
}
func (s *server) listRiskEvents(c *gin.Context) {
	if !s.requirePermission(c, "risk.read") {
		return
	}
	query := `SELECT id,COALESCE(actor_user_id,0),action,resource_type,COALESCE(resource_id,0),risk_level,risk_score,risk_reason,status,created_at FROM risk_events WHERE risk_level IN ('high','critical')`
	args := []any{}
	if v := c.Query("level"); v != "" {
		query += " AND risk_level=?"
		args = append(args, v)
	}
	if v := c.Query("actor_id"); v != "" {
		query += " AND actor_user_id=?"
		args = append(args, v)
	}
	if v := c.Query("status"); v != "" {
		query += " AND status=?"
		args = append(args, v)
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "risk.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, actor, resID int64
		var action, res, risk, reason, status, created string
		var score float64
		if rows.Scan(&id, &actor, &action, &res, &resID, &risk, &score, &reason, &status, &created) == nil {
			out = append(out, gin.H{"id": id, "actor_user_id": actor, "action": action, "resource_type": res, "resource_id": resID, "risk_level": risk, "risk_score": score, "risk_reason": reason, "status": status, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) setRiskEventStatus(c *gin.Context) {
	if !s.requirePermission(c, "risk.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"open": true, "acknowledged": true, "resolved": true, "false_positive": true}[req.Status] {
		failCode(c, 400, "risk.invalid_status", nil)
		return
	}
	_, err := s.db.Exec("UPDATE risk_events SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.Status, id)
	if err != nil {
		failCode(c, 400, "risk.update_failed", nil)
		return
	}
	s.auditPhase2(c, "risk.status.update", "risk_event", id, "global", "success", gin.H{"status": req.Status})
	c.JSON(200, gin.H{"data": true})
}

func (s *server) createApproval(c *gin.Context, typ, res string, resID int64, risk, reason string, metadata gin.H) (int64, error) {
	payload, _ := json.Marshal(metadata)
	var reviewer int64
	_ = s.db.QueryRow("SELECT u.id FROM users u JOIN role_bindings rb ON rb.user_id=u.id JOIN roles r ON r.id=rb.role_id WHERE r.name='Security Administrator' AND u.status='active' LIMIT 1").Scan(&reviewer)
	r, err := s.db.Exec("INSERT INTO approval_requests(type,requester,resource_type,resource_id,status,risk_level,current_reviewer,reason,metadata) VALUES(?,?,?,?, 'pending',?,?,?,?)", typ, currentUserID(c), res, nullableID(resID), risk, nullableID(reviewer), reason, string(payload))
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	_, _ = s.db.Exec("INSERT INTO approval_steps(approval_request_id,step_order,reviewer_id,status) VALUES(?,1,?,'pending')", id, nullableID(reviewer))
	s.auditPhase2(c, "approval.request.create", "approval_request", id, "global", "success", gin.H{"type": typ, "risk_level": risk})
	if reviewer > 0 {
		_, _ = s.db.Exec("INSERT INTO notifications(user_id,type,title,body,resource_type,resource_id) VALUES(?,'approval_assigned','Approval Assigned',?,?,?)", reviewer, reason, res, nullableID(resID))
	}
	return id, nil
}
func (s *server) createApprovalRequest(c *gin.Context) {
	if !s.requirePermission(c, "approval.create") {
		return
	}
	var req struct {
		Type         string `json:"type"`
		ResourceType string `json:"resource_type"`
		Status       string `json:"status"`
		RiskLevel    string `json:"risk_level"`
		Reason       string `json:"reason"`
		ResourceID   int64  `json:"resource_id"`
		Metadata     gin.H  `json:"metadata"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Type == "" {
		failCode(c, 400, "approval.invalid_request", nil)
		return
	}
	id, err := s.createApproval(c, req.Type, req.ResourceType, req.ResourceID, req.RiskLevel, req.Reason, req.Metadata)
	if err != nil {
		failCode(c, 400, "approval.create_failed", nil)
		return
	}
	c.JSON(201, gin.H{"data": gin.H{"id": id, "status": "pending"}})
}
func (s *server) listApprovalRequests(c *gin.Context) {
	if !s.requirePermission(c, "approval.read") {
		return
	}
	query := `SELECT a.id,a.type,a.requester,COALESCE(u.display_name,''),a.resource_type,COALESCE(a.resource_id,0),a.status,a.risk_level,COALESCE(a.current_reviewer,0),a.reason,a.metadata,a.created_at,a.resolved_at FROM approval_requests a LEFT JOIN users u ON u.id=a.requester WHERE 1=1`
	args := []any{}
	if v := c.Query("status"); v != "" {
		query += " AND a.status=?"
		args = append(args, v)
	}
	query += " ORDER BY a.created_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "approval.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, requester, reviewer, resID int64
		var typ, requesterName, resType, status, risk, reason, metadata, created string
		var resolved *time.Time
		if rows.Scan(&id, &typ, &requester, &requesterName, &resType, &resID, &status, &risk, &reviewer, &reason, &metadata, &created, &resolved) == nil {
			out = append(out, gin.H{"id": id, "type": typ, "requester": requester, "requester_name": requesterName, "resource_type": resType, "resource_id": resID, "status": status, "risk_level": risk, "current_reviewer": reviewer, "reason": reason, "metadata": json.RawMessage(metadata), "created_at": created, "resolved_at": resolved})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) decideApproval(c *gin.Context) {
	if !s.requirePermission(c, "approval.review") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.Decision != "approved" && req.Decision != "rejected") {
		failCode(c, 400, "approval.invalid_decision", nil)
		return
	}
	var requester int64
	var typ, metadata, status string
	if s.db.QueryRow("SELECT requester,type,metadata,status FROM approval_requests WHERE id=?", id).Scan(&requester, &typ, &metadata, &status) != nil {
		failCode(c, 404, "approval.not_found", nil)
		return
	}
	if requester == currentUserID(c) {
		failCode(c, 403, "approval.self_review_denied", nil)
		return
	}
	if status != "pending" {
		failCode(c, 409, "approval.already_resolved", nil)
		return
	}
	_, err := s.db.Exec("UPDATE approval_requests SET status=?,resolved_at=UTC_TIMESTAMP() WHERE id=?", req.Decision, id)
	if err != nil {
		failCode(c, 400, "approval.update_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE approval_steps SET reviewer_id=?,status=?,comment=?,resolved_at=UTC_TIMESTAMP() WHERE approval_request_id=? AND status='pending'", currentUserID(c), req.Decision, req.Comment, id)
	if req.Decision == "approved" && typ == "role_elevation" {
		var data struct {
			TargetUserID, RoleID, DepartmentID, ProfileID int64
			Scope                                         string
		}
		_ = json.Unmarshal([]byte(metadata), &data)
		_, _ = s.db.Exec("INSERT IGNORE INTO role_bindings(role_id,organization_id,department_id,user_id,profile_id,scope) VALUES(?,1,?,?,?,?,?)", data.RoleID, nullableID(data.DepartmentID), data.TargetUserID, nullableID(data.ProfileID), data.Scope)
	}
	if typ == "execution" {
		if req.Decision == "approved" {
			_, _ = s.db.Exec("UPDATE executions SET status='completed',started_at=COALESCE(started_at,UTC_TIMESTAMP()),finished_at=UTC_TIMESTAMP(),duration_ms=1250,input_tokens=420,output_tokens=180,cost=0.0024 WHERE approval_request_id=?", id)
		} else {
			_, _ = s.db.Exec("UPDATE executions SET status='rejected',finished_at=UTC_TIMESTAMP() WHERE approval_request_id=?", id)
		}
	}
	s.auditPhase2(c, "approval."+req.Decision, "approval_request", id, "global", "success", gin.H{"comment": req.Comment})
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": req.Decision}})
}

func (s *server) getSettings(c *gin.Context) {
	if !s.requirePermission(c, "settings.read") {
		return
	}
	rows, err := s.db.Query("SELECT setting_key,setting_value,value_type,updated_at FROM system_settings WHERE organization_id=1 ORDER BY setting_key")
	if err != nil {
		failCode(c, 500, "settings.load_failed", nil)
		return
	}
	defer rows.Close()
	values := gin.H{}
	meta := gin.H{}
	for rows.Next() {
		var key, val, typ, updated string
		if rows.Scan(&key, &val, &typ, &updated) == nil {
			var x any = val
			if typ == "boolean" {
				x = val == "true"
			} else if typ == "number" {
				x, _ = strconv.Atoi(val)
			}
			values[key] = x
			meta[key] = gin.H{"value_type": typ, "updated_at": updated}
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"settings": values, "meta": meta, "definitions": phase2SettingDefinitions()}})
}
func phase2SettingDefinitions() []gin.H {
	return []gin.H{{"key": "organization_name", "group": "general", "type": "string"}, {"key": "default_language", "group": "general", "type": "string"}, {"key": "timezone", "group": "general", "type": "string"}, {"key": "local_account_enabled", "group": "authentication", "type": "boolean"}, {"key": "sso_reserved_status", "group": "authentication", "type": "string"}, {"key": "session_ttl", "group": "security", "type": "number"}, {"key": "password_minimum_length", "group": "security", "type": "number"}, {"key": "login_failure_lockout", "group": "security", "type": "number"}, {"key": "audit_retention_days", "group": "security", "type": "number"}, {"key": "high_risk_threshold", "group": "security", "type": "number"}, {"key": "runtime_provisioning", "group": "runtime", "type": "string"}, {"key": "default_runtime_template", "group": "runtime", "type": "string"}, {"key": "default_runtime_provider", "group": "runtime", "type": "string"}, {"key": "default_hermes_image_version", "group": "runtime", "type": "string"}, {"key": "model_access_mode", "group": "models", "type": "string"}, {"key": "default_model", "group": "models", "type": "string"}, {"key": "skill_submission_enabled", "group": "skills", "type": "boolean"}, {"key": "review_required", "group": "skills", "type": "boolean"}, {"key": "default_risk_policy", "group": "skills", "type": "string"}, {"key": "default_document_status", "group": "knowledge", "type": "string"}, {"key": "knowledge_approval_required", "group": "knowledge", "type": "boolean"}, {"key": "default_export_format", "group": "audit", "type": "string"}, {"key": "critical_risk_notifications", "group": "notifications", "type": "boolean"}, {"key": "approval_notifications", "group": "notifications", "type": "boolean"}, {"key": "runtime_failure_notifications", "group": "notifications", "type": "boolean"}}
}
func (s *server) canManageSetting(userID int64, key string) bool {
	if s.isBreakglass(userID) {
		return true
	}
	switch key {
	case "session_ttl", "password_minimum_length", "login_failure_lockout", "high_risk_threshold", "model_access_mode", "default_model", "skill_submission_enabled", "review_required", "default_risk_policy", "default_document_status", "knowledge_approval_required":
		return s.hasRole(userID, "Security Administrator")
	case "audit_retention_days", "default_export_format":
		return s.hasRole(userID, "Audit Administrator")
	default:
		return s.hasRole(userID, "System Administrator")
	}
}

func (s *server) updateSettings(c *gin.Context) {
	if !s.requirePermission(c, "settings.manage") {
		return
	}
	var req struct {
		Settings map[string]any `json:"settings"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Settings) == 0 {
		failCode(c, 400, "settings.invalid_request", nil)
		return
	}
	before := gin.H{}
	for key, val := range req.Settings {
		if !s.canManageSetting(currentUserID(c), key) {
			s.auditPhase2(c, "settings.update", "settings", 1, "organization", "denied", gin.H{"key": key})
			failCode(c, http.StatusForbidden, "settings.key_forbidden", gin.H{"key": key})
			return
		}
		var old string
		_ = s.db.QueryRow("SELECT setting_value FROM system_settings WHERE organization_id=1 AND setting_key=?", key).Scan(&old)
		before[key] = old
		valueType := "string"
		var value string
		switch x := val.(type) {
		case bool:
			valueType = "boolean"
			value = strconv.FormatBool(x)
		case float64:
			valueType = "number"
			value = strconv.Itoa(int(x))
		default:
			value = fmt.Sprint(x)
		}
		_, err := s.db.Exec(`INSERT INTO system_settings(organization_id,setting_key,setting_value,value_type,updated_by) VALUES(1,?,?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),value_type=VALUES(value_type),updated_by=VALUES(updated_by),updated_at=UTC_TIMESTAMP()`, key, value, valueType, currentUserID(c))
		if err != nil {
			failCode(c, 400, "settings.save_failed", nil)
			return
		}
	}
	s.recordChange("settings", 1, before, req.Settings, currentUserID(c))
	s.auditPhase2(c, "settings.update", "settings", 1, "organization", "success", gin.H{"keys": keysOf(req.Settings)})
	c.JSON(200, gin.H{"data": req.Settings})
}
func keysOf(m map[string]any) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

func (s *server) systemHealth(c *gin.Context) {
	if !s.requirePermission(c, "system.health.read") {
		return
	}
	checks := []gin.H{}
	if err := s.db.Ping(); err == nil {
		checks = append(checks, gin.H{"name": "Database", "status": "healthy", "detail": "MySQL 8 connection ready"})
	} else {
		checks = append(checks, gin.H{"name": "Database", "status": "down", "detail": "database unavailable"})
	}
	checks = append(checks, gin.H{"name": "Runtime Provider", "status": "healthy", "detail": "MockRuntimeProvider"}, gin.H{"name": "Hermes Adapter", "status": "healthy", "detail": "MockAdapter"}, gin.H{"name": "Model Gateway", "status": "unknown", "detail": "No gateway configured in Phase 2"}, gin.H{"name": "Knowledge Provider", "status": "healthy", "detail": "MockKnowledgeProvider"}, gin.H{"name": "Secret Provider", "status": "degraded", "detail": "MockSecretProvider; references only"})
	c.JSON(200, gin.H{"data": checks})
}
func (s *server) listNotifications(c *gin.Context) {
	if !s.requirePermission(c, "notification.read") {
		return
	}
	rows, err := s.db.Query("SELECT id,type,title,body,COALESCE(resource_type,''),COALESCE(resource_id,0),status,created_at,read_at FROM notifications WHERE user_id=? ORDER BY created_at DESC LIMIT 100", currentUserID(c))
	if err != nil {
		failCode(c, 500, "notifications.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, resID int64
		var typ, title, body, res, status, created string
		var read *time.Time
		if rows.Scan(&id, &typ, &title, &body, &res, &resID, &status, &created, &read) == nil {
			out = append(out, gin.H{"id": id, "type": typ, "title": title, "body": body, "resource_type": res, "resource_id": resID, "status": status, "created_at": created, "read_at": read})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
func (s *server) readNotification(c *gin.Context) {
	if !s.requirePermission(c, "notification.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	_, err := s.db.Exec("UPDATE notifications SET status='read',read_at=UTC_TIMESTAMP() WHERE id=? AND user_id=?", id, currentUserID(c))
	if err != nil {
		failCode(c, 400, "notification.update_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": true})
}
func (s *server) notifyRole(c *gin.Context, title, body, res string, resID int64) {
	rows, _ := s.db.Query("SELECT DISTINCT rb.user_id FROM role_bindings rb JOIN roles r ON r.id=rb.role_id WHERE r.name IN ('Security Administrator','Audit Administrator') AND rb.user_id IS NOT NULL")
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		if rows.Scan(&uid) == nil {
			_, _ = s.db.Exec("INSERT INTO notifications(user_id,type,title,body,resource_type,resource_id) VALUES(?,'risk_event',?,?,?,?)", uid, title, body, res, nullableID(resID))
		}
	}
}

func (s *server) listQuotas(c *gin.Context) {
	if !s.requirePermission(c, "quota.read") {
		return
	}
	rows, err := s.db.Query(`SELECT q.id,q.scope,COALESCE(d.name,''),COALESCE(r.name,''),COALESCE(u.display_name,''),q.monthly_token_limit,q.monthly_cost_limit,q.max_profiles,q.max_runtime_cpu,q.max_runtime_memory,q.max_concurrent_executions,q.status,q.created_at,q.updated_at FROM quota_policies q LEFT JOIN departments d ON d.id=q.department_id LEFT JOIN roles r ON r.id=q.role_id LEFT JOIN users u ON u.id=q.user_id WHERE q.organization_id=1 ORDER BY q.scope`)
	if err != nil {
		failCode(c, 500, "quotas.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var scope, dept, role, user, cpu, mem, status, created, updated string
		var tokens, cost, profiles, concurrent any
		if rows.Scan(&id, &scope, &dept, &role, &user, &tokens, &cost, &profiles, &cpu, &mem, &concurrent, &status, &created, &updated) == nil {
			out = append(out, gin.H{"id": id, "scope": scope, "department": dept, "role": role, "user": user, "monthly_token_limit": tokens, "monthly_cost_limit": cost, "max_profiles": profiles, "max_runtime_cpu": cpu, "max_runtime_memory": mem, "max_concurrent_executions": concurrent, "status": status, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

type quotaRequest struct {
	Scope                   string   `json:"scope"`
	DepartmentID            int64    `json:"department_id"`
	RoleID                  int64    `json:"role_id"`
	UserID                  int64    `json:"user_id"`
	MonthlyTokenLimit       *int64   `json:"monthly_token_limit"`
	MonthlyCostLimit        *float64 `json:"monthly_cost_limit"`
	MaxProfiles             *int     `json:"max_profiles"`
	MaxRuntimeCPU           string   `json:"max_runtime_cpu"`
	MaxRuntimeMemory        string   `json:"max_runtime_memory"`
	MaxConcurrentExecutions *int     `json:"max_concurrent_executions"`
}

func (s *server) createQuota(c *gin.Context) {
	if !s.requirePermission(c, "quota.manage") {
		return
	}
	var req quotaRequest
	if c.ShouldBindJSON(&req) != nil || !validScope(req.Scope) {
		failCode(c, 400, "quota.invalid_request", nil)
		return
	}
	res, err := s.db.Exec(`INSERT INTO quota_policies(organization_id,scope,department_id,role_id,user_id,monthly_token_limit,monthly_cost_limit,max_profiles,max_runtime_cpu,max_runtime_memory,max_concurrent_executions,created_by) VALUES(1,?,?,?,?,?,?,?,?,?,?,?)`, req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.UserID), req.MonthlyTokenLimit, req.MonthlyCostLimit, req.MaxProfiles, req.MaxRuntimeCPU, req.MaxRuntimeMemory, req.MaxConcurrentExecutions, currentUserID(c))
	if err != nil {
		failCode(c, 400, "quota.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditPhase2(c, "quota.create", "quota_policy", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}
func (s *server) updateQuota(c *gin.Context) {
	if !s.requirePermission(c, "quota.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req quotaRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "quota.invalid_request", nil)
		return
	}
	_, err := s.db.Exec(`UPDATE quota_policies SET scope=?,department_id=?,role_id=?,user_id=?,monthly_token_limit=?,monthly_cost_limit=?,max_profiles=?,max_runtime_cpu=?,max_runtime_memory=?,max_concurrent_executions=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.UserID), req.MonthlyTokenLimit, req.MonthlyCostLimit, req.MaxProfiles, req.MaxRuntimeCPU, req.MaxRuntimeMemory, req.MaxConcurrentExecutions, id)
	if err != nil {
		failCode(c, 400, "quota.update_failed", nil)
		return
	}
	s.auditPhase2(c, "quota.update", "quota_policy", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) phase2Dashboard(c *gin.Context) {
	if !s.requirePermission(c, "dashboard.read") {
		return
	}
	var high, critical, pending, errorsCount, today int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM risk_events WHERE risk_level='high' AND status IN ('open','acknowledged')").Scan(&high)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM risk_events WHERE risk_level='critical' AND status IN ('open','acknowledged')").Scan(&critical)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM approval_requests WHERE status='pending'").Scan(&pending)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM runtimes WHERE status IN ('error','failed')").Scan(&errorsCount)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(token_input+token_output),0) FROM usage_events WHERE created_at>=UTC_DATE()").Scan(&today)
	c.JSON(200, gin.H{"data": gin.H{"high_risk": high, "critical_risk": critical, "pending_approvals": pending, "runtime_errors": errorsCount, "token_today": today}})
}

func (s *server) recordChange(resource string, id int64, before, after any, actor int64) {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	_, _ = s.db.Exec("INSERT INTO resource_change_history(resource_type,resource_id,before_state,after_state,actor_user_id) VALUES(?,?,?,?,?)", resource, id, string(b), string(a), actor)
}
func (s *server) changeHistory(c *gin.Context, resource string, id int64) {
	rows, err := s.db.Query("SELECT id,before_state,after_state,COALESCE(actor_user_id,0),created_at FROM resource_change_history WHERE resource_type=? AND resource_id=? ORDER BY created_at DESC", resource, id)
	if err != nil {
		failCode(c, 500, "history.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var hid, actor int64
		var before, after, created string
		if rows.Scan(&hid, &before, &after, &actor, &created) == nil {
			out = append(out, gin.H{"id": hid, "before": json.RawMessage(before), "after": json.RawMessage(after), "actor_user_id": actor, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

// seedPhase2Data is idempotent and intentionally uses demo-safe references;
// it never inserts a model secret or exposes a key in API responses.
func seedPhase2Data(db *sql.DB, password string) error {
	return seedPhase2DataImpl(db, password)
}
