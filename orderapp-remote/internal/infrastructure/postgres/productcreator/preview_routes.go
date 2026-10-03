package productcreator

import (
	"context"
	"fmt"
	bomapp "orderapp/internal/application/bom"
	app "orderapp/internal/application/productcreator"
	pg "orderapp/internal/infrastructure/postgres"
	"sort"
)

// Reuse publication's capacity validation on effective routes, after overrides.
// This reads only; invalid template defaults do not block a valid run override.
func (e BusinessExecutor) inspectPreviewRouteCapacities(ctx context.Context, details map[string]map[string]any) []app.ValidationIssue {
	if e.schema == "" || (e.pool == nil && queryWithTransaction(ctx) == nil) {
		return nil
	}
	cache := map[int64]string{}
	issues := []app.ValidationIssue{}
	nodeIDs := make([]string, 0, len(details))
	for id := range details {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)
	for _, nodeID := range nodeIDs {
		detail := details[nodeID]
		ids := map[int64]bool{}
		if route, ok := detail["process_route"].(map[string]any); ok {
			if id := positiveNumber(route["id"]); id > 0 {
				ids[id] = true
			}
		}
		if template, ok := detail["specification_template"].(map[string]any); ok {
			if variants, ok := template["variants"].([]bomapp.ProductionBomSpecTemplateVariant); ok {
				for _, v := range variants {
					if v.ProcessRouteID > 0 {
						ids[v.ProcessRouteID] = true
					}
				}
			}
		}
		routeIDs := make([]int64, 0, len(ids))
		for id := range ids {
			routeIDs = append(routeIDs, id)
		}
		sort.Slice(routeIDs, func(i, j int) bool { return routeIDs[i] < routeIDs[j] })
		for _, id := range routeIDs {
			message, checked := cache[id]
			if !checked {
				var issue *pg.StandardCostCapacityIssue
				var err error
				if tx := queryWithTransaction(ctx); tx != nil {
					issue, err = pg.FindStandardCostCapacityIssue(ctx, tx, e.schema, id)
				} else {
					issue, err = pg.FindStandardCostCapacityIssue(ctx, e.pool, e.schema, id)
				}
				if err != nil {
					message = fmt.Sprintf("工艺路线 #%d 的标准成本配置暂时无法校验，请稍后重试", id)
				} else if issue != nil {
					message = issue.Error()
				}
				cache[id] = message
			}
			if message != "" {
				issues = append(issues, app.ValidationIssue{NodeID: nodeID, Field: "route", Code: "process_route_capacity_invalid", Message: message})
			}
		}
	}
	return issues
}
