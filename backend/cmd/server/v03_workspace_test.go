package main

import (
	"strings"
	"testing"
)

func TestMockChatProvider(t *testing.T) {
	reply := (MockChatProvider{}).Reply("Research Assistant", "Summarize the policy")
	if !strings.Contains(reply, "Research Assistant") || !strings.Contains(reply, "Summarize the policy") {
		t.Fatalf("reply should identify the profile and prompt: %q", reply)
	}
}

func TestV03AuxiliarySlotsAndCapabilities(t *testing.T) {
	requiredSlots := []string{"title", "vision", "compression", "approval", "web_extract", "skills_hub", "mcp", "triage_specifier", "kanban_decomposer", "profile_describer", "curator"}
	for _, required := range requiredSlots {
		found := false
		for _, slot := range hermesAuxiliarySlots {
			if slot == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing auxiliary Hermes slot %q", required)
		}
	}
	for _, capability := range []string{"create_personal_profile", "change_main_model", "change_auxiliary_models", "add_model_provider", "configure_model_credentials", "install_optional_skill", "configure_channel", "configure_channel_credentials", "create_personal_knowledge"} {
		found := false
		for _, candidate := range selfServiceCapabilities {
			if candidate == capability {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing self-service capability %q", capability)
		}
	}
}
