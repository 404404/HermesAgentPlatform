package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// v0.3.3 extends the existing control-plane data. It intentionally does not
// introduce workspace copies of Models, Profiles, Skills or Knowledge.
func registerV033Routes(auth *gin.RouterGroup, s *server) {
	auth.GET("/profiles/:id/configuration", s.profileConfigurationGetV033)
	auth.PUT("/profiles/:id/configuration", s.updateProfileConfigurationV033)
	auth.GET("/users/:id/runtime-summary", s.userRuntimeSummaryV033)
	auth.PUT("/runtimes/:id/desired-configuration", s.updateRuntimeDesiredConfigurationV033)

	auth.GET("/skill-review-workflows", s.listSkillReviewWorkflowsV033)
	auth.POST("/skill-review-workflows", s.saveSkillReviewWorkflowV033)
	auth.PUT("/skill-review-workflows/:id", s.saveSkillReviewWorkflowV033)
	auth.GET("/skill-submissions/:id/timeline", s.skillSubmissionTimelineV033)
	auth.POST("/skill-submissions/:id/steps/:step_id/decision", s.decideSkillReviewStepV033)

	auth.GET("/knowledge-bases/:id/user-policy", s.knowledgeUserPolicyV033)
	auth.PUT("/knowledge-bases/:id/user-policy", s.saveKnowledgeUserPolicyV033)
	auth.POST("/knowledge-bases/:id/import/qa", s.importKnowledgeQAV033)
	auth.POST("/knowledge-bases/:id/import/markdown", s.importKnowledgeMarkdownV033)
	auth.GET("/knowledge-import-jobs", s.listKnowledgeImportJobsV033)

	auth.GET("/me/agent-groups", s.workspaceAgentGroupsV033)
	auth.PUT("/me/agents/:id/configuration", s.updateWorkspaceAgentConfigurationV033)
	auth.GET("/me/skills/market", s.workspaceSkillMarketV033)
	auth.POST("/me/skills/:id/install", s.installWorkspaceSkillV033)
	auth.GET("/me/knowledge/v033", s.workspaceKnowledgeV033)
	auth.GET("/me/conversations/:id/models", s.conversationModelsV033)
	auth.PUT("/me/conversations/:id/model", s.setConversationModelV033)
	auth.POST("/me/messages/:id/regenerate", s.regenerateWorkspaceMessageV033)
	auth.POST("/me/messages/:id/feedback", s.workspaceMessageFeedbackV033)
}

func v033JSON(raw string) []int64 {
	var values []int64
	_ = json.Unmarshal([]byte(raw), &values)
	if values == nil {
		return []int64{}
	}
	return values
}

func v033IDs(values []int64) string { b, _ := json.Marshal(values); return string(b) }

func (s *server) isProfileAdministrator(c *gin.Context, profileID int64) bool {
	if s.profileOwnedBy(profileID, currentUserID(c)) {
		return false
	}
	return s.canAccessAdmin(currentUserID(c))
}

func (s *server) profileConfigurationV033(profileID int64) gin.H {
	var auxiliary, optional, knowledge string
	_ = s.db.QueryRow("SELECT auxiliary_model_ids,optional_skill_ids,knowledge_override_ids FROM profiles WHERE id=?", profileID).Scan(&auxiliary, &optional, &knowledge)
	return gin.H{"auxiliary_model_ids": v033JSON(auxiliary), "optional_skill_ids": v033JSON(optional), "knowledge_override_ids": v033JSON(knowledge)}
}

func (s *server) profileConfigurationGetV033(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	if s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", id).Scan(&owner) != nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	if owner != currentUserID(c) && !s.canAccessAdmin(currentUserID(c)) {
		failCode(c, http.StatusForbidden, "profile.read_denied", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": s.profileConfigurationV033(id)})
}

func (s *server) updateProfileConfigurationV033(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	var managed bool
	if s.db.QueryRow("SELECT user_id,managed FROM profiles WHERE id=?", id).Scan(&owner, &managed) != nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	admin := s.isProfileAdministrator(c, id)
	if owner != currentUserID(c) && !admin {
		failCode(c, http.StatusForbidden, "profile.update_denied", nil)
		return
	}
	var req struct {
		DisplayName       string  `json:"display_name"`
		Description       string  `json:"description"`
		Status            string  `json:"status"`
		ModelID           int64   `json:"model_id"`
		AuxiliaryModelIDs []int64 `json:"auxiliary_model_ids"`
		OptionalSkillIDs  []int64 `json:"optional_skill_ids"`
		KnowledgeIDs      []int64 `json:"knowledge_ids"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "profile.invalid_request", nil)
		return
	}
	if !admin {
		if req.ModelID > 0 && !s.selfServiceAllowed(owner, "change_main_model", "") {
			failCode(c, http.StatusForbidden, "workspace.model_override_denied", nil)
			return
		}
		if len(req.AuxiliaryModelIDs) > 0 && !s.selfServiceAllowed(owner, "change_auxiliary_models", "") {
			failCode(c, http.StatusForbidden, "workspace.auxiliary_model_denied", nil)
			return
		}
		if len(req.OptionalSkillIDs) > 0 && !s.selfServiceAllowed(owner, "install_optional_skill", "") {
			failCode(c, http.StatusForbidden, "workspace.skill_install_denied", nil)
			return
		}
		if len(req.KnowledgeIDs) > 0 && !s.selfServiceAllowed(owner, "create_personal_knowledge", "") {
			failCode(c, http.StatusForbidden, "workspace.knowledge_override_denied", nil)
			return
		}
		if managed {
			req.Status = ""
		}
	}
	var currentName, currentDescription, currentStatus string
	var currentModel int64
	_ = s.db.QueryRow("SELECT display_name,description,status,COALESCE(model_id,0) FROM profiles WHERE id=?", id).Scan(&currentName, &currentDescription, &currentStatus, &currentModel)
	if req.DisplayName == "" {
		req.DisplayName = currentName
	}
	if req.Description == "" {
		req.Description = currentDescription
	}
	if req.Status == "" {
		req.Status = currentStatus
	}
	if req.ModelID == 0 {
		req.ModelID = currentModel
	}
	_, err := s.db.Exec("UPDATE profiles SET display_name=?,description=?,status=?,model_id=?,auxiliary_model_ids=?,optional_skill_ids=?,knowledge_override_ids=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.DisplayName, req.Description, req.Status, nullableID(req.ModelID), v033IDs(req.AuxiliaryModelIDs), v033IDs(req.OptionalSkillIDs), v033IDs(req.KnowledgeIDs), id)
	if err != nil {
		failCode(c, 400, "profile.update_failed", nil)
		return
	}
	s.auditControlPlane(c, "profile.configuration.update", "Agent Profile Updated", "Agent Profiles", "profile", id, "success", gin.H{"profile_id": id, "managed": managed}, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id, "managed": managed, "configuration": s.profileConfigurationV033(id)}})
}

func (s *server) updateWorkspaceAgentConfigurationV033(c *gin.Context) {
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if !s.profileOwnedBy(id, currentUserID(c)) {
		failCode(c, 404, "workspace.agent_not_found", nil)
		return
	}
	s.updateProfileConfigurationV033(c)
}

func (s *server) userRuntimeSummaryV033(c *gin.Context) {
	if !s.requirePermission(c, "user.read") {
		return
	}
	uid, ok := paramID(c, "id")
	if !ok {
		return
	}
	var id int64
	var runtimeID, container, host, address, cpu, memory, storage, desired, observed, image string
	var template sql.NullInt64
	var last sql.NullTime
	err := s.db.QueryRow(`SELECT r.id,r.runtime_id,r.container_name,COALESCE(h.name,''),COALESCE(h.address,''),r.cpu_limit,r.memory_limit,r.storage_limit,r.desired_status,r.observed_status,r.image_version,r.template_id,r.last_seen FROM runtimes r LEFT JOIN runtime_hosts h ON h.id=r.host_id WHERE r.user_id=?`, uid).Scan(&id, &runtimeID, &container, &host, &address, &cpu, &memory, &storage, &desired, &observed, &image, &template, &last)
	if err != nil {
		failCode(c, 404, "runtime.not_found", nil)
		return
	}
	profiles := s.userProfilesData(uid)
	lastValue := any(nil)
	if last.Valid {
		lastValue = last.Time.UTC().Format(time.RFC3339)
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "runtime_id": runtimeID, "container_name": container, "runtime_host": host, "host_ip": address, "container_ip": "mock://" + runtimeID, "cpu_limit": cpu, "memory_limit": memory, "storage_limit": storage, "template_id": nullableSQLID(template), "image_version": image, "desired_status": desired, "observed_status": observed, "last_seen": lastValue, "profiles": profiles}})
}

func (s *server) updateRuntimeDesiredConfigurationV033(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		HostID            int64  `json:"runtime_host_id"`
		TemplateID        int64  `json:"template_id"`
		CPULimit          string `json:"cpu_limit"`
		MemoryLimit       string `json:"memory_limit"`
		StorageLimit      string `json:"storage_limit"`
		ProfileLimit      int    `json:"profile_limit"`
		MaxConcurrentJobs int    `json:"max_concurrent_jobs"`
		ImageVersion      string `json:"image_version"`
		NetworkPolicy     string `json:"network_policy"`
		DesiredStatus     string `json:"desired_status"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime.invalid_request", nil)
		return
	}
	var cpu, mem, storage, image, network, desired string
	var profileLimit, jobs int
	var host, template sql.NullInt64
	if s.db.QueryRow("SELECT cpu_limit,memory_limit,storage_limit,image_version,network_policy,desired_status,profile_limit,max_concurrent_jobs,host_id,template_id FROM runtimes WHERE id=?", id).Scan(&cpu, &mem, &storage, &image, &network, &desired, &profileLimit, &jobs, &host, &template) != nil {
		failCode(c, 404, "runtime.not_found", nil)
		return
	}
	if req.CPULimit == "" {
		req.CPULimit = cpu
	}
	if req.MemoryLimit == "" {
		req.MemoryLimit = mem
	}
	if req.StorageLimit == "" {
		req.StorageLimit = storage
	}
	if req.ImageVersion == "" {
		req.ImageVersion = image
	}
	if req.NetworkPolicy == "" {
		req.NetworkPolicy = network
	}
	if req.DesiredStatus == "" {
		req.DesiredStatus = desired
	}
	if req.ProfileLimit == 0 {
		req.ProfileLimit = profileLimit
	}
	if req.MaxConcurrentJobs == 0 {
		req.MaxConcurrentJobs = jobs
	}
	if req.HostID == 0 && host.Valid {
		req.HostID = host.Int64
	}
	if req.TemplateID == 0 && template.Valid {
		req.TemplateID = template.Int64
	}
	restart := req.CPULimit != cpu || req.MemoryLimit != mem || req.StorageLimit != storage || req.ImageVersion != image || req.NetworkPolicy != network
	_, err := s.db.Exec("UPDATE runtimes SET host_id=?,template_id=?,cpu_limit=?,memory_limit=?,storage_limit=?,profile_limit=?,max_concurrent_jobs=?,image_version=?,network_policy=?,desired_status=?,restart_required=?,updated_at=UTC_TIMESTAMP() WHERE id=?", nullableID(req.HostID), nullableID(req.TemplateID), req.CPULimit, req.MemoryLimit, req.StorageLimit, req.ProfileLimit, req.MaxConcurrentJobs, req.ImageVersion, req.NetworkPolicy, req.DesiredStatus, restart, id)
	if err != nil {
		failCode(c, 400, "runtime.update_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime.desired_configuration.update", "Runtime Desired Configuration Updated", "Runtime", "runtime", id, "success", gin.H{"restart_required": restart}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "restart_required": restart, "desired_status": req.DesiredStatus, "observed_status": "unchanged"}})
}

type v033WorkflowStep struct {
	Name             string `json:"name"`
	RequiredRoleID   int64  `json:"required_role_id"`
	ApprovalMode     string `json:"approval_mode"`
	RequiredApproval bool   `json:"required_approval"`
}

func (s *server) listSkillReviewWorkflowsV033(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	rows, err := s.db.Query(`SELECT w.id,w.name,w.status,w.description,w.created_at,(SELECT COUNT(*) FROM skill_review_workflow_steps x WHERE x.workflow_id=w.id) FROM skill_review_workflows w WHERE w.organization_id=? ORDER BY w.name`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "skill.workflow_load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, count int64
		var name, status, description, created string
		if rows.Scan(&id, &name, &status, &description, &created, &count) == nil {
			out = append(out, gin.H{"id": id, "name": name, "status": status, "description": description, "created_at": created, "step_count": count})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) saveSkillReviewWorkflowV033(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	var req struct {
		Name        string             `json:"name"`
		Status      string             `json:"status"`
		Description string             `json:"description"`
		Steps       []v033WorkflowStep `json:"steps"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Name) == "" {
		failCode(c, 400, "skill.workflow_invalid_request", nil)
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	id, _ := paramID(c, "id")
	var err error
	if id > 0 {
		_, err = s.db.Exec("UPDATE skill_review_workflows SET name=?,status=?,description=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?", req.Name, req.Status, req.Description, id, s.currentOrg(c))
		_, _ = s.db.Exec("DELETE FROM skill_review_workflow_steps WHERE workflow_id=?", id)
	} else {
		res, e := s.db.Exec("INSERT INTO skill_review_workflows(organization_id,name,status,description,created_by) VALUES(?,?,?,?,?)", s.currentOrg(c), req.Name, req.Status, req.Description, currentUserID(c))
		err = e
		id, _ = res.LastInsertId()
	}
	if err != nil {
		failCode(c, 409, "skill.workflow_save_failed", nil)
		return
	}
	if len(req.Steps) == 0 {
		req.Steps = []v033WorkflowStep{{Name: "Automated Check", ApprovalMode: "auto", RequiredApproval: false}, {Name: "Security Review", ApprovalMode: "manual", RequiredApproval: true}, {Name: "Publish", ApprovalMode: "manual", RequiredApproval: true}}
	}
	for index, step := range req.Steps {
		if step.Name == "" {
			continue
		}
		if step.ApprovalMode == "" {
			step.ApprovalMode = "manual"
		}
		_, _ = s.db.Exec("INSERT INTO skill_review_workflow_steps(workflow_id,step_order,name,required_role_id,approval_mode,required_approval) VALUES(?,?,?,?,?,?)", id, index+1, step.Name, nullableID(step.RequiredRoleID), step.ApprovalMode, step.RequiredApproval)
	}
	s.auditControlPlane(c, "skill.workflow.update", "Skill Review Workflow Updated", "Skills", "skill_review_workflow", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id}})
}

func (s *server) ensureSkillReviewInstanceV033(submissionID int64) int64 {
	var instance int64
	if s.db.QueryRow("SELECT id FROM skill_review_instances WHERE submission_id=?", submissionID).Scan(&instance) == nil {
		return instance
	}
	var workflow sql.NullInt64
	_ = s.db.QueryRow("SELECT id FROM skill_review_workflows WHERE organization_id=1 AND status='active' ORDER BY id LIMIT 1").Scan(&workflow)
	res, err := s.db.Exec("INSERT INTO skill_review_instances(submission_id,workflow_id,status) VALUES(?,?, 'pending')", submissionID, nullableSQLID(workflow))
	if err != nil {
		return 0
	}
	instance, _ = res.LastInsertId()
	rows, _ := s.db.Query("SELECT id,step_order,name,approval_mode FROM skill_review_workflow_steps WHERE workflow_id=? ORDER BY step_order", workflow)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var stepID, order int64
			var name, mode string
			if rows.Scan(&stepID, &order, &name, &mode) == nil {
				status := "pending"
				if order == 1 {
					status = "in_progress"
				}
				if mode == "auto" {
					status = "approved"
				}
				_, _ = s.db.Exec("INSERT INTO skill_review_steps(instance_id,workflow_step_id,step_order,name,status,started_at,completed_at,decision,findings) VALUES(?,?,?,?,?,UTC_TIMESTAMP(),IF(?='approved',UTC_TIMESTAMP(),NULL),?,JSON_OBJECT())", instance, stepID, order, name, status, status, status)
			}
		}
	}
	return instance
}

func (s *server) skillSubmissionTimelineV033(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	submission, ok := paramID(c, "id")
	if !ok {
		return
	}
	instance := s.ensureSkillReviewInstanceV033(submission)
	if instance == 0 {
		failCode(c, 500, "skill.review_instance_failed", nil)
		return
	}
	rows, err := s.db.Query(`SELECT st.id,st.step_order,st.name,st.status,COALESCE(u.display_name,''),st.started_at,st.completed_at,st.decision,COALESCE(st.comment,''),st.risk_level,st.findings FROM skill_review_steps st LEFT JOIN users u ON u.id=st.reviewer_id WHERE st.instance_id=? ORDER BY st.step_order`, instance)
	if err != nil {
		failCode(c, 500, "skill.timeline_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, order int64
		var name, status, reviewer, decision, comment, risk, findings string
		var started, completed sql.NullTime
		if rows.Scan(&id, &order, &name, &status, &reviewer, &started, &completed, &decision, &comment, &risk, &findings) == nil {
			out = append(out, gin.H{"id": id, "order": order, "name": name, "status": status, "reviewer": reviewer, "started_at": nullableTimeValue(started), "completed_at": nullableTimeValue(completed), "decision": decision, "comment": comment, "risk_level": risk, "findings": phase3JSON(findings)})
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"instance_id": instance, "steps": out}})
}

func nullableTimeValue(v sql.NullTime) any {
	if v.Valid {
		return v.Time.UTC().Format(time.RFC3339)
	}
	return nil
}

func (s *server) decideSkillReviewStepV033(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	submission, ok := paramID(c, "id")
	if !ok {
		return
	}
	stepID, ok := paramID(c, "step_id")
	if !ok {
		return
	}
	instance := s.ensureSkillReviewInstanceV033(submission)
	var req struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"approved": true, "rejected": true, "request_changes": true}[req.Decision] {
		failCode(c, 400, "skill.review_decision_invalid", nil)
		return
	}
	res, err := s.db.Exec("UPDATE skill_review_steps SET status=?,reviewer_id=?,completed_at=UTC_TIMESTAMP(),decision=?,comment=? WHERE id=? AND instance_id=? AND status IN ('pending','in_progress')", map[string]string{"approved": "approved", "rejected": "rejected", "request_changes": "rejected"}[req.Decision], currentUserID(c), req.Decision, req.Comment, stepID, instance)
	affected, _ := res.RowsAffected()
	if err != nil || affected == 0 {
		failCode(c, 409, "skill.review_step_unavailable", nil)
		return
	}
	if req.Decision == "approved" {
		var next int64
		if s.db.QueryRow("SELECT id FROM skill_review_steps WHERE instance_id=? AND status='pending' ORDER BY step_order LIMIT 1", instance).Scan(&next) == nil {
			_, _ = s.db.Exec("UPDATE skill_review_steps SET status='in_progress',started_at=UTC_TIMESTAMP() WHERE id=?", next)
		} else {
			_, _ = s.db.Exec("UPDATE skill_review_instances SET status='approved',completed_at=UTC_TIMESTAMP() WHERE id=?", instance)
			_, _ = s.db.Exec("UPDATE skill_submissions SET status='approved' WHERE id=?", submission)
		}
	} else {
		_, _ = s.db.Exec("UPDATE skill_review_instances SET status='rejected',completed_at=UTC_TIMESTAMP() WHERE id=?", instance)
		_, _ = s.db.Exec("UPDATE skill_submissions SET status='rejected' WHERE id=?", submission)
	}
	s.auditControlPlane(c, "skill.review.step.decision", "Skill Review Step Decided", "Skills", "skill_submission", submission, "success", gin.H{"step_id": stepID, "decision": req.Decision}, nil)
	c.JSON(200, gin.H{"data": gin.H{"status": req.Decision}})
}

func (s *server) knowledgeUserPolicyV033(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	kb, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT p.id,p.scope,COALESCE(p.department_id,0),COALESCE(p.role_id,0),COALESCE(p.user_id,0),p.access_level,COALESCE(d.name,''),COALESCE(r.name,''),COALESCE(u.display_name,'') FROM knowledge_user_policies p LEFT JOIN departments d ON d.id=p.department_id LEFT JOIN roles r ON r.id=p.role_id LEFT JOIN users u ON u.id=p.user_id WHERE p.knowledge_base_id=? ORDER BY p.scope`, kb)
	if err != nil {
		failCode(c, 500, "knowledge.policy_load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, department, role, user int64
		var scope, access, dn, rn, un string
		if rows.Scan(&id, &scope, &department, &role, &user, &access, &dn, &rn, &un) == nil {
			out = append(out, gin.H{"id": id, "scope": scope, "department_id": nullableID(department), "role_id": nullableID(role), "user_id": nullableID(user), "access_level": access, "department": dn, "role": rn, "user": un})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) saveKnowledgeUserPolicyV033(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kb, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Scope        string `json:"scope"`
		DepartmentID int64  `json:"department_id"`
		RoleID       int64  `json:"role_id"`
		UserID       int64  `json:"user_id"`
		AccessLevel  string `json:"access_level"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"query_only": true, "read": true, "contribute": true, "manage": true}[req.AccessLevel] {
		failCode(c, 400, "knowledge.policy_invalid_request", nil)
		return
	}
	if req.Scope == "" {
		req.Scope = "organization"
	}
	_, err := s.db.Exec("INSERT INTO knowledge_user_policies(knowledge_base_id,scope,department_id,role_id,user_id,access_level,created_by) VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE access_level=VALUES(access_level),created_by=VALUES(created_by),updated_at=UTC_TIMESTAMP()", kb, req.Scope, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.UserID), req.AccessLevel, currentUserID(c))
	if err != nil {
		failCode(c, 400, "knowledge.policy_save_failed", nil)
		return
	}
	s.auditControlPlane(c, "knowledge.user_policy.update", "Knowledge User Policy Updated", "Knowledge", "knowledge_base", kb, "success", gin.H{"access_level": req.AccessLevel}, nil)
	c.JSON(200, gin.H{"data": gin.H{"knowledge_base_id": kb}})
}

func (s *server) importKnowledgeQAV033(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kb, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Rows []struct {
			Question string   `json:"question"`
			Answer   string   `json:"answer"`
			Tags     []string `json:"tags"`
		} `json:"rows"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Rows) == 0 {
		failCode(c, 400, "knowledge.import_invalid_request", nil)
		return
	}
	res, _ := s.db.Exec("INSERT INTO knowledge_import_jobs(knowledge_base_id,type,file_count,created_by,status,metadata) VALUES(?,'qa_csv',1,?,'running',JSON_OBJECT())", kb, currentUserID(c))
	job, _ := res.LastInsertId()
	success, failed, duplicates := 0, 0, 0
	for _, row := range req.Rows {
		if strings.TrimSpace(row.Question) == "" || strings.TrimSpace(row.Answer) == "" {
			failed++
			continue
		}
		var exists int
		if s.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE knowledge_base_id=? AND type='qa' AND question=?", kb, row.Question).Scan(&exists) == nil && exists > 0 {
			duplicates++
			continue
		}
		tags, _ := json.Marshal(row.Tags)
		_, err := s.db.Exec("INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,owner_user_id) VALUES(?,'qa',?,'',?,'','','',JSON_ARRAY(),'',?,'draft',?)", kb, row.Question, row.Question, row.Answer, string(tags), currentUserID(c))
		if err != nil {
			failed++
		} else {
			success++
		}
	}
	_, _ = s.db.Exec("UPDATE knowledge_import_jobs SET status='completed',success_count=?,failed_count=?,duplicate_count=?,completed_at=UTC_TIMESTAMP() WHERE id=?", success, failed, duplicates, job)
	s.auditControlPlane(c, "knowledge.import.qa", "Knowledge Q&A Imported", "Knowledge", "knowledge_base", kb, "success", gin.H{"job_id": job, "success_count": success}, nil)
	c.JSON(200, gin.H{"data": gin.H{"job_id": job, "success_count": success, "failed_count": failed, "duplicate_count": duplicates}})
}

func (s *server) importKnowledgeMarkdownV033(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kb, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Files []struct {
			Title   string   `json:"title"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		} `json:"files"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Files) == 0 {
		failCode(c, 400, "knowledge.import_invalid_request", nil)
		return
	}
	res, _ := s.db.Exec("INSERT INTO knowledge_import_jobs(knowledge_base_id,type,file_count,created_by,status,metadata) VALUES(?,'markdown',?,?,'running',JSON_OBJECT())", kb, len(req.Files), currentUserID(c))
	job, _ := res.LastInsertId()
	success, failed := 0, 0
	for _, file := range req.Files {
		if strings.TrimSpace(file.Title) == "" || strings.TrimSpace(file.Content) == "" {
			failed++
			continue
		}
		tags, _ := json.Marshal(file.Tags)
		_, err := s.db.Exec("INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,owner_user_id) VALUES(?,'markdown',?,'','','','','',JSON_ARRAY(),'',?,'draft',?)", kb, file.Title, file.Content, string(tags), currentUserID(c))
		if err != nil {
			failed++
		} else {
			success++
		}
	}
	_, _ = s.db.Exec("UPDATE knowledge_import_jobs SET status='completed',success_count=?,failed_count=?,completed_at=UTC_TIMESTAMP() WHERE id=?", success, failed, job)
	s.auditControlPlane(c, "knowledge.import.markdown", "Knowledge Markdown Imported", "Knowledge", "knowledge_base", kb, "success", gin.H{"job_id": job, "success_count": success}, nil)
	c.JSON(200, gin.H{"data": gin.H{"job_id": job, "success_count": success, "failed_count": failed}})
}

func (s *server) listKnowledgeImportJobsV033(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	rows, err := s.db.Query(`SELECT j.id,k.name,j.type,j.file_count,j.success_count,j.failed_count,j.duplicate_count,j.status,j.created_at FROM knowledge_import_jobs j JOIN knowledge_bases k ON k.id=j.knowledge_base_id ORDER BY j.created_at DESC`)
	if err != nil {
		failCode(c, 500, "knowledge.import_jobs_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, files, success, failed, dups int64
		var kb, typ, status, created string
		if rows.Scan(&id, &kb, &typ, &files, &success, &failed, &dups, &status, &created) == nil {
			out = append(out, gin.H{"id": id, "knowledge_base": kb, "type": typ, "file_count": files, "success_count": success, "failed_count": failed, "duplicate_count": dups, "status": status, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) workspaceAgentGroupsV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	rows, err := s.db.Query(`SELECT r.id,r.runtime_id,r.container_name,COALESCE(h.address,''),r.desired_status,r.observed_status,r.cpu_limit,r.memory_limit,r.storage_limit FROM runtimes r LEFT JOIN runtime_hosts h ON h.id=r.host_id WHERE r.user_id=?`, uid)
	if err != nil {
		failCode(c, 500, "workspace.runtime_load_failed", nil)
		return
	}
	defer rows.Close()
	groups := []gin.H{}
	for rows.Next() {
		var id int64
		var runtime, container, host, desired, observed, cpu, mem, storage string
		if rows.Scan(&id, &runtime, &container, &host, &desired, &observed, &cpu, &mem, &storage) == nil {
			profiles := s.userProfilesData(uid)
			groups = append(groups, gin.H{"runtime_id": id, "runtime_name": runtime, "container_name": container, "host_ip": host, "desired_status": desired, "observed_status": observed, "cpu_limit": cpu, "memory_limit": mem, "storage_limit": storage, "profiles": profiles})
		}
	}
	c.JSON(200, gin.H{"data": groups})
}

func (s *server) selfServiceModeV033(uid int64, capability string) string {
	var mode string
	_ = s.db.QueryRow(`SELECT mode FROM user_self_service_policies WHERE organization_id=1 AND capability=? AND (user_id=? OR scope='organization') ORDER BY CASE WHEN user_id=? THEN 1 ELSE 2 END LIMIT 1`, capability, uid, uid).Scan(&mode)
	if mode == "" {
		return "disabled"
	}
	return mode
}

func (s *server) workspaceSkillMarketV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	installed := []gin.H{}
	for _, profile := range s.userProfilesData(uid) {
		pid, _ := profile["id"].(int64)
		cfg := s.effectiveConfigurationData(pid)
		for _, skill := range cfg["skills"].([]gin.H) {
			installed = append(installed, gin.H{"skill_id": skill["id"], "name": skill["name"], "version": skill["latest_version"], "latest_version": skill["latest_version"], "profile": profile["display_name"], "policy": skill["policy"], "status": skill["status"]})
		}
	}
	rows, err := s.db.Query("SELECT id,display_name,category,risk_level,latest_version,status FROM skills WHERE status='published' ORDER BY display_name")
	if err != nil {
		failCode(c, 500, "workspace.skills_failed", nil)
		return
	}
	defer rows.Close()
	available := []gin.H{}
	for rows.Next() {
		var id int64
		var name, category, risk, version, status string
		if rows.Scan(&id, &name, &category, &risk, &version, &status) == nil {
			available = append(available, gin.H{"id": id, "name": name, "category": category, "risk_level": risk, "latest_version": version, "status": status, "install_mode": s.selfServiceModeV033(uid, "install_optional_skill")})
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"installed": installed, "available": available, "updates": []gin.H{}}})
}

func (s *server) installWorkspaceSkillV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	skill, ok := paramID(c, "id")
	if !ok {
		return
	}
	mode := s.selfServiceModeV033(currentUserID(c), "install_optional_skill")
	if mode == "disabled" {
		failCode(c, http.StatusForbidden, "workspace.skill_install_denied", nil)
		return
	}
	if mode == "approval_required" {
		id, _ := s.createApproval(c, "skill_install", "skill", skill, "medium", "Optional Skill installation requires approval", gin.H{"skill_id": skill})
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"status": "pending_approval", "approval_request_id": id}})
		return
	}
	var profileID int64
	_ = s.db.QueryRow("SELECT id FROM profiles WHERE user_id=? ORDER BY id LIMIT 1", currentUserID(c)).Scan(&profileID)
	var raw string
	_ = s.db.QueryRow("SELECT optional_skill_ids FROM profiles WHERE id=?", profileID).Scan(&raw)
	ids := v033JSON(raw)
	for _, existing := range ids {
		if existing == skill {
			c.JSON(200, gin.H{"data": gin.H{"status": "installed"}})
			return
		}
	}
	ids = append(ids, skill)
	_, err := s.db.Exec("UPDATE profiles SET optional_skill_ids=?,updated_at=UTC_TIMESTAMP() WHERE id=?", v033IDs(ids), profileID)
	if err != nil {
		failCode(c, 400, "workspace.skill_install_failed", nil)
		return
	}
	s.audit(c, currentUserID(c), "workspace.skill.install", "skill", skill, "user", "success", nil)
	c.JSON(200, gin.H{"data": gin.H{"status": "installed", "profile_id": profileID}})
}

func (s *server) effectiveKnowledgePolicyV033(uid, kb int64) string {
	var value string
	_ = s.db.QueryRow(`SELECT access_level FROM knowledge_user_policies WHERE knowledge_base_id=? AND (user_id=? OR department_id=(SELECT department_id FROM users WHERE id=?) OR scope='organization') ORDER BY CASE WHEN user_id=? THEN 1 WHEN department_id IS NOT NULL THEN 2 ELSE 3 END LIMIT 1`, kb, uid, uid, uid).Scan(&value)
	if value == "" {
		return "query_only"
	}
	return value
}

func (s *server) workspaceKnowledgeV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	rows, err := s.db.Query(`SELECT DISTINCT kb.id,kb.name,kb.description FROM knowledge_bases kb LEFT JOIN knowledge_bindings b ON b.knowledge_base_id=kb.id WHERE kb.status='active' ORDER BY kb.name`)
	if err != nil {
		failCode(c, 500, "workspace.knowledge_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, description string
		if rows.Scan(&id, &name, &description) == nil {
			access := s.effectiveKnowledgePolicyV033(uid, id)
			entry := gin.H{"id": id, "name": name, "description": description, "access_level": access}
			if access != "query_only" {
				var count int
				_ = s.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE knowledge_base_id=? AND status='published'", id).Scan(&count)
				entry["item_count"] = count
			}
			out = append(out, entry)
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) conversationModelsV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok || !s.conversationOwnedBy(id, currentUserID(c)) {
		failCode(c, 404, "workspace.conversation_not_found", nil)
		return
	}
	rows, err := s.db.Query("SELECT id,display_name,purpose FROM models WHERE status='active' AND user_selectable=TRUE ORDER BY display_name")
	if err != nil {
		failCode(c, 500, "workspace.models_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var mid int64
		var name, purpose string
		if rows.Scan(&mid, &name, &purpose) == nil {
			out = append(out, gin.H{"id": mid, "display_name": name, "purpose": purpose})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) setConversationModelV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok || !s.conversationOwnedBy(id, currentUserID(c)) {
		failCode(c, 404, "workspace.conversation_not_found", nil)
		return
	}
	if !s.selfServiceAllowed(currentUserID(c), "change_main_model", "") {
		failCode(c, http.StatusForbidden, "workspace.model_override_denied", nil)
		return
	}
	var req struct {
		ModelID int64 `json:"model_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ModelID == 0 {
		failCode(c, 400, "workspace.model_invalid_request", nil)
		return
	}
	var exists int
	if s.db.QueryRow("SELECT COUNT(*) FROM models WHERE id=? AND status='active' AND user_selectable=TRUE", req.ModelID).Scan(&exists) != nil || exists == 0 {
		failCode(c, 404, "workspace.model_not_found", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE chat_conversations SET model_override_id=?,updated_at=UTC_TIMESTAMP() WHERE id=?", req.ModelID, id)
	c.JSON(200, gin.H{"data": gin.H{"conversation_id": id, "model_id": req.ModelID}})
}

func (s *server) regenerateWorkspaceMessageV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	messageID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var conversation int64
	var role, content string
	err := s.db.QueryRow(`SELECT m.conversation_id,m.role,m.content FROM chat_messages m JOIN chat_conversations c ON c.id=m.conversation_id WHERE m.id=? AND c.user_id=?`, messageID, currentUserID(c)).Scan(&conversation, &role, &content)
	if err != nil || role != "assistant" {
		failCode(c, 404, "workspace.message_not_found", nil)
		return
	}
	var profile string
	_ = s.db.QueryRow("SELECT p.display_name FROM chat_conversations c JOIN profiles p ON p.id=c.profile_id WHERE c.id=?", conversation).Scan(&profile)
	reply := MockChatProvider{}.Reply(profile, "Regenerate: "+content)
	res, err := s.db.Exec("INSERT INTO chat_messages(conversation_id,role,content,metadata,regenerated_from_message_id) VALUES(?,?,?,JSON_OBJECT('provider','MockChatProvider','regenerated',true),?)", conversation, "assistant", reply, messageID)
	if err != nil {
		failCode(c, 400, "workspace.message_regenerate_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	var created string
	_ = s.db.QueryRow("SELECT created_at FROM chat_messages WHERE id=?", id).Scan(&created)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "role": "assistant", "content": reply, "created_at": created, "regenerated_from_message_id": messageID}})
}

func (s *server) workspaceMessageFeedbackV033(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var owner int64
	var role string
	if s.db.QueryRow(`SELECT c.user_id,m.role FROM chat_messages m JOIN chat_conversations c ON c.id=m.conversation_id WHERE m.id=?`, id).Scan(&owner, &role) != nil || owner != currentUserID(c) || role != "assistant" {
		failCode(c, 404, "workspace.message_not_found", nil)
		return
	}
	var req struct {
		Rating string `json:"rating"`
	}
	if c.ShouldBindJSON(&req) != nil || !map[string]bool{"positive": true, "negative": true}[req.Rating] {
		failCode(c, 400, "workspace.feedback_invalid_request", nil)
		return
	}
	_, err := s.db.Exec("INSERT INTO message_feedback(message_id,user_id,rating) VALUES(?,?,?) ON DUPLICATE KEY UPDATE rating=VALUES(rating),updated_at=UTC_TIMESTAMP()", id, currentUserID(c), req.Rating)
	if err != nil {
		failCode(c, 400, "workspace.feedback_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"message_id": id, "rating": req.Rating}})
}

func seedV033Data(db *sql.DB) error {
	var adminID int64
	_ = db.QueryRow("SELECT id FROM users WHERE username='admin'").Scan(&adminID)
	_, err := db.Exec("INSERT INTO skill_review_workflows(organization_id,name,status,description,created_by) VALUES(1,'Default Skill Review','active','Automated check, security review and publishing approval.',?) ON DUPLICATE KEY UPDATE status='active',description=VALUES(description)", nullableID(adminID))
	if err != nil {
		return err
	}
	var workflowID int64
	_ = db.QueryRow("SELECT id FROM skill_review_workflows WHERE organization_id=1 AND name='Default Skill Review'").Scan(&workflowID)
	steps := []struct {
		name, mode string
		required   bool
	}{{"Automated Check", "auto", false}, {"Security Review", "manual", true}, {"Functional Review", "manual", true}, {"Publish", "manual", true}}
	for index, step := range steps {
		_, _ = db.Exec("INSERT INTO skill_review_workflow_steps(workflow_id,step_order,name,approval_mode,required_approval) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE name=VALUES(name),approval_mode=VALUES(approval_mode),required_approval=VALUES(required_approval)", workflowID, index+1, step.name, step.mode, step.required)
	}
	var user01, user02 int64
	_ = db.QueryRow("SELECT id FROM users WHERE username='user01'").Scan(&user01)
	_ = db.QueryRow("SELECT id FROM users WHERE username='user02'").Scan(&user02)
	rows, _ := db.Query("SELECT id FROM knowledge_bases ORDER BY id LIMIT 2")
	if rows != nil {
		defer rows.Close()
		index := 0
		for rows.Next() {
			var kb int64
			if rows.Scan(&kb) == nil {
				uid, access := user01, "read"
				if index%2 == 1 {
					uid, access = user02, "query_only"
				}
				_, _ = db.Exec("INSERT INTO knowledge_user_policies(knowledge_base_id,scope,user_id,access_level,created_by) VALUES(?,'user',?,?,?) ON DUPLICATE KEY UPDATE access_level=VALUES(access_level)", kb, uid, access, nullableID(adminID))
				index++
			}
		}
	}
	return nil
}

// Keep strconv referenced in this small server package when gofmt coalesces
// future integer parsing helpers into this file.
var _ = strconv.IntSize
