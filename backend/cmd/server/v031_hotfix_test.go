package main

import (
	"strings"
	"testing"
)

func TestDemoSeedPasswordRequiresExplicitNonDemoConfiguration(t *testing.T) {
	t.Setenv("HEP_TEST_DEMO_PASSWORD", "")
	if _, err := demoSeedPassword("HEP_TEST_DEMO_PASSWORD", false, "development-only"); err == nil {
		t.Fatal("non-Demo mode must reject an unset seeded password")
	}

	value, err := demoSeedPassword("HEP_TEST_DEMO_PASSWORD", true, "development-only")
	if err != nil || value != "development-only" {
		t.Fatalf("Demo mode fallback = %q, %v", value, err)
	}

	t.Setenv("HEP_TEST_DEMO_PASSWORD", "configured-password")
	value, err = demoSeedPassword("HEP_TEST_DEMO_PASSWORD", false, "development-only")
	if err != nil || value != "configured-password" {
		t.Fatalf("configured password = %q, %v", value, err)
	}
}

func TestWorkspacePermissionSeedCodes(t *testing.T) {
	// Keep the end-user permission names stable: they are the RBAC contract
	// granted to Standard User and Developer by v031SeedWorkspacePermissions.
	codes := workspacePermissionSeedCodes
	for _, code := range codes {
		if !strings.HasPrefix(code, "workspace.") {
			t.Fatalf("workspace permission must remain namespaced: %q", code)
		}
	}
}
