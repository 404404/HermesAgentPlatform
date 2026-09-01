package main

import "testing"

func TestRiskEvaluatorRules(t *testing.T) {
	evaluator := NewRiskEvaluator()
	cases := []struct {
		action, level string
		score         float64
	}{
		{"auth.break_glass.login", "critical", 100},
		{"role.assign", "high", 85},
		{"runtime.restart", "medium", 45},
		{"user.read", "low", 10},
	}
	for _, tc := range cases {
		got := evaluator.Evaluate(tc.action, "resource")
		if got.Level != tc.level || got.Score != tc.score {
			t.Fatalf("%s: got %#v", tc.action, got)
		}
	}
}

func TestArtifactPathBoundary(t *testing.T) {
	for _, path := range []string{"SKILL.md", "scripts/check.sh", "templates/prompt.txt"} {
		if !safeArtifactPath(path) {
			t.Fatalf("expected safe path: %s", path)
		}
	}
	for _, path := range []string{"", "/etc/passwd", "../secrets", "references/../../secret"} {
		if safeArtifactPath(path) {
			t.Fatalf("expected rejected path: %s", path)
		}
	}
}

func TestValidRoleScopes(t *testing.T) {
	for _, scope := range []string{"global", "organization", "department", "user", "profile", "role"} {
		if !validScope(scope) {
			t.Fatalf("expected valid scope: %s", scope)
		}
	}
	if validScope("host") {
		t.Fatal("host must not be a role scope")
	}
}
