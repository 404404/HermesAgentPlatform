package main

import "testing"

func TestRuntimeHostStatusVocabulary(t *testing.T) {
	for _, status := range []string{"online", "offline", "degraded", "maintenance", "draining", "unknown"} {
		if !validRuntimeHostStatus(status) {
			t.Fatalf("expected %q to be accepted", status)
		}
	}
	if validRuntimeHostStatus("healthy") {
		t.Fatal("legacy healthy status must be normalized before the v0.3.2 API")
	}
}
