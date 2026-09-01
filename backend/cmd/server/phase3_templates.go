package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

type agentTemplateRequest struct {
	Name             string            `json:"name"`
	DisplayName      string            `json:"display_name"`
	Description      string            `json:"description"`
	DefaultModelID   int64             `json:"default_model_id"`
	DefaultSkills    []string          `json:"default_skills"`
	DefaultKnowledge []string          `json:"default_knowledge"`
	SkillPolicies    map[string]string `json:"skill_policies"`
	Managed          bool              `json:"managed"`
}

type agentTemplateAssignmentRequest struct {
	SourceType string  `json:"source_type"`
	TargetIDs  []int64 `json:"target_ids"`
	Policy     string  `json:"policy"`
}

func phase3JSON(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return []string{}
	}
	return value
}

func phase3StringList(raw string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil || values == nil {
		return []string{}
	}
	return values
}

func phase3StringMap(raw string) map[string]string {
	var values map[string]string
	if json.Unmarshal([]byte(raw), &values) != nil || values == nil {
		return map[string]string{}
	}
	return values
}

func validAgentAssignmentSource(value string) bool {
	return value == "organization" || value == "department" || value == "role" || value == "user"
}

func (s *server) listAgentTemplates(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.read") {
		return
	}
	rows, err := s.db.Query(`SELECT pt.id,pt.name,pt.display_name,pt.description,COALESCE(pt.default_model_id,0),
		pt.default_skills,pt.default_knowledge,pt.skill_policies,pt.managed,pt.status,pt.template_version,
		(SELECT COUNT(*) FROM profile_template_bindings b WHERE b.template_id=pt.id),
		(SELECT COUNT(*) FROM profiles p WHERE p.source_template_id=pt.id),pt.created_at,pt.updated_at
		FROM profile_templates pt WHERE pt.organization_id=? ORDER BY pt.display_name`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "agent_templates.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, model, version, assignments, instances int64
		var name, display, description, skills, knowledge, policies, status, created, updated string
		var managed bool
		if rows.Scan(&id, &name, &display, &description, &model, &skills, &knowledge, &policies, &managed, &status, &version, &assignments, &instances, &created, &updated) != nil {
			continue
		}
		out = append(out, gin.H{"id": id, "name": name, "display_name": display, "description": description,
			"default_model_id": model, "default_skills": phase3JSON(skills), "default_knowledge": phase3JSON(knowledge),
			"skill_policies": phase3JSON(policies), "managed": managed, "status": status, "template_version": version,
			"assignment_count": assignments, "instance_count": instances, "created_at": created, "updated_at": updated})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) agentTemplateDetail(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var name, display, description, skills, knowledge, policies, status, created, updated string
	var model, version int64
	var managed bool
	if s.db.QueryRow(`SELECT name,display_name,description,COALESCE(default_model_id,0),default_skills,default_knowledge,skill_policies,managed,status,template_version,created_at,updated_at FROM profile_templates WHERE id=?`, id).Scan(&name, &display, &description, &model, &skills, &knowledge, &policies, &managed, &status, &version, &created, &updated) != nil {
		failCode(c, 404, "agent_template.not_found", nil)
		return
	}
	assignments := s.agentTemplateAssignments(id)
	instances := s.agentTemplateInstancesData(id)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "display_name": display, "description": description,
		"default_model_id": model, "default_skills": phase3JSON(skills), "default_knowledge": phase3JSON(knowledge),
		"skill_policies": phase3JSON(policies), "managed": managed, "status": status, "template_version": version,
		"assignments": assignments, "instances": instances, "created_at": created, "updated_at": updated}})
}

func (s *server) createAgentTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	var req agentTemplateRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.DisplayName) == "" {
		failCode(c, 400, "agent_template.invalid_request", nil)
		return
	}
	if req.SkillPolicies == nil {
		req.SkillPolicies = map[string]string{}
	}
	skills, _ := json.Marshal(req.DefaultSkills)
	knowledge, _ := json.Marshal(req.DefaultKnowledge)
	policies, _ := json.Marshal(req.SkillPolicies)
	res, err := s.db.Exec(`INSERT INTO profile_templates(organization_id,name,display_name,description,default_model_id,runtime_class,default_skills,default_knowledge,skill_policies,managed,status,created_by) VALUES(?,?,?,?,?,'standard',?,?,? ,?,'active',?)`, s.currentOrg(c), req.Name, req.DisplayName, req.Description, nullableID(req.DefaultModelID), string(skills), string(knowledge), string(policies), req.Managed, currentUserID(c))
	if err != nil {
		failCode(c, 409, "agent_template.create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	s.auditControlPlane(c, "agent_template.create", "Agent Template Created", "Agent Profiles", "profile_template", id, "success", nil, nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "template_version": 1}})
}

func (s *server) updateAgentTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req agentTemplateRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.DisplayName) == "" {
		failCode(c, 400, "agent_template.invalid_request", nil)
		return
	}
	if req.SkillPolicies == nil {
		req.SkillPolicies = map[string]string{}
	}
	skills, _ := json.Marshal(req.DefaultSkills)
	knowledge, _ := json.Marshal(req.DefaultKnowledge)
	policies, _ := json.Marshal(req.SkillPolicies)
	before := s.agentTemplateState(id)
	_, err := s.db.Exec(`UPDATE profile_templates SET name=?,display_name=?,description=?,default_model_id=?,default_skills=?,default_knowledge=?,skill_policies=?,managed=?,template_version=template_version+1,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.DisplayName, req.Description, nullableID(req.DefaultModelID), string(skills), string(knowledge), string(policies), req.Managed, id)
	if err != nil {
		failCode(c, 400, "agent_template.update_failed", nil)
		return
	}
	after := s.agentTemplateState(id)
	s.recordChange("agent_template", id, before, after, currentUserID(c))
	s.auditControlPlane(c, "agent_template.update", "Agent Template Updated", "Agent Profiles", "profile_template", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": after})
}

func (s *server) setAgentTemplateStatus(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
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
		failCode(c, 400, "agent_template.invalid_status", nil)
		return
	}
	_, err := s.db.Exec("UPDATE profile_templates SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.Status, id)
	if err != nil {
		failCode(c, 400, "agent_template.status_failed", nil)
		return
	}
	s.consolidateAllManagedAgents()
	s.auditControlPlane(c, "agent_template.status", "Agent Template Status Changed", "Agent Profiles", "profile_template", id, "success", gin.H{"status": req.Status}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

func (s *server) agentTemplateState(id int64) gin.H {
	var name, display, description, skills, knowledge, policies, status string
	var model, version int64
	var managed bool
	_ = s.db.QueryRow("SELECT name,display_name,description,COALESCE(default_model_id,0),default_skills,default_knowledge,skill_policies,managed,status,template_version FROM profile_templates WHERE id=?", id).Scan(&name, &display, &description, &model, &skills, &knowledge, &policies, &managed, &status, &version)
	return gin.H{"id": id, "name": name, "display_name": display, "description": description, "default_model_id": model, "default_skills": phase3JSON(skills), "default_knowledge": phase3JSON(knowledge), "skill_policies": phase3JSON(policies), "managed": managed, "status": status, "template_version": version}
}

func (s *server) agentTemplateAssignments(id int64) []gin.H {
	rows, _ := s.db.Query(`SELECT b.id,b.scope,COALESCE(b.organization_id,0),COALESCE(o.name,''),COALESCE(b.department_id,0),COALESCE(d.name,''),COALESCE(b.role_id,0),COALESCE(r.name,''),COALESCE(b.user_id,0),COALESCE(u.display_name,''),b.created_at
		FROM profile_template_bindings b LEFT JOIN organizations o ON o.id=b.organization_id LEFT JOIN departments d ON d.id=b.department_id LEFT JOIN roles r ON r.id=b.role_id LEFT JOIN users u ON u.id=b.user_id WHERE b.template_id=? ORDER BY b.created_at DESC`, id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var bid, orgID, deptID, roleID, userID int64
		var scope, org, dept, role, user, created string
		if rows.Scan(&bid, &scope, &orgID, &org, &deptID, &dept, &roleID, &role, &userID, &user, &created) != nil {
			continue
		}
		label := org
		if dept != "" {
			label = dept
		}
		if role != "" {
			label = role
		}
		if user != "" {
			label = user
		}
		out = append(out, gin.H{"id": bid, "source_type": scope, "organization_id": nullableID(orgID), "department_id": nullableID(deptID), "role_id": nullableID(roleID), "user_id": nullableID(userID), "target": label, "created_at": created})
	}
	return out
}

func (s *server) agentTemplateInstancesData(id int64) []gin.H {
	rows, _ := s.db.Query(`SELECT p.id,u.display_name,p.display_name,p.profile_type,p.status,COALESCE(p.source_template_version,0),COALESCE(p.assignment_sources,'[]') FROM profiles p JOIN users u ON u.id=p.user_id WHERE p.source_template_id=? ORDER BY u.display_name,p.display_name`, id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var profileID, version int64
		var user, profile, typ, status, sources string
		if rows.Scan(&profileID, &user, &profile, &typ, &status, &version, &sources) == nil {
			out = append(out, gin.H{"profile_id": profileID, "user": user, "agent_profile": profile, "profile_type": typ, "status": status, "template_version": version, "assignment_sources": phase3JSON(sources)})
		}
	}
	return out
}

func (s *server) assignAgentTemplate(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	templateID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req agentTemplateAssignmentRequest
	if c.ShouldBindJSON(&req) != nil || !validAgentAssignmentSource(req.SourceType) || len(req.TargetIDs) == 0 {
		failCode(c, 400, "agent_template.assignment_invalid", nil)
		return
	}
	if req.Policy == "" {
		req.Policy = "default"
	}
	if req.Policy != "default" && req.Policy != "mandatory" && req.Policy != "optional" && req.Policy != "blocked" {
		failCode(c, 400, "agent_template.assignment_policy_invalid", nil)
		return
	}
	for _, targetID := range req.TargetIDs {
		var query string
		args := []any{templateID, req.SourceType, s.currentOrg(c), currentUserID(c)}
		switch req.SourceType {
		case "organization":
			query = "INSERT INTO profile_template_bindings(template_id,scope,organization_id,created_by) VALUES(?,?,?,?)"
			args = append(args[:2], append([]any{s.currentOrg(c)}, args[3:]...)...)
		case "department":
			query = "INSERT INTO profile_template_bindings(template_id,scope,organization_id,department_id,created_by) VALUES(?,?,?, ?,?)"
			args = []any{templateID, req.SourceType, s.currentOrg(c), targetID, currentUserID(c)}
		case "role":
			query = "INSERT INTO profile_template_bindings(template_id,scope,organization_id,role_id,created_by) VALUES(?,?,?, ?,?)"
			args = []any{templateID, req.SourceType, s.currentOrg(c), targetID, currentUserID(c)}
		case "user":
			query = "INSERT INTO profile_template_bindings(template_id,scope,organization_id,user_id,created_by) VALUES(?,?,?, ?,?)"
			args = []any{templateID, req.SourceType, s.currentOrg(c), targetID, currentUserID(c)}
		}
		if req.SourceType == "organization" && targetID != s.currentOrg(c) {
			continue
		}
		if _, err := s.db.Exec(query, args...); err != nil {
			failCode(c, 409, "agent_template.assignment_failed", nil)
			return
		}
	}
	if req.SourceType == "user" {
		for _, userID := range req.TargetIDs {
			s.consolidateManagedAgents(userID)
		}
	} else {
		s.consolidateAllManagedAgents()
	}
	s.auditControlPlane(c, "agent_template.assign", "Agent Template Assigned", "Agent Profiles", "profile_template", templateID, "success", gin.H{"source_type": req.SourceType, "target_count": len(req.TargetIDs), "policy": req.Policy}, nil)
	c.JSON(201, gin.H{"data": gin.H{"template_id": templateID, "source_type": req.SourceType, "assigned": len(req.TargetIDs)}})
}

func (s *server) removeAgentTemplateAssignment(c *gin.Context) {
	if !s.requirePermission(c, "profile_template.manage") {
		return
	}
	_, ok := paramID(c, "id")
	if !ok {
		return
	}
	assignmentID, ok := paramID(c, "assignment_id")
	if !ok {
		return
	}
	var userIDs []int64
	rows, _ := s.db.Query("SELECT DISTINCT user_id FROM profile_template_bindings WHERE id=? AND user_id IS NOT NULL", assignmentID)
	if rows != nil {
		for rows.Next() {
			var userID int64
			if rows.Scan(&userID) == nil {
				userIDs = append(userIDs, userID)
			}
		}
		rows.Close()
	}
	if _, err := s.db.Exec("DELETE FROM profile_template_bindings WHERE id=?", assignmentID); err != nil {
		failCode(c, 400, "agent_template.assignment_remove_failed", nil)
		return
	}
	if len(userIDs) == 0 {
		s.consolidateAllManagedAgents()
	} else {
		for _, userID := range userIDs {
			s.consolidateManagedAgents(userID)
		}
	}
	s.auditControlPlane(c, "agent_template.unassign", "Agent Template Assignment Removed", "Agent Profiles", "profile_template_binding", assignmentID, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) consolidateAllManagedAgents() {
	rows, _ := s.db.Query("SELECT id FROM users WHERE status='active'")
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		if rows.Scan(&userID) == nil {
			s.consolidateManagedAgents(userID)
		}
	}
}

func (s *server) consolidateManagedAgents(userID int64) {
	rows, _ := s.db.Query(`SELECT DISTINCT pt.id,pt.name,pt.display_name,pt.description,COALESCE(pt.default_model_id,0),pt.runtime_class,pt.template_version
		FROM profile_templates pt JOIN profile_template_bindings b ON b.template_id=pt.id JOIN users u ON u.id=?
		WHERE pt.status='active' AND (b.scope='organization' AND b.organization_id=u.organization_id OR b.scope='department' AND b.department_id=u.department_id OR b.scope='user' AND b.user_id=u.id OR b.scope='role' AND b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=u.id))`, userID)
	if rows == nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var templateID, modelID, version int64
		var name, display, description, runtimeClass string
		if rows.Scan(&templateID, &name, &display, &description, &modelID, &runtimeClass, &version) != nil {
			continue
		}
		_, _ = s.db.Exec(`INSERT INTO profiles(user_id,model_id,name,display_name,description,status,runtime_class,profile_type,managed,source_template_id,source_template_version,assignment_sources) VALUES(?,?,?,?,?,'active',?,'managed',TRUE,?,?,JSON_ARRAY()) ON DUPLICATE KEY UPDATE model_id=VALUES(model_id),display_name=VALUES(display_name),description=VALUES(description),runtime_class=VALUES(runtime_class),status='active',managed=TRUE,source_template_id=VALUES(source_template_id),source_template_version=VALUES(source_template_version)`, userID, nullableID(modelID), name, display, description, runtimeClass, templateID, version)
		var profileID int64
		if s.db.QueryRow("SELECT id FROM profiles WHERE user_id=? AND name=?", userID, name).Scan(&profileID) != nil {
			continue
		}
		s.recordTemplateSources(profileID, templateID, userID)
	}
	rows.Close()
	_, _ = s.db.Exec(`UPDATE profiles p LEFT JOIN (
		SELECT DISTINCT pt.id
		FROM profile_templates pt JOIN profile_template_bindings b ON b.template_id=pt.id JOIN users u ON u.id=?
		WHERE pt.status='active' AND (b.scope='organization' AND b.organization_id=u.organization_id OR b.scope='department' AND b.department_id=u.department_id OR b.scope='user' AND b.user_id=u.id OR b.scope='role' AND b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=u.id))
	) matched ON matched.id=p.source_template_id
	SET p.status='disabled',p.updated_at=UTC_TIMESTAMP()
	WHERE p.user_id=? AND p.managed=TRUE AND matched.id IS NULL`, userID, userID)
}

func (s *server) recordTemplateSources(profileID, templateID, userID int64) {
	rows, _ := s.db.Query(`SELECT DISTINCT b.scope,COALESCE(b.organization_id,0),COALESCE(b.department_id,0),COALESCE(b.role_id,0),COALESCE(b.user_id,0),COALESCE(o.name,''),COALESCE(d.name,''),COALESCE(r.name,''),COALESCE(u.display_name,'')
		FROM profile_template_bindings b LEFT JOIN organizations o ON o.id=b.organization_id LEFT JOIN departments d ON d.id=b.department_id LEFT JOIN roles r ON r.id=b.role_id LEFT JOIN users u ON u.id=b.user_id
		JOIN users target ON target.id=? WHERE b.template_id=? AND ((b.scope='organization' AND b.organization_id=target.organization_id) OR (b.scope='department' AND b.department_id=target.department_id) OR (b.scope='user' AND b.user_id=target.id) OR (b.scope='role' AND b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=target.id)))`, userID, templateID)
	if rows == nil {
		return
	}
	defer rows.Close()
	sources := []string{}
	for rows.Next() {
		var sourceType, org, dept, role, user string
		var orgID, deptID, roleID, sourceUserID int64
		if rows.Scan(&sourceType, &orgID, &deptID, &roleID, &sourceUserID, &org, &dept, &role, &user) != nil {
			continue
		}
		label := org
		sourceID := orgID
		if sourceType == "department" {
			label, sourceID = dept, deptID
		}
		if sourceType == "role" {
			label, sourceID = role, roleID
		}
		if sourceType == "user" {
			label, sourceID = user, sourceUserID
		}
		if label == "" {
			label = sourceType
		}
		sources = append(sources, fmt.Sprintf("%s: %s", sourceType, label))
		_, _ = s.db.Exec(`INSERT INTO profile_assignment_sources(profile_id,template_id,source_type,source_id,source_label) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE source_label=VALUES(source_label)`, profileID, templateID, sourceType, sourceID, label)
	}
	b, _ := json.Marshal(sources)
	_, _ = s.db.Exec("UPDATE profiles SET assignment_sources=? WHERE id=?", string(b), profileID)
}

func (s *server) profileTemplateAssignmentSources(c *gin.Context) {
	if !s.requirePermission(c, "profile.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query("SELECT source_type,source_id,source_label,created_at FROM profile_assignment_sources WHERE profile_id=? ORDER BY created_at", id)
	if err != nil {
		failCode(c, 500, "profile.sources_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var typ, label, created string
		var sourceID sql.NullInt64
		if rows.Scan(&typ, &sourceID, &label, &created) == nil {
			var value any
			if sourceID.Valid {
				value = sourceID.Int64
			}
			out = append(out, gin.H{"source_type": typ, "source_id": value, "source_label": label, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}
