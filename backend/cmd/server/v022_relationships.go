package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (s *server) profileSourceRolesData(userID int64) []gin.H {
	rows, err := s.db.Query(`SELECT DISTINCT r.id,r.name FROM role_bindings rb JOIN roles r ON r.id=rb.role_id WHERE rb.user_id=? ORDER BY r.name`, userID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			out = append(out, gin.H{"id": id, "name": name, "source": "User Role"})
		}
	}
	return out
}

func (s *server) agentTemplateInstanceNames(templateID int64) []string {
	rows, err := s.db.Query("SELECT CONCAT(u.username, \x27 / \x27, p.display_name) FROM profiles p JOIN users u ON u.id=p.user_id WHERE p.source_template_id=? ORDER BY u.username,p.display_name", templateID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if rows.Scan(&value) == nil && value != "" {
			values = append(values, value)
		}
	}
	return values
}

func (s *server) agentTemplateRelationNames(templateID int64, kind string) []string {
	queries := map[string]string{
		"role":       `SELECT DISTINCT r.name FROM profile_template_bindings b JOIN roles r ON r.id=b.role_id WHERE b.template_id=? AND b.scope='role' ORDER BY r.name`,
		"department": `SELECT DISTINCT d.name FROM profile_template_bindings b JOIN departments d ON d.id=b.department_id WHERE b.template_id=? AND b.scope='department' ORDER BY d.name`,
		"user":       `SELECT DISTINCT u.display_name FROM profile_template_bindings b JOIN users u ON u.id=b.user_id WHERE b.template_id=? AND b.scope='user' ORDER BY u.display_name`,
	}
	query, ok := queries[kind]
	if !ok {
		return []string{}
	}
	rows, err := s.db.Query(query, templateID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if rows.Scan(&value) == nil && value != "" {
			values = append(values, value)
		}
	}
	return values
}

// v0.2.2 relationship endpoints keep RoleBinding and the existing template
// tables as the source of truth. Runtime policy bindings are intentionally
// separate from Agent Template bindings: one describes infrastructure, the
// other describes agent behavior.

func (s *server) runtimeTemplateBindingsData(templateID int64) []gin.H {
	rows, err := s.db.Query(`SELECT b.id,b.binding_type,COALESCE(b.role_id,0),COALESCE(r.name,''),COALESCE(b.department_id,0),COALESCE(d.name,''),b.binding_priority,b.policy,b.created_at
		FROM runtime_template_bindings b LEFT JOIN roles r ON r.id=b.role_id LEFT JOIN departments d ON d.id=b.department_id
		WHERE b.runtime_template_id=? ORDER BY b.binding_priority DESC,b.id`, templateID)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, roleID, departmentID, priority int64
		var typ, role, department, policy, created string
		if rows.Scan(&id, &typ, &roleID, &role, &departmentID, &department, &priority, &policy, &created) == nil {
			out = append(out, gin.H{"id": id, "binding_type": typ, "role_id": roleID, "role": role, "department_id": departmentID, "department": department, "binding_priority": priority, "policy": policy, "created_at": created})
		}
	}
	return out
}

func (s *server) listRuntimeTemplateBindings(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": s.runtimeTemplateBindingsData(id)})
}

func (s *server) addRuntimeTemplateBinding(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		BindingType  string `json:"binding_type"`
		RoleID       int64  `json:"role_id"`
		DepartmentID int64  `json:"department_id"`
		Priority     int    `json:"binding_priority"`
		Policy       string `json:"policy"`
	}
	if c.ShouldBindJSON(&req) != nil || (req.BindingType != "role" && req.BindingType != "department") || (req.RoleID == 0 && req.DepartmentID == 0) || (req.BindingType == "role" && req.RoleID == 0) || (req.BindingType == "department" && req.DepartmentID == 0) {
		failCode(c, http.StatusBadRequest, "runtime_template.binding_invalid", nil)
		return
	}
	if req.Priority < 0 {
		req.Priority = 0
	}
	if req.Policy == "" {
		req.Policy = "default"
	}
	_, err := s.db.Exec(`INSERT INTO runtime_template_bindings(runtime_template_id,binding_type,role_id,department_id,binding_priority,policy,created_by) VALUES(?,?,?,?,?,?,?)`, id, req.BindingType, nullableID(req.RoleID), nullableID(req.DepartmentID), req.Priority, req.Policy, currentUserID(c))
	if err != nil {
		failCode(c, http.StatusConflict, "runtime_template.binding_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_template.binding.create", "Runtime Policy Binding Created", "Runtime", "runtime_template", id, "success", gin.H{"binding_type": req.BindingType, "binding_priority": req.Priority}, nil)
	c.JSON(http.StatusCreated, gin.H{"data": s.runtimeTemplateBindingsData(id)})
}

func (s *server) updateRuntimeTemplateBinding(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
		return
	}
	templateID, ok := paramID(c, "id")
	if !ok {
		return
	}
	bindingID, ok := paramID(c, "binding_id")
	if !ok {
		return
	}
	var ownerID int64
	if s.db.QueryRow("SELECT runtime_template_id FROM runtime_template_bindings WHERE id=?", bindingID).Scan(&ownerID) != nil || ownerID != templateID {
		failCode(c, http.StatusNotFound, "runtime_template.binding_not_found", nil)
		return
	}
	var req struct {
		Priority int    `json:"binding_priority"`
		Policy   string `json:"policy"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, http.StatusBadRequest, "runtime_template.binding_invalid", nil)
		return
	}
	if req.Priority < 0 {
		req.Priority = 0
	}
	if req.Policy == "" {
		req.Policy = "default"
	}
	if _, err := s.db.Exec("UPDATE runtime_template_bindings SET binding_priority=?,policy=? WHERE id=? AND runtime_template_id=?", req.Priority, req.Policy, bindingID, templateID); err != nil {
		failCode(c, http.StatusBadRequest, "runtime_template.binding_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_template.binding.update", "Runtime Policy Binding Updated", "Runtime", "runtime_template", templateID, "success", gin.H{"binding_id": bindingID, "binding_priority": req.Priority}, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) deleteRuntimeTemplateBinding(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.manage") {
		return
	}
	bindingID, ok := paramID(c, "binding_id")
	if !ok {
		return
	}
	var templateID int64
	if s.db.QueryRow("SELECT runtime_template_id FROM runtime_template_bindings WHERE id=?", bindingID).Scan(&templateID) != nil {
		failCode(c, http.StatusNotFound, "runtime_template.binding_not_found", nil)
		return
	}
	if _, err := s.db.Exec("DELETE FROM runtime_template_bindings WHERE id=?", bindingID); err != nil {
		failCode(c, http.StatusBadRequest, "runtime_template.binding_delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_template.binding.delete", "Runtime Policy Binding Deleted", "Runtime", "runtime_template", templateID, "success", gin.H{"binding_id": bindingID}, nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

// resolveRuntimeTemplate is deliberately deterministic. An explicit runtime
// assignment wins, then role policy, then department policy, then the org
// default. Equal-priority different templates are returned as a conflict and
// are never silently selected.
func (s *server) resolveRuntimeTemplate(userID int64) gin.H {
	var explicit sql.NullInt64
	_ = s.db.QueryRow("SELECT template_id FROM runtimes WHERE user_id=?", userID).Scan(&explicit)
	if explicit.Valid {
		var name string
		_ = s.db.QueryRow("SELECT name FROM runtime_templates WHERE id=?", explicit.Int64).Scan(&name)
		return gin.H{"status": "resolved", "template_id": explicit.Int64, "template": name, "source": "explicit_user", "priority": 1000}
	}
	type candidate struct {
		id       int64
		name     string
		source   string
		priority int
	}
	rows, _ := s.db.Query(`SELECT DISTINCT rt.id,rt.name,'role',b.binding_priority FROM runtime_template_bindings b JOIN runtime_templates rt ON rt.id=b.runtime_template_id JOIN role_bindings rb ON rb.role_id=b.role_id AND rb.user_id=? WHERE b.binding_type='role' AND rt.status='active'`, userID)
	if rows != nil {
		defer rows.Close()
	}
	candidates := []candidate{}
	if rows != nil {
		for rows.Next() {
			var x candidate
			if rows.Scan(&x.id, &x.name, &x.source, &x.priority) == nil {
				candidates = append(candidates, x)
			}
		}
	}
	var departmentID int64
	_ = s.db.QueryRow("SELECT department_id FROM users WHERE id=?", userID).Scan(&departmentID)
	if departmentID > 0 {
		deptRows, _ := s.db.Query(`SELECT DISTINCT rt.id,rt.name,'department',b.binding_priority FROM runtime_template_bindings b JOIN runtime_templates rt ON rt.id=b.runtime_template_id WHERE b.binding_type='department' AND b.department_id=? AND rt.status='active'`, departmentID)
		if deptRows != nil {
			for deptRows.Next() {
				var x candidate
				if deptRows.Scan(&x.id, &x.name, &x.source, &x.priority) == nil {
					candidates = append(candidates, x)
				}
			}
			deptRows.Close()
		}
	}
	if len(candidates) == 0 {
		var x candidate
		if s.db.QueryRow("SELECT id,name,'organization',0 FROM runtime_templates WHERE organization_id=1 AND is_default=TRUE AND status='active' LIMIT 1").Scan(&x.id, &x.name, &x.source, &x.priority) == nil {
			candidates = append(candidates, x)
		}
	}
	if len(candidates) == 0 {
		return gin.H{"status": "unresolved", "source": "none"}
	}
	best := candidates[0]
	for _, x := range candidates[1:] {
		if x.priority > best.priority {
			best = x
		}
	}
	conflicts := []gin.H{}
	for _, x := range candidates {
		if x.priority == best.priority && x.id != best.id {
			conflicts = append(conflicts, gin.H{"template_id": x.id, "template": x.name, "source": x.source, "priority": x.priority})
		}
	}
	if len(conflicts) > 0 {
		return gin.H{"status": "conflict", "priority": best.priority, "candidates": append([]gin.H{{"template_id": best.id, "template": best.name, "source": best.source, "priority": best.priority}}, conflicts...)}
	}
	return gin.H{"status": "resolved", "template_id": best.id, "template": best.name, "source": best.source, "priority": best.priority}
}

func (s *server) runtimeTemplateResolution(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.read") {
		return
	}
	userID := intQuery(c, "user_id")
	if userID == 0 {
		userID = currentUserID(c)
	}
	c.JSON(http.StatusOK, gin.H{"data": s.resolveRuntimeTemplate(userID)})
}

func (s *server) roleMembers(c *gin.Context) {
	if !s.requirePermission(c, "role.read") {
		return
	}
	roleID, ok := paramID(c, "id")
	if !ok {
		return
	}
	rows, err := s.db.Query(`SELECT rb.id,u.id,u.username,u.display_name,COALESCE(d.id,0),COALESCE(d.name,''),rb.scope,rb.created_at FROM role_bindings rb JOIN users u ON u.id=rb.user_id LEFT JOIN departments d ON d.id=u.department_id WHERE rb.role_id=? ORDER BY u.display_name`, roleID)
	if err != nil {
		failCode(c, http.StatusInternalServerError, "roles.members_load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var bindingID, userID, departmentID int64
		var username, display, department, scope, created string
		if rows.Scan(&bindingID, &userID, &username, &display, &departmentID, &department, &scope, &created) == nil {
			out = append(out, gin.H{"binding_id": bindingID, "user_id": userID, "username": username, "display_name": display, "department_id": departmentID, "department": department, "scope": scope, "created_at": created})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (s *server) addRoleMembers(c *gin.Context) {
	if !s.requirePermission(c, "role.assign") {
		return
	}
	roleID, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		UserIDs      []int64 `json:"user_ids"`
		Scope        string  `json:"scope"`
		DepartmentID int64   `json:"department_id"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.UserIDs) == 0 || !validScope(req.Scope) {
		if req.Scope == "" {
			req.Scope = "user"
		}
		if len(req.UserIDs) == 0 || !validScope(req.Scope) {
			failCode(c, http.StatusBadRequest, "role.invalid_request", nil)
			return
		}
	}
	protected, roleName := s.protectedRole(roleID)
	added := []int64{}
	for _, userID := range req.UserIDs {
		if userID == currentUserID(c) && protected {
			s.auditPhase2(c, "role.self_elevate", "role", roleID, "user", "denied", gin.H{"role": roleName, "source": "role_members"})
			continue
		}
		if protected && !s.isBreakglass(currentUserID(c)) {
			_, _ = s.createApproval(c, "role_elevation", "role", roleID, "high", "Protected role assignment requires independent approval", gin.H{"target_user_id": userID, "role_id": roleID, "scope": req.Scope})
			continue
		}
		if _, err := s.db.Exec(`INSERT INTO role_bindings(role_id,organization_id,department_id,user_id,scope) VALUES(?,1,?,?,?)`, roleID, nullableID(req.DepartmentID), userID, req.Scope); err == nil {
			s.consolidateManagedAgents(userID)
			added = append(added, userID)
		}
	}
	s.auditPhase2(c, "role.assign", "role", roleID, req.Scope, "success", gin.H{"user_ids": req.UserIDs, "added": added})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"added_user_ids": added, "members": s.roleMembersData(roleID)}})
}

func (s *server) roleMembersData(roleID int64) []gin.H {
	rows, _ := s.db.Query(`SELECT rb.id,u.id,u.username,u.display_name,COALESCE(d.id,0),COALESCE(d.name,''),rb.scope,rb.created_at FROM role_bindings rb JOIN users u ON u.id=rb.user_id LEFT JOIN departments d ON d.id=u.department_id WHERE rb.role_id=? ORDER BY u.display_name`, roleID)
	if rows == nil {
		return []gin.H{}
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var bindingID, userID, departmentID int64
		var username, display, department, scope, created string
		if rows.Scan(&bindingID, &userID, &username, &display, &departmentID, &department, &scope, &created) == nil {
			out = append(out, gin.H{"binding_id": bindingID, "user_id": userID, "username": username, "display_name": display, "department_id": departmentID, "department": department, "scope": scope, "created_at": created})
		}
	}
	return out
}

func (s *server) removeRoleMember(c *gin.Context) {
	if !s.requirePermission(c, "role.assign") {
		return
	}
	bindingID, ok := paramID(c, "binding_id")
	if !ok {
		return
	}
	var roleID, userID int64
	if s.db.QueryRow("SELECT role_id,user_id FROM role_bindings WHERE id=?", bindingID).Scan(&roleID, &userID) != nil {
		failCode(c, http.StatusNotFound, "role.binding_not_found", nil)
		return
	}
	protected, roleName := s.protectedRole(roleID)
	if userID == currentUserID(c) && protected {
		s.auditPhase2(c, "role.self_elevate", "role", roleID, "user", "denied", gin.H{"operation": "remove", "role": roleName})
		failCode(c, http.StatusForbidden, "role.self_change_denied", nil)
		return
	}
	if _, err := s.db.Exec("DELETE FROM role_bindings WHERE id=?", bindingID); err != nil {
		failCode(c, http.StatusBadRequest, "role.remove_failed", nil)
		return
	}
	s.consolidateManagedAgents(userID)
	s.auditPhase2(c, "role.remove", "role", roleID, "user", "success", gin.H{"user_id": userID, "role": roleName})
	c.JSON(http.StatusOK, gin.H{"data": true})
}

type importUserRow struct {
	Row          int    `json:"row"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Email        string `json:"email"`
	Password     string `json:"password,omitempty"`
	Department   string `json:"department"`
	DepartmentID int64  `json:"department_id"`
	Error        string `json:"error,omitempty"`
}

func parseUserCSV(content string) ([]importUserRow, []gin.H, error) {
	r := csv.NewReader(strings.NewReader(content))
	r.TrimLeadingSpace = true
	headers, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	index := map[string]int{}
	for i, h := range headers {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range []string{"username", "display_name", "email", "password", "department"} {
		if _, ok := index[required]; !ok {
			return nil, nil, fmt.Errorf("missing column %s", required)
		}
	}
	rows := []importUserRow{}
	errors := []gin.H{}
	seen := map[string]bool{}
	line := 1
	for {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		line++
		if readErr != nil {
			errors = append(errors, gin.H{"row": line, "error": "invalid CSV row"})
			continue
		}
		get := func(key string) string {
			i := index[key]
			if i >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[i])
		}
		x := importUserRow{Row: line, Username: get("username"), DisplayName: get("display_name"), Email: get("email"), Password: get("password"), Department: get("department")}
		if x.Username == "" || x.DisplayName == "" || x.Email == "" || len(x.Password) < 12 || x.Department == "" {
			x.Error = "username, display_name, email, department and a 12+ character password are required"
		}
		if seen[x.Username] {
			x.Error = "username duplicated in file"
		}
		seen[x.Username] = true
		if x.Error == "" {
			rows = append(rows, x)
		} else {
			errors = append(errors, gin.H{"row": x.Row, "username": x.Username, "error": x.Error})
		}
	}
	return rows, errors, nil
}

func (s *server) validateImportRows(rows []importUserRow) ([]importUserRow, []gin.H) {
	valid := []importUserRow{}
	errors := []gin.H{}
	for _, x := range rows {
		var id int64
		if s.db.QueryRow("SELECT id FROM users WHERE username=? OR email=? LIMIT 1", x.Username, x.Email).Scan(&id) == nil {
			errors = append(errors, gin.H{"row": x.Row, "username": x.Username, "error": "username or email already exists"})
			continue
		}
		if s.db.QueryRow("SELECT id FROM departments WHERE organization_id=1 AND (name=? OR code=?)", x.Department, x.Department).Scan(&x.DepartmentID) != nil {
			errors = append(errors, gin.H{"row": x.Row, "username": x.Username, "error": "department not found"})
			continue
		}
		valid = append(valid, x)
	}
	return valid, errors
}

func safeImportRows(rows []importUserRow) []importUserRow {
	result := make([]importUserRow, 0, len(rows))
	for _, row := range rows {
		row.Password = ""
		result = append(result, row)
	}
	return result
}

func (s *server) validateUserImport(c *gin.Context) {
	if !s.requirePermission(c, "user.create") {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Content) == "" {
		failCode(c, http.StatusBadRequest, "user.import_invalid", nil)
		return
	}
	rows, parseErrors, err := parseUserCSV(req.Content)
	if err != nil {
		failCode(c, http.StatusBadRequest, "user.import_invalid", gin.H{"reason": err.Error()})
		return
	}
	valid, dbErrors := s.validateImportRows(rows)
	safeValid := safeImportRows(valid)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"valid_rows": safeValid, "invalid_rows": append(parseErrors, dbErrors...), "valid_count": len(valid), "invalid_count": len(parseErrors) + len(dbErrors)}})
}

func (s *server) createImportedUser(x importUserRow, actor int64) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(x.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`INSERT INTO users(organization_id,department_id,username,display_name,email,password_hash,status) VALUES(1,?,?,?,?,?,'active')`, nullableID(x.DepartmentID), x.Username, x.DisplayName, x.Email, string(hash))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	_, _ = s.db.Exec(`INSERT IGNORE INTO auth_identities(user_id,provider_type,provider_id,external_subject) VALUES(?,'local','local',?)`, id, x.Username)
	if roleID := s.roleID("Standard User"); roleID > 0 {
		_, _ = s.db.Exec(`INSERT INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')`, roleID, id)
	}
	_, _ = s.db.Exec(`INSERT INTO runtimes(user_id,runtime_id,status,provider,hermes_version,cpu_limit,memory_limit) VALUES(?,?, 'stopped','mock','mock-hermes-0.2','1 CPU','512Mi')`, id, fmt.Sprintf("mock-runtime-%d", id))
	s.ensureAutomaticProvisioned(id)
	_ = actor
	return id, nil
}

func (s *server) confirmUserImport(c *gin.Context) {
	if !s.requirePermission(c, "user.create") {
		return
	}
	var req struct {
		Content string          `json:"content"`
		Rows    []importUserRow `json:"rows"`
	}
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, http.StatusBadRequest, "user.import_invalid", nil)
		return
	}
	rows := req.Rows
	if strings.TrimSpace(req.Content) != "" {
		parsed, parseErrors, err := parseUserCSV(req.Content)
		if err != nil || len(parseErrors) > 0 {
			failCode(c, http.StatusBadRequest, "user.import_invalid", gin.H{"invalid_count": len(parseErrors)})
			return
		}
		rows = parsed
	}
	if len(rows) == 0 {
		failCode(c, http.StatusBadRequest, "user.import_invalid", nil)
		return
	}
	valid, errors := s.validateImportRows(rows)
	if len(errors) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error_code": "user.import_has_errors", "message_params": gin.H{"invalid_count": len(errors)}, "data": gin.H{"invalid_rows": errors}})
		return
	}
	created := []int64{}
	for _, x := range valid {
		id, err := s.createImportedUser(x, currentUserID(c))
		if err != nil {
			failCode(c, http.StatusConflict, "user.import_failed", gin.H{"username": x.Username})
			return
		}
		created = append(created, id)
	}
	s.auditControlPlane(c, "user.import", "Users Imported", "Identity & Access", "user", 0, "success", gin.H{"record_count": len(created)}, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"created_ids": created, "count": len(created)}})
}

func (s *server) batchUsers(c *gin.Context) {
	if !s.requirePermission(c, "user.update") {
		return
	}
	var req struct {
		UserIDs      []int64 `json:"user_ids"`
		Action       string  `json:"action"`
		DepartmentID int64   `json:"department_id"`
		RoleID       int64   `json:"role_id"`
		Status       string  `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.UserIDs) == 0 {
		failCode(c, http.StatusBadRequest, "user.batch_invalid", nil)
		return
	}
	if req.Action != "enable" && req.Action != "suspend" && req.Action != "move_department" && req.Action != "assign_role" {
		failCode(c, http.StatusBadRequest, "user.batch_invalid", nil)
		return
	}
	changed := 0
	for _, id := range req.UserIDs {
		var err error
		switch req.Action {
		case "enable", "suspend":
			status := "active"
			if req.Action == "suspend" {
				status = "suspended"
			}
			_, err = s.db.Exec("UPDATE users SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?", status, id)
			if err == nil {
				s.orchestrateUserLifecycle(id, status)
			}
		case "move_department":
			_, err = s.db.Exec("UPDATE users SET department_id=?,updated_at=UTC_TIMESTAMP() WHERE id=?", nullableID(req.DepartmentID), id)
			if err == nil {
				s.consolidateManagedAgents(id)
			}
		case "assign_role":
			if req.RoleID == 0 {
				err = fmt.Errorf("role required")
			} else {
				_, err = s.db.Exec("INSERT INTO role_bindings(role_id,organization_id,user_id,scope) VALUES(?,1,?,'user')", req.RoleID, id)
				if err == nil {
					s.consolidateManagedAgents(id)
				}
			}
		}
		if err == nil {
			changed++
		}
	}
	s.auditControlPlane(c, "user.batch."+req.Action, "Batch User Operation", "Identity & Access", "user", 0, "success", gin.H{"user_ids": req.UserIDs, "changed": changed}, nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"changed": changed}})
}

func (s *server) exportUsers(c *gin.Context) {
	if !s.requirePermission(c, "user.read") {
		return
	}
	// Reuse the same query contract as the list endpoint. Export is backend
	// generated, so filters never apply only to the current browser page.
	rows, err := s.filteredUserRows(c)
	if err != nil {
		failCode(c, http.StatusInternalServerError, "users.load_failed", nil)
		return
	}
	format := strings.ToLower(c.DefaultQuery("format", "csv"))
	s.auditControlPlane(c, "user.export", "Users Exported", "Exports", "user", 0, "success", gin.H{"format": format, "record_count": len(rows)}, nil)
	if format == "json" {
		c.Header("Content-Disposition", `attachment; filename="users.json"`)
		c.JSON(http.StatusOK, rows)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="users.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"username", "display_name", "email", "department", "status", "runtime_status", "profile_count", "created_at"})
	for _, row := range rows {
		_ = w.Write([]string{row.Username, row.DisplayName, row.Email, row.Department, row.Status, row.Runtime, strconv.Itoa(row.ProfileCount), row.CreatedAt})
	}
	w.Flush()
}

func (s *server) filteredUserRows(c *gin.Context) ([]userView, error) {
	where := " WHERE 1=1"
	args := []any{}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		where += " AND (u.username LIKE ? OR u.display_name LIKE ? OR u.email LIKE ?)"
		needle := "%" + q + "%"
		args = append(args, needle, needle, needle)
	}
	for _, key := range []string{"status"} {
		if v := c.Query(key); v != "" {
			where += " AND u." + key + "=?"
			args = append(args, v)
		}
	}
	if v := c.Query("department_id"); v != "" {
		where += " AND u.department_id=?"
		args = append(args, v)
	}
	if v := c.Query("role_id"); v != "" {
		where += " AND EXISTS (SELECT 1 FROM role_bindings frb WHERE frb.user_id=u.id AND frb.role_id=?)"
		args = append(args, v)
	}
	if v := c.Query("runtime_status"); v != "" {
		where += " AND COALESCE(rt.status,'not_created')=?"
		args = append(args, v)
	}
	if v := c.Query("template_id"); v != "" {
		where += " AND EXISTS (SELECT 1 FROM profiles fp WHERE fp.user_id=u.id AND fp.source_template_id=?)"
		args = append(args, v)
	}
	query := `SELECT u.id,u.username,u.display_name,u.email,u.status,COALESCE(d.name,''),COALESCE(GROUP_CONCAT(DISTINCT r.name ORDER BY r.name SEPARATOR ','),''),(SELECT COUNT(*) FROM profiles p WHERE p.user_id=u.id),COALESCE(rt.status,'not_created'),u.last_login_at,u.created_at FROM users u LEFT JOIN departments d ON d.id=u.department_id LEFT JOIN role_bindings rb ON (rb.user_id=u.id OR (rb.user_id IS NULL AND rb.organization_id=u.organization_id)) LEFT JOIN roles r ON r.id=rb.role_id LEFT JOIN runtimes rt ON rt.user_id=u.id` + where + ` GROUP BY u.id ORDER BY u.created_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []userView{}
	for rows.Next() {
		var v userView
		var roles string
		var lastLogin sql.NullTime
		if err := rows.Scan(&v.ID, &v.Username, &v.DisplayName, &v.Email, &v.Status, &v.Department, &roles, &v.ProfileCount, &v.Runtime, &lastLogin, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Roles = []string{}
		if roles != "" {
			v.Roles = strings.Split(roles, ",")
		}
		if lastLogin.Valid {
			x := lastLogin.Time.UTC().Format("2006-01-02T15:04:05Z")
			v.LastLoginAt = &x
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *server) markAllNotificationsRead(c *gin.Context) {
	if !s.requirePermission(c, "notification.read") {
		return
	}
	if _, err := s.db.Exec("UPDATE notifications SET status='read',read_at=UTC_TIMESTAMP() WHERE user_id=? AND status='unread'", currentUserID(c)); err != nil {
		failCode(c, http.StatusBadRequest, "notification.update_failed", nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) userRuntimeResolution(c *gin.Context) {
	if !s.requirePermission(c, "runtime_template.read") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": s.resolveRuntimeTemplate(id)})
}

// Keep json imported in this file for the stable import row contract used by
// clients and future adapters.
var _ = json.RawMessage{}
