package db

import (
	"context"
	"encoding/json"
	"time"
)

type PlanNode struct {
	NodeType            string     `json:"Node Type"`
	RelationName        string     `json:"Relation Name"`
	Alias               string     `json:"Alias"`
	StartupCost         float64    `json:"Startup Cost"`
	TotalCost           float64    `json:"Total Cost"`
	PlanRows            int64      `json:"Plan Rows"`
	PlanWidth           int64      `json:"Plan Width"`
	ActualStartupTime   float64    `json:"Actual Startup Time"`
	ActualTotalTime     float64    `json:"Actual Total Time"`
	ActualRows          int64      `json:"Actual Rows"`
	ActualLoops         int64      `json:"Actual Loops"`
	SharedHitBlocks     int64      `json:"Shared Hit Blocks"`
	SharedReadBlocks    int64      `json:"Shared Read Blocks"`
	SharedDirtiedBlocks int64      `json:"Shared Dirtied Blocks"`
	Plans               []PlanNode `json:"Plans"`
}

type ExplainPlan struct {
	Plan PlanNode `json:"Plan"`
}

func (c *Client) Explain(ctx context.Context, query string, analyze bool) (*ExplainPlan, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	start := time.Now()
	prefix := "EXPLAIN (BUFFERS, FORMAT JSON) "
	if analyze {
		prefix = "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "
	}
	var raw []byte
	err := c.current().QueryRow(ctx, prefix+query).Scan(&raw)
	if err != nil {
		return nil, time.Since(start), err
	}
	var plan ExplainPlan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, time.Since(start), err
	}
	return &plan, time.Since(start), nil
}
