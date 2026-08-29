package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"snoopg/internal/config"
)

const opTimeout = 60 * time.Second

func dur(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	if d < time.Second {
		return d.Round(time.Millisecond).String()
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func head(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func runOp(ctx context.Context, pool *pgxpool.Pool, scenario string) (string, error) {
	opCtx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	start := time.Now()

	switch scenario {
	case "seqscan":
		var n int64
		if err := pool.QueryRow(opCtx, "SELECT count(*) FROM big").Scan(&n); err != nil {
			return "", err
		}
		return fmt.Sprintf("[seqscan] count(*) = %d in %s", n, dur(time.Since(start))), nil

	case "updates":
		id := rand.IntN(2000000) + 1
		tag, err := pool.Exec(opCtx, "UPDATE big SET payload = md5(random()::text) WHERE id = $1", id)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("[updates] rows=%d in %s", tag.RowsAffected(), dur(time.Since(start))), nil

	case "oltp":
		id := rand.IntN(10000) + 1
		var payload string
		if err := pool.QueryRow(opCtx, "SELECT payload FROM hot WHERE id = $1", id).Scan(&payload); err != nil {
			return "", err
		}
		return fmt.Sprintf("[oltp] payload=%s in %s", head(payload, 8), dur(time.Since(start))), nil

	case "indexscan":
		id := rand.IntN(2000000) + 1
		var payload string
		if err := pool.QueryRow(opCtx, "SELECT payload FROM big WHERE id = $1", id).Scan(&payload); err != nil {
			return "", err
		}
		return fmt.Sprintf("[indexscan] id=%d payload=%s in %s", id, head(payload, 8), dur(time.Since(start))), nil

	case "vacuum":
		if _, err := pool.Exec(opCtx, "VACUUM (ANALYZE) big"); err != nil {
			return "", err
		}
		return fmt.Sprintf("[vacuum] done in %s", dur(time.Since(start))), nil

	case "checkpoint":
		if _, err := pool.Exec(opCtx, "CHECKPOINT"); err != nil {
			return "", err
		}
		if _, err := pool.Exec(opCtx, "SELECT pg_sleep(1)"); err != nil {
			return "", err
		}
		return fmt.Sprintf("[checkpoint] done in %s", dur(time.Since(start))), nil

	case "burst":
		switch rand.IntN(4) {
		case 0:
			var n int64
			if err := pool.QueryRow(opCtx, "SELECT count(*) FROM hot").Scan(&n); err != nil {
				return "", err
			}
			return fmt.Sprintf("[burst:seqscan] count(*) = %d in %s", n, dur(time.Since(start))), nil
		case 1:
			id := rand.IntN(2000000) + 1
			tag, err := pool.Exec(opCtx, "UPDATE big SET payload = md5(random()::text) WHERE id = $1", id)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("[burst:updates] rows=%d in %s", tag.RowsAffected(), dur(time.Since(start))), nil
		case 2:
			id := rand.IntN(10000) + 1
			var payload string
			if err := pool.QueryRow(opCtx, "SELECT payload FROM hot WHERE id = $1", id).Scan(&payload); err != nil {
				return "", err
			}
			return fmt.Sprintf("[burst:oltp] payload=%s in %s", head(payload, 8), dur(time.Since(start))), nil
		default:
			if _, err := pool.Exec(opCtx, "CHECKPOINT"); err != nil {
				return "", err
			}
			return fmt.Sprintf("[burst:checkpoint] done in %s", dur(time.Since(start))), nil
		}
	}
	return "", fmt.Errorf("unknown scenario: %s", scenario)
}

func main() {
	dsnFlag := flag.String("dsn", "", "PostgreSQL DSN (env SNOOPG_DSN or saved profile if empty)")
	scenario := flag.String("scenario", "seqscan", "seqscan|updates|oltp|indexscan|vacuum|checkpoint|burst")
	count := flag.Int("count", -1, "number of operations (default depends on scenario)")
	sleepMS := flag.Int("sleep", 0, "milliseconds to sleep between operations")
	flag.Parse()

	switch *scenario {
	case "seqscan", "updates", "oltp", "indexscan", "vacuum", "checkpoint", "burst":
	default:
		fmt.Fprintf(os.Stderr, "snoopg-load: unknown scenario: %s\n", *scenario)
		os.Exit(1)
	}

	if *count < 0 {
		*count = 100
		if *scenario == "seqscan" || *scenario == "vacuum" || *scenario == "burst" {
			*count = 5
		}
	}

	dsn := os.Getenv("SNOOPG_DSN")
	if dsn == "" {
		dsn = *dsnFlag
	}
	if dsn == "" {
		if cfg, err := config.Load(); err == nil {
			dsn = cfg.Profiles[cfg.Last]
		}
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "snoopg-load: no connection configured — set SNOOPG_DSN, pass -dsn, or run snoopg once to save one")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snoopg-load: %v\n", err)
		os.Exit(1)
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snoopg-load: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "snoopg-load: %v\n", err)
		os.Exit(1)
	}

	completed := 0
	for i := 0; i < *count; i++ {
		if ctx.Err() != nil {
			break
		}
		line, err := runOp(ctx, pool, *scenario)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] %v\n", *scenario, err)
			continue
		}
		fmt.Println(line)
		completed++
		if *sleepMS > 0 {
			time.Sleep(time.Duration(*sleepMS) * time.Millisecond)
		}
	}
	fmt.Printf("done: %d operations\n", completed)
}
