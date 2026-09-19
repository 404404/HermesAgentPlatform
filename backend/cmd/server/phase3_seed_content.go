package main

import (
	"database/sql"
	"encoding/json"
)

func seedPhase3Content(db *sql.DB) {
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM knowledge_items").Scan(&count)
	if count != 0 {
		return
	}
	var kbID, owner int64
	if db.QueryRow("SELECT id FROM knowledge_bases ORDER BY id LIMIT 1").Scan(&kbID) != nil {
		return
	}
	_ = db.QueryRow("SELECT id FROM users WHERE username='admin' LIMIT 1").Scan(&owner)
	type item struct {
		Type, Title, Content, Question, Answer, Purpose, Prerequisites, Notes string
		Steps, Tags                                                           []string
	}
	items := []item{
		{Type: "background", Title: "Company Context", Content: "Demo Corporation operates regulated internal services.", Purpose: "Background for enterprise assistants.", Tags: []string{"company"}},
		{Type: "qa", Title: "Access Request Q&A", Question: "How do I request access?", Answer: "Open an approval request with the owning administrator.", Purpose: "Common access guidance.", Tags: []string{"access"}},
		{Type: "markdown", Title: "Engineering Handbook", Content: "# Engineering Handbook\n\nUse reviewed skills and least privilege.", Purpose: "Engineering working agreement.", Tags: []string{"engineering"}},
		{Type: "procedure", Title: "Incident Response SOP", Purpose: "Respond consistently to runtime incidents.", Prerequisites: "Runtime failure has been detected.", Steps: []string{"Acknowledge the event", "Inspect the runtime", "Create an incident record"}, Tags: []string{"sop", "operations"}},
	}
	for _, it := range items {
		steps, _ := json.Marshal(it.Steps)
		tags, _ := json.Marshal(it.Tags)
		res, err := db.Exec(`INSERT INTO knowledge_items(knowledge_base_id,type,title,content,question,answer,purpose,prerequisites,steps,notes,tags,status,owner_user_id,version,index_status) VALUES(?,?,?,?,?,?,?,?,?,?,?,'published',?,1,'indexed')`, kbID, it.Type, it.Title, it.Content, it.Question, it.Answer, it.Purpose, it.Prerequisites, string(steps), it.Notes, string(tags), owner)
		if err != nil {
			continue
		}
		id, _ := res.LastInsertId()
		payload, _ := json.Marshal(it)
		_, _ = db.Exec("INSERT INTO knowledge_item_versions(item_id,version,payload,content_hash,created_by,status) VALUES(?,1,?,?,?,'published')", id, string(payload), sha256Text(string(payload)), owner)
	}
}

func seedPhase3Executions(db *sql.DB) {
	var count int
	if db.QueryRow("SELECT COUNT(*) FROM executions").Scan(&count) != nil || count > 0 {
		return
	}
	var orgID, deptID, userID, profileID, runtimeID, modelID int64
	if db.QueryRow("SELECT organization_id,COALESCE(department_id,0),id FROM users WHERE username=\x27admin\x27 LIMIT 1").Scan(&orgID, &deptID, &userID) != nil {
		return
	}
	_ = db.QueryRow("SELECT id FROM profiles WHERE user_id=? ORDER BY id LIMIT 1", userID).Scan(&profileID)
	_ = db.QueryRow("SELECT id FROM runtimes WHERE user_id=? LIMIT 1", userID).Scan(&runtimeID)
	_ = db.QueryRow("SELECT id FROM models ORDER BY id LIMIT 1").Scan(&modelID)
	if orgID == 0 {
		orgID = 1
	}
	skills := `["Corporate Security"]`
	tools := `["policy.check"]`
	res, err := db.Exec(`INSERT INTO executions(execution_id,organization_id,department_id,user_id,profile_id,runtime_id,model_id,skill_ids,tool_names,status,started_at,finished_at,duration_ms,input_tokens,output_tokens,cost,risk_level,risk_score,risk_reason,requires_approval) VALUES(?,?,?,?,?,?,?,?,?,'completed',UTC_TIMESTAMP(),UTC_TIMESTAMP(),1250,420,180,0.0024,'low',10,'Seeded demo execution',FALSE)`, "seed-exec-completed", orgID, nullableID(deptID), userID, nullableID(profileID), nullableID(runtimeID), nullableID(modelID), skills, tools)
	if err == nil {
		id, _ := res.LastInsertId()
		_, _ = db.Exec("INSERT INTO execution_events(execution_id,event_type,message) VALUES(?,?,?)", id, "completed", "Seeded mock execution completed")
		_, _ = db.Exec(`INSERT INTO usage_events(organization_id,department_id,user_id,profile_id,execution_id,model_id,runtime_id,token_input,token_output,requests,executions,latency_ms) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, orgID, nullableID(deptID), userID, nullableID(profileID), "seed-exec-completed", nullableID(modelID), nullableID(runtimeID), 420, 180, 1, 1, 1250)
	}
	var reviewer int64
	_ = db.QueryRow("SELECT id FROM users WHERE username='security-admin' LIMIT 1").Scan(&reviewer)
	if reviewer == 0 {
		reviewer = userID
	}
	approval, err := db.Exec("INSERT INTO approval_requests(type,requester,resource_type,status,risk_level,current_reviewer,reason,metadata) VALUES('execution',?,'execution','pending','high',?,'Seeded high-risk execution requires approval','{}')", userID, reviewer)
	if err != nil {
		return
	}
	approvalID, _ := approval.LastInsertId()
	_, _ = db.Exec("INSERT INTO approval_steps(approval_request_id,step_order,reviewer_id,status) VALUES(?,1,?,'pending')", approvalID, reviewer)
	_, _ = db.Exec(`INSERT INTO executions(execution_id,organization_id,department_id,user_id,profile_id,runtime_id,model_id,skill_ids,tool_names,status,risk_level,risk_score,risk_reason,requires_approval,approval_request_id) VALUES(?,?,?,?,?,?,?,?,?,'waiting_approval','high',75,'Seeded high-risk execution requires approval',TRUE,?)`, "seed-exec-approval", orgID, nullableID(deptID), userID, nullableID(profileID), nullableID(runtimeID), nullableID(modelID), skills, tools, approvalID)
}
