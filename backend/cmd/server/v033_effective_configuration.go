package main

import (
	"database/sql"

	"github.com/gin-gonic/gin"
)

// These wrappers preserve the original centralized resolver and add the two
// new user-owned relationship sources introduced by the completion hotfix.
func (s *server) effectiveSkillsV033Completion(profileID, uid, orgID, deptID int64, templateID sql.NullInt64, templateName string) []gin.H {
	base := s.effectiveSkills(profileID, uid, orgID, deptID, templateID, templateName)
	seen := map[int64]bool{}
	for _, item := range base {
		if id, ok := item["id"].(int64); ok {
			seen[id] = true
		}
	}
	rows, err := s.db.Query(`SELECT psi.skill_id,s.name,s.display_name,s.category,s.risk_level,psi.skill_version_id,COALESCE(sv.version,''),psi.runtime_apply_status
		FROM profile_skill_installs psi
		JOIN skills s ON s.id=psi.skill_id
		LEFT JOIN skill_versions sv ON sv.id=psi.skill_version_id
		WHERE psi.profile_id=? AND psi.status='installed'`, profileID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, versionID int64
			var name, display, category, risk, version, applyStatus string
			if rows.Scan(&id, &name, &display, &category, &risk, &versionID, &version, &applyStatus) == nil && !seen[id] {
				base = append(base, gin.H{"id": id, "name": name, "display_name": display, "category": category, "risk": risk,
					"policy": "explicit", "source": "profile_install", "enabled": true, "skill_version_id": versionID,
					"version": version, "runtime_apply_status": applyStatus})
				seen[id] = true
			}
		}
	}
	return base
}

func (s *server) effectiveKnowledgeV033Completion(profileID, uid, orgID, deptID int64, templateID sql.NullInt64, templateName string) []gin.H {
	base := s.effectiveKnowledgeSources(profileID, uid, orgID, deptID, templateID, templateName)
	filtered := make([]gin.H, 0, len(base))
	seen := map[int64]bool{}
	for _, item := range base {
		id, ok := item["id"].(int64)
		if !ok || s.effectiveKnowledgeAccessV033(uid, id) == "none" {
			continue
		}
		filtered = append(filtered, item)
		seen[id] = true
	}
	var overrides string
	_ = s.db.QueryRow("SELECT knowledge_override_ids FROM profiles WHERE id=?", profileID).Scan(&overrides)
	for _, kbID := range completionJSONIDs(overrides) {
		if seen[kbID] || s.effectiveKnowledgeAccessV033(uid, kbID) == "none" {
			continue
		}
		var name, owner, status string
		if s.db.QueryRow(`SELECT kb.name,COALESCE(d.name,''),kb.status FROM knowledge_bases kb
			LEFT JOIN departments d ON d.id=kb.owner_department_id WHERE kb.id=?`, kbID).Scan(&name, &owner, &status) == nil {
			filtered = append(filtered, gin.H{"id": kbID, "name": name, "owner_department": owner, "status": status,
				"source": "profile_override", "policy": "explicit"})
			seen[kbID] = true
		}
	}
	return filtered
}
