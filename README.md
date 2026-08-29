# snoopg

A terminal UI for inspecting live PostgreSQL instances — built as a hands-on
companion to studying PostgreSQL internals (buffer cache, WAL, locks, MVCC,
query execution). Written in Go with [bubbletea](https://github.com/charmbracelet/bubbletea).

The main goal is to **debug a running instance by watching its internals react
to your own queries**: type SQL, press enter, and see the buffer cache, WAL,
locks, and page structures respond in real time.

## Concept

snoopg is a shell with pluggable modules. Each module visualizes one subsystem
of a live PostgreSQL server:

- **Buffer cache** — live view of `shared_buffers` via `pg_buffercache`:
  pages per relation, dirty pages, pins, usage counts.
- **Tables** — table/index/FK catalog browser (size-sorted table list,
  per-table index details).
- **Heap pages** *(planned)* — decode raw 8KB pages: page header, line
  pointers, tuple headers, MVCC fields (`xmin`/`xmax`, infomask).
- **WAL** *(planned)* — write-ahead log activity: LSN movement, WAL
  generation rate, checkpoints, and eventually raw WAL record parsing.
- **Locks** *(planned)* — blocking chains from `pg_locks` and wait events.
- **Query plans** *(planned)* — `EXPLAIN` trees with cost highlighting.

Switch between modules with `tab` / `shift+tab` or `1`–`9`. The query bar and
event log are shared across all modules; each module refreshes itself after a
query runs.

## Requirements

- Go 1.27+
- A reachable PostgreSQL instance (any recent version; developed against 18)
- `pg_buffercache` extension installed: `CREATE EXTENSION pg_buffercache;`

## Connecting

No credentials are hardcoded. The connection is resolved in this order:

1. `SNOOPG_DSN` environment variable
2. `-dsn` command line flag
3. a saved profile (last used) from `~/Library/Application Support/snoopg/config.json`
4. an interactive connection form in the TUI

The form asks for a DSN (`postgres://user:pass@host:5432/db`), validates it
by actually connecting, and can save it locally for next time (`tab` toggles
saving — opt-in, stored with 0600 permissions). Passwords are stored in
plaintext, so only save connections you are comfortable keeping on disk.

```sh
go run .                                       # saved profile, or the form
SNOOPG_DSN='postgres://user:pass@host/db' go run .
go run . -dsn 'postgres://user:pass@host/db'
```

## Lab

The load generator and most demos run against a dedicated lab instance so
experiments never compete with other traffic for the buffer cache:

```sh
docker run -d --name snoopg-lab -p 5433:5432 \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=snoopg_lab \
  postgres:18 -c shared_buffers=64MB
docker exec snoopg-lab psql -U postgres -d snoopg_lab <<'SQL'
CREATE EXTENSION pg_buffercache;
CREATE TABLE big (id int PRIMARY KEY, payload text);
INSERT INTO big SELECT g, repeat(md5(g::text), 8) FROM generate_series(1, 2000000) g;

CREATE TABLE hot (id int PRIMARY KEY, payload text);
INSERT INTO hot SELECT g, md5(g::text) FROM generate_series(1, 10000) g;
SQL
```

- `big` — 2M rows, ≈ 622MB. About 10x `shared_buffers` (64MB), so scans
  and random reads churn the cache with real evictions.
- `hot` — 10k rows, ≈ 952kB. Small enough to stay resident; index scans
  show as a handful of pinned pages.

## Load generator

`cmd/snoopg-load` generates query traffic against the lab database so the TUI
has something to react to:

```sh
go run ./cmd/snoopg-load -scenario seqscan
go run ./cmd/snoopg-load -scenario oltp -count 200 -sleep 50
```

Flags: `-dsn` (empty by default — falls back to `SNOOPG_DSN` env, then the
saved profile), `-scenario`, `-count` (default 5 for seqscan/vacuum/burst,
100 otherwise), `-sleep` (milliseconds between operations).

| Scenario | What it does | What to watch in the TUI |
| --- | --- | --- |
| `seqscan` | `SELECT count(*) FROM big` | buffer count fills to 100%, usage counts reset |
| `updates` | random-row `UPDATE` on `big` | dirty page count climbs |
| `oltp` | index lookup on `hot` | hot relation stays pinned, cache barely moves |
| `vacuum` | `VACUUM (ANALYZE) big` | all of `big` paged in, then available again |
| `checkpoint` | `CHECKPOINT` + 1s sleep | dirty count drops to zero |
| `burst` | random mix of the above | cache churn from mixed workloads |

Two-pane tmux workflow: TUI in one pane, generator in the other:

```sh
tmux new-session -s snoopg
# left pane
go run .
# ctrl+b % — right pane
go run ./cmd/snoopg-load -scenario burst -count 50 -sleep 200
```

## Usage

```sh
go run .    # saved profile, SNOOPG_DSN, or -dsn; connection form if none
```

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | cycle modules |
| `1`–`9` | jump to module |
| `enter` | run query from the query bar |
| `esc` | toggle between typing in the query bar and browsing the active module |
| `f2` / `e` | toggle external query watch (`pg_stat_activity`; `e` works when not typing) |
| `q` / `ctrl+c` | quit |

## Layout

```
snoopg [buffer cache] [wal] [locks] ...     tab bar (one module each)
+------------------------------+-----------+
| active module view           | EVENTS    |  query log, module-driven
|                              |           |
+------------------------------+-----------+
query: <type SQL here>                       shared query bar
```

## Architecture

```
main.go                  wiring: DSN, db client, module list
internal/db/             PostgreSQL access (pgx pool + per-subsystem queries)
internal/ui/             TUI shell: tabs, query bar, event log
internal/ui/module.go    Module interface (Title/Init/Update/View)
internal/ui/modules/     one file per subsystem module
```

A module satisfies:

```go
type Module interface {
    Title() string
    Init() tea.Cmd
    Update(msg tea.Msg) (Module, tea.Cmd)
    View(width, height int) string
}
```

The shell owns the tab bar, query input, and event log; modules own their data
polling and rendering. `main.go` injects the module list into the shell
(dependency injection) to avoid an import cycle between the shell and modules.

## Development

```sh
go build ./...
go vet ./...
go test ./internal/db/ -v   # integration tests against a live PostgreSQL
```

Integration tests skip automatically if no instance is reachable.
