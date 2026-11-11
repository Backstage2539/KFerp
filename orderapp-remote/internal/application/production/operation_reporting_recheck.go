package production

import (
	"context"
	"fmt"
)

type ReportingTypeChange struct {
	Seq         int    `json:"seq"`
	OperationID int64  `json:"operation_id"`
	Operation   string `json:"operation"`
	From        string `json:"from"`
	To          string `json:"to"`
}

type ReportingRecheck struct {
	Version        string                `json:"version"`
	Changes        []ReportingTypeChange `json:"changes"`
	RequiresReplan bool                  `json:"requires_replan"`
}

type reportingRecheckRepository interface {
	RecheckReportingTypes(context.Context, string, int64, string, string) (ReportingRecheck, error)
}

func (s *Service) RecheckReportingTypes(ctx context.Context, kind string, id int64, expectedVersion, actor string) (ReportingRecheck, error) {
	repo, ok := s.repo.(reportingRecheckRepository)
	if !ok {
		return ReportingRecheck{}, fmt.Errorf("当前环境不支持工序类型重核")
	}
	if id <= 0 || (kind != "work_order" && kind != "plan_item") {
		return ReportingRecheck{}, fmt.Errorf("无效的重核对象")
	}
	return repo.RecheckReportingTypes(ctx, kind, id, expectedVersion, actor)
}
