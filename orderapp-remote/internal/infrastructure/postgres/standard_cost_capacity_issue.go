package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// StandardCostCapacityIssue describes the first invalid standard-cost capacity
// binding on a process route. Keeping the route, operation, capacity and
// workstation identities together makes publish failures actionable.
type StandardCostCapacityIssue struct {
	RouteID               int64
	RouteName             string
	Sequence              int
	OperationID           int64
	OperationName         string
	CapacityID            int64
	CapacityName          string
	CapacityStatus        string
	WorkstationID         int64
	WorkstationName       string
	WorkstationStatus     string
	WorkstationApplicable bool
}

type standardCostCapacityIssueQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// FindStandardCostCapacityIssue returns the first invalid route-operation
// binding in route order. A nil issue means every operation has an active
// capacity from an active workstation that applies to the selected operation.
func FindStandardCostCapacityIssue(ctx context.Context, q standardCostCapacityIssueQuerier, schema string, routeID int64) (*StandardCostCapacityIssue, error) {
	var issue StandardCostCapacityIssue
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT pr.id,
		       COALESCE(pr.name,''),
		       pro.seq,
		       pro.operation_id,
		       COALESCE(NULLIF(pro.operation,''), NULLIF(o.name,''), '工序'),
		       COALESCE(pro.standard_cost_capacity_id,0),
		       COALESCE(c.name,''),
		       COALESCE(c.status,''),
		       COALESCE(c.workstation_id,0),
		       COALESCE(w.name,''),
		       COALESCE(w.status,''),
		       EXISTS(
		           SELECT 1
		           FROM %[1]s.manufacturing_workstation_operations wo
		           WHERE wo.workstation_id=c.workstation_id
		             AND wo.operation_id=pro.operation_id
		       )
		FROM %[1]s.process_route_operations pro
		JOIN %[1]s.process_routes pr ON pr.id=pro.route_id
		LEFT JOIN %[1]s.manufacturing_operations o ON o.id=pro.operation_id
		LEFT JOIN %[1]s.manufacturing_workstation_capacities c ON c.id=pro.standard_cost_capacity_id
		LEFT JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id
		WHERE pro.route_id=$1
		  AND (
		      COALESCE(pro.standard_cost_capacity_id,0)<=0
		      OR c.id IS NULL
		      OR c.status<>'active'
		      OR w.id IS NULL
		      OR w.status<>'active'
		      OR NOT EXISTS(
		          SELECT 1
		          FROM %[1]s.manufacturing_workstation_operations wo
		          WHERE wo.workstation_id=c.workstation_id
		            AND wo.operation_id=pro.operation_id
		      )
		  )
		ORDER BY pro.seq, pro.id
		LIMIT 1
	`, schema), routeID).Scan(
		&issue.RouteID,
		&issue.RouteName,
		&issue.Sequence,
		&issue.OperationID,
		&issue.OperationName,
		&issue.CapacityID,
		&issue.CapacityName,
		&issue.CapacityStatus,
		&issue.WorkstationID,
		&issue.WorkstationName,
		&issue.WorkstationStatus,
		&issue.WorkstationApplicable,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

func (issue StandardCostCapacityIssue) Error() string {
	routeName := standardCostCapacityObjectName(issue.RouteName, issue.RouteID)
	operationName := standardCostCapacityObjectName(issue.OperationName, issue.OperationID)
	capacityName := standardCostCapacityObjectName(issue.CapacityName, issue.CapacityID)
	workstationName := standardCostCapacityObjectName(issue.WorkstationName, issue.WorkstationID)

	reasons := make([]string, 0, 3)
	if issue.CapacityID <= 0 {
		reasons = append(reasons, "未选择标准成本产能档")
	} else if strings.TrimSpace(issue.CapacityStatus) == "" {
		reasons = append(reasons, fmt.Sprintf("标准成本产能档「%s」不存在", capacityName))
	} else if issue.CapacityStatus != "active" {
		reasons = append(reasons, fmt.Sprintf("标准成本产能档「%s」已停用", capacityName))
	}

	if issue.CapacityID > 0 && strings.TrimSpace(issue.CapacityStatus) != "" {
		if issue.WorkstationID <= 0 {
			reasons = append(reasons, fmt.Sprintf("标准成本产能档「%s」未关联有效工位", capacityName))
		} else if strings.TrimSpace(issue.WorkstationStatus) == "" {
			reasons = append(reasons, fmt.Sprintf("工位「%s」不存在", workstationName))
		} else if issue.WorkstationStatus != "active" {
			reasons = append(reasons, fmt.Sprintf("工位「%s」已停用", workstationName))
		}
		if issue.WorkstationID > 0 && !issue.WorkstationApplicable {
			reasons = append(reasons, fmt.Sprintf("工位「%s」不适用工序「%s」", workstationName, operationName))
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "标准成本产能档当前不可用")
	}

	sequence := issue.Sequence
	if sequence <= 0 {
		sequence = 1
	}
	return fmt.Sprintf(
		"工艺路线「%s」第%d道工序「%s」的标准成本配置失效：%s；请在该工艺路线中重新选择有效的标准成本产能档后再发布",
		routeName,
		sequence,
		operationName,
		strings.Join(reasons, "；"),
	)
}

func standardCostCapacityObjectName(name string, id int64) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	if id > 0 {
		return fmt.Sprintf("#%d", id)
	}
	return "未命名"
}
