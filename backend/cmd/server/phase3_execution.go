package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type executionRequest struct {
	UserID     int64    `json:"user_id"`
	ProfileID  int64    `json:"profile_id"`
	RuntimeID  int64    `json:"runtime_id"`
	ModelID    int64    `json:"model_id"`
	Skills     []string `json:"skills"`
	Tools      []string `json:"tools"`
	RiskLevel  string   `json:"risk_level"`
	RiskReason string   `json:"risk_reason"`
}

func (s *server) createExecution(c *gin.Context) {
	if !s.requirePermission(c, "execution.create") {
		return
	}
	var req executionRequest
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "execution.invalid_request", nil)
		return
	}
	if req.UserID == 0 {
		req.UserID = currentUserID(c)
	}
	if req.Skills == nil {
		req.Skills = []string{}
	}
	if req.Tools == nil {
		req.Tools = []string{}
	}
	if req.UserID != currentUserID(c) && !s.requirePermission(c, "execution.manage") {
		return
	}
	var orgID, deptID int64
	if s.db.QueryRow("SELECT organization_id,COALESCE(department_id,0) FROM users WHERE id=?", req.UserID).Scan(&orgID, &deptID) != nil {
		failCode(c, 404, "user.not_found", nil)
		return
	}
	if req.RuntimeID == 0 {
		_ = s.db.QueryRow("SELECT id FROM runtimes WHERE user_id=?", req.UserID).Scan(&req.RuntimeID)
	}
	if req.ProfileID > 0 && req.RuntimeID == 0 {
		_ = s.db.QueryRow("SELECT r.id FROM runtimes r JOIN profiles p ON p.user_id=r.user_id WHERE p.id=?", req.ProfileID).Scan(&req.RuntimeID)
	}
	if req.ModelID == 0 && req.ProfileID > 0 {
		_ = s.db.QueryRow("SELECT model_id FROM profiles WHERE id=?", req.ProfileID).Scan(&req.ModelID)
	}
	risk := NewRiskEvaluator().Evaluate("execution.request", "execution")
	if req.RiskLevel == "high" || req.RiskLevel == "critical" {
		risk = RiskResult{Level: req.RiskLevel, Score: 75, Reason: req.RiskReason}
		if risk.Level == "critical" {
			risk.Score = 100
		}
	}
	if req.RiskReason != "" {
		risk.Reason = req.RiskReason
	}
	skillJSON, _ := json.Marshal(req.Skills)
	toolJSON, _ := json.Marshal(req.Tools)
	executionID := fmt.Sprintf("exec-%d-%d", time.Now().UnixNano(), req.UserID)
	status := "completed"
	requiresApproval := risk.Level == "high" || risk.Level == "critical"
	var approvalID any
	if requiresApproval {
		status = "waiting_approval"
		id, err := s.createApproval(c, "execution", "execution", 0, risk.Level, risk.Reason, gin.H{"execution_id": executionID, "user_id": req.UserID, "profile_id": req.ProfileID, "runtime_id": req.RuntimeID})
		if err != nil {
			failCode(c, 500, "approval.create_failed", nil)
			return
		}
		approvalID = id
	}
	var aid int64
	if v, ok := approvalID.(int64); ok {
		aid = v
	}
	var result sql.Result
	var err error
	if aid > 0 {
		result, err = s.db.Exec(`INSERT INTO executions(execution_id,organization_id,department_id,user_id,profile_id,runtime_id,model_id,skill_ids,tool_names,status,risk_level,risk_score,risk_reason,requires_approval,approval_request_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, executionID, orgID, nullableID(deptID), req.UserID, nullableID(req.ProfileID), nullableID(req.RuntimeID), nullableID(req.ModelID), string(skillJSON), string(toolJSON), status, risk.Level, risk.Score, risk.Reason, requiresApproval, aid)
	} else {
		result, err = s.db.Exec(`INSERT INTO executions(execution_id,organization_id,department_id,user_id,profile_id,runtime_id,model_id,skill_ids,tool_names,status,started_at,finished_at,duration_ms,input_tokens,output_tokens,cost,risk_level,risk_score,risk_reason,requires_approval) VALUES(?,?,?,?,?,?,?,?,?,'completed',UTC_TIMESTAMP(),UTC_TIMESTAMP(),1250,420,180,0.0024,?,?,?,FALSE)`, executionID, orgID, nullableID(deptID), req.UserID, nullableID(req.ProfileID), nullableID(req.RuntimeID), nullableID(req.ModelID), string(skillJSON), string(toolJSON), risk.Level, risk.Score, risk.Reason)
	}
	if err != nil {
		failCode(c, 400, "execution.create_failed", nil)
		return
	}
	id, _ := result.LastInsertId()
	_, _ = s.db.Exec("INSERT INTO execution_events(execution_id,event_type,message) VALUES(?,?,?)", id, "request", risk.Reason)
	if status == "completed" {
		_, _ = s.db.Exec("INSERT INTO execution_events(execution_id,event_type,message) VALUES(?,?,?)", id, "completed", "Mock execution completed")
		_, _ = s.db.Exec(`INSERT INTO usage_events(organization_id,department_id,user_id,profile_id,execution_id,model_id,runtime_id,token_input,token_output,requests,executions,latency_ms) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, orgID, nullableID(deptID), req.UserID, nullableID(req.ProfileID), executionID, nullableID(req.ModelID), nullableID(req.RuntimeID), 420, 180, 1, 1, 1250)
	}
	s.auditControlPlane(c, "execution.request", "Execution Requested", "Agent Profiles", "execution", id, "success", gin.H{"profile_id": req.ProfileID, "runtime_id": req.RuntimeID, "risk_level": risk.Level}, &risk)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "execution_id": executionID, "status": status, "risk_level": risk.Level, "risk_reason": risk.Reason, "approval_request_id": approvalID}})
}

func (s *server) listExecutions(c *gin.Context) {
	if !s.requirePermission(c, "execution.read") {
		return
	}
	query := `SELECT e.id,e.execution_id,e.user_id,COALESCE(u.display_name,''),COALESCE(d.name,''),COALESCE(p.display_name,''),COALESCE(r.runtime_id,''),COALESCE(m.display_name,''),e.skill_ids,e.tool_names,e.status,e.started_at,e.finished_at,e.duration_ms,e.input_tokens,e.output_tokens,e.cost,e.risk_level,e.risk_reason,e.requires_approval,COALESCE(e.approval_request_id,0),e.created_at FROM executions e JOIN users u ON u.id=e.user_id LEFT JOIN departments d ON d.id=e.department_id LEFT JOIN profiles p ON p.id=e.profile_id LEFT JOIN runtimes r ON r.id=e.runtime_id LEFT JOIN models m ON m.id=e.model_id WHERE 1=1`
	args := []any{}
	for key, expr := range map[string]string{"user_id": "e.user_id=?", "profile_id": "e.profile_id=?", "runtime_id": "e.runtime_id=?", "risk_level": "e.risk_level=?", "status": "e.status=?"} {
		if v := c.Query(key); v != "" {
			query += " AND " + expr
			args = append(args, v)
		}
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		query += " AND (e.execution_id LIKE ? OR u.display_name LIKE ? OR p.display_name LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	query += " ORDER BY e.created_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "executions.load_failed", nil)
		return
	}
	defer rows.Close()
	c.JSON(200, gin.H{"data": scanExecutionRowsFull(rows)})
}

func scanExecutionRows(rows *sql.Rows) []gin.H {
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var eid, profile, model, status, risk string
		var approval bool
		var started, finished sql.NullTime
		var in, outTokens int64
		if rows.Scan(&id, &eid, &profile, &model, &status, &risk, &approval, &started, &finished, &in, &outTokens) == nil {
			out = append(out, gin.H{"id": id, "execution_id": eid, "profile": profile, "model": model, "status": status, "risk_level": risk, "requires_approval": approval, "started_at": nullableTime(started), "finished_at": nullableTime(finished), "input_tokens": in, "output_tokens": outTokens})
		}
	}
	return out
}
func scanExecutionRowsFull(rows *sql.Rows) []gin.H {
	out := []gin.H{}
	for rows.Next() {
		var id, uid, duration, in, outTokens, approvalID int64
		var eid, user, dept, profile, runtime, model, skills, tools, status, risk, reason, created string
		var cost float64
		var approval bool
		var started, finished sql.NullTime
		if rows.Scan(&id, &eid, &uid, &user, &dept, &profile, &runtime, &model, &skills, &tools, &status, &started, &finished, &duration, &in, &outTokens, &cost, &risk, &reason, &approval, &approvalID, &created) == nil {
			out = append(out, gin.H{"id": id, "execution_id": eid, "user_id": uid, "user": user, "department": dept, "profile": profile, "runtime": runtime, "model": model, "skills": phase3JSON(skills), "tools": phase3JSON(tools), "status": status, "started_at": nullableTime(started), "finished_at": nullableTime(finished), "duration_ms": duration, "input_tokens": in, "output_tokens": outTokens, "cost": cost, "risk_level": risk, "risk_reason": reason, "requires_approval": approval, "approval_request_id": approvalID, "created_at": created})
		}
	}
	return out
}
func nullableTime(v sql.NullTime) any {
	if v.Valid {
		return v.Time.UTC().Format(time.RFC3339)
	}
	return nil
}

func (s *server) executionDetail(c *gin.Context) {
	if !s.requirePermission(c, "execution.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var row gin.H
	var eid, user, dept, profile, runtime, model, skills, tools, status, risk, reason, created string
	var uid, duration, in, outTokens, approvalID int64
	var cost float64
	var approval bool
	var started, finished sql.NullTime
	if s.db.QueryRow(`SELECT e.execution_id, e.user_id,COALESCE(u.display_name,''),COALESCE(d.name,''),COALESCE(p.display_name,''),COALESCE(r.runtime_id,''),COALESCE(m.display_name,''),e.skill_ids,e.tool_names,e.status,e.started_at,e.finished_at,e.duration_ms,e.input_tokens,e.output_tokens,e.cost,e.risk_level,e.risk_reason,e.requires_approval,COALESCE(e.approval_request_id,0),e.created_at FROM executions e JOIN users u ON u.id=e.user_id LEFT JOIN departments d ON d.id=e.department_id LEFT JOIN profiles p ON p.id=e.profile_id LEFT JOIN runtimes r ON r.id=e.runtime_id LEFT JOIN models m ON m.id=e.model_id WHERE e.id=?`, id).Scan(&eid, &uid, &user, &dept, &profile, &runtime, &model, &skills, &tools, &status, &started, &finished, &duration, &in, &outTokens, &cost, &risk, &reason, &approval, &approvalID, &created) != nil {
		failCode(c, 404, "execution.not_found", nil)
		return
	}
	timeline := []gin.H{}
	events, _ := s.db.Query("SELECT event_type,message,created_at FROM execution_events WHERE execution_id=? ORDER BY created_at", id)
	if events != nil {
		defer events.Close()
		for events.Next() {
			var typ, msg, at string
			if events.Scan(&typ, &msg, &at) == nil {
				timeline = append(timeline, gin.H{"event_type": typ, "message": msg, "created_at": at})
			}
		}
	}
	row = gin.H{"id": id, "execution_id": eid, "user_id": uid, "user": user, "department": dept, "profile": profile, "runtime": runtime, "model": model, "skills": phase3JSON(skills), "tools": phase3JSON(tools), "status": status, "started_at": nullableTime(started), "finished_at": nullableTime(finished), "duration_ms": duration, "input_tokens": in, "output_tokens": outTokens, "cost": cost, "risk_level": risk, "risk_reason": reason, "requires_approval": approval, "approval_request_id": approvalID, "created_at": created, "timeline": timeline, "tabs": []string{"Summary", "Timeline", "Tools", "Skills", "Model Usage", "Risk", "Approval"}}
	c.JSON(200, gin.H{"data": row})
}
