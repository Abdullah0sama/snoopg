package db

import (
	"context"
	"fmt"
	"time"

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
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Client, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Client{pool: pool}, nil
}

func (c *Client) Query(ctx context.Context, query string) (QueryResult, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := time.Now()
	rows, err := c.pool.Query(ctx, query)
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
	rows, err := c.pool.Query(ctx, `
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
	tag, err := c.pool.Exec(ctx, query)
	if err != nil {
		return 0, time.Since(start), err
	}
	return tag.RowsAffected(), time.Since(start), nil
}

func (c *Client) CurrentDatabase(ctx context.Context) (string, error) {
	var name string
	err := c.pool.QueryRow(ctx, "SELECT current_database()").Scan(&name)
	return name, err
}

func (c *Client) Close() {
	c.pool.Close()
}
