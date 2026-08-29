# pgspy

A terminal UI for inspecting live PostgreSQL instances — built as a hands-on
companion to studying PostgreSQL internals (buffer cache, WAL, locks, MVCC,
query execution). Written in Go with [bubbletea](https://github.com/charmbracelet/bubbletea).

The main goal is to **debug a running instance by watching its internals react
to your own queries**: type SQL, press enter, and see the buffer cache, WAL,
locks, and page structures respond in real time.

## Concept

pgspy is a shell with pluggable modules. Each module visualizes one subsystem
of a live PostgreSQL server:

- **Buffer cache** — live view of `shared_buffers` via `pg_buffercache`:
  pages per relation, dirty pages, pins, usage counts.
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

## Usage

```sh
go run .                       # connects to the default local DSN
PGSPY_DSN='postgres://user:pass@host:5432/db' go run .
```

Default DSN: `postgres://postgres:postgres@localhost:5432/automation_db?sslmode=disable`

| Key | Action |
| --- | --- |
| `tab` / `shift+tab` | cycle modules |
| `1`–`9` | jump to module |
| `enter` | run query from the query bar |
| `q` / `ctrl+c` | quit |

## Layout

```
pgspy [buffer cache] [wal] [locks] ...     tab bar (one module each)
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
