package main

import (
	"strings"
	"testing"
)

func TestIntegrityChecksAreReadOnlyAndComplete(t *testing.T) {
	if len(integrityChecks) < 10 {
		t.Fatalf("expected broad integrity coverage, got %d checks", len(integrityChecks))
	}
	for _, check := range integrityChecks {
		query := strings.ToUpper(check.Query)
		if !strings.Contains(query, "SELECT") || strings.Contains(query, " INSERT ") || strings.Contains(query, " UPDATE ") || strings.Contains(query, " DELETE ") {
			t.Errorf("%s is not a read-only SELECT", check.Name)
		}
		if check.Name == "" || strings.TrimSpace(check.Query) == "" {
			t.Errorf("check has no name or query: %#v", check)
		}
	}
}

func TestCountedTablesHaveNoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(countedTables))
	for _, table := range countedTables {
		if seen[table] {
			t.Fatalf("duplicate table count: %s", table)
		}
		seen[table] = true
	}
}

func TestGetenvDefault(t *testing.T) {
	if got := getenvDefault("MIGRATION_INTEGRITY_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("getenvDefault() = %q, want fallback", got)
	}
}
