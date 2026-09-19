package main

import (
	"strings"
	"testing"
)

func TestSplitSQL(t *testing.T) {
	statements := splitSQL("CREATE TABLE a (id INT);\n\nCREATE TABLE b (id INT);")
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}
	commented := splitSQL("-- migration note\nCREATE TABLE c (id INT);\n-- next note\nALTER TABLE c ADD COLUMN name VARCHAR(32);")
	if len(commented) != 2 || strings.HasPrefix(commented[0], "--") {
		t.Fatalf("line comments should not swallow SQL statements: %#v", commented)
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
