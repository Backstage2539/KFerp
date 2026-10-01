package productcreator

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/productcreator"
	"sort"
)

func summarizeResultObjects(run app.Run) []app.ResultObject {
	refs := map[string][]createdReference{}
	raw, _ := json.Marshal(run.BusinessResults["objects"])
	_ = json.Unmarshal(raw, &refs)
	nodes := []string{}
	seen := map[string]bool{}
	for _, n := range run.Workflow.Nodes {
		nodes = append(nodes, n.ID)
		seen[n.ID] = true
	}
	extra := []string{}
	for id := range refs {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	nodes = append(nodes, extra...)
	rows := []app.ResultObject{}
	index := map[string]int{}
	for _, nodeID := range nodes {
		for _, ref := range refs[nodeID] {
			if ref.ID <= 0 || (ref.Type != "material" && ref.Type != "product" && ref.Type != "bom" && ref.Type != "spec") {
				continue
			}
			key := fmt.Sprintf("%s:%d", ref.Type, ref.ID)
			i, exists := index[key]
			if !exists {
				i = len(rows)
				index[key] = i
				rows = append(rows, app.ResultObject{Type: ref.Type, ID: ref.ID, Name: ref.Name, CreationName: ref.Name, Code: ref.Code, Unit: ref.Unit, SourceNodeIDs: []string{}, BOMIDs: []int64{}})
			}
			row := &rows[i]
			found := false
			for _, id := range row.SourceNodeIDs {
				if id == nodeID {
					found = true
				}
			}
			if !found {
				row.SourceNodeIDs = append(row.SourceNodeIDs, nodeID)
			}
			if ref.Code != "" {
				row.Code = ref.Code
			}
			if ref.Unit != "" {
				row.Unit = ref.Unit
			}
			if ref.BOMID > 0 {
				found = false
				for _, id := range row.BOMIDs {
					if id == ref.BOMID {
						found = true
					}
				}
				if !found {
					row.BOMIDs = append(row.BOMIDs, ref.BOMID)
				}
			}
		}
	}
	return rows
}

// Keep commit_result immutable. Current names are a separate read projection,
// loaded by the same authorized run endpoint with bounded, per-type queries.
func (r Repository) currentResultObjects(ctx context.Context, run app.Run) ([]app.ResultObject, error) {
	result := summarizeResultObjects(run)
	queries := map[string]string{
		"material": "SELECT id,name,code,unit FROM %s.materials WHERE id=ANY($1)",
		"product":  "SELECT id,name,''::text,''::text FROM %s.products WHERE id=ANY($1)",
		"bom":      "SELECT id,name,code,''::text FROM %s.production_boms WHERE id=ANY($1)",
		"spec":     "SELECT id,name,code,inventory_unit FROM %s.production_bom_specs WHERE id=ANY($1)",
	}
	for kind, query := range queries {
		ids := []int64{}
		indices := map[int64]int{}
		for i, row := range result {
			if row.Type == kind {
				ids = append(ids, row.ID)
				indices[row.ID] = i
			}
		}
		if len(ids) == 0 {
			continue
		}
		rows, err := r.pool.Query(ctx, fmt.Sprintf(query, r.schema), ids)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var name, code, unit string
			if err := rows.Scan(&id, &name, &code, &unit); err != nil {
				rows.Close()
				return nil, err
			}
			row := &result[indices[id]]
			row.Name = name
			row.Code = code
			row.Unit = unit
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
