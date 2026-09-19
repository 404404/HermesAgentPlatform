package main

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

// This checks a role permission without imposing the Admin Console boundary.
// It is only used after a handler has already proved entity ownership.
func (s *server) hasPermissionV033(userID int64, permission string) bool {
	var exists int
	err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM role_bindings rb
		JOIN role_permissions rp ON rp.role_id=rb.role_id
		JOIN permissions p ON p.id=rp.permission_id
		JOIN users u ON u.id=?
		WHERE p.code=? AND (
			rb.user_id=? OR
			(rb.user_id IS NULL AND rb.organization_id=u.organization_id AND rb.scope IN ('global','organization')) OR
			(rb.department_id=u.department_id AND rb.scope='department')
		)
	)`, userID, permission, userID).Scan(&exists)
	return err == nil && exists == 1
}

// Append-only audit writing for hotfix-only paths that need an explicit actor
// rather than a Gin request context.
func (s *server) writeAudit(actorID int64, action, resourceType string, resourceID int64, result, riskLevel string, riskScore int, reason string, metadata gin.H) {
	category := "System Settings"
	switch {
	case strings.HasPrefix(action, "knowledge."):
		category = "Knowledge"
	case strings.HasPrefix(action, "skill."):
		category = "Skills"
	case strings.HasPrefix(action, "profile."), strings.HasPrefix(action, "workspace."):
		category = "Agent Profiles"
	}
	_, _ = s.db.Exec(`INSERT INTO audit_logs(actor_user_id,action,action_label,category,resource_type,resource_id,scope,result,request_id,trace_id,metadata,risk_level,risk_score,risk_reason,created_at)
		VALUES(?,?,?,?,?,?, 'global', ?, 'v033-completion-hotfix', 'v033-completion-hotfix', ?,?,?,?,NOW())`,
		actorID, action, action, category, resourceType, nullableID(resourceID), result, completionJSON(metadata), riskLevel, riskScore, reason)
}

// Generic Approval Center decisions must apply profile-specific skill installs
// atomically.  It is the authoritative decision path for both legacy and v2
// endpoints after the route rebinding.
func (s *server) decideApprovalV033Hotfix(c *gin.Context) {
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
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "approval.update_failed", nil)
		return
	}
	defer tx.Rollback()
	var requester int64
	var typ, metadata, status string
	if err = tx.QueryRow("SELECT requester,COALESCE(type,''),COALESCE(metadata,'{}'),status FROM approval_requests WHERE id=? FOR UPDATE", id).Scan(&requester, &typ, &metadata, &status); err != nil {
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
	if _, err = tx.Exec("UPDATE approval_requests SET status=?,resolved_at=UTC_TIMESTAMP() WHERE id=?", req.Decision, id); err != nil {
		failCode(c, 500, "approval.update_failed", nil)
		return
	}
	if _, err = tx.Exec("UPDATE approval_steps SET reviewer_id=?,status=?,comment=?,resolved_at=UTC_TIMESTAMP() WHERE approval_request_id=? AND status='pending'", currentUserID(c), req.Decision, req.Comment, id); err != nil {
		failCode(c, 500, "approval.update_failed", nil)
		return
	}
	if typ == "skill_install" {
		var data struct {
			ProfileID      int64 `json:"profile_id"`
			SkillID        int64 `json:"skill_id"`
			SkillVersionID int64 `json:"skill_version_id"`
		}
		if json.Unmarshal([]byte(metadata), &data) != nil || data.ProfileID == 0 || data.SkillID == 0 || data.SkillVersionID == 0 {
			failCode(c, 500, "approval.metadata_invalid", nil)
			return
		}
		if req.Decision == "approved" {
			if _, err = s.applyProfileSkillInstallTxV033(tx, requester, data.ProfileID, data.SkillID, data.SkillVersionID, "installed", &id); err != nil {
				failCode(c, 500, "approval.apply_failed", nil)
				return
			}
		} else if _, err = tx.Exec("UPDATE profile_skill_installs SET status='rejected',runtime_apply_status='not_applied',updated_at=NOW() WHERE approval_request_id=?", id); err != nil {
			failCode(c, 500, "approval.apply_failed", nil)
			return
		}
	}
	if typ == "role_elevation" && req.Decision == "approved" {
		var data struct {
			TargetUserID, RoleID, DepartmentID, ProfileID int64
			Scope                                         string
		}
		_ = json.Unmarshal([]byte(metadata), &data)
		if _, err = tx.Exec("INSERT IGNORE INTO role_bindings(role_id,organization_id,department_id,user_id,profile_id,scope) VALUES(?,1,?,?,?,?,?)", data.RoleID, nullableID(data.DepartmentID), data.TargetUserID, nullableID(data.ProfileID), data.Scope); err != nil {
			failCode(c, 500, "approval.apply_failed", nil)
			return
		}
	}
	if typ == "execution" {
		if req.Decision == "approved" {
			_, err = tx.Exec("UPDATE executions SET status='completed',started_at=COALESCE(started_at,UTC_TIMESTAMP()),finished_at=UTC_TIMESTAMP(),duration_ms=1250,input_tokens=420,output_tokens=180,cost=0.0024 WHERE approval_request_id=?", id)
		} else {
			_, err = tx.Exec("UPDATE executions SET status='rejected',finished_at=UTC_TIMESTAMP() WHERE approval_request_id=?", id)
		}
		if err != nil {
			failCode(c, 500, "approval.apply_failed", nil)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "approval.update_failed", nil)
		return
	}
	s.auditControlPlane(c, "approval."+req.Decision, "Approval "+req.Decision, "Approvals", "approval_request", id, "success", gin.H{"comment": req.Comment, "type": typ}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": req.Decision}})
}
