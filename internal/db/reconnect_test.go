package db

import (
	"context"
	"sync"
	"testing"
)

const automationDSN = "postgres://postgres:postgres@localhost:5432/automation_db?sslmode=disable"

func mustConnect(t *testing.T, dsn string, readOnly bool) *Client {
	t.Helper()
	client, err := NewWithMode(context.Background(), dsn, readOnly)
	if err != nil {
		t.Skipf("skipping: cannot connect: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestReconnectBetweenServers(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx := context.Background()
	client := mustConnect(t, labDSN, false)

	if name, err := client.CurrentDatabase(ctx); err != nil || name != "snoopg_lab" {
		t.Fatalf("initial database = %q, err = %v", name, err)
	}

	if err := client.Reconnect(ctx, automationDSN, false); err != nil {
		t.Skipf("skipping: automation server unreachable: %v", err)
	}
	if name, err := client.CurrentDatabase(ctx); err != nil || name != "automation_db" {
		t.Fatalf("after reconnect database = %q, err = %v", name, err)
	}

	stats, err := client.CacheStats(ctx)
	if err != nil {
		t.Fatalf("CacheStats on automation: %v", err)
	}
	if stats.Total != 16384 {
		t.Errorf("automation shared_buffers = %d pages, want 16384", stats.Total)
	}

	if err := client.Reconnect(ctx, labDSN, false); err != nil {
		t.Fatalf("reconnect back: %v", err)
	}
	if name, err := client.CurrentDatabase(ctx); err != nil || name != "snoopg_lab" {
		t.Fatalf("back on lab: database = %q, err = %v", name, err)
	}
}

func TestSwitchDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx := context.Background()
	client := mustConnect(t, labDSN, false)

	dbs, err := client.ListDatabases(ctx)
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	hasLab, hasPostgres := false, false
	for _, d := range dbs {
		if d == "snoopg_lab" {
			hasLab = true
		}
		if d == "postgres" {
			hasPostgres = true
		}
	}
	if !hasLab || !hasPostgres {
		t.Fatalf("ListDatabases = %v, want snoopg_lab and postgres", dbs)
	}

	if _, err := client.SwitchDatabase(ctx, "postgres"); err != nil {
		t.Fatalf("switch to postgres: %v", err)
	}
	if name, err := client.CurrentDatabase(ctx); err != nil || name != "postgres" {
		t.Fatalf("database after switch = %q, err = %v", name, err)
	}

	if _, err := client.SwitchDatabase(ctx, "snoopg_lab"); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if name, err := client.CurrentDatabase(ctx); err != nil || name != "snoopg_lab" {
		t.Fatalf("database after switch back = %q, err = %v", name, err)
	}
}

func TestReadOnlyToggle(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx := context.Background()
	client := mustConnect(t, labDSN, true)

	if _, _, err := client.Exec(ctx, "CREATE TABLE ro_toggle_probe(i int)"); err == nil {
		t.Fatal("write succeeded in read-only mode")
	}

	if err := client.Reconnect(ctx, labDSN, false); err != nil {
		t.Fatalf("reconnect as read-write: %v", err)
	}
	if _, _, err := client.Exec(ctx, "CREATE TABLE ro_toggle_probe(i int)"); err != nil {
		t.Fatalf("write failed after switching to read-write: %v", err)
	}
	_, _, _ = client.Exec(ctx, "DROP TABLE ro_toggle_probe")
}

func TestConcurrentQueriesDuringReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	ctx := context.Background()
	client := mustConnect(t, labDSN, false)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _, err := client.Exec(ctx, "SELECT 1")
					if err != nil {
						t.Errorf("query during reconnect failed: %v", err)
						return
					}
				}
			}
		}()
	}

	for i := 0; i < 5; i++ {
		if _, err := client.SwitchDatabase(ctx, "postgres"); err != nil {
			t.Fatalf("switch to postgres: %v", err)
		}
		if _, err := client.SwitchDatabase(ctx, "snoopg_lab"); err != nil {
			t.Fatalf("switch back: %v", err)
		}
	}
	close(stop)
	wg.Wait()
}
