package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// A once file runs in a single transaction, where DSQL refuses DDL; catching
// it here beats failing the deploy.
func TestOnceFilesHoldOnlyDML(t *testing.T) {
	files, err := filepath.Glob("../../migrations/*.once.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no once files found")
	}
	ddl := regexp.MustCompile(`(?i)^\s*(CREATE|ALTER|DROP|GRANT|REVOKE|TRUNCATE|AWS)\b`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		stmts := statements(string(raw))
		if len(stmts) == 0 {
			t.Errorf("%s: no statements", f)
		}
		for _, s := range stmts {
			if ddl.MatchString(s) {
				t.Errorf("%s: DDL in a once file: %s", f, firstLine(s))
			}
		}
	}
}

func TestResetFreeDecisionsIsOnceAndOnlyTouchesTheFreeAllowance(t *testing.T) {
	const f = "../../migrations/004_reset_free_decisions.once.sql"
	if !isOnce(f) {
		t.Fatal("the reset must run once, not on every deploy")
	}
	raw, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	got := statements(string(raw))
	if len(got) != 1 || got[0] != "DELETE FROM free_decisions" {
		t.Fatalf("statements = %q", got)
	}
}
