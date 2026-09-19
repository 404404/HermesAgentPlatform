package main

import "database/sql"

// Seed only an empty review-workflow collection.  Earlier v0.3.3 startup code
// updated every demo workflow and policy on every boot, overwriting legitimate
// administrator edits.  One-time data belongs in a migration; this is only a
// safe empty-environment convenience.
func seedV033Data(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM skill_review_workflows WHERE organization_id=1").Scan(&count); err != nil || count > 0 {
		return err
	}
	var adminID int64
	_ = db.QueryRow("SELECT id FROM users WHERE username='admin'").Scan(&adminID)
	result, err := db.Exec(`INSERT INTO skill_review_workflows(organization_id,name,status,description,created_by,created_at,updated_at)
		VALUES(1,'Default Skill Review','active','Initial demo workflow. Changes are retained after startup.',?,NOW(),NOW())`, nullableID(adminID))
	if err != nil {
		return err
	}
	workflowID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	roleID := func(name string) any {
		var id int64
		if db.QueryRow("SELECT id FROM roles WHERE name=?", name).Scan(&id) == nil {
			return id
		}
		return nil
	}
	steps := []struct {
		name     string
		role     any
		mode     string
		required bool
	}{
		{"Automated Check", nil, "auto", false},
		{"Security Review", roleID("Security Administrator"), "manual", true},
		{"Functional Review", roleID("System Administrator"), "manual", true},
		{"Publish", roleID("Audit Administrator"), "manual", true},
	}
	for order, step := range steps {
		if _, err := db.Exec(`INSERT INTO skill_review_workflow_steps(workflow_id,step_order,name,required_role_id,approval_mode,required_approval,created_at)
			VALUES(?,?,?,?,?,?,NOW())`, workflowID, order+1, step.name, step.role, step.mode, step.required); err != nil {
			return err
		}
	}
	return nil
}
