package db

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxResultRows = 200

type QueryResult struct {
	Columns      []string
	Rows         [][]string
	RowsAffected int64
	Truncated    bool
}

func sprintVal(v any) string {
	if v == nil {
		return "NULL"
	}
	switch b := v.(type) {
	case []byte:
		return string(b)
	default:
		return fmt.Sprint(v)
	}
}

type Client struct {
	mu       sync.RWMutex
	pool     *pgxpool.Pool
	dsn      string
	readOnly bool
}

func buildPoolConfig(dsn string, readOnly bool) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if readOnly {
		cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	}
	cfg.MaxConns = 4
	return cfg, nil
}

func New(ctx context.Context, dsn string) (*Client, error) {
	return NewWithMode(ctx, dsn, false)
}

func NewWithMode(ctx context.Context, dsn string, readOnly bool) (*Client, error) {
	c := &Client{dsn: dsn, readOnly: readOnly}
	if err := c.Reconnect(ctx, dsn, readOnly); err != nil {
		return nil, err
	}
	return c, nil
}

func newPool(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func (c *Client) swap(pool *pgxpool.Pool, dsn string, readOnly bool) {
	c.mu.Lock()
	old := c.pool
	c.pool = pool
	c.dsn = dsn
	c.readOnly = readOnly
	c.mu.Unlock()
	if old != nil {
		go func() {
			time.Sleep(10 * time.Second)
			old.Close()
		}()
	}
}

func (c *Client) Reconnect(ctx context.Context, dsn string, readOnly bool) error {
	cfg, err := buildPoolConfig(dsn, readOnly)
	if err != nil {
		return err
	}
	pool, err := newPool(ctx, cfg)
	if err != nil {
		return err
	}
	c.swap(pool, dsn, readOnly)
	return nil
}

func dsnWithDatabase(dsn, database string) string {	if u, err := url.Parse(dsn); err == nil && u.Path != "" && u.Path != "/" {
		u.Path = "/" + database
		return u.String()
	}
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return dsn
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   "/" + database,
	}
	q := u.Query()
	if cfg.TLSConfig == nil {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) SwitchDatabase(ctx context.Context, database string) (string, error) {
	c.mu.RLock()
	dsn, readOnly := c.dsn, c.readOnly
	c.mu.RUnlock()
	cfg, err := buildPoolConfig(dsn, readOnly)
	if err != nil {
		return "", err
	}
	cfg.ConnConfig.Database = database
	pool, err := newPool(ctx, cfg)
	if err != nil {
		return "", err
	}
	c.swap(pool, dsnWithDatabase(dsn, database), readOnly)
	return c.dsn, nil
}

func (c *Client) Info() (dsn string, readOnly bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dsn, c.readOnly
}

func (c *Client) DSN() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dsn
}

func (c *Client) current() *pgxpool.Pool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pool
}

func (c *Client) CurrentDatabase(ctx context.Context) (string, error) {
	var name string
	err := c.current().QueryRow(ctx, "SELECT current_database()").Scan(&name)
	return name, err
}

func (c *Client) ListDatabases(ctx context.Context) ([]string, error) {
	rows, err := c.current().Query(ctx, `
		SELECT datname FROM pg_database
		WHERE NOT datistemplate
		ORDER BY datname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (c *Client) Query(ctx context.Context, query string) (QueryResult, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := time.Now()
	rows, err := c.current().Query(ctx, query)
	if err != nil {
		return QueryResult{}, time.Since(start), err
	}
	defer rows.Close()

	res := QueryResult{}
	for _, fd := range rows.FieldDescriptions() {
		res.Columns = append(res.Columns, string(fd.Name))
	}
	for rows.Next() {
		if len(res.Rows) >= MaxResultRows {
			res.Truncated = true
			break
		}
		vals, err := rows.Values()
		if err != nil {
			return QueryResult{}, time.Since(start), err
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = sprintVal(v)
		}
		res.Rows = append(res.Rows, row)
	}
	res.RowsAffected = rows.CommandTag().RowsAffected()
	return res, time.Since(start), rows.Err()
}

func (c *Client) CompletionWords(ctx context.Context) ([]string, error) {
	rows, err := c.current().Query(ctx, `
		SELECT n.nspname || '.' || c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'v') AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		UNION
		SELECT a.attname
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE a.attnum > 0 AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (c *Client) Exec(ctx context.Context, query string) (rowsAffected int64, elapsed time.Duration, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := time.Now()
	tag, err := c.current().Exec(ctx, query)
	if err != nil {
		return 0, time.Since(start), err
	}
	return tag.RowsAffected(), time.Since(start), nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
}
