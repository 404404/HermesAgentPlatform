package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/example/hermes-enterprise-platform/backend/internal/providers"
	"github.com/gin-gonic/gin"
)

// ensureInlineSecret receives a write-only operational credential and stores it
// using the AES-256-GCM SecretProvider. API responses only carry metadata.
func (s *server) ensureInlineSecret(c *gin.Context, name, secretType string, existingID int64, value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return existingID, nil
	}
	if existingID > 0 {
		metadata, err := s.secrets.UpdateSecret(c.Request.Context(), existingID, []byte(value))
		return metadata.ID, err
	}
	secretName := "inline-" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
	metadata, err := s.secrets.CreateSecret(c.Request.Context(), providers.SecretInput{
		OrganizationID: s.currentOrg(c), Name: secretName, Type: secretType, Scope: "organization", Value: []byte(value),
	})
	return metadata.ID, err
}

type runtimeHostRequestV032 struct {
	Name             string   `json:"name"`
	Hostname         string   `json:"hostname"`
	Address          string   `json:"address"`
	SSHPort          int      `json:"ssh_port"`
	SSHUsername      string   `json:"ssh_username"`
	AuthType         string   `json:"auth_type"`
	Credential       string   `json:"credential"`
	DockerSocketPath string   `json:"docker_socket_path"`
	DockerBinary     string   `json:"docker_binary"`
	CPUTotal         string   `json:"cpu_total"`
	MemoryTotal      string   `json:"memory_total"`
	StorageTotal     string   `json:"storage_total"`
	Labels           []string `json:"labels"`
	Status           string   `json:"status"`
	Description      string   `json:"description"`
}

func validRuntimeHostStatus(status string) bool {
	for _, value := range []string{"online", "offline", "degraded", "maintenance", "draining", "unknown"} {
		if status == value {
			return true
		}
	}
	return false
}

func (s *server) listRuntimeHostsV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	rows, err := s.db.Query(`SELECT id,name,hostname,address,ssh_port,ssh_username,auth_type,credential_reference_id,docker_socket_path,docker_binary,docker_version,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,cpu_actual,memory_actual,storage_actual,runtime_count,container_count,status,labels,description,last_seen,last_inventory_at FROM runtime_hosts WHERE organization_id=? ORDER BY name`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "runtime_hosts.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, port, runtimeCount, containers int64
		var name, hostname, address, username, auth, socket, binary, version, cpu, memory, storage, allocatedCPU, allocatedMemory, allocatedStorage, actualCPU, actualMemory, actualStorage, status, labels string
		var credential sql.NullInt64
		var description sql.NullString
		var last, inventory sql.NullTime
		if rows.Scan(&id, &name, &hostname, &address, &port, &username, &auth, &credential, &socket, &binary, &version, &cpu, &memory, &storage, &allocatedCPU, &allocatedMemory, &allocatedStorage, &actualCPU, &actualMemory, &actualStorage, &runtimeCount, &containers, &status, &labels, &description, &last, &inventory) == nil {
			out = append(out, gin.H{"id": id, "name": name, "hostname": hostname, "address": address, "ssh_port": port, "ssh_username": username, "auth_type": auth, "credential_status": s.credentialStatus(c.Request.Context(), credential), "credential_reference_configured": s.credentialStatus(c.Request.Context(), credential) == "configured", "docker_socket_path": socket, "docker_binary": binary, "docker_version": version, "cpu_total": cpu, "memory_total": memory, "storage_total": storage, "cpu_allocated": allocatedCPU, "memory_allocated": allocatedMemory, "storage_allocated": allocatedStorage, "cpu_actual": actualCPU, "memory_actual": actualMemory, "storage_actual": actualStorage, "runtime_count": runtimeCount, "container_count": containers, "status": status, "labels": phase3JSON(labels), "description": description.String, "last_seen": nullableTime(last), "last_inventory_at": nullableTime(inventory)})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "provider": "MockRuntimeHostProvider", "security": gin.H{"docker_socket_exposed": false, "credential_write_only": true}})
}

func (s *server) saveRuntimeHostV032(c *gin.Context, id int64, req runtimeHostRequestV032) (int64, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Hostname) == "" || strings.TrimSpace(req.Address) == "" {
		return 0, fmt.Errorf("name, hostname and address are required")
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.AuthType == "" {
		req.AuthType = "password"
	}
	if req.AuthType != "password" && req.AuthType != "ssh_key" && req.AuthType != "certificate" && req.AuthType != "agent" {
		return 0, fmt.Errorf("unsupported auth type")
	}
	if req.DockerSocketPath == "" {
		req.DockerSocketPath = "/var/run/docker.sock"
	}
	if req.DockerBinary == "" {
		req.DockerBinary = "docker"
	}
	if req.CPUTotal == "" {
		req.CPUTotal = "8 CPU"
	}
	if req.MemoryTotal == "" {
		req.MemoryTotal = "16 GB"
	}
	if req.StorageTotal == "" {
		req.StorageTotal = "200 GB"
	}
	if req.Status == "" {
		req.Status = "unknown"
	}
	if !validRuntimeHostStatus(req.Status) {
		return 0, fmt.Errorf("invalid host status")
	}
	var existingCredential int64
	if id > 0 {
		_ = s.db.QueryRow(`SELECT COALESCE(credential_reference_id,0) FROM runtime_hosts WHERE id=? AND organization_id=?`, id, s.currentOrg(c)).Scan(&existingCredential)
	}
	credential, err := s.ensureInlineSecret(c, "runtime-host-"+req.Name+"-credential", "ssh_password", existingCredential, req.Credential)
	if err != nil {
		return 0, err
	}
	labels, _ := jsonMarshalV032(req.Labels)
	if id == 0 {
		res, err := s.db.Exec(`INSERT INTO runtime_hosts(organization_id,name,hostname,address,ssh_port,ssh_username,auth_type,credential_reference_id,docker_socket_path,docker_binary,cpu_total,memory_total,storage_total,labels,description,status,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.currentOrg(c), req.Name, req.Hostname, req.Address, req.SSHPort, req.SSHUsername, req.AuthType, nullableID(credential), req.DockerSocketPath, req.DockerBinary, req.CPUTotal, req.MemoryTotal, req.StorageTotal, labels, req.Description, req.Status, currentUserID(c))
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = s.db.Exec(`UPDATE runtime_hosts SET name=?,hostname=?,address=?,ssh_port=?,ssh_username=?,auth_type=?,credential_reference_id=?,docker_socket_path=?,docker_binary=?,cpu_total=?,memory_total=?,storage_total=?,labels=?,description=?,status=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, req.Name, req.Hostname, req.Address, req.SSHPort, req.SSHUsername, req.AuthType, nullableID(credential), req.DockerSocketPath, req.DockerBinary, req.CPUTotal, req.MemoryTotal, req.StorageTotal, labels, req.Description, req.Status, id, s.currentOrg(c))
	return id, err
}

func jsonMarshalV032(v []string) (string, error) { b, err := json.Marshal(v); return string(b), err }

func (s *server) createRuntimeHostV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	var req runtimeHostRequestV032
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime_hosts.invalid_request", nil)
		return
	}
	id, err := s.saveRuntimeHostV032(c, 0, req)
	if err != nil {
		failCode(c, 409, "runtime_hosts.save_failed", gin.H{"reason": err.Error()})
		return
	}
	s.auditControlPlane(c, "runtime_host.create", "Runtime Host Created", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"id": id, "status": req.Status}})
}
func (s *server) updateRuntimeHostV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req runtimeHostRequestV032
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "runtime_hosts.invalid_request", nil)
		return
	}
	if _, err := s.saveRuntimeHostV032(c, id, req); err != nil {
		failCode(c, 400, "runtime_hosts.save_failed", gin.H{"reason": err.Error()})
		return
	}
	s.auditControlPlane(c, "runtime_host.update", "Runtime Host Updated", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) testRuntimeHostV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var auth string
	var credential sql.NullInt64
	if s.db.QueryRow(`SELECT auth_type,credential_reference_id FROM runtime_hosts WHERE id=? AND organization_id=?`, id, s.currentOrg(c)).Scan(&auth, &credential) != nil {
		failCode(c, 404, "runtime_hosts.not_found", nil)
		return
	}
	if auth == "password" {
		if err := s.decryptCredentialForIntegration(c.Request.Context(), credential); err != nil {
			failCode(c, http.StatusConflict, "runtime_hosts.credential_unavailable", gin.H{"status": s.credentialStatus(c.Request.Context(), credential)})
			return
		}
	}
	_ = providers.MockRuntimeHostProvider{}
	if _, err := s.db.Exec(`UPDATE runtime_hosts SET status='online',docker_version='mock-docker-27',cpu_actual=cpu_allocated,memory_actual=memory_allocated,storage_actual=storage_allocated,last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, id, s.currentOrg(c)); err != nil {
		failCode(c, 400, "runtime_hosts.test_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.test", "Runtime Host Tested", "Runtime", "runtime_host", id, "success", gin.H{"provider": "MockRuntimeHostProvider"}, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": "online", "provider": "MockRuntimeHostProvider", "checks": []string{"ssh", "authentication", "os", "docker_binary", "docker_version", "docker_socket", "cpu", "memory", "disk"}}})
}
func (s *server) inventoryRuntimeHostV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	_, err := s.db.Exec(`UPDATE runtime_hosts h SET h.runtime_count=(SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id),h.container_count=(SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id),h.cpu_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id),' CPU'),h.memory_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id)*2,' GB'),h.storage_allocated=CONCAT((SELECT COUNT(*) FROM runtimes r WHERE r.host_id=h.id)*10,' GB'),h.cpu_actual=h.cpu_allocated,h.memory_actual=h.memory_allocated,h.storage_actual=h.storage_allocated,h.last_inventory_at=UTC_TIMESTAMP(),h.last_seen=UTC_TIMESTAMP(),h.status='online',h.updated_at=UTC_TIMESTAMP() WHERE h.id=? AND h.organization_id=?`, id, s.currentOrg(c))
	if err != nil {
		failCode(c, 400, "runtime_hosts.inventory_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.inventory", "Runtime Host Inventory Updated", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": gin.H{"id": id, "status": "online", "provider": "MockRuntimeHostProvider"}})
}

func (s *server) runtimeHostDetailV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	hosts := []gin.H{}
	rows, _ := s.db.Query(`SELECT r.id,r.runtime_id,u.display_name,r.status FROM runtimes r JOIN users u ON u.id=r.user_id WHERE r.host_id=? ORDER BY r.runtime_id`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var rid int64
			var runtimeID, user, status string
			if rows.Scan(&rid, &runtimeID, &user, &status) == nil {
				hosts = append(hosts, gin.H{"id": rid, "runtime_id": runtimeID, "user_name": user, "status": status})
			}
		}
	}
	var value gin.H
	list := gin.Context{}
	_ = list // resolved below using the same safe projection as list endpoint
	var name, hostname, address, username, auth, socket, binary, version, cpu, memory, storage, allocatedCPU, allocatedMemory, allocatedStorage, actualCPU, actualMemory, actualStorage, status, labels string
	var port, count, containers int64
	var credential sql.NullInt64
	var description sql.NullString
	var last, inventory sql.NullTime
	err := s.db.QueryRow(`SELECT name,hostname,address,ssh_port,ssh_username,auth_type,credential_reference_id,docker_socket_path,docker_binary,docker_version,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,cpu_actual,memory_actual,storage_actual,status,labels,description,runtime_count,container_count,last_seen,last_inventory_at FROM runtime_hosts WHERE id=? AND organization_id=?`, id, s.currentOrg(c)).Scan(&name, &hostname, &address, &port, &username, &auth, &credential, &socket, &binary, &version, &cpu, &memory, &storage, &allocatedCPU, &allocatedMemory, &allocatedStorage, &actualCPU, &actualMemory, &actualStorage, &status, &labels, &description, &count, &containers, &last, &inventory)
	if err != nil {
		failCode(c, 404, "runtime_hosts.not_found", nil)
		return
	}
	value = gin.H{"id": id, "name": name, "hostname": hostname, "address": address, "ssh_port": port, "ssh_username": username, "auth_type": auth, "credential_status": s.credentialStatus(c.Request.Context(), credential), "credential_reference_configured": s.credentialStatus(c.Request.Context(), credential) == "configured", "docker_socket_path": socket, "docker_binary": binary, "docker_version": version, "cpu_total": cpu, "memory_total": memory, "storage_total": storage, "cpu_allocated": allocatedCPU, "memory_allocated": allocatedMemory, "storage_allocated": allocatedStorage, "cpu_actual": actualCPU, "memory_actual": actualMemory, "storage_actual": actualStorage, "runtime_count": count, "container_count": containers, "status": status, "labels": phase3JSON(labels), "description": description.String, "last_seen": nullableTime(last), "last_inventory_at": nullableTime(inventory), "runtimes": hosts}
	c.JSON(200, gin.H{"data": value})
}
func (s *server) setRuntimeHostStatusV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil || !validRuntimeHostStatus(req.Status) {
		failCode(c, 400, "runtime_hosts.invalid_status", nil)
		return
	}
	_, err := s.db.Exec(`UPDATE runtime_hosts SET status=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, req.Status, id, s.currentOrg(c))
	if err != nil {
		failCode(c, 400, "runtime_hosts.status_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.status", "Runtime Host Status Changed", "Runtime", "runtime_host", id, "success", gin.H{"status": req.Status}, nil)
	c.JSON(200, gin.H{"data": true})
}
func (s *server) deleteRuntimeHostV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var count int
	if s.db.QueryRow(`SELECT COUNT(*) FROM runtimes WHERE host_id=?`, id).Scan(&count) != nil {
		failCode(c, 404, "runtime_hosts.not_found", nil)
		return
	}
	if count > 0 {
		failCode(c, 409, "runtime_hosts.has_runtimes", gin.H{"runtime_count": count})
		return
	}
	if _, err := s.db.Exec(`DELETE FROM runtime_hosts WHERE id=? AND organization_id=?`, id, s.currentOrg(c)); err != nil {
		failCode(c, 400, "runtime_hosts.delete_failed", nil)
		return
	}
	s.auditControlPlane(c, "runtime_host.delete", "Runtime Host Deleted", "Runtime", "runtime_host", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

type providerRequestV032 struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Mode        string `json:"mode"`
	BaseURL     string `json:"base_url"`
	AuthType    string `json:"auth_type"`
	Description string `json:"description"`
	Credential  string `json:"credential"`
}

func (s *server) saveModelProviderV032(c *gin.Context, id int64, req providerRequestV032) (int64, error) {
	if strings.TrimSpace(req.Name) == "" {
		return 0, fmt.Errorf("name required")
	}
	if req.Type == "" {
		req.Type = "custom"
	}
	if req.Mode == "" {
		req.Mode = "hermes_native"
	}
	if req.AuthType == "" {
		req.AuthType = "api_key"
	}
	var existing int64
	if id > 0 {
		_ = s.db.QueryRow(`SELECT COALESCE(secret_reference_id,0) FROM model_providers WHERE id=? AND organization_id=?`, id, s.currentOrg(c)).Scan(&existing)
	}
	secret, err := s.ensureInlineSecret(c, "model-provider-"+req.Name+"-credential", "api_key", existing, req.Credential)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		res, err := s.db.Exec(`INSERT INTO model_providers(organization_id,name,type,mode,base_url,auth_type,secret_reference_id,status,description,created_by) VALUES(?,?,?,?,?,?,?,'active',?,?)`, s.currentOrg(c), req.Name, req.Type, req.Mode, req.BaseURL, req.AuthType, nullableID(secret), req.Description, currentUserID(c))
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}
	_, err = s.db.Exec(`UPDATE model_providers SET name=?,type=?,mode=?,base_url=?,auth_type=?,secret_reference_id=?,description=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND organization_id=?`, req.Name, req.Type, req.Mode, req.BaseURL, req.AuthType, nullableID(secret), req.Description, id, s.currentOrg(c))
	return id, err
}
func (s *server) createModelProviderV032(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	var req providerRequestV032
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "provider.invalid_request", nil)
		return
	}
	id, err := s.saveModelProviderV032(c, 0, req)
	if err != nil {
		failCode(c, 409, "provider.create_failed", nil)
		return
	}
	s.auditControlPlane(c, "model_provider.create", "Model Provider Created", "Models", "model_provider", id, "success", nil, nil)
	c.JSON(201, gin.H{"data": gin.H{"id": id}})
}
func (s *server) updateModelProviderV032(c *gin.Context) {
	if !s.requirePermission(c, "model_provider.manage") {
		return
	}
	id, ok := paramID(c, "id")
	if !ok {
		return
	}
	var req providerRequestV032
	if c.ShouldBindJSON(&req) != nil {
		failCode(c, 400, "provider.invalid_request", nil)
		return
	}
	if _, err := s.saveModelProviderV032(c, id, req); err != nil {
		failCode(c, 400, "provider.update_failed", nil)
		return
	}
	s.auditControlPlane(c, "model_provider.update", "Model Provider Updated", "Models", "model_provider", id, "success", nil, nil)
	c.JSON(200, gin.H{"data": true})
}

func (s *server) resourceUsageV032(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	rows, err := s.db.Query(`SELECT name,cpu_total,memory_total,storage_total,cpu_allocated,memory_allocated,storage_allocated,cpu_actual,memory_actual,storage_actual,runtime_count,container_count,status FROM runtime_hosts WHERE organization_id=? ORDER BY name`, s.currentOrg(c))
	if err != nil {
		failCode(c, 500, "runtime_hosts.load_failed", nil)
		return
	}
	defer rows.Close()
	hosts := []gin.H{}
	for rows.Next() {
		var name, cpu, memory, storage, allocatedCPU, allocatedMemory, allocatedStorage, actualCPU, actualMemory, actualStorage, status string
		var runtimes, containers int
		if rows.Scan(&name, &cpu, &memory, &storage, &allocatedCPU, &allocatedMemory, &allocatedStorage, &actualCPU, &actualMemory, &actualStorage, &runtimes, &containers, &status) == nil {
			hosts = append(hosts, gin.H{"name": name, "cpu_total": cpu, "memory_total": memory, "storage_total": storage, "cpu_allocated": allocatedCPU, "memory_allocated": allocatedMemory, "storage_allocated": allocatedStorage, "cpu_actual": actualCPU, "memory_actual": actualMemory, "storage_actual": actualStorage, "runtime_count": runtimes, "container_count": containers, "status": status})
		}
	}
	var users, profiles, runtimes int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE organization_id=? AND status='active' AND system_account=FALSE", s.currentOrg(c)).Scan(&users)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM profiles p JOIN users u ON u.id=p.user_id WHERE u.organization_id=?", s.currentOrg(c)).Scan(&profiles)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM runtimes r JOIN users u ON u.id=r.user_id WHERE u.organization_id=?", s.currentOrg(c)).Scan(&runtimes)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"active_users": users, "profiles": profiles, "runtimes": runtimes, "host_count": len(hosts), "hosts": hosts, "note": "AI usage and host capacity are separate accounting domains in the demo."}})
}
