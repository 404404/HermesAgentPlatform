package main

import "strings"

// MySQL commits DDL implicitly. If a process restarts after a partially
// applied migration, only duplicate schema definitions are safe to skip.
func ignorableSchemaError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Error 1050") || // table already exists
		strings.Contains(message, "Error 1060") || // duplicate column
		strings.Contains(message, "Error 1826") || // duplicate foreign key
		strings.Contains(message, "Error 1831") // duplicate index
}
