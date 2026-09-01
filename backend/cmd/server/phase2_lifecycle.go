package main

func (s *server) ensureAutomaticProvisioned(userID int64) {
	var mode string
	if s.db.QueryRow("SELECT setting_value FROM system_settings WHERE organization_id=1 AND setting_key='runtime_provisioning'").Scan(&mode) != nil || mode != "Automatic" {
		s.consolidateManagedAgents(userID)
		return
	}
	var templateID int64
	_ = s.db.QueryRow("SELECT id FROM runtime_templates WHERE organization_id=1 AND is_default=TRUE AND status='active' LIMIT 1").Scan(&templateID)
	if templateID > 0 {
		_, _ = s.db.Exec(`UPDATE runtimes r JOIN runtime_templates t ON t.id=? SET r.status=IF(t.auto_start=TRUE,'running','stopped'),r.template_id=t.id,r.cpu_limit=t.cpu_limit,r.memory_limit=t.memory_limit,r.storage_limit=t.storage_limit,r.profile_limit=t.profile_limit,r.max_concurrent_jobs=t.max_concurrent_jobs,r.image_version=t.image_version,r.runtime_provider=t.runtime_provider,r.runtime_class=t.runtime_class,r.network_policy=t.network_policy,r.auto_start=t.auto_start,r.auto_suspend=t.auto_suspend,r.last_seen=IF(t.auto_start=TRUE,UTC_TIMESTAMP(),r.last_seen),r.updated_at=UTC_TIMESTAMP() WHERE r.user_id=?`, templateID, userID)
	} else {
		_, _ = s.db.Exec("UPDATE runtimes SET status='running',last_seen=UTC_TIMESTAMP(),updated_at=UTC_TIMESTAMP() WHERE user_id=?", userID)
	}
	s.assignProfileTemplates(userID)
	s.consolidateManagedAgents(userID)
}
