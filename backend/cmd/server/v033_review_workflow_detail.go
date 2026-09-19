package main

import (
	"database/sql"
	"strconv"

	"github.com/gin-gonic/gin"
)

// workflow details are read-only and never create review instances.
func (s *server) skillReviewWorkflowDetailV033Hotfix(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var name, description, status string
	if s.db.QueryRow("SELECT name,description,status FROM skill_review_workflows WHERE id=? AND organization_id=?", id, s.currentOrg(c)).Scan(&name, &description, &status) != nil {
		failCode(c, 404, "skill_review.workflow_not_found", nil)
		return
	}
	rows, err := s.db.Query("SELECT step_order,name,required_role_id,approval_mode,required_approval FROM skill_review_workflow_steps WHERE workflow_id=? ORDER BY step_order", id)
	if err != nil {
		failCode(c, 500, "skill_review.workflow_load_failed", nil)
		return
	}
	defer rows.Close()
	steps := []gin.H{}
	for rows.Next() {
		var order int
		var stepName, mode string
		var role sql.NullInt64
		var required bool
		if rows.Scan(&order, &stepName, &role, &mode, &required) == nil {
			item := gin.H{"step_order": order, "name": stepName, "approval_mode": mode, "required_approval": required}
			if role.Valid {
				item["required_role_id"] = role.Int64
			}
			steps = append(steps, item)
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "name": name, "description": description, "status": status, "steps": steps}})
}
