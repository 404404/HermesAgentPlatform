package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/example/hermes-enterprise-platform/backend/internal/providers"
	"github.com/gin-gonic/gin"
)

type knowledgeItemRequest struct {
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Question      string   `json:"question"`
	Answer        string   `json:"answer"`
	Purpose       string   `json:"purpose"`
	Prerequisites string   `json:"prerequisites"`
	Steps         []string `json:"steps"`
	Notes         string   `json:"notes"`
	Tags          []string `json:"tags"`
	Status        string   `json:"status"`
}

func validKnowledgeItemType(value string) bool {
	return value == "background" || value == "qa" || value == "markdown" || value == "procedure"
}

func (s *server) listKnowledgeItems(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.read") {
		return
	}
	kbID, ok := paramID(c, "id")
	if !ok {
		return
	}
	query := `SELECT i.id,i.type,i.title,i.status,COALESCE(u.display_name,''),i.version,i.index_status,i.tags,i.updated_at FROM knowledge_items i LEFT JOIN users u ON u.id=i.owner_user_id WHERE i.knowledge_base_id=?`
	args := []any{kbID}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		query += " AND (i.title LIKE ? OR i.content LIKE ? OR i.question LIKE ? OR i.answer LIKE ?)"
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	if typ := c.Query("type"); typ != "" {
		query += " AND i.type=?"
		args = append(args, typ)
	}
	if status := c.Query("status"); status != "" {
		query += " AND i.status=?"
		args = append(args, status)
	}
	query += " ORDER BY i.updated_at DESC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		failCode(c, 500, "knowledge.items_load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, version int64
		var typ, title, status, owner, index, tags, updated string
		if rows.Scan(&id, &typ, &title, &status, &owner, &version, &index, &tags, &updated) == nil {
			out = append(out, gin.H{"id": id, "type": typ, "title": title, "status": status, "owner": owner, "version": version, "index_status": index, "tags": phase3JSON(tags), "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createKnowledgeItem(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.manage") {
		return
	}
	kbID, ok := paramID(c, "id")
	if !ok {
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
	steps, _ := json.Marshal(req.Steps)
	tags, _ := json.Marshal(req.Tags)
	res, err := s.db.Exec(`INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,owner_user_id,version,index_status) VALUES(?,?,?,?,?,?,?,?,?,?,?, ?,?,1,'not_indexed')`, kbID, req.Type, req.Title, req.Content, req.Question, req.Answer, req.Purpose, req.Prerequisites, string(steps), req.Notes, string(tags), req.Status, currentUserID(c))
	if err != nil {
		failCode(c, 400, "knowledge.item_create_failed", nil)
		return
	}
	id, _ := res.LastInsertId()
	payload := s.knowledgeItemPayload(req)
	_, err = s.db.Exec("INSERT INTO knowledge_item_versions(item_id,version,payload,content_hash,created_by,status) VALUES(?,1,?,?,?,?)", id, payload, sha256Text(payload), currentUserID(c), req.Status)
	if err != nil {
		failCode(c, 400, "knowledge.item_version_failed", nil)
		return
	}
	s.auditControlPlane(c, "knowledge.item.create", "Knowledge Item Created", "Knowledge", "knowledge_item", id, "success", nil, nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "version": 1, "status": req.Status}})
}

func (s *server) knowledgeItemPayload(req knowledgeItemRequest) string {
	if req.Steps == nil {
		req.Steps = []string{}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	b, _ := json.Marshal(req)
	return string(b)
}

func (s *server) knowledgeItemDetail(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var kbID, version int64
	var typ, title, content, question, answer, purpose, prerequisites, steps, notes, tags, status, owner, index, updated string
	if s.db.QueryRow(`SELECT i.knowledge_base_id,i.type,i.title,i.content,i.question,i.answer,i.purpose,i.prerequisites,i.steps,i.notes,i.tags,i.status,COALESCE(u.display_name,''),i.index_status,i.version,i.updated_at FROM knowledge_items i LEFT JOIN users u ON u.id=i.owner_user_id WHERE i.id=?`, id).Scan(&kbID, &typ, &title, &content, &question, &answer, &purpose, &prerequisites, &steps, &notes, &tags, &status, &owner, &index, &version, &updated) != nil {
		failCode(c, 404, "knowledge.item_not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "knowledge_base_id": kbID, "type": typ, "title": title, "content": content, "question": question, "answer": answer, "purpose": purpose, "prerequisites": prerequisites, "steps": phase3JSON(steps), "notes": notes, "tags": phase3JSON(tags), "status": status, "owner": owner, "index_status": index, "version": version, "updated_at": updated, "versions": s.knowledgeItemVersionsData(id)}})
}

func (s *server) updateKnowledgeItem(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req knowledgeItemRequest
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Title) == "" || !validKnowledgeItemType(req.Type) {
		failCode(c, 400, "knowledge.item_invalid", nil)
		return
	}
	if req.Steps == nil {
		req.Steps = []string{}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	payload := s.knowledgeItemPayload(req)
	var version int
	_ = s.db.QueryRow("SELECT version FROM knowledge_items WHERE id=?", id).Scan(&version)
	version++
	steps, _ := json.Marshal(req.Steps)
	tags, _ := json.Marshal(req.Tags)
	if _, err := s.db.Exec(`UPDATE knowledge_items SET type=?,title=?,content=?,question=?,answer=?,purpose=?,prerequisites=?,steps=?,notes=?,tags=?,status='draft',version=?,index_status='not_indexed',updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Type, req.Title, req.Content, req.Question, req.Answer, req.Purpose, req.Prerequisites, string(steps), req.Notes, string(tags), version, id); err != nil {
		failCode(c, 400, "knowledge.item_update_failed", nil)
		return
	}
	if _, err := s.db.Exec("INSERT INTO knowledge_item_versions(item_id,version,payload,content_hash,created_by,status) VALUES(?,?,?,?,?,'draft')", id, version, payload, sha256Text(payload), currentUserID(c)); err != nil {
		failCode(c, 400, "knowledge.item_version_failed", nil)
		return
	}
	s.auditControlPlane(c, "knowledge.item.update", "Knowledge Item Updated", "Knowledge", "knowledge_item", id, "success", gin.H{"version": version}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "version": version, "status": "draft"}})
}

func (s *server) publishKnowledgeItem(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var kbID, version int64
	var payload string
	if s.db.QueryRow("SELECT knowledge_base_id,version FROM knowledge_items WHERE id=?", id).Scan(&kbID, &version) != nil || s.db.QueryRow("SELECT payload FROM knowledge_item_versions WHERE item_id=? AND version=?", id, version).Scan(&payload) != nil {
		failCode(c, 404, "knowledge.item_not_found", nil)
		return
	}
	if _, err := s.db.Exec("UPDATE knowledge_items SET status='published',index_status='pending',updated_at=UTC_TIMESTAMP() WHERE id=?", id); err != nil {
		failCode(c, 400, "knowledge.item_publish_failed", nil)
		return
	}
	_, _ = s.db.Exec("UPDATE knowledge_item_versions SET status='published' WHERE item_id=? AND version=?", id, version)
	_ = (providers.MockKnowledgeProvider{}).IndexDocument(context.Background(), kbID, fmt.Sprintf("item-%d", id), []byte(payload))
	_, _ = s.db.Exec("UPDATE knowledge_items SET index_status='indexed' WHERE id=?", id)
	s.auditControlPlane(c, "knowledge.item.publish", "Knowledge Item Published", "Knowledge", "knowledge_item", id, "success", gin.H{"version": version}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "version": version, "status": "published", "index_status": "indexed"}})
}

func (s *server) deleteKnowledgeItem(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec("DELETE FROM knowledge_items WHERE id=?", id); err != nil {
		failCode(c, 400, "knowledge.item_delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "knowledge.item.delete", "Knowledge Item Deleted", "Knowledge", "knowledge_item", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) knowledgeItemVersions(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(200, gin.H{"data": s.knowledgeItemVersionsData(id)})
}

func (s *server) knowledgeItemVersionsData(id int64) []gin.H {
	rows, _ := s.db.Query("SELECT id,version,content_hash,created_by,status,created_at FROM knowledge_item_versions WHERE item_id=? ORDER BY version DESC", id)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var vid, version, creator int64
		var hash, status, created string
		if rows.Scan(&vid, &version, &hash, &creator, &status, &created) == nil {
			out = append(out, gin.H{"id": vid, "version": version, "content_hash": hash, "created_by": creator, "status": status, "created_at": created})
		}
	}
	return out
}

func (s *server) knowledgeItemVersion(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.document.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var itemID, version, creator int64
	var payload, hash, status, created string
	if s.db.QueryRow("SELECT item_id,version,payload,content_hash,created_by,status,created_at FROM knowledge_item_versions WHERE id=?", id).Scan(&itemID, &version, &payload, &hash, &creator, &status, &created) != nil {
		failCode(c, 404, "knowledge.item_version_not_found", nil)
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"id": id, "item_id": itemID, "version": version, "payload": phase3JSON(payload), "content_hash": hash, "created_by": creator, "status": status, "created_at": created}})
}

func (s *server) knowledgeConsumers(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	kbID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var direct int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM knowledge_bindings WHERE knowledge_base_id=?", kbID).Scan(&direct)
	var users, profiles int
	_ = s.db.QueryRow(`SELECT COUNT(DISTINCT u.id) FROM users u WHERE u.status='active' AND (EXISTS(SELECT 1 FROM knowledge_bindings b WHERE b.knowledge_base_id=? AND (b.organization_id=u.organization_id OR b.department_id=u.department_id OR b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=u.id))))`, kbID).Scan(&users)
	_ = s.db.QueryRow(`SELECT COUNT(DISTINCT p.id) FROM profiles p JOIN users u ON u.id=p.user_id WHERE u.status='active' AND (EXISTS(SELECT 1 FROM knowledge_bindings b WHERE b.knowledge_base_id=? AND (b.organization_id=u.organization_id OR b.department_id=u.department_id OR b.profile_id=p.id OR b.role_id IN (SELECT role_id FROM role_bindings WHERE user_id=u.id))))`, kbID).Scan(&profiles)
	c.JSON(200, gin.H{"data": gin.H{"direct_bindings": direct, "effective_user_count": users, "effective_profile_count": profiles}})
}
