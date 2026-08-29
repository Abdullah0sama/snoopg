package db

import (
	"context"
)

type RelationStat struct {
	Relation string
	Buffers  int
	Dirty    int
	Pinned   int
	AvgUsage float64
}

type CacheStats struct {
	Total    int
	Dirty    int
	Pinned   int
	AvgUsage float64
}

func (c *Client) CacheStats(ctx context.Context) (CacheStats, error) {
	var s CacheStats
	err := c.current().QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE isdirty),
		       count(*) FILTER (WHERE pinning_backends > 0),
		       COALESCE(avg(usagecount), 0)
		FROM pg_buffercache`).Scan(&s.Total, &s.Dirty, &s.Pinned, &s.AvgUsage)
	if err != nil {
		return CacheStats{}, err
	}
	return s, nil
}

func (c *Client) RelationStats(ctx context.Context) ([]RelationStat, error) {
	rows, err := c.current().Query(ctx, `
		SELECT c.relname, count(*),
		       count(*) FILTER (WHERE b.isdirty),
		       count(*) FILTER (WHERE b.pinning_backends > 0),
		       COALESCE(avg(b.usagecount), 0)
		FROM pg_buffercache b
		JOIN pg_class c ON c.relfilenode = b.relfilenode
		JOIN pg_database d ON d.oid = b.reldatabase
		WHERE d.datname = current_database()
		GROUP BY c.relname
		ORDER BY count(*) DESC
		LIMIT 30`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RelationStat
	for rows.Next() {
		var r RelationStat
		if err := rows.Scan(&r.Relation, &r.Buffers, &r.Dirty, &r.Pinned, &r.AvgUsage); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
