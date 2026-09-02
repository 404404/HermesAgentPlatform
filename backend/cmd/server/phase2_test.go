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

func TestSkillPolicyPriority(t *testing.T) {
	if skillPolicyPriority("blocked") <= skillPolicyPriority("mandatory") {
		t.Fatal("blocked policy must win")
	}
	if skillPolicyPriority("mandatory") <= skillPolicyPriority("explicit") {
		t.Fatal("mandatory policy must win")
	}
}

func TestKnowledgeItemTypes(t *testing.T) {
	for _, typ := range []string{"background", "qa", "markdown", "procedure"} {
		if !validKnowledgeItemType(typ) {
			t.Fatalf("expected knowledge type %s", typ)
		}
	}
	if validKnowledgeItemType("plain_text") {
		t.Fatal("plain_text must not be a v0.2.1 item type")
	}
}

func TestKillSwitchIsCritical(t *testing.T) {
	risk := runtimeKillSwitchRisk("operator incident")
	if risk.Level != "critical" || risk.Score != 100 || risk.Reason == "" {
		t.Fatalf("unexpected kill switch risk: %#v", risk)
	}
}

func TestDomainAssignmentScopes(t *testing.T) {
	for _, scope := range []string{"global", "organization", "department", "user", "profile"} {
		if !validScope(scope) {
			t.Fatalf("scope %s should be valid", scope)
		}
	}
}

func TestUserCSVValidationAndRedaction(t *testing.T) {
	content := `username,display_name,email,password,department
new.user,New User,new.user.com,ChangeMe-2026!,R&D
`
	rows, errors, err := parseUserCSV(content)
	if err != nil || len(errors) != 0 || len(rows) != 1 {
		t.Fatalf("unexpected csv parse: rows=%d errors=%d err=%v", len(rows), len(errors), err)
	}
	if rows[0].Password == "" {
		t.Fatal("parser must retain the password for server-side hashing")
	}
	redacted := safeImportRows(rows)
	if redacted[0].Password != "" || redacted[0].Username != rows[0].Username {
		t.Fatal("preview rows must redact password only")
	}
}

func TestUserCSVRejectsDuplicateAndShortPassword(t *testing.T) {
	content := `username,display_name,email,password,department
new.user,New User,new.user.com,short,R&D
new.user,New User 2,new2.com,ChangeMe-2026!,R&D
`
	rows, errors, err := parseUserCSV(content)
	if err != nil || len(rows) != 0 || len(errors) != 2 {
		t.Fatalf("expected two invalid rows, rows=%d errors=%d err=%v", len(rows), len(errors), err)
	}
}
