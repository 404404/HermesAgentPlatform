package main

// Completion hotfixes for the v0.3.3 demo.  The handlers in this file are
// deliberately additive: existing database data and legacy route contracts
// continue to work while the v0.3.3 routes are rebound to the safer handlers.

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	completionImportMaxItems = 500
	completionImportMaxBytes = 1024 * 1024
)

type completionProfileConfigurationRequest struct {
	DisplayName          *string  `json:"display_name"`
	Description          *string  `json:"description"`
	Status               *string  `json:"status"`
	ModelID              *int64   `json:"model_id"`
	OptionalSkillIDs     *[]int64 `json:"optional_skill_ids"`
	KnowledgeOverrideIDs *[]int64 `json:"knowledge_override_ids"`
}

type completionKnowledgePolicyRequest struct {
	Scope        string `json:"scope"`
	DepartmentID *int64 `json:"department_id"`
	RoleID       *int64 `json:"role_id"`
	UserID       *int64 `json:"user_id"`
	AccessLevel  string `json:"access_level"`
}

type completionQAInput struct {
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Tags     []string `json:"tags"`
}

type completionQAImportRequest struct {
	CSV     string              `json:"csv"`
	Rows    []completionQAInput `json:"rows"`
	Confirm bool                `json:"confirm"`
}

type completionMarkdownInput struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type completionMarkdownImportRequest struct {
	Files   []completionMarkdownInput `json:"files"`
	Confirm bool                      `json:"confirm"`
}

type completionWorkflowStep struct {
	StepOrder        int    `json:"step_order"`
	Name             string `json:"name"`
	RequiredRoleID   *int64 `json:"required_role_id"`
	ApprovalMode     string `json:"approval_mode"`
	RequiredApproval bool   `json:"required_approval"`
}

type completionWorkflowRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Status      string                   `json:"status"`
	Steps       []completionWorkflowStep `json:"steps"`
}

type completionInstallSkillRequest struct {
	ProfileID      int64 `json:"profile_id"`
	SkillVersionID int64 `json:"skill_version_id"`
}

func completionIDs(values []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func completionJSONIDs(value string) []int64 {
	var ids []int64
	_ = json.Unmarshal([]byte(value), &ids)
	return completionIDs(ids)
}

func completionJSON(value any) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func completionBool(v *bool) bool { return v != nil && *v }

func (s *server) profileConfigurationGetV033Hotfix(c *gin.Context) {
	profileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid := currentUserID(c)
	var ownerID int64
	var displayName, description, profileType, status, optionalSkills, knowledgeOverrides string
	var modelID sql.NullInt64
	if err := s.db.QueryRow(`SELECT user_id,display_name,description,profile_type,status,model_id,optional_skill_ids,knowledge_override_ids
		FROM profiles WHERE id=?`, profileID).Scan(&ownerID, &displayName, &description, &profileType, &status, &modelID, &optionalSkills, &knowledgeOverrides); err != nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	if ownerID != uid && !s.hasPermissionV033(uid, "profile.read") {
		failCode(c, 403, "permission.denied", nil)
		return
	}
	config := gin.H{"id": profileID, "user_id": ownerID, "display_name": displayName, "description": description, "profile_type": profileType, "status": status,
		"model_id": nil, "optional_skill_ids": completionJSONIDs(optionalSkills), "knowledge_override_ids": completionJSONIDs(knowledgeOverrides)}
	if modelID.Valid {
		config["model_id"] = modelID.Int64
	}
	config["effective"] = s.effectiveConfigurationData(profileID)
	c.JSON(200, gin.H{"data": config})
}

func (s *server) updateProfileConfigurationV033Hotfix(c *gin.Context) {
	profileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid := currentUserID(c)
	var req completionProfileConfigurationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	if req.DisplayName == nil && req.Description == nil && req.Status == nil && req.ModelID == nil && req.OptionalSkillIDs == nil && req.KnowledgeOverrideIDs == nil {
		failCode(c, 400, "profile.no_changes", nil)
		return
	}
	var ownerID int64
	var profileType, currentName, currentDescription, currentStatus, optionalSkills, knowledgeOverrides string
	var currentModel sql.NullInt64
	if err := s.db.QueryRow(`SELECT user_id,profile_type,display_name,description,status,model_id,optional_skill_ids,knowledge_override_ids FROM profiles WHERE id=?`, profileID).
		Scan(&ownerID, &profileType, &currentName, &currentDescription, &currentStatus, &currentModel, &optionalSkills, &knowledgeOverrides); err != nil {
		failCode(c, 404, "profile.not_found", nil)
		return
	}
	isOwner := ownerID == uid
	if !isOwner && !s.hasPermissionV033(uid, "profile.update") {
		failCode(c, 403, "permission.denied", nil)
		return
	}
	// Managed profiles own their enterprise-controlled identity and model.  A
	// user may only change explicitly allowed optional capability lists.
	if isOwner && profileType == "managed" && (req.DisplayName != nil || req.Description != nil || req.Status != nil || req.ModelID != nil) {
		failCode(c, 403, "profile.managed_configuration_locked", nil)
		return
	}
	if isOwner && profileType == "personal" && (req.Status != nil && *req.Status != "active") {
		failCode(c, 403, "profile.personal_status_locked", nil)
		return
	}

	name, description, status := currentName, currentDescription, currentStatus
	model := currentModel
	newSkills, newKnowledge := completionJSONIDs(optionalSkills), completionJSONIDs(knowledgeOverrides)
	if req.DisplayName != nil {
		name = strings.TrimSpace(*req.DisplayName)
	}
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		status = *req.Status
	}
	if req.ModelID != nil {
		if !s.profileModelAllowedV033(uid, profileID, *req.ModelID) {
			failCode(c, 403, "model.not_effective_for_profile", nil)
			return
		}
		model = sql.NullInt64{Int64: *req.ModelID, Valid: true}
	}
	if req.OptionalSkillIDs != nil {
		newSkills = completionIDs(*req.OptionalSkillIDs)
		for _, skillID := range newSkills {
			if !s.skillAvailableForProfileV033(uid, profileID, skillID) {
				failCode(c, 403, "skill.not_effective_for_profile", gin.H{"skill_id": skillID})
				return
			}
		}
	}
	if req.KnowledgeOverrideIDs != nil {
		newKnowledge = completionIDs(*req.KnowledgeOverrideIDs)
		for _, knowledgeID := range newKnowledge {
			if level := s.effectiveKnowledgeAccessV033(uid, knowledgeID); level == "none" || level == "query_only" {
				failCode(c, 403, "knowledge.not_readable_for_profile", gin.H{"knowledge_base_id": knowledgeID})
				return
			}
		}
	}
	if name == "" {
		failCode(c, 400, "validation.display_name_required", nil)
		return
	}
	if status != "active" && status != "disabled" && status != "archived" {
		failCode(c, 400, "validation.invalid_status", nil)
		return
	}
	_, err := s.db.Exec(`UPDATE profiles SET display_name=?,description=?,status=?,model_id=?,optional_skill_ids=?,knowledge_override_ids=?,updated_at=NOW() WHERE id=?`,
		name, description, status, nullableInt64(model), completionJSON(newSkills), completionJSON(newKnowledge), profileID)
	if err != nil {
		failCode(c, 500, "profile.update_failed", nil)
		return
	}
	s.writeAudit(uid, "profile.updated", "agent_profile", profileID, "success", "medium", 35, "Profile configuration changed", gin.H{"user_id": ownerID})
	c.JSON(200, gin.H{"data": gin.H{"id": profileID, "effective": s.effectiveConfigurationData(profileID)}})
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func (s *server) profileModelAllowedV033(uid, profileID, modelID int64) bool {
	var ownerID int64
	if err := s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", profileID).Scan(&ownerID); err != nil || ownerID != uid {
		return false
	}
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM models m LEFT JOIN model_providers mp ON mp.id=m.provider_id
		WHERE m.id=? AND m.status='active' AND m.user_selectable=1 AND (m.provider_id IS NULL OR mp.status='active')`, modelID).Scan(&count)
	return err == nil && count == 1
}

func (s *server) skillAvailableForProfileV033(uid, profileID, skillID int64) bool {
	var ownerID int64
	if err := s.db.QueryRow("SELECT user_id FROM profiles WHERE id=?", profileID).Scan(&ownerID); err != nil || ownerID != uid {
		return false
	}
	return s.skillVisibleToUserV033(uid, profileID, skillID)
}

func (s *server) skillVisibleToUserV033(uid, profileID, skillID int64) bool {
	var published, assigned, matched int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM skills WHERE id=? AND status='published'`, skillID).Scan(&published); err != nil || published == 0 {
		return false
	}
	_ = s.db.QueryRow("SELECT COUNT(*) FROM skill_assignments WHERE skill_id=?", skillID).Scan(&assigned)
	if assigned == 0 {
		return true
	}
	var departmentID int64
	_ = s.db.QueryRow("SELECT COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&departmentID)
	roleRows, _ := s.db.Query("SELECT role_id FROM role_bindings WHERE user_id=?", uid)
	roleIDs := []int64{}
	if roleRows != nil {
		defer roleRows.Close()
		for roleRows.Next() {
			var roleID int64
			_ = roleRows.Scan(&roleID)
			roleIDs = append(roleIDs, roleID)
		}
	}
	rows, err := s.db.Query(`SELECT scope,COALESCE(department_id,0),COALESCE(role_id,0),COALESCE(user_id,0),COALESCE(profile_id,0) FROM skill_assignments WHERE skill_id=?`, skillID)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var scope string
		var dept, role, user, profile int64
		if rows.Scan(&scope, &dept, &role, &user, &profile) != nil {
			continue
		}
		if scope == "organization" || (scope == "department" && dept == departmentID) || (scope == "user" && user == uid) || (scope == "profile" && profile == profileID) {
			matched++
			continue
		}
		if scope == "role" {
			for _, id := range roleIDs {
				if role == id {
					matched++
					break
				}
			}
		}
	}
	return matched > 0
}

/* legacy ACL resolver retained for comparison:
func (s *server) effectiveKnowledgeAccessV033Legacy(uid, knowledgeBaseID int64) string {
	var organizationID, departmentID int64
	if err := s.db.QueryRow("SELECT organization_id,COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&organizationID, &departmentID); err != nil {
		return "none"
	}
	roleIDs := []int64{}
	rows, _ := s.db.Query("SELECT role_id FROM role_bindings WHERE user_id=?", uid)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				roleIDs = append(roleIDs, id)
			}
		}
	}
	profileIDs := []int64{}
	profiles, _ := s.db.Query("SELECT id FROM profiles WHERE user_id=? AND status='active'", uid)
	if profiles != nil {
		defer profiles.Close()
		for profiles.Next() {
			var id int64
			if profiles.Scan(&id) == nil {
				profileIDs = append(profileIDs, id)
			}
		}
	}
	type candidate struct {
		rank  int
		level string
	}
	best := candidate{rank: -1, level: "none"}
	policyRows, err := s.db.Query(`SELECT scope,scope_subject_key,access_level FROM knowledge_user_policies WHERE knowledge_base_id=?`, knowledgeBaseID)
	if err == nil {
		defer policyRows.Close()
		for policyRows.Next() {
			var scope, key, level string
			if policyRows.Scan(&scope, &key, &level) != nil {
				continue
			}
			rank, ok := 0, false
			switch scope {
			case "organization":
				rank, ok = 1, key == "organization:"+strconv.FormatInt(organizationID, 10) || key == "organization:0"
			case "department":
				rank, ok = 2, key == "department:"+strconv.FormatInt(departmentID, 10)
			case "role":
				for _, id := range roleIDs {
					if key == "role:"+strconv.FormatInt(id, 10) {
						rank, ok = 3, true
						break
					}
				}
			case "user":
				rank, ok = 4, key == "user:"+strconv.FormatInt(uid, 10)
			}
			if ok && rank >= best.rank {
				best = candidate{rank: rank, level: level}
			}
		}
	}
	if best.rank >= 0 {
		return best.level
	}
	// Bindings are allow-lists.  In the absence of a more specific policy a
	// matching binding grants read access, never global access by default.
	bindingRows, err := s.db.Query(`SELECT scope,COALESCE(department_id,0),COALESCE(role_id,0),COALESCE(profile_id,0) FROM knowledge_bindings WHERE knowledge_base_id=?`, knowledgeBaseID)
	if err != nil {
		return "none"
	}
	defer bindingRows.Close()
	for bindingRows.Next() {
		var scope string
		var dept, role, profile int64
		if bindingRows.Scan(&scope, &dept, &role, &profile) != nil {
			continue
		}
		if scope == "organization" || (scope == "department" && dept == departmentID) {
			return "read"
		}
		if scope == "role" {
			for _, id := range roleIDs {
				if role == id {
					return "read"
				}
			}
		}
		if scope == "profile" {
			for _, id := range profileIDs {
				if profile == id {
					return "read"
				}
			}
		}
	}
	return "none"
}

*/

func completionPolicySubject(req completionKnowledgePolicyRequest, organizationID int64) (string, error) {
	if req.AccessLevel != "query_only" && req.AccessLevel != "read" && req.AccessLevel != "contribute" && req.AccessLevel != "manage" {
		return "", fmt.Errorf("invalid access level")
	}
	switch req.Scope {
	case "organization":
		if req.DepartmentID != nil || req.RoleID != nil || req.UserID != nil {
			return "", fmt.Errorf("organization policy cannot include a target")
		}
		return "organization:" + strconv.FormatInt(organizationID, 10), nil
	case "department":
		if req.DepartmentID == nil || req.RoleID != nil || req.UserID != nil {
			return "", fmt.Errorf("department policy requires exactly department")
		}
		return "department:" + strconv.FormatInt(*req.DepartmentID, 10), nil
	case "role":
		if req.RoleID == nil || req.DepartmentID != nil || req.UserID != nil {
			return "", fmt.Errorf("role policy requires exactly role")
		}
		return "role:" + strconv.FormatInt(*req.RoleID, 10), nil
	case "user":
		if req.UserID == nil || req.DepartmentID != nil || req.RoleID != nil {
			return "", fmt.Errorf("user policy requires exactly user")
		}
		return "user:" + strconv.FormatInt(*req.UserID, 10), nil
	default:
		return "", fmt.Errorf("invalid scope")
	}
}

func (s *server) knowledgeUserPolicyV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	rows, err := s.db.Query(`SELECT id,scope,scope_subject_key,access_level,created_at,updated_at FROM knowledge_user_policies WHERE knowledge_base_id=? ORDER BY scope,scope_subject_key`, kbID)
	if err != nil {
		failCode(c, 500, "knowledge.policy_list_failed", nil)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var scope, key, level string
		var created, updated time.Time
		if rows.Scan(&id, &scope, &key, &level, &created, &updated) == nil {
			item := gin.H{"id": id, "scope": scope, "scope_subject_key": key, "access_level": level, "created_at": created, "updated_at": updated}
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				if targetID, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					item[scope+"_id"] = targetID
				}
			}
			items = append(items, item)
		}
	}
	c.JSON(200, gin.H{"data": items})
}

func (s *server) saveKnowledgeUserPolicyV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	orgID := s.currentOrg(c)
	if !s.knowledgeBaseInOrganization(kbID, orgID) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	var req completionKnowledgePolicyRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	key, err := completionPolicySubject(req, orgID)
	if err != nil {
		failCode(c, 400, "knowledge.invalid_policy_target", gin.H{"detail": err.Error()})
		return
	}
	if !s.validPolicyTargetV033(orgID, req) {
		failCode(c, 400, "knowledge.policy_target_not_found", nil)
		return
	}
	_, err = s.db.Exec(`INSERT INTO knowledge_user_policies(knowledge_base_id,scope,scope_subject_key,access_level,created_at,updated_at)
		VALUES(?,?,?,?,NOW(),NOW()) ON DUPLICATE KEY UPDATE access_level=VALUES(access_level),updated_at=NOW()`, kbID, req.Scope, key, req.AccessLevel)
	if err != nil {
		failCode(c, 500, "knowledge.policy_save_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "knowledge.policy_changed", "knowledge_base", kbID, "success", "medium", 45, "Knowledge access policy changed", gin.H{"scope": req.Scope, "subject": key, "access_level": req.AccessLevel})
	c.JSON(200, gin.H{"data": gin.H{"scope": req.Scope, "scope_subject_key": key, "access_level": req.AccessLevel}})
}

func (s *server) validPolicyTargetV033(orgID int64, req completionKnowledgePolicyRequest) bool {
	var count int
	switch req.Scope {
	case "organization":
		return true
	case "department":
		return req.DepartmentID != nil && s.db.QueryRow("SELECT COUNT(*) FROM departments WHERE id=? AND organization_id=?", *req.DepartmentID, orgID).Scan(&count) == nil && count == 1
	case "role":
		return req.RoleID != nil && s.db.QueryRow("SELECT COUNT(*) FROM roles WHERE id=?", *req.RoleID).Scan(&count) == nil && count == 1
	case "user":
		return req.UserID != nil && s.db.QueryRow("SELECT COUNT(*) FROM users WHERE id=? AND organization_id=?", *req.UserID, orgID).Scan(&count) == nil && count == 1
	}
	return false
}

func (s *server) knowledgeBaseInOrganization(kbID, orgID int64) bool {
	var count int
	return s.db.QueryRow("SELECT COUNT(*) FROM knowledge_bases WHERE id=? AND organization_id=?", kbID, orgID).Scan(&count) == nil && count == 1
}

func completionTags(tags []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, tag := range tags {
		for _, part := range strings.Split(tag, "|") {
			value := strings.TrimSpace(part)
			if value != "" && !seen[value] {
				seen[value] = true
				result = append(result, value)
			}
		}
	}
	return result
}

func completionParseQA(req completionQAImportRequest) ([]completionQAInput, []gin.H) {
	if len(req.Rows) > 0 {
		return req.Rows, nil
	}
	if len(req.CSV) > completionImportMaxBytes {
		return nil, []gin.H{{"line": 0, "error": "file_too_large"}}
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(req.CSV, "\ufeff")))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	headers, err := reader.Read()
	if err != nil {
		return nil, []gin.H{{"line": 1, "error": "invalid_csv"}}
	}
	indexes := map[string]int{}
	for i, header := range headers {
		indexes[strings.ToLower(strings.TrimSpace(header))] = i
	}
	qIndex, qOK := indexes["question"]
	aIndex, aOK := indexes["answer"]
	if !qOK || !aOK {
		return nil, []gin.H{{"line": 1, "error": "question_and_answer_columns_required"}}
	}
	tagsIndex := indexes["tags"]
	rows, errors := []completionQAInput{}, []gin.H{}
	for line := 2; ; line++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			errors = append(errors, gin.H{"line": line, "error": "invalid_csv_row"})
			continue
		}
		if qIndex >= len(record) || aIndex >= len(record) {
			errors = append(errors, gin.H{"line": line, "error": "missing_required_value"})
			continue
		}
		row := completionQAInput{Question: strings.TrimSpace(record[qIndex]), Answer: strings.TrimSpace(record[aIndex])}
		if tagsIndex < len(record) {
			row.Tags = completionTags([]string{record[tagsIndex]})
		}
		if row.Question == "" || row.Answer == "" {
			errors = append(errors, gin.H{"line": line, "error": "question_and_answer_required"})
			continue
		}
		rows = append(rows, row)
		if len(rows) > completionImportMaxItems {
			errors = append(errors, gin.H{"line": line, "error": "too_many_items"})
			break
		}
	}
	return rows, errors
}

func (s *server) knowledgeQAImportPreviewV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	var req completionQAImportRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	rows, errors := completionParseQA(req)
	duplicates := s.knowledgeQADuplicatesV033(kbID, rows)
	c.JSON(200, gin.H{"data": gin.H{"valid_rows": len(rows) - len(duplicates), "invalid_rows": len(errors), "duplicate_rows": len(duplicates), "errors": errors, "duplicates": duplicates, "preview": rows}})
}

func (s *server) knowledgeQADuplicatesV033(kbID int64, rows []completionQAInput) []gin.H {
	seen := map[string]bool{}
	duplicates := []gin.H{}
	for i, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.Question))
		if seen[key] {
			duplicates = append(duplicates, gin.H{"row": i + 1, "reason": "duplicate_in_file"})
			continue
		}
		seen[key] = true
		var count int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE knowledge_base_id=? AND type='qa' AND title=? AND status <> 'deleted'", kbID, row.Question).Scan(&count)
		if count > 0 {
			duplicates = append(duplicates, gin.H{"row": i + 1, "reason": "duplicate_existing_question"})
		}
	}
	return duplicates
}

func (s *server) importKnowledgeQAV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	var req completionQAImportRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	if !req.Confirm {
		failCode(c, 409, "knowledge.import_confirmation_required", nil)
		return
	}
	rows, parseErrors := completionParseQA(req)
	duplicates := s.knowledgeQADuplicatesV033(kbID, rows)
	duplicateRows := map[int]bool{}
	for _, duplicate := range duplicates {
		if value, ok := duplicate["row"].(int); ok {
			duplicateRows[value] = true
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	defer tx.Rollback()
	jobID, err := completionCreateImportJobTx(tx, kbID, currentUserID(c), "csv_qa")
	if err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	success, failures := 0, append([]gin.H{}, parseErrors...)
	for index, row := range rows {
		if duplicateRows[index+1] {
			continue
		}
		payload := knowledgeItemRequest{Type: "qa", Title: row.Question, Question: row.Question, Answer: row.Answer, Tags: completionTags(row.Tags), Status: "draft"}
		if _, err := s.completionCreateKnowledgeItemTx(tx, kbID, currentUserID(c), payload); err != nil {
			failures = append(failures, gin.H{"row": index + 1, "error": "database_write_failed"})
			continue
		}
		success++
	}
	status := completionImportStatus(success, len(failures), len(duplicates))
	if _, err = tx.Exec("UPDATE knowledge_import_jobs SET status=?,success_count=?,failed_count=?,duplicate_count=?,metadata=?,completed_at=NOW() WHERE id=?", status, success, len(failures), len(duplicates), completionJSON(gin.H{"errors": failures, "duplicates": duplicates}), jobID); err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "knowledge.qa_imported", "knowledge_base", kbID, status, "medium", 30, "Knowledge Q&A import completed", gin.H{"job_id": jobID, "imported": success, "failed": len(failures), "duplicates": len(duplicates)})
	c.JSON(200, gin.H{"data": gin.H{"job_id": jobID, "status": status, "imported_count": success, "failed_count": len(failures), "duplicate_count": len(duplicates), "errors": failures, "duplicates": duplicates}})
}

func completionCreateImportJobTx(tx *sql.Tx, kbID, userID int64, importType string) (int64, error) {
	result, err := tx.Exec("INSERT INTO knowledge_import_jobs(knowledge_base_id,type,file_count,created_by,status,metadata) VALUES(?,?,0,?, 'pending', JSON_OBJECT())", kbID, importType, userID)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

/* legacy v0.3.3 importer retained for forensic comparison:
func completionCreateKnowledgeItemTxLegacy(tx *sql.Tx, kbID, userID int64, req knowledgeItemRequest) (int64, error) {
	payload, err := knowledgeItemPayload(req)
	if err != nil {
		return 0, err
	}
	result, err := tx.Exec(`INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,user_id,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`, kbID, req.ItemType, req.Title, payload.Content, payload.Question, payload.Answer, payload.Purpose, payload.Prerequisites, payload.Steps, payload.Notes, payload.Tags, req.Status, userID)
	if err != nil {
		return 0, err
	}
	itemID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(`INSERT INTO knowledge_item_versions(knowledge_item_id,version,content_snapshot,content_hash,status,created_by,created_at)
		VALUES(?,1,?,?,?, ?,NOW())`, itemID, payload.Snapshot, sha256Text(payload.Snapshot), req.Status, userID)
	return itemID, err
}

*/

func completionImportStatus(success, failures, duplicates int) string {
	if success == 0 && (failures > 0 || duplicates > 0) {
		return "failed"
	}
	if failures > 0 || duplicates > 0 {
		return "partial"
	}
	return "completed"
}

func (s *server) knowledgeMarkdownImportPreviewV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	var req completionMarkdownImportRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	valid, errors, duplicates, total := s.validateMarkdownImportV033(kbID, req.Files)
	c.JSON(200, gin.H{"data": gin.H{"valid_files": valid, "invalid_files": len(errors), "duplicate_files": len(duplicates), "errors": errors, "duplicates": duplicates, "total_bytes": total}})
}

func (s *server) validateMarkdownImportV033(kbID int64, files []completionMarkdownInput) (int, []gin.H, []gin.H, int) {
	if len(files) > completionImportMaxItems {
		return 0, []gin.H{{"file": "*", "error": "too_many_files"}}, nil, 0
	}
	valid, total := 0, 0
	errors, duplicates := []gin.H{}, []gin.H{}
	seen := map[string]bool{}
	for index, file := range files {
		title, content := strings.TrimSpace(file.Title), strings.TrimPrefix(file.Content, "\ufeff")
		total += len(content)
		if title == "" || strings.TrimSpace(content) == "" {
			errors = append(errors, gin.H{"file": index + 1, "error": "title_and_content_required"})
			continue
		}
		if total > completionImportMaxBytes {
			errors = append(errors, gin.H{"file": index + 1, "error": "total_size_exceeded"})
			continue
		}
		key := strings.ToLower(title)
		if seen[key] {
			duplicates = append(duplicates, gin.H{"file": index + 1, "reason": "duplicate_in_upload"})
			continue
		}
		seen[key] = true
		var count int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM knowledge_items WHERE knowledge_base_id=? AND type='markdown' AND title=? AND status <> 'deleted'", kbID, title).Scan(&count)
		if count > 0 {
			duplicates = append(duplicates, gin.H{"file": index + 1, "reason": "duplicate_existing_title"})
			continue
		}
		valid++
	}
	return valid, errors, duplicates, total
}

func (s *server) importKnowledgeMarkdownV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	var req completionMarkdownImportRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	if !req.Confirm {
		failCode(c, 409, "knowledge.import_confirmation_required", nil)
		return
	}
	_, errors, duplicates, _ := s.validateMarkdownImportV033(kbID, req.Files)
	bad := map[int]bool{}
	for _, item := range append(append([]gin.H{}, errors...), duplicates...) {
		if value, ok := item["file"].(int); ok {
			bad[value] = true
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	defer tx.Rollback()
	jobID, err := completionCreateImportJobTx(tx, kbID, currentUserID(c), "markdown_files")
	if err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	success := 0
	for index, file := range req.Files {
		if bad[index+1] {
			continue
		}
		_, err := s.completionCreateKnowledgeItemTx(tx, kbID, currentUserID(c), knowledgeItemRequest{Type: "markdown", Title: strings.TrimSpace(file.Title), Content: strings.TrimPrefix(file.Content, "\ufeff"), Tags: completionTags(file.Tags), Status: "draft"})
		if err != nil {
			errors = append(errors, gin.H{"file": index + 1, "error": "database_write_failed"})
			continue
		}
		success++
	}
	status := completionImportStatus(success, len(errors), len(duplicates))
	if _, err = tx.Exec("UPDATE knowledge_import_jobs SET status=?,success_count=?,failed_count=?,duplicate_count=?,metadata=?,completed_at=NOW() WHERE id=?", status, success, len(errors), len(duplicates), completionJSON(gin.H{"errors": errors, "duplicates": duplicates}), jobID); err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "knowledge.import_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "knowledge.markdown_imported", "knowledge_base", kbID, status, "medium", 30, "Knowledge markdown import completed", gin.H{"job_id": jobID, "imported": success, "failed": len(errors), "duplicates": len(duplicates)})
	c.JSON(200, gin.H{"data": gin.H{"job_id": jobID, "status": status, "imported_count": success, "failed_count": len(errors), "duplicate_count": len(duplicates), "errors": errors, "duplicates": duplicates}})
}

func (s *server) listKnowledgeImportJobsV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if kbID == 0 {
		kbID, _ = strconv.ParseInt(c.Query("knowledge_base_id"), 10, 64)
	}
	if !s.knowledgeBaseInOrganization(kbID, s.currentOrg(c)) {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	rows, err := s.db.Query(`SELECT id,type,status,success_count,failed_count,duplicate_count,metadata,created_by,created_at,completed_at FROM knowledge_import_jobs WHERE knowledge_base_id=? ORDER BY id DESC`, kbID)
	if err != nil {
		failCode(c, 500, "knowledge.import_list_failed", nil)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id, imported, failed, duplicates, requested int64
		var typ, status, metadata string
		var created time.Time
		var completed sql.NullTime
		if rows.Scan(&id, &typ, &status, &imported, &failed, &duplicates, &metadata, &requested, &created, &completed) == nil {
			item := gin.H{"id": id, "import_type": typ, "status": status, "imported_count": imported, "failed_count": failed, "duplicate_count": duplicates, "metadata": json.RawMessage(metadata), "created_by": requested, "created_at": created}
			if completed.Valid {
				item["completed_at"] = completed.Time
			}
			items = append(items, item)
		}
	}
	c.JSON(200, gin.H{"data": items})
}

// --- Skill review workflow and immutable submission instances ----------------

func (s *server) saveSkillReviewWorkflowV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	var req completionWorkflowRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Steps) == 0 {
		failCode(c, 400, "skill_review.workflow_name_and_steps_required", nil)
		return
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Status != "active" && req.Status != "disabled" {
		failCode(c, 400, "validation.invalid_status", nil)
		return
	}
	if err := s.validateWorkflowStepsV033(s.currentOrg(c), req.Steps); err != nil {
		failCode(c, 400, "skill_review.invalid_workflow_steps", gin.H{"detail": err.Error()})
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "skill_review.workflow_save_failed", nil)
		return
	}
	defer tx.Rollback()
	workflowID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if workflowID == 0 {
		result, err := tx.Exec("INSERT INTO skill_review_workflows(organization_id,name,description,status,created_by,created_at,updated_at) VALUES(?,?,?,?,?,NOW(),NOW())", s.currentOrg(c), req.Name, req.Description, req.Status, currentUserID(c))
		if err != nil {
			failCode(c, 500, "skill_review.workflow_save_failed", nil)
			return
		}
		workflowID, _ = result.LastInsertId()
	} else {
		result, err := tx.Exec("UPDATE skill_review_workflows SET name=?,description=?,status=?,updated_at=NOW() WHERE id=? AND organization_id=?", req.Name, req.Description, req.Status, workflowID, s.currentOrg(c))
		if err != nil {
			failCode(c, 500, "skill_review.workflow_save_failed", nil)
			return
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			failCode(c, 404, "skill_review.workflow_not_found", nil)
			return
		}
		if _, err = tx.Exec("DELETE FROM skill_review_workflow_steps WHERE workflow_id=?", workflowID); err != nil {
			failCode(c, 500, "skill_review.workflow_save_failed", nil)
			return
		}
	}
	for index, step := range req.Steps {
		if _, err = tx.Exec(`INSERT INTO skill_review_workflow_steps(workflow_id,step_order,name,required_role_id,approval_mode,required_approval) VALUES(?,?,?,?,?,?)`, workflowID, index+1, strings.TrimSpace(step.Name), step.RequiredRoleID, step.ApprovalMode, step.RequiredApproval); err != nil {
			failCode(c, 500, "skill_review.workflow_save_failed", nil)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "skill_review.workflow_save_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "skill_review.workflow_saved", "skill_review_workflow", workflowID, "success", "high", 60, "Skill review workflow changed", gin.H{"steps": len(req.Steps)})
	c.JSON(200, gin.H{"data": gin.H{"id": workflowID}})
}

func (s *server) validateWorkflowStepsV033(orgID int64, steps []completionWorkflowStep) error {
	for _, step := range steps {
		if strings.TrimSpace(step.Name) == "" {
			return fmt.Errorf("step name is required")
		}
		if step.ApprovalMode != "manual" && step.ApprovalMode != "auto" {
			return fmt.Errorf("invalid approval mode")
		}
		if step.RequiredRoleID != nil {
			var count int
			if err := s.db.QueryRow("SELECT COUNT(*) FROM roles WHERE id=?", *step.RequiredRoleID).Scan(&count); err != nil || count != 1 {
				return fmt.Errorf("reviewer role not found")
			}
		}
		if step.ApprovalMode == "manual" && step.RequiredRoleID == nil {
			return fmt.Errorf("manual step requires a reviewer role")
		}
	}
	return nil
}

func (s *server) submitSkillV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	skillID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var versionID int64
	if err := s.db.QueryRow("SELECT id FROM skill_versions WHERE skill_id=? ORDER BY id DESC LIMIT 1", skillID).Scan(&versionID); err != nil {
		failCode(c, 404, "skill.version_not_found", nil)
		return
	}
	s.submitSkillVersionByIDV033Hotfix(c, skillID, versionID)
}

func (s *server) submitSkillVersionV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	versionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var skillID int64
	if err := s.db.QueryRow("SELECT skill_id FROM skill_versions WHERE id=?", versionID).Scan(&skillID); err != nil {
		failCode(c, 404, "skill.version_not_found", nil)
		return
	}
	s.submitSkillVersionByIDV033Hotfix(c, skillID, versionID)
}

func (s *server) submitSkillVersionByIDV033Hotfix(c *gin.Context, skillID, versionID int64) {
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "skill.submit_failed", nil)
		return
	}
	defer tx.Rollback()
	var existing int64
	err = tx.QueryRow("SELECT id FROM skill_submissions WHERE skill_version_id=? AND status IN ('pending','in_review','approved') ORDER BY id DESC LIMIT 1 FOR UPDATE", versionID).Scan(&existing)
	if err == nil {
		failCode(c, 409, "skill.submission_already_open", gin.H{"submission_id": existing})
		return
	}
	result, err := tx.Exec(`INSERT INTO skill_submissions(skill_id,skill_version_id,submitted_by,status,created_at,updated_at) VALUES(?,?,?,'in_review',NOW(),NOW())`, skillID, versionID, currentUserID(c))
	if err != nil {
		failCode(c, 500, "skill.submit_failed", nil)
		return
	}
	submissionID, _ := result.LastInsertId()
	if _, err = tx.Exec("UPDATE skill_versions SET status='in_review',updated_at=NOW() WHERE id=?", versionID); err != nil {
		failCode(c, 500, "skill.submit_failed", nil)
		return
	}
	instanceID, err := s.createSkillReviewInstanceV033Hotfix(tx, submissionID)
	if err != nil {
		failCode(c, 400, "skill.review_workflow_unavailable", gin.H{"detail": err.Error()})
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "skill.submit_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "skill.submitted", "skill", skillID, "success", "medium", 45, "Skill version submitted for review", gin.H{"skill_version_id": versionID, "submission_id": submissionID, "review_instance_id": instanceID})
	c.JSON(201, gin.H{"data": gin.H{"submission_id": submissionID, "skill_version_id": versionID, "review_instance_id": instanceID, "status": "in_review"}})
}

func (s *server) createSkillReviewInstanceV033Hotfix(tx *sql.Tx, submissionID int64) (int64, error) {
	var existing int64
	err := tx.QueryRow("SELECT id FROM skill_review_instances WHERE submission_id=? FOR UPDATE", submissionID).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	var orgID, versionID int64
	if err = tx.QueryRow(`SELECT u.organization_id,ss.skill_version_id FROM skill_submissions ss JOIN users u ON u.id=ss.submitted_by WHERE ss.id=?`, submissionID).Scan(&orgID, &versionID); err != nil {
		return 0, err
	}
	var workflowID int64
	if err = tx.QueryRow("SELECT id FROM skill_review_workflows WHERE organization_id=? AND status='active' ORDER BY id LIMIT 1", orgID).Scan(&workflowID); err != nil {
		return 0, fmt.Errorf("no active workflow")
	}
	rows, err := tx.Query("SELECT step_order,name,required_role_id,approval_mode,required_approval FROM skill_review_workflow_steps WHERE workflow_id=? ORDER BY step_order", workflowID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	steps := []completionWorkflowStep{}
	for rows.Next() {
		var st completionWorkflowStep
		if err = rows.Scan(&st.StepOrder, &st.Name, &st.RequiredRoleID, &st.ApprovalMode, &st.RequiredApproval); err != nil {
			return 0, err
		}
		steps = append(steps, st)
	}
	if len(steps) == 0 {
		return 0, fmt.Errorf("workflow has no steps")
	}
	result, err := tx.Exec(`INSERT INTO skill_review_instances(submission_id,organization_id,skill_version_id,workflow_id,workflow_snapshot,status,created_at,updated_at) VALUES(?,?,?,?,?,'in_progress',NOW(),NOW())`, submissionID, orgID, versionID, workflowID, completionJSON(steps))
	if err != nil {
		return 0, err
	}
	instanceID, _ := result.LastInsertId()
	for _, st := range steps {
		status := "pending"
		if st.StepOrder == 1 {
			status = "in_progress"
		}
		if _, err = tx.Exec(`INSERT INTO skill_review_steps(instance_id,step_order,name,required_role_id,approval_mode,required_approval,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,NOW(),NOW())`, instanceID, st.StepOrder, st.Name, st.RequiredRoleID, st.ApprovalMode, st.RequiredApproval, status); err != nil {
			return 0, err
		}
	}
	return instanceID, s.advanceAutoSkillReviewStepsV033Hotfix(tx, instanceID)
}

func (s *server) advanceAutoSkillReviewStepsV033Hotfix(tx *sql.Tx, instanceID int64) error {
	for {
		var stepID int64
		var mode, status string
		err := tx.QueryRow("SELECT id,approval_mode,status FROM skill_review_steps WHERE instance_id=? AND status='in_progress' ORDER BY step_order LIMIT 1 FOR UPDATE", instanceID).Scan(&stepID, &mode, &status)
		if err == sql.ErrNoRows {
			return s.finishSkillReviewIfCompleteV033Hotfix(tx, instanceID)
		}
		if err != nil {
			return err
		}
		if mode != "auto" {
			return nil
		}
		if _, err = tx.Exec(`UPDATE skill_review_steps SET status='approved',decision='approved',decision_note='Approved by MockSkillReviewProvider',reviewed_at=NOW(),mock_source='MockSkillReviewProvider',updated_at=NOW() WHERE id=?`, stepID); err != nil {
			return err
		}
		var nextID int64
		err = tx.QueryRow("SELECT id FROM skill_review_steps WHERE instance_id=? AND status='pending' ORDER BY step_order LIMIT 1", instanceID).Scan(&nextID)
		if err == sql.ErrNoRows {
			return s.finishSkillReviewIfCompleteV033Hotfix(tx, instanceID)
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE skill_review_steps SET status='in_progress',updated_at=NOW() WHERE id=?", nextID); err != nil {
			return err
		}
	}
}

func (s *server) finishSkillReviewIfCompleteV033Hotfix(tx *sql.Tx, instanceID int64) error {
	var pending int
	if err := tx.QueryRow("SELECT COUNT(*) FROM skill_review_steps WHERE instance_id=? AND status IN ('pending','in_progress')", instanceID).Scan(&pending); err != nil {
		return err
	}
	if pending > 0 {
		return nil
	}
	var submissionID, versionID int64
	if err := tx.QueryRow("SELECT submission_id,skill_version_id FROM skill_review_instances WHERE id=?", instanceID).Scan(&submissionID, &versionID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE skill_review_instances SET status='approved',updated_at=NOW() WHERE id=?", instanceID); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE skill_submissions SET status='approved',updated_at=NOW() WHERE id=?", submissionID); err != nil {
		return err
	}
	_, err := tx.Exec("UPDATE skill_versions SET status='approved',updated_at=NOW() WHERE id=?", versionID)
	return err
}

func (s *server) skillSubmissionTimelineV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	submissionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var instanceID int64
	var status string
	var workflowID sql.NullInt64
	err := s.db.QueryRow("SELECT id,status,workflow_id FROM skill_review_instances WHERE submission_id=?", submissionID).Scan(&instanceID, &status, &workflowID)
	if err == sql.ErrNoRows {
		c.JSON(200, gin.H{"data": gin.H{"submission_id": submissionID, "instance": nil, "steps": []gin.H{}}})
		return
	}
	if err != nil {
		failCode(c, 500, "skill.review_timeline_failed", nil)
		return
	}
	rows, err := s.db.Query(`SELECT id,step_order,name,required_role_id,approval_mode,required_approval,status,decision,decision_note,reviewed_by,reviewed_at,mock_source FROM skill_review_steps WHERE instance_id=? ORDER BY step_order`, instanceID)
	if err != nil {
		failCode(c, 500, "skill.review_timeline_failed", nil)
		return
	}
	defer rows.Close()
	steps := []gin.H{}
	for rows.Next() {
		var id, order int64
		var name, mode, st string
		var role sql.NullInt64
		var required bool
		var decision, note, mock sql.NullString
		var by sql.NullInt64
		var reviewed sql.NullTime
		if rows.Scan(&id, &order, &name, &role, &mode, &required, &st, &decision, &note, &by, &reviewed, &mock) == nil {
			step := gin.H{"id": id, "step_order": order, "name": name, "approval_mode": mode, "required_approval": required, "status": st}
			if role.Valid {
				step["required_role_id"] = role.Int64
			}
			if decision.Valid {
				step["decision"] = decision.String
			}
			if note.Valid {
				step["decision_note"] = note.String
			}
			if by.Valid {
				step["reviewed_by"] = by.Int64
			}
			if reviewed.Valid {
				step["reviewed_at"] = reviewed.Time
			}
			if mock.Valid {
				step["mock_source"] = mock.String
			}
			steps = append(steps, step)
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"submission_id": submissionID, "instance": gin.H{"id": instanceID, "status": status, "workflow_id": workflowID}, "steps": steps}})
}

func (s *server) decideSkillReviewStepV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	submissionID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		StepID   int64  `json:"step_id"`
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if c.ShouldBindJSON(&req) != nil || req.StepID == 0 {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	if req.Decision != "approved" && req.Decision != "rejected" && req.Decision != "changes_requested" {
		failCode(c, 400, "skill.review_invalid_decision", nil)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "skill.review_decision_failed", nil)
		return
	}
	defer tx.Rollback()
	var instanceID int64
	var instanceStatus string
	if err = tx.QueryRow("SELECT id,status FROM skill_review_instances WHERE submission_id=? FOR UPDATE", submissionID).Scan(&instanceID, &instanceStatus); err != nil {
		failCode(c, 404, "skill.review_instance_not_found", nil)
		return
	}
	if instanceStatus != "in_progress" {
		failCode(c, 409, "skill.review_not_open", nil)
		return
	}
	var stepStatus, mode string
	var requiredRole sql.NullInt64
	if err = tx.QueryRow("SELECT status,approval_mode,required_role_id FROM skill_review_steps WHERE id=? AND instance_id=? FOR UPDATE", req.StepID, instanceID).Scan(&stepStatus, &mode, &requiredRole); err != nil {
		failCode(c, 404, "skill.review_step_not_found", nil)
		return
	}
	if stepStatus != "in_progress" || mode != "manual" {
		failCode(c, 409, "skill.review_step_not_actionable", nil)
		return
	}
	if requiredRole.Valid && !s.userHasRoleV033(currentUserID(c), requiredRole.Int64) {
		failCode(c, 403, "skill.review_reviewer_role_required", nil)
		return
	}
	if _, err = tx.Exec(`UPDATE skill_review_steps SET status=?,decision=?,decision_note=?,reviewed_by=?,reviewed_at=NOW(),updated_at=NOW() WHERE id=?`, req.Decision, req.Decision, strings.TrimSpace(req.Note), currentUserID(c), req.StepID); err != nil {
		failCode(c, 500, "skill.review_decision_failed", nil)
		return
	}
	if req.Decision != "approved" {
		if _, err = tx.Exec("UPDATE skill_review_instances SET status=?,updated_at=NOW() WHERE id=?", req.Decision, instanceID); err != nil {
			failCode(c, 500, "skill.review_decision_failed", nil)
			return
		}
		if _, err = tx.Exec("UPDATE skill_submissions SET status=?,updated_at=NOW() WHERE id=?", req.Decision, submissionID); err != nil {
			failCode(c, 500, "skill.review_decision_failed", nil)
			return
		}
	}
	if req.Decision == "approved" {
		var nextID int64
		err = tx.QueryRow("SELECT id FROM skill_review_steps WHERE instance_id=? AND status='pending' ORDER BY step_order LIMIT 1", instanceID).Scan(&nextID)
		if err == nil {
			_, err = tx.Exec("UPDATE skill_review_steps SET status='in_progress',updated_at=NOW() WHERE id=?", nextID)
		} else if err == sql.ErrNoRows {
			err = s.finishSkillReviewIfCompleteV033Hotfix(tx, instanceID)
		}
		if err != nil {
			failCode(c, 500, "skill.review_decision_failed", nil)
			return
		}
		if err = s.advanceAutoSkillReviewStepsV033Hotfix(tx, instanceID); err != nil {
			failCode(c, 500, "skill.review_decision_failed", nil)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "skill.review_decision_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "skill.review_decided", "skill_submission", submissionID, "success", "high", 65, "Skill review decision recorded", gin.H{"step_id": req.StepID, "decision": req.Decision})
	c.JSON(200, gin.H{"data": gin.H{"submission_id": submissionID, "decision": req.Decision}})
}

func (s *server) userHasRoleV033(uid, roleID int64) bool {
	var count int
	return s.db.QueryRow("SELECT COUNT(*) FROM role_bindings WHERE user_id=? AND role_id=?", uid, roleID).Scan(&count) == nil && count > 0
}

func (s *server) legacySkillReviewV033Hotfix(c *gin.Context) {
	failCode(c, 409, "skill.workflow_decision_required", nil)
}

// --- Profile specific workspace skills ---------------------------------------

func (s *server) workspaceSkillMarketV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	profiles := s.workspaceProfilesV033(uid)
	installedRows, err := s.db.Query(`SELECT psi.id,psi.profile_id,p.display_name,psi.skill_id,s.name,psi.skill_version_id,COALESCE(sv.version,''),psi.status,psi.runtime_apply_status,s.latest_version,psi.created_at
		FROM profile_skill_installs psi JOIN profiles p ON p.id=psi.profile_id JOIN skills s ON s.id=psi.skill_id LEFT JOIN skill_versions sv ON sv.id=psi.skill_version_id
		WHERE p.user_id=? ORDER BY psi.created_at DESC`, uid)
	if err != nil {
		failCode(c, 500, "workspace.skills_failed", nil)
		return
	}
	defer installedRows.Close()
	installed := []gin.H{}
	updates := []gin.H{}
	for installedRows.Next() {
		var id, profileID, skillID, versionID int64
		var profile, name, version, status, applyStatus, latest string
		var created time.Time
		if installedRows.Scan(&id, &profileID, &profile, &skillID, &name, &versionID, &version, &status, &applyStatus, &latest, &created) == nil {
			item := gin.H{"id": id, "profile_id": profileID, "profile": profile, "skill_id": skillID, "name": name, "skill_version_id": versionID, "version": version, "status": status, "runtime_apply_status": applyStatus, "latest_version": latest, "created_at": created}
			installed = append(installed, item)
			if latest != "" && latest != version {
				updates = append(updates, item)
			}
		}
	}
	available := []gin.H{}
	rows, err := s.db.Query("SELECT s.id,s.name,s.category,s.risk_level,s.latest_version,(SELECT sv.id FROM skill_versions sv WHERE sv.skill_id=s.id AND sv.status='published' ORDER BY sv.id DESC LIMIT 1) FROM skills s WHERE s.status='published' ORDER BY s.name")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var versionID sql.NullInt64
			var name, category, risk, latest string
			if rows.Scan(&id, &name, &category, &risk, &latest, &versionID) != nil {
				continue
			}
			eligible := []gin.H{}
			for _, profile := range profiles {
				pid, _ := profile["id"].(int64)
				if s.skillVisibleToUserV033(uid, pid, id) {
					eligible = append(eligible, gin.H{"profile_id": pid, "profile": profile["display_name"]})
				}
			}
			if len(eligible) > 0 {
				available = append(available, gin.H{"id": id, "name": name, "category": category, "risk_level": risk, "latest_version": latest, "latest_skill_version_id": nullableInt64(versionID), "eligible_profiles": eligible})
			}
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"installed": installed, "available": available, "updates": updates, "profiles": profiles}})
}

func (s *server) workspaceProfilesV033(uid int64) []gin.H {
	rows, err := s.db.Query("SELECT id,display_name,profile_type,status FROM profiles WHERE user_id=? AND status='active' ORDER BY id", uid)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var name, typ, status string
		if rows.Scan(&id, &name, &typ, &status) == nil {
			items = append(items, gin.H{"id": id, "display_name": name, "profile_type": typ, "status": status})
		}
	}
	return items
}

func (s *server) installWorkspaceSkillV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	skillID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req completionInstallSkillRequest
	if c.ShouldBindJSON(&req) != nil || req.ProfileID == 0 || req.SkillVersionID == 0 {
		failCode(c, 400, "workspace.skill_profile_and_version_required", nil)
		return
	}
	if !s.skillVisibleToUserV033(uid, req.ProfileID, skillID) {
		failCode(c, 403, "skill.not_effective_for_profile", nil)
		return
	}
	var versionStatus string
	var versionSkillID int64
	if err := s.db.QueryRow("SELECT skill_id,status FROM skill_versions WHERE id=?", req.SkillVersionID).Scan(&versionSkillID, &versionStatus); err != nil || versionSkillID != skillID || versionStatus != "published" {
		failCode(c, 400, "skill.version_not_published", nil)
		return
	}
	mode, _ := s.effectiveSelfServicePolicy(uid, "install_optional_skill")
	if mode == "disabled" {
		failCode(c, 403, "workspace.skill_install_disabled", nil)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "workspace.skill_install_failed", nil)
		return
	}
	defer tx.Rollback()
	if mode == "approval_required" {
		approvalID, err := s.createWorkspaceSkillInstallApprovalV033(tx, uid, req.ProfileID, skillID, req.SkillVersionID)
		if err != nil {
			failCode(c, 500, "workspace.skill_install_failed", nil)
			return
		}
		if err = tx.Commit(); err != nil {
			failCode(c, 500, "workspace.skill_install_failed", nil)
			return
		}
		c.JSON(202, gin.H{"data": gin.H{"approval_id": approvalID, "status": "pending"}})
		return
	}
	installID, err := s.applyProfileSkillInstallTxV033(tx, uid, req.ProfileID, skillID, req.SkillVersionID, "installed", nil)
	if err != nil {
		failCode(c, 500, "workspace.skill_install_failed", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "workspace.skill_install_failed", nil)
		return
	}
	s.writeAudit(uid, "workspace.skill_installed", "skill", skillID, "success", "medium", 30, "Optional skill installed to profile", gin.H{"profile_id": req.ProfileID, "skill_version_id": req.SkillVersionID, "install_id": installID})
	c.JSON(201, gin.H{"data": gin.H{"id": installID, "status": "installed", "runtime_apply_status": "pending_mock_apply"}})
}

func (s *server) applyProfileSkillInstallTxV033(tx *sql.Tx, uid, profileID, skillID, versionID int64, status string, approvalID *int64) (int64, error) {
	var owner int64
	if err := tx.QueryRow("SELECT user_id FROM profiles WHERE id=? FOR UPDATE", profileID).Scan(&owner); err != nil || owner != uid {
		return 0, fmt.Errorf("profile ownership mismatch")
	}
	result, err := tx.Exec(`INSERT INTO profile_skill_installs(profile_id,skill_id,skill_version_id,status,runtime_apply_status,approval_request_id,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,NOW(),NOW()) ON DUPLICATE KEY UPDATE skill_version_id=VALUES(skill_version_id),status=VALUES(status),runtime_apply_status=VALUES(runtime_apply_status),approval_request_id=VALUES(approval_request_id),updated_at=NOW()`, profileID, skillID, versionID, status, "pending_mock_apply", approvalID, uid)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *server) createWorkspaceSkillInstallApprovalV033(tx *sql.Tx, uid, profileID, skillID, versionID int64) (int64, error) {
	metadata := completionJSON(gin.H{"profile_id": profileID, "skill_id": skillID, "skill_version_id": versionID, "apply_mode": "mock_runtime"})
	result, err := tx.Exec(`INSERT INTO approval_requests(type,requester_user_id,resource_type,resource_id,status,risk_level,metadata,created_at) VALUES('skill_install',?,'skill',?,'pending','medium',?,NOW())`, uid, skillID, metadata)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec("INSERT INTO approval_steps(approval_request_id,step_order,status,created_at) VALUES(?,1,'pending',NOW())", id); err != nil {
		return 0, err
	}
	_, err = tx.Exec("INSERT INTO profile_skill_installs(profile_id,skill_id,skill_version_id,status,runtime_apply_status,approval_request_id,created_by,created_at,updated_at) VALUES(?,?,?,'pending_approval','not_applied',?,?,NOW(),NOW()) ON DUPLICATE KEY UPDATE skill_version_id=VALUES(skill_version_id),status='pending_approval',approval_request_id=VALUES(approval_request_id),updated_at=NOW()", profileID, skillID, versionID, id, uid)
	return id, err
}

// --- Workspace knowledge and profile/runtime views ---------------------------

func (s *server) workspaceKnowledgeV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	rows, err := s.db.Query("SELECT id,name,description,status FROM knowledge_bases WHERE organization_id=? AND status='active' ORDER BY name", s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "workspace.knowledge_failed", nil)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var name, description, status string
		if rows.Scan(&id, &name, &description, &status) != nil {
			continue
		}
		level := s.effectiveKnowledgeAccessV033(uid, id)
		if level == "none" {
			continue
		}
		items = append(items, gin.H{"id": id, "name": name, "description": description, "status": status, "access_level": level})
	}
	c.JSON(200, gin.H{"data": items})
}

func (s *server) workspaceKnowledgeItemsV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	level := s.effectiveKnowledgeAccessV033(uid, kbID)
	if level == "none" {
		failCode(c, 404, "knowledge_base.not_found", nil)
		return
	}
	rows, err := s.db.Query(`SELECT id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,updated_at FROM knowledge_items WHERE knowledge_base_id=? AND status IN ('published','active') ORDER BY updated_at DESC`, kbID)
	if err != nil {
		failCode(c, 500, "workspace.knowledge_failed", nil)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var typ, title, content, question, answer, purpose, pre, steps, notes, tags, status string
		var updated time.Time
		if rows.Scan(&id, &typ, &title, &content, &question, &answer, &purpose, &pre, &steps, &notes, &tags, &status, &updated) == nil {
			item := gin.H{"id": id, "type": typ, "title": title, "tags": tags, "status": status, "updated_at": updated}
			if level != "query_only" {
				item["content"] = content
				item["question"] = question
				item["answer"] = answer
				item["purpose"] = purpose
				item["prerequisites"] = pre
				item["steps"] = steps
				item["notes"] = notes
			}
			items = append(items, item)
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"access_level": level, "items": items}})
}

func (s *server) workspaceAgentGroupsV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	runtimeRows, err := s.db.Query(`SELECT id,runtime_id,desired_status,observed_status,host_id FROM runtimes WHERE user_id=? ORDER BY id`, uid)
	if err != nil {
		failCode(c, 500, "workspace.runtime_failed", nil)
		return
	}
	defer runtimeRows.Close()
	groups := []gin.H{}
	for runtimeRows.Next() {
		var runtimeID int64
		var name, desired, observed string
		var host sql.NullInt64
		if runtimeRows.Scan(&runtimeID, &name, &desired, &observed, &host) != nil {
			continue
		}
		profiles := []gin.H{}
		rows, _ := s.db.Query(`SELECT p.id,p.display_name,p.profile_type,p.status,COALESCE(m.name,'Default model') FROM profiles p LEFT JOIN models m ON m.id=p.model_id WHERE p.user_id=? AND p.runtime_id=? ORDER BY p.id`, uid, runtimeID)
		if rows != nil {
			for rows.Next() {
				var id int64
				var display, typ, status, model string
				if rows.Scan(&id, &display, &typ, &status, &model) == nil {
					profiles = append(profiles, gin.H{"id": id, "display_name": display, "profile_type": typ, "status": status, "model": model})
				}
			}
			rows.Close()
		}
		group := gin.H{"runtime_id": runtimeID, "runtime_name": name, "desired_status": desired, "observed_status": observed, "profiles": profiles}
		if host.Valid {
			group["runtime_host_id"] = host.Int64
		}
		groups = append(groups, group)
	}
	c.JSON(200, gin.H{"data": groups})
}

func (s *server) updateWorkspaceAgentConfigurationV033Hotfix(c *gin.Context) {
	s.updateProfileConfigurationV033Hotfix(c)
}

// --- Conversation model authority, regeneration candidates ------------------

func (s *server) conversationModelsV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	conversationID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	profileID, ok := s.conversationProfileForUserV033(uid, conversationID)
	if !ok {
		failCode(c, 404, "conversation.not_found", nil)
		return
	}
	items := s.allowedModelsForProfileV033(uid, profileID)
	c.JSON(200, gin.H{"data": items})
}

func (s *server) allowedModelsForProfileV033(uid, profileID int64) []gin.H {
	rows, err := s.db.Query(`SELECT m.id,m.name,COALESCE(pm.upstream_model,m.upstream_model,' ') FROM models m LEFT JOIN model_providers mp ON mp.id=m.provider_id LEFT JOIN provider_models pm ON pm.id=m.provider_model_id WHERE m.status='active' AND m.user_selectable=1 AND (m.provider_id IS NULL OR mp.status='active') ORDER BY m.name`)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var name, upstream string
		if rows.Scan(&id, &name, &upstream) == nil && s.profileModelAllowedV033(uid, profileID, id) {
			items = append(items, gin.H{"id": id, "name": name, "upstream_model": upstream})
		}
	}
	return items
}

func (s *server) conversationProfileForUserV033(uid, conversationID int64) (int64, bool) {
	var profileID int64
	err := s.db.QueryRow(`SELECT profile_id FROM chat_conversations WHERE id=? AND user_id=?`, conversationID, uid).Scan(&profileID)
	return profileID, err == nil
}

func (s *server) setConversationModelV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	conversationID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	profileID, ok := s.conversationProfileForUserV033(uid, conversationID)
	if !ok {
		failCode(c, 404, "conversation.not_found", nil)
		return
	}
	var req struct {
		ModelID int64 `json:"model_id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ModelID == 0 {
		failCode(c, 400, "validation.invalid_payload", nil)
		return
	}
	if !s.profileModelAllowedV033(uid, profileID, req.ModelID) {
		failCode(c, 403, "model.not_effective_for_profile", nil)
		return
	}
	if mode, _ := s.effectiveSelfServicePolicy(uid, "change_model"); mode == "disabled" {
		failCode(c, 403, "workspace.model_change_disabled", nil)
		return
	}
	if _, err := s.db.Exec("UPDATE chat_conversations SET model_override_id=?,updated_at=NOW() WHERE id=? AND user_id=?", req.ModelID, conversationID, uid); err != nil {
		failCode(c, 500, "conversation.model_update_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"model_id": req.ModelID}})
}

func (s *server) resolveConversationModelV033(uid, conversationID int64) (int64, error) {
	var profileID int64
	var override sql.NullInt64
	if err := s.db.QueryRow("SELECT profile_id,model_override_id FROM chat_conversations WHERE id=? AND user_id=?", conversationID, uid).Scan(&profileID, &override); err != nil {
		return 0, err
	}
	if override.Valid && s.profileModelAllowedV033(uid, profileID, override.Int64) {
		return override.Int64, nil
	}
	var model sql.NullInt64
	if err := s.db.QueryRow("SELECT model_id FROM profiles WHERE id=?", profileID).Scan(&model); err == nil && model.Valid && s.profileModelAllowedV033(uid, profileID, model.Int64) {
		return model.Int64, nil
	}
	models := s.allowedModelsForProfileV033(uid, profileID)
	if len(models) == 0 {
		return 0, fmt.Errorf("no effective model")
	}
	return models[0]["id"].(int64), nil
}

func (s *server) createConversationMessageV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	conversationID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, ok := s.conversationProfileForUserV033(uid, conversationID); !ok {
		failCode(c, 404, "conversation.not_found", nil)
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Content) == "" {
		failCode(c, 400, "validation.message_content_required", nil)
		return
	}
	modelID, err := s.resolveConversationModelV033(uid, conversationID)
	if err != nil {
		failCode(c, 409, "workspace.no_effective_model", nil)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "conversation.message_failed", nil)
		return
	}
	defer tx.Rollback()
	userResult, err := tx.Exec(`INSERT INTO chat_messages(conversation_id,role,content,metadata,selected_for_context,created_at) VALUES(?,'user',?,?,1,NOW())`, conversationID, strings.TrimSpace(req.Content), completionJSON(gin.H{"model_id": modelID}))
	if err != nil {
		failCode(c, 500, "conversation.message_failed", nil)
		return
	}
	userID, _ := userResult.LastInsertId()
	reply := fmt.Sprintf("[MockChatProvider · model %d] I received: %s", modelID, strings.TrimSpace(req.Content))
	assistantResult, err := tx.Exec(`INSERT INTO chat_messages(conversation_id,role,content,metadata,selected_for_context,created_at) VALUES(?,'assistant',?,?,1,NOW())`, conversationID, reply, completionJSON(gin.H{"model_id": modelID, "source_user_message_id": userID}))
	if err != nil {
		failCode(c, 500, "conversation.message_failed", nil)
		return
	}
	assistantID, _ := assistantResult.LastInsertId()
	if _, err = tx.Exec("UPDATE chat_conversations SET updated_at=NOW() WHERE id=?", conversationID); err != nil {
		failCode(c, 500, "conversation.message_failed", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "conversation.message_failed", nil)
		return
	}
	c.JSON(201, gin.H{"data": gin.H{"user_message_id": userID, "assistant_message_id": assistantID, "model_id": modelID}})
}

func (s *server) listConversationMessagesV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	conversationID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if _, ok := s.conversationProfileForUserV033(uid, conversationID); !ok {
		failCode(c, 404, "conversation.not_found", nil)
		return
	}
	rows, err := s.db.Query(`SELECT id,role,content,metadata,regenerated_from_message_id,selected_for_context,created_at FROM chat_messages WHERE conversation_id=? ORDER BY id`, conversationID)
	if err != nil {
		failCode(c, 500, "conversation.messages_failed", nil)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var role, content, metadata string
		var root sql.NullInt64
		var selected bool
		var created time.Time
		if rows.Scan(&id, &role, &content, &metadata, &root, &selected, &created) == nil {
			item := gin.H{"id": id, "role": role, "content": content, "metadata": json.RawMessage(metadata), "selected_for_context": selected, "created_at": created}
			if root.Valid {
				item["regenerated_from_message_id"] = root.Int64
			}
			items = append(items, item)
		}
	}
	c.JSON(200, gin.H{"data": items})
}

func (s *server) regenerateWorkspaceMessageV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	messageID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var conversationID int64
	var role string
	if err := s.db.QueryRow(`SELECT cm.conversation_id,cm.role FROM chat_messages cm JOIN chat_conversations cc ON cc.id=cm.conversation_id WHERE cm.id=? AND cc.user_id=?`, messageID, uid).Scan(&conversationID, &role); err != nil || role != "assistant" {
		failCode(c, 404, "conversation.message_not_found", nil)
		return
	}
	modelID, err := s.resolveConversationModelV033(uid, conversationID)
	if err != nil {
		failCode(c, 409, "workspace.no_effective_model", nil)
		return
	}
	var source string
	if err = s.db.QueryRow(`SELECT content FROM chat_messages WHERE conversation_id=? AND role='user' AND id<? ORDER BY id DESC LIMIT 1`, conversationID, messageID).Scan(&source); err != nil {
		failCode(c, 409, "conversation.regeneration_source_not_found", nil)
		return
	}
	var root int64
	_ = s.db.QueryRow("SELECT COALESCE(regenerated_from_message_id,id) FROM chat_messages WHERE id=?", messageID).Scan(&root)
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "conversation.regenerate_failed", nil)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE chat_messages SET selected_for_context=0 WHERE id=? OR regenerated_from_message_id=?", root, root); err != nil {
		failCode(c, 500, "conversation.regenerate_failed", nil)
		return
	}
	reply := fmt.Sprintf("[MockChatProvider · regenerated with model %d] I received: %s", modelID, source)
	result, err := tx.Exec(`INSERT INTO chat_messages(conversation_id,role,content,metadata,regenerated_from_message_id,selected_for_context,created_at) VALUES(?,'assistant',?,?,?,?,NOW())`, conversationID, reply, completionJSON(gin.H{"model_id": modelID, "source_user_content": source, "candidate_root_id": root}), root, true)
	if err != nil {
		failCode(c, 500, "conversation.regenerate_failed", nil)
		return
	}
	id, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "conversation.regenerate_failed", nil)
		return
	}
	c.JSON(201, gin.H{"data": gin.H{"id": id, "regenerated_from_message_id": root, "model_id": modelID}})
}

func (s *server) selectWorkspaceMessageCandidateV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	messageID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var conversationID, root int64
	var role string
	if err := s.db.QueryRow(`SELECT cm.conversation_id,COALESCE(cm.regenerated_from_message_id,cm.id),cm.role FROM chat_messages cm JOIN chat_conversations cc ON cc.id=cm.conversation_id WHERE cm.id=? AND cc.user_id=?`, messageID, uid).Scan(&conversationID, &root, &role); err != nil || role != "assistant" {
		failCode(c, 404, "conversation.message_not_found", nil)
		return
	}
	_, err := s.db.Exec("UPDATE chat_messages SET selected_for_context=CASE WHEN id=? THEN 1 ELSE 0 END WHERE id=? OR regenerated_from_message_id=?", messageID, root, root)
	if err != nil {
		failCode(c, 500, "conversation.candidate_select_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": messageID, "conversation_id": conversationID, "selected_for_context": true}})
}

func (s *server) workspaceMessageFeedbackV033Hotfix(c *gin.Context) {
	uid := currentUserID(c)
	messageID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Rating  string `json:"rating"`
		Comment string `json:"comment"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "validation.invalid_feedback", nil)
		return
	}
	if req.Rating == "positive" {
		req.Rating = "up"
	}
	if req.Rating == "negative" {
		req.Rating = "down"
	}
	if req.Rating != "up" && req.Rating != "down" {
		failCode(c, 400, "validation.invalid_feedback", nil)
		return
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM chat_messages cm JOIN chat_conversations cc ON cc.id=cm.conversation_id WHERE cm.id=? AND cc.user_id=?`, messageID, uid).Scan(&count); err != nil || count == 0 {
		failCode(c, 404, "conversation.message_not_found", nil)
		return
	}
	_, err := s.db.Exec(`INSERT INTO message_feedback(message_id,user_id,rating,comment,created_at) VALUES(?,?,?,?,NOW()) ON DUPLICATE KEY UPDATE rating=VALUES(rating),comment=VALUES(comment),created_at=NOW()`, messageID, uid, req.Rating, strings.TrimSpace(req.Comment))
	if err != nil {
		failCode(c, 500, "conversation.feedback_failed", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"message_id": messageID, "rating": req.Rating}})
}

// A read-only response compiler used by workspace Markdown previews.  It is
// intentionally not an HTML renderer: clients can render the returned text
// without accepting executable HTML from imported documents.
func completionMarkdownPlainText(content string) string {
	return string(bytes.TrimSpace([]byte(content)))
}
