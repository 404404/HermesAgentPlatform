package main

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *server) requireKnowledgeContributionV033(c *gin.Context, kbID int64) bool {
	level := s.effectiveKnowledgeAccessV033(currentUserID(c), kbID)
	if level != "contribute" && level != "manage" {
		failCode(c, 403, "knowledge.contribution_denied", nil)
		return false
	}
	return true
}

func (s *server) createWorkspaceKnowledgeItemV033Hotfix(c *gin.Context) {
	kbID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if !s.requireKnowledgeContributionV033(c, kbID) {
		return
	}
	var req knowledgeItemRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" || !validKnowledgeItemType(req.Type) {
		failCode(c, 400, "knowledge.item_invalid", nil)
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "knowledge.item_create_failed", nil)
		return
	}
	defer tx.Rollback()
	id, err := s.completionCreateKnowledgeItemTx(tx, kbID, currentUserID(c), req)
	if err != nil || tx.Commit() != nil {
		failCode(c, 500, "knowledge.item_create_failed", nil)
		return
	}
	s.writeAudit(currentUserID(c), "workspace.knowledge_item_created", "knowledge_item", id, "success", "low", 10, "Knowledge item created from workspace", gin.H{"knowledge_base_id": kbID})
	c.JSON(201, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}

func (s *server) updateWorkspaceKnowledgeItemV033Hotfix(c *gin.Context) {
	itemID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid := currentUserID(c)
	var kbID, ownerID int64
	if err := s.db.QueryRow("SELECT knowledge_base_id,COALESCE(owner_user_id,0) FROM knowledge_items WHERE id=? AND status<>'deleted'", itemID).Scan(&kbID, &ownerID); err != nil {
		failCode(c, 404, "knowledge.item_not_found", nil)
		return
	}
	if ownerID != uid || !s.requireKnowledgeContributionV033(c, kbID) {
		if ownerID != uid {
			failCode(c, 403, "knowledge.item_owner_required", nil)
		}
		return
	}
	var req knowledgeItemRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" || !validKnowledgeItemType(req.Type) {
		failCode(c, 400, "knowledge.item_invalid", nil)
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Steps == nil {
		req.Steps = []string{}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	tx, err := s.db.Begin()
	if err != nil {
		failCode(c, 500, "knowledge.item_update_failed", nil)
		return
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRow("SELECT version FROM knowledge_items WHERE id=? FOR UPDATE", itemID).Scan(&version); err != nil {
		failCode(c, 404, "knowledge.item_not_found", nil)
		return
	}
	steps := completionJSON(req.Steps)
	tags := completionJSON(req.Tags)
	if _, err = tx.Exec(`UPDATE knowledge_items SET type=?,title=?,content=?,question=?,answer=?,purpose=?,prerequisites=?,steps=?,notes=?,tags=?,status=?,version=?,index_status='not_indexed',updated_at=NOW() WHERE id=?`, req.Type, req.Title, req.Content, req.Question, req.Answer, req.Purpose, req.Prerequisites, steps, req.Notes, tags, req.Status, version+1, itemID); err != nil {
		failCode(c, 500, "knowledge.item_update_failed", nil)
		return
	}
	payload := s.knowledgeItemPayload(req)
	if _, err = tx.Exec("INSERT INTO knowledge_item_versions(item_id,version,payload,content_hash,created_by,status) VALUES(?,?,?,?,?,?)", itemID, version+1, payload, sha256Text(payload), uid, req.Status); err != nil {
		failCode(c, 500, "knowledge.item_update_failed", nil)
		return
	}
	if err = tx.Commit(); err != nil {
		failCode(c, 500, "knowledge.item_update_failed", nil)
		return
	}
	s.writeAudit(uid, "workspace.knowledge_item_updated", "knowledge_item", itemID, "success", "low", 10, "Knowledge item updated from workspace", gin.H{"knowledge_base_id": kbID})
	c.JSON(200, gin.H{"data": gin.H{"id": itemID, "version": version + 1}})
}

func (s *server) deleteWorkspaceKnowledgeItemV033Hotfix(c *gin.Context) {
	itemID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid := currentUserID(c)
	var kbID, ownerID int64
	if err := s.db.QueryRow("SELECT knowledge_base_id,COALESCE(owner_user_id,0) FROM knowledge_items WHERE id=? AND status<>'deleted'", itemID).Scan(&kbID, &ownerID); err != nil {
		failCode(c, 404, "knowledge.item_not_found", nil)
		return
	}
	if ownerID != uid || !s.requireKnowledgeContributionV033(c, kbID) {
		if ownerID != uid {
			failCode(c, 403, "knowledge.item_owner_required", nil)
		}
		return
	}
	if _, err := s.db.Exec("UPDATE knowledge_items SET status='deleted',updated_at=NOW() WHERE id=?", itemID); err != nil {
		failCode(c, 500, "knowledge.item_delete_failed", nil)
		return
	}
	s.writeAudit(uid, "workspace.knowledge_item_deleted", "knowledge_item", itemID, "success", "medium", 25, "Knowledge item deleted from workspace", gin.H{"knowledge_base_id": kbID})
	c.JSON(200, gin.H{"data": true})
}

var _ sql.NullInt64
