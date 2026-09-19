package main

import "github.com/gin-gonic/gin"

// Workspace Models is a union of models effective for the user's own active
// profiles. It never exposes a provider disabled by the administrator.
func (s *server) workspaceModelsV033Hotfix(c *gin.Context) {
	if !s.requireWorkspaceUser(c) {
		return
	}
	uid := currentUserID(c)
	profiles := s.workspaceProfilesV033(uid)
	byID := map[int64]gin.H{}
	for _, profile := range profiles {
		profileID, _ := profile["id"].(int64)
		for _, model := range s.allowedModelsForProfileV033(uid, profileID) {
			modelID, _ := model["id"].(int64)
			if modelID == 0 || byID[modelID] != nil {
				continue
			}
			var display, providerName, purpose, status string
			if s.db.QueryRow(`SELECT m.display_name,COALESCE(mp.name,m.provider),m.purpose,m.status
				FROM models m LEFT JOIN model_providers mp ON mp.id=m.provider_id WHERE m.id=?`, modelID).
				Scan(&display, &providerName, &purpose, &status) == nil {
				model["display_name"] = display
				model["provider_name"] = providerName
				model["purpose"] = purpose
				model["status"] = status
				byID[modelID] = model
			}
		}
	}
	items := make([]gin.H, 0, len(byID))
	for _, item := range byID {
		items = append(items, item)
	}
	c.JSON(200, gin.H{"data": gin.H{"models": items, "slots": hermesAuxiliarySlots, "policy": s.effectiveModelPolicy(uid)}})
}
