package main

import "strconv"

// effectiveKnowledgeAccessV033 resolves only rows that are actually related
// to the current user.  It deliberately treats Knowledge bindings as an
// allow-list, while the most-specific explicit user policy remains authoritative.
func (s *server) effectiveKnowledgeAccessV033(uid, knowledgeBaseID int64) string {
	var organizationID, departmentID int64
	if s.db.QueryRow("SELECT organization_id,COALESCE(department_id,0) FROM users WHERE id=?", uid).Scan(&organizationID, &departmentID) != nil {
		return "none"
	}
	roleIDs := []int64{}
	roleRows, _ := s.db.Query("SELECT role_id FROM role_bindings WHERE user_id=?", uid)
	if roleRows != nil {
		defer roleRows.Close()
		for roleRows.Next() {
			var roleID int64
			if roleRows.Scan(&roleID) == nil {
				roleIDs = append(roleIDs, roleID)
			}
		}
	}
	profileIDs, templateIDs := []int64{}, []int64{}
	profileRows, _ := s.db.Query("SELECT id,COALESCE(source_template_id,0) FROM profiles WHERE user_id=? AND status='active'", uid)
	if profileRows != nil {
		defer profileRows.Close()
		for profileRows.Next() {
			var profileID, templateID int64
			if profileRows.Scan(&profileID, &templateID) == nil {
				profileIDs = append(profileIDs, profileID)
				if templateID > 0 {
					templateIDs = append(templateIDs, templateID)
				}
			}
		}
	}

	bestRank, bestAccess := -1, "none"
	policyRows, err := s.db.Query("SELECT scope,scope_subject_key,access_level FROM knowledge_user_policies WHERE knowledge_base_id=?", knowledgeBaseID)
	if err == nil {
		defer policyRows.Close()
		for policyRows.Next() {
			var scope, key, access string
			if policyRows.Scan(&scope, &key, &access) != nil {
				continue
			}
			rank, match := 0, false
			switch scope {
			case "organization":
				rank, match = 1, key == "organization" || key == "organization:0" || key == "organization:"+strconv.FormatInt(organizationID, 10)
			case "department":
				rank, match = 2, key == "department:"+strconv.FormatInt(departmentID, 10)
			case "role":
				for _, roleID := range roleIDs {
					if key == "role:"+strconv.FormatInt(roleID, 10) {
						rank, match = 3, true
						break
					}
				}
			case "user":
				rank, match = 4, key == "user:"+strconv.FormatInt(uid, 10)
			}
			if match && rank >= bestRank {
				bestRank, bestAccess = rank, access
			}
		}
	}
	if bestRank >= 0 {
		return bestAccess
	}

	bindings, err := s.db.Query(`SELECT binding_type,COALESCE(organization_id,0),COALESCE(department_id,0),COALESCE(role_id,0),COALESCE(profile_id,0),COALESCE(agent_template_id,0),COALESCE(policy,'allow')
		FROM knowledge_bindings WHERE knowledge_base_id=?`, knowledgeBaseID)
	if err != nil {
		return "none"
	}
	defer bindings.Close()
	for bindings.Next() {
		var bindingType, policy string
		var orgID, deptID, roleID, profileID, templateID int64
		if bindings.Scan(&bindingType, &orgID, &deptID, &roleID, &profileID, &templateID, &policy) != nil || policy == "blocked" {
			continue
		}
		if (bindingType == "organization" && orgID == organizationID) ||
			(bindingType == "department" && deptID == departmentID) ||
			completionContains(roleIDs, roleID) || completionContains(profileIDs, profileID) || completionContains(templateIDs, templateID) {
			return "read"
		}
	}
	return "none"
}

func completionContains(values []int64, target int64) bool {
	for _, value := range values {
		if value == target && target > 0 {
			return true
		}
	}
	return false
}
