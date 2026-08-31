package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/hermes-enterprise-platform/backend/internal/hermes"
	"github.com/example/hermes-enterprise-platform/backend/internal/providers"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type config struct {
	dsn           string
	port          string
	migrationsDir string
	adminPassword string
	cookieSecure  bool
	allowedOrigin string
}

type session struct {
	userID    int64
	expiresAt time.Time
}

type sessionStore struct {
	mu sync.RWMutex
	m  map[string]session
}

func newSessionStore() *sessionStore { return &sessionStore{m: map[string]session{}} }

func (s *sessionStore) create(userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	s.mu.Lock()
	s.m[token] = session{userID: userID, expiresAt: time.Now().Add(24 * time.Hour)}
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) get(token string) (session, bool) {
	s.mu.RLock()
	v, ok := s.m[token]
	s.mu.RUnlock()
	if !ok || time.Now().After(v.expiresAt) {
		if ok {
			s.delete(token)
		}
		return session{}, false
	}
	return v, true
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	delete(s.m, token)
	s.mu.Unlock()
}

type server struct {
	db       *sql.DB
	cfg      config
	sessions *sessionStore
	runtime  providers.RuntimeProvider
	hermes   hermes.Adapter
}

func main() {
	cfg := config{
		dsn:           env("DB_DSN", "hep:hep_password@tcp(mysql:3306)/hep?parseTime=true&charset=utf8mb4&loc=UTC"),
		port:          env("PORT", "8080"),
		migrationsDir: env("MIGRATIONS_DIR", "./migrations"),
		adminPassword: env("SEED_ADMIN_PASSWORD", "ChangeMe-Admin-2026!"),
		cookieSecure:  envBool("COOKIE_SECURE", false),
		allowedOrigin: env("ALLOWED_ORIGIN", "http://localhost:18080"),
	}

	db, err := connectWithRetry(cfg.dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := applyMigrations(db, cfg.migrationsDir); err != nil {
		log.Fatal(err)
	}
	if err := seedDemoData(db, cfg.adminPassword); err != nil {
		log.Fatal(err)
	}

	s := &server{
		db:       db,
		cfg:      cfg,
		sessions: newSessionStore(),
		runtime:  providers.NewMockRuntimeProvider(),
		hermes:   hermes.MockAdapter{},
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), s.securityHeaders, s.cors)
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "hep-api"}) })

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", s.login)
	auth := v1.Group("")
	auth.Use(s.authMiddleware)
	auth.GET("/auth/me", s.me)
	auth.POST("/auth/logout", s.logout)
	auth.GET("/dashboard", s.dashboard)
	auth.GET("/users", s.listUsers)
	auth.POST("/users", s.createUser)
	auth.PUT("/users/:id", s.updateUser)
	auth.POST("/users/:id/status", s.setUserStatus)
	auth.GET("/departments/tree", s.departmentTree)
	auth.POST("/departments", s.createDepartment)
	auth.PUT("/departments/:id", s.updateDepartment)
	auth.DELETE("/departments/:id", s.deleteDepartment)
	auth.GET("/roles", s.listRoles)
	auth.GET("/profiles", s.listProfiles)
	auth.POST("/profiles", s.createProfile)
	auth.PUT("/profiles/:id", s.updateProfile)
	auth.DELETE("/profiles/:id", s.deleteProfile)
	auth.POST("/profiles/:id/status", s.setProfileStatus)
	auth.GET("/runtimes", s.listRuntimes)
	auth.POST("/runtimes/:id/action", s.runtimeAction)
	auth.GET("/models", s.listModels)
	auth.POST("/models", s.createModel)
	auth.PUT("/models/:id", s.updateModel)
	auth.GET("/skills", s.listSkills)
	auth.POST("/skills", s.createSkill)
	auth.POST("/skills/:id/submit", s.submitSkill)
	auth.GET("/skill-submissions", s.listSubmissions)
	auth.POST("/skill-submissions/:id/review", s.reviewSubmission)
	auth.POST("/skill-submissions/:id/publish", s.publishSubmission)
	auth.GET("/knowledge-bases", s.listKnowledgeBases)
	auth.POST("/knowledge-bases", s.createKnowledgeBase)
	auth.POST("/knowledge-bases/:id/bindings", s.createKnowledgeBinding)
	auth.GET("/usage/overview", s.usageOverview)
	auth.GET("/audit-logs", s.auditLogs)

	log.Printf("HEP API listening on :%s", cfg.port)
	if err := r.Run(":" + cfg.port); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "true" || v == "1" || v == "yes" {
		return true
	}
	if v == "false" || v == "0" || v == "no" {
		return false
	}
	return fallback
}

func connectWithRetry(dsn string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for attempt := 1; attempt <= 60; attempt++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			return db, nil
		}
		if db != nil {
			db.Close()
		}
		log.Printf("waiting for MySQL (%d/60): %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("mysql unavailable: %w", err)
}

func applyMigrations(db *sql.DB, dir string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		var exists int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", name).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, statement := range splitSQL(string(content)) {
			if _, err := tx.Exec(statement); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("applied migration %s", name)
	}
	return nil
}

func splitSQL(sqlText string) []string {
	var result []string
	for _, part := range strings.Split(sqlText, ";") {
		statement := strings.TrimSpace(part)
		if statement != "" && !strings.HasPrefix(statement, "--") {
			result = append(result, statement)
		}
	}
	return result
}

func (s *server) securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("Content-Security-Policy", "default-src 'self'; connect-src 'self' http://localhost:18081; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
	c.Next()
}

func (s *server) cors(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" && origin == s.cfg.allowedOrigin {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Vary", "Origin")
	}
	if c.Request.Method == http.MethodOptions {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()
}

func (s *server) authMiddleware(c *gin.Context) {
	token, _ := c.Cookie("hep_session")
	if token == "" {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	current, ok := s.sessions.get(token)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodDelete {
		csrfCookie, _ := c.Cookie("hep_csrf")
		csrfHeader := c.GetHeader("X-CSRF-Token")
		if csrfCookie == "" || csrfCookie != csrfHeader {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "csrf token required"})
			return
		}
	}
	c.Set("userID", current.userID)
	c.Set("sessionToken", token)
	c.Next()
}

func (s *server) login(c *gin.Context) {
	var request struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		fail(c, http.StatusBadRequest, "username and password are required")
		return
	}
	var user userView
	var hash string
	err := s.db.QueryRow(`SELECT u.id, u.username, u.display_name, u.email, u.status, COALESCE(d.name, ''), u.password_hash
		FROM users u LEFT JOIN departments d ON d.id = u.department_id WHERE u.username = ? LIMIT 1`, request.Username).
		Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.Status, &user.Department, &hash)
	if err != nil || user.Status != "active" || bcrypt.CompareHashAndPassword([]byte(hash), []byte(request.Password)) != nil {
		fail(c, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.sessions.create(user.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not create session")
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not create csrf token")
		return
	}
	_, _ = s.db.Exec("UPDATE users SET last_login_at = UTC_TIMESTAMP() WHERE id = ?", user.ID)
	s.setCookies(c, token, csrf)
	s.audit(c, user.ID, "auth.login", "user", user.ID, "global", "success", nil)
	fullUser, fullErr := s.getUser(user.ID)
	if fullErr == nil {
		user = fullUser
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (s *server) me(c *gin.Context) {
	user, err := s.getUser(currentUserID(c))
	if err != nil {
		fail(c, http.StatusNotFound, "user not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (s *server) logout(c *gin.Context) {
	if token, ok := c.Get("sessionToken"); ok {
		s.sessions.delete(token.(string))
	}
	c.SetCookie("hep_session", "", -1, "/", "", s.cfg.cookieSecure, true)
	c.SetCookie("hep_csrf", "", -1, "/", "", s.cfg.cookieSecure, false)
	s.audit(c, currentUserID(c), "auth.logout", "user", currentUserID(c), "global", "success", nil)
	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (s *server) setCookies(c *gin.Context, sessionToken, csrf string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("hep_session", sessionToken, 86400, "/", "", s.cfg.cookieSecure, true)
	c.SetCookie("hep_csrf", csrf, 86400, "/", "", s.cfg.cookieSecure, false)
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type userView struct {
	ID           int64    `json:"id"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"display_name"`
	Email        string   `json:"email"`
	Status       string   `json:"status"`
	Department   string   `json:"department"`
	Roles        []string `json:"roles"`
	ProfileCount int      `json:"profile_count"`
	Runtime      string   `json:"runtime_status"`
	LastLoginAt  *string  `json:"last_login_at"`
	CreatedAt    string   `json:"created_at"`
}

func (s *server) getUser(id int64) (userView, error) {
	var v userView
	var roles string
	var lastLogin sql.NullTime
	err := s.db.QueryRow(`SELECT u.id, u.username, u.display_name, u.email, u.status, COALESCE(d.name, ''),
		COALESCE(GROUP_CONCAT(DISTINCT r.name ORDER BY r.name SEPARATOR ','), ''),
		(SELECT COUNT(*) FROM profiles p WHERE p.user_id = u.id), COALESCE(rt.status, 'not_created'),
		u.last_login_at, u.created_at
		FROM users u LEFT JOIN departments d ON d.id=u.department_id
		LEFT JOIN role_bindings rb ON (rb.user_id=u.id OR (rb.user_id IS NULL AND rb.organization_id=u.organization_id))
		LEFT JOIN roles r ON r.id=rb.role_id LEFT JOIN runtimes rt ON rt.user_id=u.id
		WHERE u.id=? GROUP BY u.id`, id).
		Scan(&v.ID, &v.Username, &v.DisplayName, &v.Email, &v.Status, &v.Department, &roles, &v.ProfileCount, &v.Runtime, &lastLogin, &v.CreatedAt)
	if err != nil {
		return v, err
	}
	if roles != "" {
		v.Roles = strings.Split(roles, ",")
	} else {
		v.Roles = []string{}
	}
	if lastLogin.Valid {
		formatted := lastLogin.Time.UTC().Format(time.RFC3339)
		v.LastLoginAt = &formatted
	}
	return v, nil
}

func (s *server) listUsers(c *gin.Context) {
	if !s.requirePermission(c, "user.read") {
		return
	}
	rows, err := s.db.Query(`SELECT u.id, u.username, u.display_name, u.email, u.status, COALESCE(d.name, ''),
		COALESCE(GROUP_CONCAT(DISTINCT r.name ORDER BY r.name SEPARATOR ','), ''),
		(SELECT COUNT(*) FROM profiles p WHERE p.user_id=u.id), COALESCE(rt.status, 'not_created'), u.last_login_at, u.created_at
		FROM users u LEFT JOIN departments d ON d.id=u.department_id
		LEFT JOIN role_bindings rb ON (rb.user_id=u.id OR (rb.user_id IS NULL AND rb.organization_id=u.organization_id))
		LEFT JOIN roles r ON r.id=rb.role_id LEFT JOIN runtimes rt ON rt.user_id=u.id
		GROUP BY u.id ORDER BY u.created_at DESC`)
	if err != nil {
		fail(c, 500, "could not load users")
		return
	}
	defer rows.Close()
	users := []userView{}
	for rows.Next() {
		var v userView
		var roles string
		var lastLogin sql.NullTime
		if err := rows.Scan(&v.ID, &v.Username, &v.DisplayName, &v.Email, &v.Status, &v.Department, &roles, &v.ProfileCount, &v.Runtime, &lastLogin, &v.CreatedAt); err != nil {
			fail(c, 500, "could not read users")
			return
		}
		v.Roles = []string{}
		if roles != "" {
			v.Roles = strings.Split(roles, ",")
		}
		if lastLogin.Valid {
			x := lastLogin.Time.UTC().Format(time.RFC3339)
			v.LastLoginAt = &x
		}
		users = append(users, v)
	}
	c.JSON(200, gin.H{"data": users})
}

func (s *server) createUser(c *gin.Context) {
	if !s.requirePermission(c, "user.create") {
		return
	}
	var req struct {
		Username     string `json:"username"`
		DisplayName  string `json:"display_name"`
		Email        string `json:"email"`
		Password     string `json:"password"`
		DepartmentID int64  `json:"department_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.DisplayName == "" || len(req.Password) < 12 {
		fail(c, 400, "username, display name and a 12+ character password are required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		fail(c, 500, "could not hash password")
		return
	}
	orgID := s.currentOrg(c)
	result, err := s.db.Exec(`INSERT INTO users (organization_id, department_id, username, display_name, email, password_hash, status) VALUES (?,?,?,?,?,?, 'active')`, orgID, nullableID(req.DepartmentID), req.Username, req.DisplayName, req.Email, string(hash))
	if err != nil {
		fail(c, 409, "could not create user; username or email may already exist")
		return
	}
	id, _ := result.LastInsertId()
	_, _ = s.db.Exec(`INSERT INTO auth_identities (user_id, provider_type, provider_id, external_subject) VALUES (?, 'local', 'local', ?)`, id, req.Username)
	if roleID := s.roleID("Standard User"); roleID > 0 {
		_, _ = s.db.Exec(`INSERT INTO role_bindings (role_id, organization_id, user_id, scope) VALUES (?, ?, ?, 'user')`, roleID, orgID, id)
	}
	_, _ = s.db.Exec(`INSERT INTO runtimes (user_id, runtime_id, status, provider, hermes_version, cpu_limit, memory_limit) VALUES (?, ?, 'stopped', 'mock', 'mock-hermes-0.1', '1 CPU', '512Mi')`, id, fmt.Sprintf("mock-runtime-%d", id))
	s.audit(c, currentUserID(c), "user.create", "user", id, "global", "success", map[string]any{"username": req.Username})
	v, err := s.getUser(id)
	if err != nil {
		fail(c, 500, "user created but could not load it")
		return
	}
	c.JSON(201, gin.H{"data": v})
}

func (s *server) updateUser(c *gin.Context) {
	if !s.requirePermission(c, "user.update") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName  string `json:"display_name"`
		Email        string `json:"email"`
		DepartmentID int64  `json:"department_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DisplayName == "" {
		fail(c, 400, "display_name is required")
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET display_name=?, email=?, department_id=?, updated_at=UTC_TIMESTAMP() WHERE id=?`, req.DisplayName, req.Email, nullableID(req.DepartmentID), id); err != nil {
		fail(c, 400, "could not update user")
		return
	}
	s.audit(c, currentUserID(c), "user.update", "user", id, "global", "success", nil)
	v, err := s.getUser(id)
	if err != nil {
		fail(c, 404, "user not found")
		return
	}
	c.JSON(200, gin.H{"data": v})
}

func (s *server) setUserStatus(c *gin.Context) {
	if !s.requirePermission(c, "user.disable") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "active" && req.Status != "disabled") {
		fail(c, 400, "status must be active or disabled")
		return
	}
	if _, err := s.db.Exec(`UPDATE users SET status=?, updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Status, id); err != nil {
		fail(c, 400, "could not update status")
		return
	}
	s.audit(c, currentUserID(c), "user.disable", "user", id, "global", "success", map[string]any{"status": req.Status})
	v, err := s.getUser(id)
	if err != nil {
		fail(c, 404, "user not found")
		return
	}
	c.JSON(200, gin.H{"data": v})
}

type departmentNode struct {
	ID             int64             `json:"id"`
	ParentID       *int64            `json:"parent_id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Status         string            `json:"status"`
	MemberCount    int               `json:"member_count"`
	KnowledgeCount int               `json:"knowledge_count"`
	Children       []*departmentNode `json:"children"`
}

func (s *server) departmentTree(c *gin.Context) {
	if !s.requirePermission(c, "department.read") {
		return
	}
	rows, err := s.db.Query(`SELECT d.id,d.parent_id,d.name,d.description,d.status,(SELECT COUNT(*) FROM users u WHERE u.department_id=d.id),(SELECT COUNT(*) FROM knowledge_bindings kb WHERE kb.binding_type='department' AND kb.department_id=d.id) FROM departments d ORDER BY d.name`)
	if err != nil {
		fail(c, 500, "could not load departments")
		return
	}
	defer rows.Close()
	nodes := map[int64]*departmentNode{}
	roots := []*departmentNode{}
	for rows.Next() {
		n := &departmentNode{}
		var pid sql.NullInt64
		if err := rows.Scan(&n.ID, &pid, &n.Name, &n.Description, &n.Status, &n.MemberCount, &n.KnowledgeCount); err != nil {
			fail(c, 500, "could not read departments")
			return
		}
		if pid.Valid {
			x := pid.Int64
			n.ParentID = &x
		}
		n.Children = []*departmentNode{}
		nodes[n.ID] = n
	}
	for _, n := range nodes {
		if n.ParentID != nil && nodes[*n.ParentID] != nil {
			nodes[*n.ParentID].Children = append(nodes[*n.ParentID].Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	c.JSON(200, gin.H{"data": roots})
}

func (s *server) createDepartment(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ParentID    int64  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		fail(c, 400, "name is required")
		return
	}
	org := s.currentOrg(c)
	res, err := s.db.Exec(`INSERT INTO departments (organization_id,parent_id,name,description,status) VALUES (?,?,?,?, 'active')`, org, nullableID(req.ParentID), req.Name, req.Description)
	if err != nil {
		fail(c, 400, "could not create department")
		return
	}
	id, _ := res.LastInsertId()
	s.audit(c, currentUserID(c), "department.create", "department", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id, "name": req.Name}})
}

func (s *server) updateDepartment(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ParentID    int64  `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		fail(c, 400, "name is required")
		return
	}
	if _, err := s.db.Exec(`UPDATE departments SET name=?,description=?,parent_id=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Name, req.Description, nullableID(req.ParentID), id); err != nil {
		fail(c, 400, "could not update department")
		return
	}
	s.audit(c, currentUserID(c), "department.update", "department", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) deleteDepartment(c *gin.Context) {
	if !s.requirePermission(c, "department.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var count int
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM users WHERE department_id=?)+(SELECT COUNT(*) FROM departments WHERE parent_id=?)`, id, id).Scan(&count); err != nil {
		fail(c, 500, "could not check department")
		return
	}
	if count > 0 {
		fail(c, 409, "only empty departments can be deleted")
		return
	}
	if _, err := s.db.Exec(`DELETE FROM departments WHERE id=?`, id); err != nil {
		fail(c, 400, "could not delete department")
		return
	}
	s.audit(c, currentUserID(c), "department.delete", "department", id, "organization", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) listRoles(c *gin.Context) {
	rows, err := s.db.Query(`SELECT r.id,r.name,r.description,r.is_system,COALESCE(GROUP_CONCAT(p.code ORDER BY p.code SEPARATOR ','),'') FROM roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id GROUP BY r.id ORDER BY r.name`)
	if err != nil {
		fail(c, 500, "could not load roles")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var n, d, perms string
		var sys bool
		if err := rows.Scan(&id, &n, &d, &sys, &perms); err != nil {
			fail(c, 500, "could not read roles")
			return
		}
		p := []string{}
		if perms != "" {
			p = strings.Split(perms, ",")
		}
		out = append(out, gin.H{"id": id, "name": n, "description": d, "is_system": sys, "permissions": p})
	}
	c.JSON(200, gin.H{"data": out})
}

type profileView struct {
	ID           int64  `json:"id"`
	UserID       int64  `json:"user_id"`
	User         string `json:"user"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	ModelID      int64  `json:"model_id"`
	Model        string `json:"model"`
	RuntimeClass string `json:"runtime_class"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (s *server) listProfiles(c *gin.Context) {
	if !s.requirePermission(c, "profile.read") {
		return
	}
	rows, err := s.db.Query(`SELECT p.id,p.user_id,u.display_name,p.name,p.display_name,p.description,p.status,p.model_id,COALESCE(m.display_name,''),p.runtime_class,p.created_at,p.updated_at FROM profiles p JOIN users u ON u.id=p.user_id LEFT JOIN models m ON m.id=p.model_id ORDER BY p.created_at DESC`)
	if err != nil {
		fail(c, 500, "could not load profiles")
		return
	}
	defer rows.Close()
	out := []profileView{}
	for rows.Next() {
		var v profileView
		if err := rows.Scan(&v.ID, &v.UserID, &v.User, &v.Name, &v.DisplayName, &v.Description, &v.Status, &v.ModelID, &v.Model, &v.RuntimeClass, &v.CreatedAt, &v.UpdatedAt); err != nil {
			fail(c, 500, "could not read profiles")
			return
		}
		out = append(out, v)
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createProfile(c *gin.Context) {
	if !s.requirePermission(c, "profile.create") {
		return
	}
	var req struct {
		UserID       int64  `json:"user_id"`
		Name         string `json:"name"`
		DisplayName  string `json:"display_name"`
		Description  string `json:"description"`
		ModelID      int64  `json:"model_id"`
		RuntimeClass string `json:"runtime_class"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.DisplayName == "" {
		fail(c, 400, "name and display_name are required")
		return
	}
	if req.UserID == 0 {
		req.UserID = currentUserID(c)
	}
	if req.RuntimeClass == "" {
		req.RuntimeClass = "shared-user"
	}
	if req.RuntimeClass != "shared-user" && req.RuntimeClass != "dedicated" && req.RuntimeClass != "high-security" {
		fail(c, 400, "unsupported runtime class")
		return
	}
	res, err := s.db.Exec(`INSERT INTO profiles (user_id,model_id,name,display_name,description,status,runtime_class) VALUES (?,?,?,?,?,'active',?)`, req.UserID, nullableID(req.ModelID), req.Name, req.DisplayName, req.Description, req.RuntimeClass)
	if err != nil {
		fail(c, 400, "could not create profile")
		return
	}
	id, _ := res.LastInsertId()
	s.audit(c, currentUserID(c), "profile.create", "profile", id, "user/profile", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}

func (s *server) updateProfile(c *gin.Context) {
	if !s.requirePermission(c, "profile.update") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName  string `json:"display_name"`
		Description  string `json:"description"`
		ModelID      int64  `json:"model_id"`
		RuntimeClass string `json:"runtime_class"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DisplayName == "" {
		fail(c, 400, "display_name is required")
		return
	}
	if req.RuntimeClass == "" {
		req.RuntimeClass = "shared-user"
	}
	if _, err := s.db.Exec(`UPDATE profiles SET display_name=?,description=?,model_id=?,runtime_class=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.DisplayName, req.Description, nullableID(req.ModelID), req.RuntimeClass, id); err != nil {
		fail(c, 400, "could not update profile")
		return
	}
	s.audit(c, currentUserID(c), "profile.update", "profile", id, "user/profile", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) deleteProfile(c *gin.Context) {
	if !s.requirePermission(c, "profile.delete") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	if _, err := s.db.Exec(`DELETE FROM profiles WHERE id=?`, id); err != nil {
		fail(c, 400, "could not delete profile")
		return
	}
	s.audit(c, currentUserID(c), "profile.delete", "profile", id, "user/profile", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) setProfileStatus(c *gin.Context) {
	if !s.requirePermission(c, "profile.update") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "active" && req.Status != "disabled") {
		fail(c, 400, "status must be active or disabled")
		return
	}
	if _, err := s.db.Exec(`UPDATE profiles SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.Status, id); err != nil {
		fail(c, 400, "could not update profile status")
		return
	}
	s.audit(c, currentUserID(c), "profile.update", "profile", id, "user/profile", "success", map[string]any{"status": req.Status})
	c.JSON(200, gin.H{"data": true})
}

func (s *server) listRuntimes(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	rows, err := s.db.Query(`SELECT r.id,r.user_id,u.display_name,r.runtime_id,r.status,r.provider,r.hermes_version,(SELECT COUNT(*) FROM profiles p WHERE p.user_id=r.user_id),r.cpu_limit,r.memory_limit,r.created_at,r.last_seen FROM runtimes r JOIN users u ON u.id=r.user_id ORDER BY r.created_at DESC`)
	if err != nil {
		fail(c, 500, "could not load runtimes")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, uid int64
		var user, rid, status, provider, version, cpu, memory, created string
		var profiles int
		var last sql.NullTime
		if err := rows.Scan(&id, &uid, &user, &rid, &status, &provider, &version, &profiles, &cpu, &memory, &created, &last); err != nil {
			fail(c, 500, "could not read runtimes")
			return
		}
		lastSeen := any(nil)
		if last.Valid {
			lastSeen = last.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, gin.H{"id": id, "user_id": uid, "user": user, "runtime_id": rid, "status": status, "provider": provider, "hermes_version": version, "profile_count": profiles, "cpu_limit": cpu, "memory_limit": memory, "created_at": created, "last_seen": lastSeen})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) runtimeAction(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
	var rid string
	var userID int64
	if err := s.db.QueryRow(`SELECT runtime_id,user_id FROM runtimes WHERE id=?`, id).Scan(&rid, &userID); err != nil {
		fail(c, 404, "runtime not found")
		return
	}
	var providerErr error
	switch req.Action {
	case "start":
		providerErr = s.runtime.StartRuntime(c, rid)
	case "stop":
		providerErr = s.runtime.StopRuntime(c, rid)
	case "restart":
		providerErr = s.runtime.RestartRuntime(c, rid)
	default:
		fail(c, 400, "action must be start, stop or restart")
		return
	}
	if providerErr != nil {
		fail(c, 500, "runtime provider operation failed")
		return
	}
	status := "running"
	if req.Action == "stop" {
		status = "stopped"
	}
	if _, err := s.db.Exec(`UPDATE runtimes SET status=?,last_seen=UTC_TIMESTAMP() WHERE id=?`, status, id); err != nil {
		fail(c, 500, "could not persist runtime status")
		return
	}
	s.audit(c, currentUserID(c), "runtime."+req.Action, "runtime", id, "user", "success", nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": status, "user_id": userID}})
}

func (s *server) listModels(c *gin.Context) {
	if !s.requirePermission(c, "profile.model.select") {
		return
	}
	rows, err := s.db.Query(`SELECT id,name,display_name,provider,upstream_model,status,description,cost_class,data_classification,user_selectable,created_at,updated_at FROM models ORDER BY name`)
	if err != nil {
		fail(c, 500, "could not load models")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, display, provider, upstream, status, description, cost, data, created, updated string
		var selectable bool
		if err := rows.Scan(&id, &name, &display, &provider, &upstream, &status, &description, &cost, &data, &selectable, &created, &updated); err != nil {
			fail(c, 500, "could not read models")
			return
		}
		out = append(out, gin.H{"id": id, "name": name, "display_name": display, "provider": provider, "upstream_model": upstream, "status": status, "description": description, "cost_class": cost, "data_classification": data, "user_selectable": selectable, "created_at": created, "updated_at": updated})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createModel(c *gin.Context) {
	if !s.requirePermission(c, "profile.model.select") {
		return
	}
	var req struct {
		Name               string `json:"name"`
		DisplayName        string `json:"display_name"`
		Provider           string `json:"provider"`
		UpstreamModel      string `json:"upstream_model"`
		Description        string `json:"description"`
		CostClass          string `json:"cost_class"`
		DataClassification string `json:"data_classification"`
		UserSelectable     bool   `json:"user_selectable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.DisplayName == "" {
		fail(c, 400, "name and display_name are required")
		return
	}
	res, err := s.db.Exec(`INSERT INTO models (name,display_name,provider,upstream_model,status,description,cost_class,data_classification,user_selectable) VALUES (?,?,?,?, 'active',?,?,?,?)`, req.Name, req.DisplayName, req.Provider, req.UpstreamModel, req.Description, req.CostClass, req.DataClassification, req.UserSelectable)
	if err != nil {
		fail(c, 400, "could not create model")
		return
	}
	id, _ := res.LastInsertId()
	s.audit(c, currentUserID(c), "model.create", "model", id, "global", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}

func (s *server) updateModel(c *gin.Context) {
	if !s.requirePermission(c, "profile.model.select") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName    string `json:"display_name"`
		Description    string `json:"description"`
		Status         string `json:"status"`
		UserSelectable bool   `json:"user_selectable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DisplayName == "" {
		fail(c, 400, "display_name is required")
		return
	}
	if _, err := s.db.Exec(`UPDATE models SET display_name=?,description=?,status=?,user_selectable=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, req.DisplayName, req.Description, req.Status, req.UserSelectable, id); err != nil {
		fail(c, 400, "could not update model")
		return
	}
	s.audit(c, currentUserID(c), "model.update", "model", id, "global", "success", nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) listSkills(c *gin.Context) {
	if !s.requirePermission(c, "skill.read") {
		return
	}
	rows, err := s.db.Query(`SELECT sk.id,sk.name,sk.display_name,sk.description,COALESCE(u.display_name,sk.name),sk.status,sk.latest_version,sk.risk_level,sk.install_count,sk.use_count,sk.created_at FROM skills sk LEFT JOIN users u ON u.id=sk.publisher_id ORDER BY sk.created_at DESC`)
	if err != nil {
		fail(c, 500, "could not load skills")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, display, desc, publisher, status, version, risk, created string
		var installs, uses int
		if err := rows.Scan(&id, &name, &display, &desc, &publisher, &status, &version, &risk, &installs, &uses, &created); err != nil {
			fail(c, 500, "could not read skills")
			return
		}
		out = append(out, gin.H{"id": id, "name": name, "display_name": display, "description": desc, "publisher": publisher, "status": status, "latest_version": version, "risk_level": risk, "install_count": installs, "use_count": uses, "created_at": created})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createSkill(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Version     string `json:"version"`
		RiskLevel   string `json:"risk_level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.DisplayName == "" {
		fail(c, 400, "name and display_name are required")
		return
	}
	if req.Version == "" {
		req.Version = "0.1.0"
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "low"
	}
	res, err := s.db.Exec(`INSERT INTO skills (name,display_name,description,category,publisher_id,status,latest_version,risk_level) VALUES (?,?,?,?,?,'draft',?,?)`, req.Name, req.DisplayName, req.Description, req.Category, currentUserID(c), req.Version, req.RiskLevel)
	if err != nil {
		fail(c, 400, "could not create skill")
		return
	}
	id, _ := res.LastInsertId()
	_, _ = s.db.Exec(`INSERT INTO skill_versions (skill_id,version,artifact_hash,status,required_tools,required_network,required_secrets) VALUES (?,?,'pending','draft','[]','[]','[]')`, id, req.Version)
	s.audit(c, currentUserID(c), "skill.create", "skill", id, "user", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}

func (s *server) submitSkill(c *gin.Context) {
	if !s.requirePermission(c, "skill.submit") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var note string
	var req struct {
		Notes string `json:"notes"`
	}
	_ = c.ShouldBindJSON(&req)
	note = req.Notes
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM skills WHERE id=?`, id).Scan(&exists); err != nil || exists == 0 {
		fail(c, 404, "skill not found")
		return
	}
	res, err := s.db.Exec(`INSERT INTO skill_submissions (skill_id,submitted_by,status,notes) VALUES (?,?, 'submitted',?)`, id, currentUserID(c), note)
	if err != nil {
		fail(c, 400, "could not submit skill")
		return
	}
	submissionID, _ := res.LastInsertId()
	_, _ = s.db.Exec(`UPDATE skills SET status='submitted' WHERE id=?`, id)
	s.audit(c, currentUserID(c), "skill.submit", "skill", id, "user", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": submissionID, "status": "submitted"}})
}

func (s *server) listSubmissions(c *gin.Context) {
	if !s.requirePermission(c, "skill.read") {
		return
	}
	rows, err := s.db.Query(`SELECT ss.id,ss.skill_id,sk.display_name,ss.submitted_by,COALESCE(u.display_name,''),ss.status,ss.notes,ss.created_at,ss.updated_at FROM skill_submissions ss JOIN skills sk ON sk.id=ss.skill_id JOIN users u ON u.id=ss.submitted_by ORDER BY ss.created_at DESC`)
	if err != nil {
		fail(c, 500, "could not load submissions")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, sid, uid int64
		var skill, submitter, status, notes, created, updated string
		if err := rows.Scan(&id, &sid, &skill, &uid, &submitter, &status, &notes, &created, &updated); err != nil {
			fail(c, 500, "could not read submissions")
			return
		}
		out = append(out, gin.H{"id": id, "skill_id": sid, "skill": skill, "submitted_by": uid, "submitter": submitter, "status": status, "notes": notes, "created_at": created, "updated_at": updated})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) reviewSubmission(c *gin.Context) {
	if !s.requirePermission(c, "skill.review") {
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
	if err := c.ShouldBindJSON(&req); err != nil || (req.Decision != "approve" && req.Decision != "reject") {
		fail(c, 400, "decision must be approve or reject")
		return
	}
	status := "approved"
	if req.Decision == "reject" {
		status = "rejected"
	}
	if _, err := s.db.Exec(`UPDATE skill_submissions SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=?`, status, id); err != nil {
		fail(c, 400, "could not review submission")
		return
	}
	_, _ = s.db.Exec(`INSERT INTO skill_reviews (submission_id,reviewer_id,decision,comment) VALUES (?,?,?,?)`, id, currentUserID(c), req.Decision, req.Comment)
	s.audit(c, currentUserID(c), "skill.review", "skill_submission", id, "global", "success", map[string]any{"decision": req.Decision})
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": status}})
}

func (s *server) publishSubmission(c *gin.Context) {
	if !s.requirePermission(c, "skill.publish") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var skillID int64
	if err := s.db.QueryRow(`SELECT skill_id FROM skill_submissions WHERE id=? AND status='approved'`, id).Scan(&skillID); err != nil {
		fail(c, 409, "submission must be approved before publishing")
		return
	}
	if _, err := s.db.Exec(`UPDATE skill_submissions SET status='published',updated_at=UTC_TIMESTAMP() WHERE id=?`, id); err != nil {
		fail(c, 500, "could not publish submission")
		return
	}
	_, _ = s.db.Exec(`UPDATE skills SET status='published' WHERE id=?`, skillID)
	s.audit(c, currentUserID(c), "skill.publish", "skill", skillID, "global", "success", nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": "published"}})
}

func (s *server) listKnowledgeBases(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.read") {
		return
	}
	rows, err := s.db.Query(`SELECT kb.id,kb.name,kb.description,COALESCE(d.name,''),kb.visibility,kb.status,kb.document_count,kb.last_indexed,kb.created_at,(SELECT COUNT(*) FROM knowledge_bindings b WHERE b.knowledge_base_id=kb.id) FROM knowledge_bases kb LEFT JOIN departments d ON d.id=kb.owner_department_id ORDER BY kb.name`)
	if err != nil {
		fail(c, 500, "could not load knowledge bases")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, desc, owner, visibility, status, created string
		var docs, bindings int
		var indexed sql.NullTime
		if err := rows.Scan(&id, &name, &desc, &owner, &visibility, &status, &docs, &indexed, &created, &bindings); err != nil {
			fail(c, 500, "could not read knowledge bases")
			return
		}
		last := any(nil)
		if indexed.Valid {
			last = indexed.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, gin.H{"id": id, "name": name, "description": desc, "owner_department": owner, "visibility": visibility, "status": status, "document_count": docs, "last_indexed": last, "binding_count": bindings, "created_at": created})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) createKnowledgeBase(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	var req struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		OwnerDepartmentID int64  `json:"owner_department_id"`
		Visibility        string `json:"visibility"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		fail(c, 400, "name is required")
		return
	}
	if req.Visibility == "" {
		req.Visibility = "department"
	}
	res, err := s.db.Exec(`INSERT INTO knowledge_bases (organization_id,owner_department_id,name,description,visibility,status) VALUES (?,?,?,?,?,'active')`, s.currentOrg(c), nullableID(req.OwnerDepartmentID), req.Name, req.Description, req.Visibility)
	if err != nil {
		fail(c, 400, "could not create knowledge base")
		return
	}
	id, _ := res.LastInsertId()
	s.audit(c, currentUserID(c), "knowledge.create", "knowledge_base", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}

func (s *server) createKnowledgeBinding(c *gin.Context) {
	if !s.requirePermission(c, "knowledge.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		BindingType  string `json:"binding_type"`
		DepartmentID int64  `json:"department_id"`
		RoleID       int64  `json:"role_id"`
		ProfileID    int64  `json:"profile_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.BindingType == "" {
		fail(c, 400, "binding_type is required")
		return
	}
	if req.BindingType != "department" && req.BindingType != "role" && req.BindingType != "profile" {
		fail(c, 400, "unsupported binding type")
		return
	}
	_, err := s.db.Exec(`INSERT INTO knowledge_bindings (knowledge_base_id,binding_type,department_id,role_id,profile_id) VALUES (?,?,?,?,?)`, id, req.BindingType, nullableID(req.DepartmentID), nullableID(req.RoleID), nullableID(req.ProfileID))
	if err != nil {
		fail(c, 400, "could not create binding")
		return
	}
	s.audit(c, currentUserID(c), "knowledge.binding.create", "knowledge_base", id, "organization", "success", nil)
	c.JSON(201, gin.H{"data": true})
}

func (s *server) usageOverview(c *gin.Context) {
	if !s.requirePermission(c, "usage.global.read") {
		return
	}
	var input, output, totalReq, execs, active, skills, tools int64
	var latency sql.NullFloat64
	err := s.db.QueryRow(`SELECT COALESCE(SUM(token_input),0),COALESCE(SUM(token_output),0),COALESCE(SUM(requests),0),COALESCE(SUM(executions),0),COALESCE(AVG(latency_ms),0),COUNT(DISTINCT user_id),COALESCE(SUM(skill_calls),0),COALESCE(SUM(tool_calls),0) FROM usage_events`).Scan(&input, &output, &totalReq, &execs, &latency, &active, &skills, &tools)
	if err != nil {
		fail(c, 500, "could not load usage")
		return
	}
	var today, month int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(token_input+token_output),0) FROM usage_events WHERE created_at >= UTC_DATE()`).Scan(&today)
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(token_input+token_output),0) FROM usage_events WHERE created_at >= DATE_FORMAT(UTC_DATE(), '%Y-%m-01')`).Scan(&month)
	models := []gin.H{}
	rows, _ := s.db.Query(`SELECT COALESCE(m.display_name,'Unknown'),COALESCE(SUM(e.token_input+e.token_output),0) FROM usage_events e LEFT JOIN models m ON m.id=e.model_id GROUP BY e.model_id,m.display_name ORDER BY 2 DESC`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var n string
			var t int64
			_ = rows.Scan(&n, &t)
			models = append(models, gin.H{"model": n, "tokens": t})
		}
	}
	avg := float64(0)
	if latency.Valid {
		avg = latency.Float64
	}
	c.JSON(200, gin.H{"data": gin.H{"token_input": input, "token_output": output, "total_tokens": input + output, "requests": totalReq, "executions": execs, "active_users": active, "skill_calls": skills, "tool_calls": tools, "average_latency_ms": avg, "today_tokens": today, "month_tokens": month, "by_model": models}})
}

func (s *server) auditLogs(c *gin.Context) {
	if !s.requirePermission(c, "audit.read") {
		return
	}
	query := `SELECT a.id,a.actor_user_id,COALESCE(u.display_name,''),a.action,a.resource_type,a.resource_id,a.scope,a.result,a.ip_address,a.metadata,a.created_at FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE 1=1`
	args := []any{}
	if v := c.Query("action"); v != "" {
		query += " AND a.action LIKE ?"
		args = append(args, "%"+v+"%")
	}
	if v := c.Query("resource_type"); v != "" {
		query += " AND a.resource_type=?"
		args = append(args, v)
	}
	query += " ORDER BY a.created_at DESC LIMIT 100"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		fail(c, 500, "could not load audit logs")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, actor int64
		var actorName, action, resource, scope, result, ip, metadata, created string
		var resourceID sql.NullInt64
		if err := rows.Scan(&id, &actor, &actorName, &action, &resource, &resourceID, &scope, &result, &ip, &metadata, &created); err != nil {
			fail(c, 500, "could not read audit logs")
			return
		}
		out = append(out, gin.H{"id": id, "actor_user_id": actor, "actor": actorName, "action": action, "resource_type": resource, "resource_id": resourceID.Int64, "scope": scope, "result": result, "ip_address": ip, "metadata": json.RawMessage(metadata), "created_at": created})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) dashboard(c *gin.Context) {
	counts := map[string]int64{}
	queries := map[string]string{"users": "SELECT COUNT(*) FROM users", "active_users": "SELECT COUNT(*) FROM users WHERE status='active'", "departments": "SELECT COUNT(*) FROM departments", "profiles": "SELECT COUNT(*) FROM profiles", "runtimes": "SELECT COUNT(*) FROM runtimes WHERE status='running'"}
	for key, q := range queries {
		var value int64
		_ = s.db.QueryRow(q).Scan(&value)
		counts[key] = value
	}
	usage := gin.H{}
	var in, out int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(token_input),0),COALESCE(SUM(token_output),0) FROM usage_events`).Scan(&in, &out)
	usage["today_tokens"] = in + out
	var month int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(token_input+token_output),0) FROM usage_events WHERE created_at >= DATE_FORMAT(UTC_DATE(), '%Y-%m-01')`).Scan(&month)
	usage["month_tokens"] = month
	var skillCalls int64
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(skill_calls),0) FROM usage_events`).Scan(&skillCalls)
	usage["skill_calls"] = skillCalls
	recent := []gin.H{}
	rows, _ := s.db.Query(`SELECT action,resource_type,COALESCE(resource_id,0),created_at FROM audit_logs ORDER BY created_at DESC LIMIT 6`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var action, resource, created string
			var id int64
			_ = rows.Scan(&action, &resource, &id, &created)
			recent = append(recent, gin.H{"action": action, "resource_type": resource, "resource_id": id, "created_at": created})
		}
	}
	c.JSON(200, gin.H{"data": gin.H{"counts": counts, "usage": usage, "runtime_status": gin.H{"running": counts["runtimes"]}, "recent_activity": recent}})
}

func (s *server) requirePermission(c *gin.Context, permission string) bool {
	id := currentUserID(c)
	var exists int
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM role_bindings rb JOIN role_permissions rp ON rp.role_id=rb.role_id JOIN permissions p ON p.id=rp.permission_id JOIN users u ON u.id=? WHERE p.code=? AND (rb.user_id=? OR (rb.user_id IS NULL AND rb.organization_id=u.organization_id AND rb.scope IN ('global','organization') ) OR (rb.department_id=u.department_id AND rb.scope='department')))`, id, permission, id).Scan(&exists)
	if err != nil || exists == 0 {
		fail(c, 403, "permission denied")
		return false
	}
	return true
}

func (s *server) currentOrg(c *gin.Context) int64 {
	var id int64
	_ = s.db.QueryRow("SELECT organization_id FROM users WHERE id=?", currentUserID(c)).Scan(&id)
	return id
}
func (s *server) roleID(name string) int64 {
	var id int64
	_ = s.db.QueryRow("SELECT id FROM roles WHERE name=?", name).Scan(&id)
	return id
}
func (s *server) audit(c *gin.Context, actor int64, action, resource string, resourceID int64, scope, result string, metadata map[string]any) {
	payload := "{}"
	if metadata != nil {
		b, _ := json.Marshal(metadata)
		payload = string(b)
	}
	_, _ = s.db.Exec(`INSERT INTO audit_logs (actor_user_id,action,resource_type,resource_id,scope,result,ip_address,metadata) VALUES (?,?,?,?,?,?,?,?)`, actor, action, resource, nullableID(resourceID), scope, result, c.ClientIP(), payload)
}
func (s *server) isAdmin(id int64) bool {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM role_bindings rb JOIN roles r ON r.id=rb.role_id WHERE r.name='Super Admin' AND rb.user_id=?`, id).Scan(&n)
	return n > 0
}
func currentUserID(c *gin.Context) int64 { v, _ := c.Get("userID"); id, _ := v.(int64); return id }
func nullableID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}
func paramID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id < 1 {
		fail(c, 400, "invalid id")
		return 0, false
	}
	return id, true
}
func fail(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

func seedDemoData(db *sql.DB, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO organizations (id,name,slug,status) VALUES (1,'Demo Corporation','demo-corporation','active') ON DUPLICATE KEY UPDATE name=VALUES(name)`)
	if err != nil {
		return err
	}
	departments := []struct {
		id     int
		parent any
		name   string
	}{{1, nil, "研发中心"}, {2, 1, "软件开发部"}, {3, 1, "网络部"}, {4, 1, "运维部"}, {5, nil, "财务部"}, {6, nil, "行政部"}}
	for _, d := range departments {
		if _, err := db.Exec(`INSERT INTO departments (id,organization_id,parent_id,name,status) VALUES (?,?,?,?, 'active') ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id)`, d.id, 1, d.parent, d.name); err != nil {
			return err
		}
	}
	permissions := []string{"user.read", "user.create", "user.update", "user.disable", "department.read", "department.manage", "profile.read", "profile.create", "profile.update", "profile.delete", "profile.model.select", "skill.read", "skill.install", "skill.submit", "skill.review", "skill.publish", "knowledge.read", "knowledge.manage", "usage.self.read", "usage.department.read", "usage.global.read", "runtime.read", "runtime.manage", "audit.read"}
	for _, code := range permissions {
		if _, err := db.Exec(`INSERT INTO permissions (code,description) VALUES (?,?) ON DUPLICATE KEY UPDATE description=VALUES(description)`, code, code); err != nil {
			return err
		}
	}
	roles := []struct {
		id   int
		name string
		desc string
	}{{1, "Super Admin", "Full platform administration"}, {2, "Department Admin", "Manage a department scope"}, {3, "Skill Reviewer", "Review and publish internal skills"}, {4, "Standard User", "Use approved platform resources"}}
	for _, role := range roles {
		var roleErr error
		if role.id == 1 {
			_, roleErr = db.Exec(`INSERT INTO roles (id,organization_id,name,description,is_system) VALUES (1,1,?,?,1) ON DUPLICATE KEY UPDATE description=VALUES(description)`, role.name, role.desc)
		} else {
			_, roleErr = db.Exec(`INSERT INTO roles (id,organization_id,name,description,is_system) VALUES (?,?,?, ?,1) ON DUPLICATE KEY UPDATE description=VALUES(description)`, role.id, 1, role.name, role.desc)
		}
		if roleErr != nil {
			return roleErr
		}
	}
	var superID, standardID, reviewerID int64
	_ = db.QueryRow(`SELECT id FROM roles WHERE name='Super Admin'`).Scan(&superID)
	_ = db.QueryRow(`SELECT id FROM roles WHERE name='Standard User'`).Scan(&standardID)
	_ = db.QueryRow(`SELECT id FROM roles WHERE name='Skill Reviewer'`).Scan(&reviewerID)
	for _, code := range permissions {
		var pid int64
		_ = db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&pid)
		_, err = db.Exec(`INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES (?,?)`, superID, pid)
		if err != nil {
			return err
		}
	}
	for _, code := range []string{"profile.read", "profile.create", "profile.update", "profile.delete", "profile.model.select", "skill.read", "skill.install", "skill.submit", "knowledge.read", "usage.self.read", "runtime.read"} {
		var pid int64
		_ = db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&pid)
		_, err = db.Exec(`INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES (?,?)`, standardID, pid)
		if err != nil {
			return err
		}
	}
	for _, code := range []string{"skill.read", "skill.review", "skill.publish", "skill.submit", "knowledge.read"} {
		var pid int64
		_ = db.QueryRow("SELECT id FROM permissions WHERE code=?", code).Scan(&pid)
		_, err = db.Exec(`INSERT IGNORE INTO role_permissions(role_id,permission_id) VALUES (?,?)`, reviewerID, pid)
		if err != nil {
			return err
		}
	}
	users := []struct {
		username, display, email string
		dept                     int
		roleID                   int
	}{{"admin", "Platform Administrator", "admin@demo.local", 1, 1}, {"developer01", "Lin Developer", "developer01@demo.local", 2, 4}, {"network01", "Wang Network", "network01@demo.local", 3, 4}, {"finance01", "Chen Finance", "finance01@demo.local", 5, 4}}
	for _, u := range users {
		var id int64
		err = db.QueryRow(`SELECT id FROM users WHERE username=?`, u.username).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			res, e := db.Exec(`INSERT INTO users (organization_id,department_id,username,display_name,email,password_hash,status) VALUES (1,?,?,?,?,?,'active')`, u.dept, u.username, u.display, u.email, string(hash))
			if e != nil {
				return e
			}
			id, _ = res.LastInsertId()
			_, _ = db.Exec(`INSERT INTO auth_identities(user_id,provider_type,provider_id,external_subject) VALUES (?, 'local','local',?)`, id, u.username)
			_, _ = db.Exec(`INSERT INTO runtimes(user_id,runtime_id,status,provider,hermes_version,cpu_limit,memory_limit,last_seen) VALUES (?,?,'running','mock','mock-hermes-0.1','1 CPU','512Mi',UTC_TIMESTAMP())`, id, fmt.Sprintf("mock-runtime-%d", id))
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = db.Exec(`INSERT IGNORE INTO role_bindings(role_id,organization_id,user_id,scope) VALUES (?,1,?,'user')`, func() int {
			if u.roleID == 1 {
				return int(superID)
			}
			return int(standardID)
		}(), id)
		if err != nil {
			return err
		}
	}
	// A reviewer is an additional role binding for the platform administrator.
	var adminID int64
	_ = db.QueryRow(`SELECT id FROM users WHERE username='admin'`).Scan(&adminID)
	_, _ = db.Exec(`INSERT IGNORE INTO role_bindings(role_id,organization_id,user_id,scope) VALUES (?,1,?,'user')`, reviewerID, adminID)
	models := []struct{ name, display, upstream, cost, data string }{{"fast", "Fast", "gpt-4o-mini", "low", "internal"}, {"standard", "Standard", "gpt-4o", "medium", "internal"}, {"reasoning", "Reasoning", "o3", "high", "internal"}, {"confidential", "Confidential", "enterprise-confidential", "controlled", "confidential"}}
	for _, m := range models {
		_, err = db.Exec(`INSERT INTO models(name,display_name,provider,upstream_model,status,description,cost_class,data_classification,user_selectable) VALUES (?,?, 'model-gateway',?,'active',?,?,?,1) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name)`, m.name, m.display, m.upstream, m.display, m.cost, m.data)
		if err != nil {
			return err
		}
	}
	var standardModel, reasoningModel int64
	_ = db.QueryRow(`SELECT id FROM models WHERE name='standard'`).Scan(&standardModel)
	_ = db.QueryRow(`SELECT id FROM models WHERE name='reasoning'`).Scan(&reasoningModel)
	profiles := []struct {
		user, name, display, desc string
		model                     int64
	}{{"developer01", "coding-assistant", "Coding Assistant", "Software development companion", standardModel}, {"developer01", "research-assistant", "Research Assistant", "Research and synthesis", reasoningModel}, {"network01", "network-operations", "Network Operations", "Network operations helper", standardModel}, {"network01", "research-assistant-network", "Research Assistant", "Network research helper", reasoningModel}, {"finance01", "finance-assistant", "Finance Assistant", "Finance reporting helper", standardModel}}
	for _, p := range profiles {
		var uid int64
		_ = db.QueryRow(`SELECT id FROM users WHERE username=?`, p.user).Scan(&uid)
		_, err = db.Exec(`INSERT INTO profiles(user_id,model_id,name,display_name,description,status,runtime_class) VALUES (?,?,?,?,?,'active','shared-user') ON DUPLICATE KEY UPDATE display_name=VALUES(display_name)`, uid, p.model, p.name, p.display, p.desc)
		if err != nil {
			return err
		}
	}
	skills := []struct{ name, display, desc, cat, risk string }{{"coding-policy", "Coding Policy", "Secure coding conventions", "Engineering", "medium"}, {"github-review", "GitHub Review", "Review pull requests with policy context", "Engineering", "medium"}, {"network-troubleshooting", "Network Troubleshooting", "Troubleshoot enterprise networks", "Operations", "high"}, {"report-generator", "Report Generator", "Generate structured reports", "Productivity", "low"}, {"database-review", "Database Review", "Review schema and query changes", "Engineering", "high"}}
	for _, sk := range skills {
		_, err = db.Exec(`INSERT INTO skills(name,display_name,description,category,publisher_id,status,latest_version,risk_level,install_count,use_count) VALUES (?,?,?,? ,(SELECT id FROM users WHERE username='admin'),'published','1.0.0',?,12,28) ON DUPLICATE KEY UPDATE display_name=VALUES(display_name)`, sk.name, sk.display, sk.desc, sk.cat, sk.risk)
		if err != nil {
			return err
		}
	}
	know := []struct {
		name, desc string
		dept       int
	}{{"Corporate Policies", "Company-wide policies and handbook", 0}, {"R&D Knowledge Base", "Engineering patterns and runbooks", 1}, {"Network Operations KB", "Network operation references", 3}, {"Finance Policies", "Finance policies and controls", 5}}
	for _, k := range know {
		_, err = db.Exec(`INSERT INTO knowledge_bases(organization_id,owner_department_id,name,description,visibility,status,document_count,last_indexed) VALUES (1,?,?,?,?, 'active',?,UTC_TIMESTAMP()) ON DUPLICATE KEY UPDATE description=VALUES(description)`, nullableID(int64(k.dept)), k.name, k.desc, "department", []int{24, 86, 42, 18}[len(k.name)%4])
		if err != nil {
			return err
		}
	}
	var userID, deptID, profileID, modelID, runtimeID int64
	_ = db.QueryRow(`SELECT id FROM users WHERE username='developer01'`).Scan(&userID)
	_ = db.QueryRow(`SELECT id FROM departments WHERE name='软件开发部'`).Scan(&deptID)
	_ = db.QueryRow(`SELECT id FROM profiles WHERE name='coding-assistant'`).Scan(&profileID)
	_ = db.QueryRow(`SELECT id FROM models WHERE name='standard'`).Scan(&modelID)
	_ = db.QueryRow(`SELECT id FROM runtimes WHERE user_id=?`, userID).Scan(&runtimeID)
	var usageCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&usageCount)
	if usageCount == 0 {
		for i := 0; i < 12; i++ {
			_, err = db.Exec(`INSERT INTO usage_events(organization_id,department_id,user_id,profile_id,session_id,execution_id,model_id,runtime_id,token_input,token_output,requests,executions,skill_calls,tool_calls,latency_ms,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,UTC_TIMESTAMP()-INTERVAL ? DAY)`, 1, deptID, userID, profileID, fmt.Sprintf("session-%d", i), fmt.Sprintf("execution-%d", i), modelID, runtimeID, 420+i*18, 810+i*31, 1, 1, 2+i%4, 1, 680+i*22, i%8)
			if err != nil {
				return err
			}
		}
	}
	_, _ = db.Exec(`INSERT INTO audit_logs(actor_user_id,action,resource_type,resource_id,scope,result,ip_address,metadata) VALUES (1,'seed.bootstrap','organization',1,'global','success','127.0.0.1','{"source":"demo-seed"}')`)
	return nil
}
