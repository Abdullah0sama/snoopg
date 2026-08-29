package db

import (
	"context"
	"os"
	"testing"
)

const testDSN = "postgres://postgres:postgres@localhost:5432/automation_db?sslmode=disable"

func dsnForTest() string {
	if d := os.Getenv("SNOOPG_DSN"); d != "" {
		return d
	}
	return testDSN
}

func TestSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	client, err := New(ctx, dsnForTest())
	if err != nil {
		t.Skipf("skipping: cannot connect to postgres: %v", err)
	}
	defer client.Close()

	stats, err := client.CacheStats(ctx)
	if err != nil {
		t.Fatalf("CacheStats: %v", err)
	}
	if stats.Total != 16384 {
		t.Errorf("CacheStats.Total = %d, want 16384", stats.Total)
	}

	relations, err := client.RelationStats(ctx)
	if err != nil {
		t.Fatalf("RelationStats: %v", err)
	}
	if len(relations) == 0 {
		t.Error("RelationStats returned 0 relations")
	}

	rows, elapsed, err := client.Exec(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if rows != 1 {
		t.Errorf("Exec SELECT 1 affected %d rows, want 1", rows)
	}
	if elapsed <= 0 {
		t.Errorf("elapsed = %v, want > 0", elapsed)
	}

	tables, err := client.Catalog(ctx)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(tables) == 0 {
		t.Error("Catalog returned 0 tables")
	}
	found := false
	for _, tb := range tables {
		if tb.Name != "" && tb.SizeBytes >= 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("no table with non-empty Name and SizeBytes >= 0")
	}
}
