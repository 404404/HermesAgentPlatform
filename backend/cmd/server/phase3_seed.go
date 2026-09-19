package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// seedPhase3Data is deliberately idempotent. It enriches the Phase 2 demo
// records instead of replacing them, so an existing demo database upgrades in
// place and keeps its configured users and resources.
func seedPhase3Data(db *sql.DB, password string) error {
	_ = password
	permissions := []string{"agent_template.read", "agent_template.manage", "execution.read", "execution.create", "execution.manage"}
	for _, code := range permissions {
		if _, err := db.Exec("INSERT INTO permissions(code,description) VALUES(?,?) ON DUPLICATE KEY UPDATE description=VALUES(description)", code, code); err != nil {
			return err
		}
	}
	roleID := func(name string) int64 {
		var id int64
		_ = db.QueryRow("SELECT id FROM roles WHERE name=? LIMIT 1", name).Scan(&id)
		return id
	}
	grant := func(role string, codes ...string) error {
		rid := roleID(role)
		for _, code := range codes {
			var pid int64
			if db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&pid) == nil {
				if _, err := db.Exec("INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES(?,?)", rid, pid); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := grant("System Administrator", "agent_template.read", "agent_template.manage", "execution.read", "execution.manage"); err != nil {
		return err
	}
	if err := grant("Security Administrator", "agent_template.read", "execution.read", "execution.create"); err != nil {
		return err
	}
	if err := grant("Audit Administrator", "execution.read"); err != nil {
		return err
	}
	if err := grant("Break-glass Super Administrator", permissions...); err != nil {
		return err
	}
	if err := grant("Developer", "agent_template.read", "execution.read", "execution.create", "profile.read", "profile.create"); err != nil {
		return err
	}
	if err := grant("Standard User", "execution.read", "execution.create", "agent_template.read"); err != nil {
		return err
	}

	// Add stable department codes and connect the department policy to the
	// existing Lightweight default without changing the old tree structure.
	rows, _ := db.Query("SELECT id,name FROM departments WHERE code='' OR code IS NULL")
	if rows != nil {
		for rows.Next() {
			var id int64
			var name string
			if rows.Scan(&id, &name) == nil {
				code := fmt.Sprintf("dept-%d", id)
				_, _ = db.Exec("UPDATE departments SET code=? WHERE id=?", code, id)
			}
		}
		rows.Close()
	}
	var defaultRuntime int64
	_ = db.QueryRow("SELECT id FROM runtime_templates WHERE is_default=TRUE AND status='active' LIMIT 1").Scan(&defaultRuntime)
	if defaultRuntime > 0 {
		_, _ = db.Exec("UPDATE departments SET default_runtime_template_id=COALESCE(default_runtime_template_id,?)", defaultRuntime)
	}

	var developerRole int64
	_ = db.QueryRow("SELECT id FROM roles WHERE name=? LIMIT 1", "Developer").Scan(&developerRole)
	if defaultRuntime > 0 && developerRole > 0 {
		_, _ = db.Exec("INSERT IGNORE INTO runtime_template_bindings(runtime_template_id,binding_type,role_id,binding_priority,policy) VALUES(?, ?, ?, ?, ?)", defaultRuntime, "role", developerRole, 50, "default")
	}

	// Backfill template policy metadata. The JSON is a policy map keyed by the
	// skill name; future policy services can extend it without changing the API.
	rows, _ = db.Query("SELECT id,default_skills FROM profile_templates")
	if rows != nil {
		for rows.Next() {
			var id int64
			var raw string
			if rows.Scan(&id, &raw) != nil {
				continue
			}
			var names []string
			_ = json.Unmarshal([]byte(raw), &names)
			policies := map[string]string{}
			for _, name := range names {
				policies[name] = "default"
			}
			b, _ := json.Marshal(policies)
			_, _ = db.Exec("UPDATE profile_templates SET skill_policies=? WHERE id=?", string(b), id)
		}
		rows.Close()
	}

	seedPhase3Content(db)
	seedPhase3Executions(db)

	// Reconcile additive Role + Department + User template sources into one
	// managed profile per user/template.
	s := &server{db: db}
	s.consolidateAllManagedAgents()
	return nil
}
