package main

import (
	"database/sql"
	"encoding/json"
)

// completionCreateKnowledgeItemTx is shared by CSV and multi-file Markdown
// imports.  The main item and its immutable first version commit together.
func (s *server) completionCreateKnowledgeItemTx(tx *sql.Tx, kbID, userID int64, req knowledgeItemRequest) (int64, error) {
	if req.Steps == nil {
		req.Steps = []string{}
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	steps, _ := json.Marshal(req.Steps)
	tags, _ := json.Marshal(req.Tags)
	result, err := tx.Exec(`INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,owner_user_id,version,index_status)
		VALUES(?,?,?,?,?,?,?,?,?,?,?, ?,?,1,'not_indexed')`,
		kbID, req.Type, req.Title, req.Content, req.Question, req.Answer, req.Purpose, req.Prerequisites, string(steps), req.Notes, string(tags), req.Status, userID)
	if err != nil {
		return 0, err
	}
	itemID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	payload := s.knowledgeItemPayload(req)
	_, err = tx.Exec(`INSERT INTO knowledge_item_versions(item_id,version,payload,content_hash,created_by,status)
		VALUES(?,1,?,?,?,?)`, itemID, payload, sha256Text(payload), userID, req.Status)
	return itemID, err
}
