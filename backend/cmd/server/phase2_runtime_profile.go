package main

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *server) listRuntimesV2(c *gin.Context) {
	if !s.requirePermission(c, "runtime.read") {
		return
	}
	rows, err := s.db.Query(`SELECT r.id,r.user_id,u.display_name,r.runtime_id,r.status,r.desired_status,r.observed_status,r.provider,r.hermes_version,
		(SELECT COUNT(*) FROM profiles p WHERE p.user_id=r.user_id),r.cpu_limit,r.memory_limit,r.storage_limit,r.profile_limit,
		r.max_concurrent_jobs,r.image_version,r.runtime_provider,r.runtime_class,r.network_policy,r.auto_start,r.auto_suspend,
		r.template_id,r.created_at,r.last_seen FROM runtimes r JOIN users u ON u.id=r.user_id ORDER BY r.created_at DESC`)
	if err != nil {
		failCode(c, 500, "runtimes.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, uid, profileLimit, jobs int64
		var user, rid, status, desired, observed, provider, version, cpu, memory, storage, image, runtimeProvider, class, network, created string
		var autoStart, autoSuspend bool
		var templateID sql.NullInt64
		var last sql.NullTime
		var profiles int
		if err := rows.Scan(&id, &uid, &user, &rid, &status, &desired, &observed, &provider, &version, &profiles, &cpu, &memory, &storage, &profileLimit, &jobs, &image, &runtimeProvider, &class, &network, &autoStart, &autoSuspend, &templateID, &created, &last); err != nil {
			continue
		}
		templateValue := any(nil)
		if templateID.Valid {
			templateValue = templateID.Int64
		}
		lastValue := any(nil)
		if last.Valid {
			lastValue = last.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, gin.H{"id": id, "user_id": uid, "user": user, "runtime_id": rid, "status": status, "desired_status": desired, "observed_status": observed, "provider": provider, "hermes_version": version, "profile_count": profiles, "cpu_limit": cpu, "memory_limit": memory, "storage_limit": storage, "profile_limit": profileLimit, "max_concurrent_jobs": jobs, "image_version": image, "runtime_provider": runtimeProvider, "runtime_class": class, "network_policy": network, "auto_start": autoStart, "auto_suspend": autoSuspend, "template_id": templateValue, "created_at": created, "last_seen": lastValue})
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) listProfilesV2(c *gin.Context) {
	if !s.requirePermission(c, "profile.read") {
		return
	}
	rows, err := s.db.Query(`SELECT p.id,p.user_id,u.display_name,p.name,p.display_name,p.description,p.status,p.model_id,
		COALESCE(m.display_name,''),p.runtime_class,p.profile_type,p.managed,COALESCE(p.source_template_id,0),p.created_at,p.updated_at
		FROM profiles p JOIN users u ON u.id=p.user_id LEFT JOIN models m ON m.id=p.model_id ORDER BY p.created_at DESC`)
	if err != nil {
		failCode(c, 500, "profiles.load_failed", nil)
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, uid, model, template int64
		var user, name, display, desc, status, class, profileType, modelName, created, updated string
		var managed bool
		if rows.Scan(&id, &uid, &user, &name, &display, &desc, &status, &model, &modelName, &class, &profileType, &managed, &template, &created, &updated) == nil {
			out = append(out, gin.H{"id": id, "user_id": uid, "user": user, "name": name, "display_name": display, "description": desc, "status": status, "model_id": model, "model": modelName, "runtime_class": class, "profile_type": profileType, "managed": managed, "source_template_id": template, "created_at": created, "updated_at": updated})
		}
	}
	c.JSON(200, gin.H{"data": out})
}

func (s *server) orchestrateUserLifecycle(userID int64, status string) {
	switch status {
	case "pending":
		_, _ = s.db.Exec("UPDATE runtimes SET status='not_provisioned', updated_at=UTC_TIMESTAMP() WHERE user_id=?", userID)
	case "suspended", "disabled", "archived":
		_, _ = s.db.Exec("UPDATE runtimes SET status='stopped', updated_at=UTC_TIMESTAMP() WHERE user_id=?", userID)
		_, _ = s.db.Exec("UPDATE profiles SET status='disabled', updated_at=UTC_TIMESTAMP() WHERE user_id=? AND managed=TRUE", userID)
	case "active":
		_, _ = s.db.Exec("UPDATE profiles SET status='active', updated_at=UTC_TIMESTAMP() WHERE user_id=? AND managed=TRUE", userID)
		s.ensureAutomaticProvisioned(userID)
	}
}
