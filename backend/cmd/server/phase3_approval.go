package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
)

// The v2 decision endpoint keeps the original Approval Center API compatible
// while completing the Execution domain transition for execution approvals.
func (s *server) decideApprovalV21(c *gin.Context) {
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
	if _, err := s.db.Exec("UPDATE approval_requests SET status=?,resolved_at=UTC_TIMESTAMP() WHERE id=?", req.Decision, id); err != nil {
		failCode(c, 400, "approval.update_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE approval_steps SET reviewer_id=?,status=?,comment=?,resolved_at=UTC_TIMESTAMP() WHERE approval_request_id=? AND status='pending'", currentUserID(c), req.Decision, req.Comment, id)
	if typ == "execution" {
		var m struct {
			ExecutionID string `json:"execution_id"`
		}
		_ = json.Unmarshal([]byte(metadata), &m)
		if req.Decision == "approved" {
			_, _ = s.db.Exec("UPDATE executions SET status='completed',started_at=COALESCE(started_at,UTC_TIMESTAMP()),finished_at=UTC_TIMESTAMP(),duration_ms=1100 WHERE approval_request_id=?", id)
		} else {
			_, _ = s.db.Exec("UPDATE executions SET status='rejected',finished_at=UTC_TIMESTAMP() WHERE approval_request_id=?", id)
		}
		_ = m
	}
	s.auditControlPlane(c, "approval."+req.Decision, "Approval "+req.Decision, "Approvals", "approval_request", id, "success", gin.H{"comment": req.Comment, "type": typ}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": req.Decision}})
}
