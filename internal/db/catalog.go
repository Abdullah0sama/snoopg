package db

import (
	"context"
)

type IndexInfo struct {
	Name      string
	Columns   string
	Unique    bool
	Primary   bool
	SizeBytes int64
}

type TableInfo struct {
	Schema    string
	Name      string
	SizeBytes int64
	Indexes   []IndexInfo
	Refs      []string
	RefBy     []string
}

func (c *Client) Catalog(ctx context.Context) ([]TableInfo, error) {
	rows, err := c.current().Query(ctx, `
		SELECT n.nspname, c.relname, pg_total_relation_size(c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r' AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 3 DESC`)
	if err != nil {
		return nil, err
	}

	tblMap := make(map[string]*TableInfo)
	var order []string
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Schema, &t.Name, &t.SizeBytes); err != nil {
			rows.Close()
			return nil, err
		}
		key := t.Schema + "." + t.Name
		tblMap[key] = &t
		order = append(order, key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(order) == 0 {
		return []TableInfo{}, nil
	}

	rows, err = c.current().Query(ctx, `
		SELECT n.nspname, c.relname, ic.relname, ix.indisunique, ix.indisprimary,
		       COALESCE(string_agg(a.attname || ' ' || pg_catalog.format_type(a.atttypid, a.atttypmod), ', ' ORDER BY ord.n), ''),
		       pg_relation_size(ix.indexrelid)
		FROM pg_index ix
		JOIN pg_class c ON c.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class ic ON ic.oid = ix.indexrelid
		LEFT JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS ord(attnum, n) ON true
		LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ord.attnum
		WHERE ix.indisvalid AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		GROUP BY n.nspname, c.relname, ic.relname, ix.indisunique, ix.indisprimary, ix.indexrelid
		ORDER BY n.nspname, c.relname, ic.relname`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var (
			schema, table, name string
			unique, primary     bool
			cols                string
			size                int64
		)
		if err := rows.Scan(&schema, &table, &name, &unique, &primary, &cols, &size); err != nil {
			rows.Close()
			return nil, err
		}
		if t, ok := tblMap[schema+"."+table]; ok {
			t.Indexes = append(t.Indexes, IndexInfo{Name: name, Columns: cols, Unique: unique, Primary: primary, SizeBytes: size})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = c.current().Query(ctx, `
		SELECT rn.nspname, rc.relname, fn.nspname, fc.relname
		FROM pg_constraint co
		JOIN pg_class rc ON rc.oid = co.conrelid
		JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		JOIN pg_class fc ON fc.oid = co.confrelid
		JOIN pg_namespace fn ON fn.oid = fc.relnamespace
		WHERE co.contype = 'f' AND rn.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1, 2, 3, 4`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var refSchema, refTable, fkSchema, fkTable string
		if err := rows.Scan(&refSchema, &refTable, &fkSchema, &fkTable); err != nil {
			rows.Close()
			return nil, err
		}
		if t, ok := tblMap[refSchema+"."+refTable]; ok {
			t.Refs = append(t.Refs, fkSchema+"."+fkTable)
		}
		if t, ok := tblMap[fkSchema+"."+fkTable]; ok {
			t.RefBy = append(t.RefBy, refSchema+"."+refTable)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	out := make([]TableInfo, 0, len(order))
	for _, key := range order {
		out = append(out, *tblMap[key])
	}
	return out, nil
}
