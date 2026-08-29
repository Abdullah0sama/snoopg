package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
