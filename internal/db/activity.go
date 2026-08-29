package db

import (
	"context"
)

type Activity struct {
	PID       int
	Database  string
	WaitEvent string
	Seconds   float64
	Query     string
}

func (c *Client) Activity(ctx context.Context) ([]Activity, error) {
	rows, err := c.current().Query(ctx, `
		SELECT pid, datname, COALESCE(wait_event_type || '/' || wait_event, ''),
		       EXTRACT(EPOCH FROM now() - query_start),
		       regexp_replace(query, '\s+', ' ', 'g')
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		  AND state = 'active'
		  AND application_name <> 'snoopg'
		  AND query NOT LIKE '%pg_stat_activity%'
		ORDER BY query_start
		LIMIT 8`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.PID, &a.Database, &a.WaitEvent, &a.Seconds, &a.Query); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
