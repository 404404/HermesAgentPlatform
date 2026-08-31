package main

import "testing"

func TestSplitSQL(t *testing.T) {
	statements := splitSQL("CREATE TABLE a (id INT);\n\nCREATE TABLE b (id INT);")
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}
}

func TestNullableID(t *testing.T) {
	if nullableID(0) != nil {
		t.Fatal("zero id should map to SQL NULL")
	}
	if nullableID(4) != int64(4) {
		t.Fatal("non-zero id should be preserved")
	}
}
