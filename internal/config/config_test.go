package config

import (
	"testing"
)

func TestUniqueNameAvoidsCollisions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dsnA := "postgres://u:p@h:5432/appdb?sslmode=disable"
	dsnB := "postgres://u:p@other:5433/appdb?sslmode=disable"

	if got := UniqueName(dsnA); got != "appdb" {
		t.Fatalf("first name = %q, want appdb", got)
	}
	if err := Save("appdb", dsnA, false); err != nil {
		t.Fatal(err)
	}
	if got := UniqueName(dsnA); got != "appdb" {
		t.Fatalf("same DSN should reuse existing name, got %q", got)
	}
	if got := UniqueName(dsnB); got != "appdb-2" {
		t.Fatalf("colliding DSN should get suffix, got %q", got)
	}
	if err := Save("appdb-2", dsnB, false); err != nil {
		t.Fatal(err)
	}
	if got := UniqueName(dsnB); got != "appdb-3" {
		t.Fatalf("third colliding DSN should get next suffix, got %q", got)
	}
}

func TestSaveHistoryRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := []string{"select 1", "select 2"}
	if err := SaveHistory(want); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.History) != 2 || cfg.History[0] != "select 1" || cfg.History[1] != "select 2" {
		t.Fatalf("history = %v", cfg.History)
	}
}
