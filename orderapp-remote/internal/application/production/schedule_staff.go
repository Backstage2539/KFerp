package production

import (
	"context"
	"fmt"
	manufacturing "orderapp/internal/application/manufacturing"
	"sort"
	"strings"
	"time"
)

type ScheduleEmployee struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
type ScheduleTask struct {
	JobCardRow
	MaterialStatus            string             `json:"material_status"`
	ProductionPlanID          int64              `json:"production_plan_id"`
	ProductionPlanNo          string             `json:"production_plan_no"`
	WorkOrderStatus           string             `json:"work_order_status"`
	InventoryUnit             string             `json:"inventory_unit"`
	PlannedUnits              int64              `json:"planned_units"`
	PlannedOutputInventoryQty float64            `json:"planned_output_inventory_qty"`
	BatchIndex                int                `json:"batch_index"`
	BatchCount                int                `json:"batch_count"`
	ArrangementState          string             `json:"arrangement_state"`
	DefaultEmployeeID         int64              `json:"default_employee_id"`
	DefaultCollaboratorIDs    []int64            `json:"default_collaborator_ids"`
	EligibleEmployees         []ScheduleEmployee `json:"eligible_employees"`
	StaffingReady             bool               `json:"staffing_ready"`
}
type ScheduleLoad struct {
	WorkCenter       string `json:"work_center"`
	WorkDate         string `json:"work_date"`
	LoadMinutes      int    `json:"load_minutes"`
	AvailableMinutes *int   `json:"available_minutes"`
}
type ScheduleOptions struct {
	Operations []manufacturing.ManufacturingOperation           `json:"operations"`
	Capacities []manufacturing.ManufacturingWorkstationCapacity `json:"capacities"`
	Employees  []ScheduleEmployee                               `json:"employees"`
}
type ScheduleTaskPatch struct {
	WorkOrderID             int64    `json:"work_order_id"`
	JobCardID               int64    `json:"job_card_id"`
	ExpectedVersion         *int64   `json:"expected_version,omitempty"`
	AssignedEmployeeID      *int64   `json:"assigned_employee_id,omitempty"`
	CollaboratorEmployeeIDs *[]int64 `json:"collaborator_employee_ids,omitempty"`
	PlannedStartAt          *string  `json:"planned_start_at,omitempty"`
	PlannedEndAt            *string  `json:"planned_end_at,omitempty"`
	ShiftCode               *string  `json:"shift_code,omitempty"`
	Priority                *int     `json:"priority,omitempty"`
	Note                    *string  `json:"note,omitempty"`
	WorkstationCapacityID   *int64   `json:"workstation_capacity_id,omitempty"`
	// Legacy name is resolved only when a single eligible employee matches.
	AssignedTo *string `json:"assigned_to,omitempty"`
	WorkCenter *string `json:"work_center,omitempty"`
	Claim      bool    `json:"-"`
}
type ScheduleBatchCommand struct {
	Items        []ScheduleTaskPatch `json:"items"`
	RequestID    string              `json:"request_id"`
	PreviewToken string              `json:"preview_token"`
	Operator     string              `json:"-"`
}
type StaffScheduleConflict struct {
	EmployeeID     int64  `json:"employee_id"`
	EmployeeName   string `json:"employee_name"`
	JobCardID      int64  `json:"job_card_id"`
	OtherJobCardID int64  `json:"other_job_card_id"`
	Message        string `json:"message"`
}
type ScheduleBatchResult struct {
	Rows         []ScheduleTask          `json:"rows"`
	Conflicts    []StaffScheduleConflict `json:"conflicts"`
	PreviewToken string                  `json:"preview_token"`
	Saved        bool                    `json:"saved"`
	Replayed     bool                    `json:"replayed"`
}
type ScheduleError struct {
	Code      string                  `json:"code"`
	Message   string                  `json:"error"`
	Conflicts []StaffScheduleConflict `json:"conflicts,omitempty"`
}

func (e *ScheduleError) Error() string { return e.Message }

type scheduleStaffRepository interface {
	ScheduleBatch(context.Context, ScheduleBatchCommand, bool) (ScheduleBatchResult, error)
	ScheduleOptions(context.Context) (ScheduleOptions, error)
}

func (s *Service) ScheduleOptions(ctx context.Context) (ScheduleOptions, error) {
	r, ok := s.repo.(scheduleStaffRepository)
	if !ok {
		return ScheduleOptions{}, fmt.Errorf("排程人员配置暂不可用")
	}
	return r.ScheduleOptions(ctx)
}
func (s *Service) ScheduleBatch(ctx context.Context, cmd ScheduleBatchCommand, preview bool) (ScheduleBatchResult, error) {
	if len(cmd.Items) == 0 || len(cmd.Items) > 100 {
		return ScheduleBatchResult{}, fmt.Errorf("请选择 1 至 100 项任务")
	}
	if strings.TrimSpace(cmd.Operator) == "" {
		return ScheduleBatchResult{}, fmt.Errorf("operator required")
	}
	if !preview && (strings.TrimSpace(cmd.RequestID) == "" || len(cmd.RequestID) > 100) {
		return ScheduleBatchResult{}, fmt.Errorf("缺少有效请求编号，请重新预览")
	}
	seen := map[int64]bool{}
	for _, item := range cmd.Items {
		if item.JobCardID <= 0 || item.WorkOrderID <= 0 || item.ExpectedVersion == nil {
			return ScheduleBatchResult{}, fmt.Errorf("任务信息或版本不完整，请重新加载")
		}
		if seen[item.JobCardID] {
			return ScheduleBatchResult{}, fmt.Errorf("同一任务不能重复安排")
		}
		if item.CollaboratorEmployeeIDs != nil && len(*item.CollaboratorEmployeeIDs) > 0 {
			return ScheduleBatchResult{}, fmt.Errorf("协作人员功能已停用，请只设置负责人")
		}
		seen[item.JobCardID] = true
	}
	r, ok := s.repo.(scheduleStaffRepository)
	if !ok {
		return ScheduleBatchResult{}, fmt.Errorf("排程暂不可用")
	}
	return r.ScheduleBatch(ctx, cmd, preview)
}
func scheduleLocalTimestamp(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts.In(loc).Format("2006-01-02 15:04"), nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if ts, err := time.ParseInLocation(layout, value, loc); err == nil {
			return ts.Format("2006-01-02 15:04"), nil
		}
	}
	return "", fmt.Errorf("请输入有效的计划时间")
}
func MergeSchedulePatch(row ScheduleTask, p ScheduleTaskPatch) (ScheduleTask, error) {
	if row.Status == "completed" || row.Status == "cancelled" || row.WorkOrderStatus == "completed" || row.WorkOrderStatus == "cancelled" {
		return row, &ScheduleError{Code: "task_closed", Message: "已完成或已取消任务不能修改排程"}
	}
	if p.ExpectedVersion != nil && *p.ExpectedVersion != row.ScheduleVersion {
		return row, &ScheduleError{Code: "version_conflict", Message: "任务安排已被他人修改，请重新加载后调整"}
	}
	if p.Claim && row.AssignedEmployeeID > 0 {
		return row, fmt.Errorf("任务已有负责人，请刷新后查看")
	}
	var err error
	if p.PlannedStartAt != nil {
		row.PlannedStartAt, err = scheduleLocalTimestamp(*p.PlannedStartAt)
		if err != nil {
			return row, err
		}
	}
	if p.PlannedEndAt != nil {
		row.PlannedEndAt, err = scheduleLocalTimestamp(*p.PlannedEndAt)
		if err != nil {
			return row, err
		}
	}
	if (row.PlannedStartAt == "") != (row.PlannedEndAt == "") {
		return row, fmt.Errorf("计划开始和结束时间需要一起填写")
	}
	if row.PlannedStartAt != "" && row.PlannedEndAt <= row.PlannedStartAt {
		return row, fmt.Errorf("计划结束时间必须晚于开始时间")
	}
	if p.AssignedEmployeeID != nil {
		row.AssignedEmployeeID = *p.AssignedEmployeeID
	}
	if p.CollaboratorEmployeeIDs != nil {
		if len(*p.CollaboratorEmployeeIDs) > 0 {
			return row, fmt.Errorf("协作人员功能已停用，请只设置负责人")
		}
		row.CollaboratorEmployeeIDs = []int64{}
		row.Collaborators = []ScheduleEmployee{}
	}
	if p.ShiftCode != nil {
		row.ShiftCode = strings.TrimSpace(*p.ShiftCode)
	}
	if p.Note != nil {
		row.SchedulingNote = strings.TrimSpace(*p.Note)
	}
	if p.Priority != nil {
		if *p.Priority < 0 {
			return row, fmt.Errorf("优先级不能为负数")
		}
		row.Priority = *p.Priority
	}
	return row, nil
}
func StaffScheduleConflicts(changed, others []ScheduleTask) []StaffScheduleConflict {
	out := []StaffScheduleConflict{}
	seen := map[string]bool{}
	for _, a := range changed {
		if a.PlannedStartAt == "" || a.PlannedEndAt == "" {
			continue
		}
		people := []int64{a.AssignedEmployeeID}
		for _, b := range others {
			if a.ID == b.ID || b.PlannedStartAt == "" || b.PlannedEndAt == "" || a.PlannedEndAt <= b.PlannedStartAt || b.PlannedEndAt <= a.PlannedStartAt {
				continue
			}
			otherPeople := []int64{b.AssignedEmployeeID}
			for _, id := range people {
				if id <= 0 {
					continue
				}
				for _, otherID := range otherPeople {
					if id != otherID {
						continue
					}
					lo, hi := a.ID, b.ID
					if lo > hi {
						lo, hi = hi, lo
					}
					key := fmt.Sprintf("%d:%d:%d", id, lo, hi)
					if seen[key] {
						continue
					}
					seen[key] = true
					name := fmt.Sprintf("员工 %d", id)
					if a.AssignedEmployeeID == id && a.AssignedTo != "" {
						name = a.AssignedTo
					}
					out = append(out, StaffScheduleConflict{EmployeeID: id, EmployeeName: name, JobCardID: a.ID, OtherJobCardID: b.ID, Message: fmt.Sprintf("%s：%s / %s 与 %s / %s 时间重叠（%s — %s）", name, a.WorkOrderNo, a.Operation, b.WorkOrderNo, b.Operation, b.PlannedStartAt, b.PlannedEndAt)})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EmployeeID != out[j].EmployeeID {
			return out[i].EmployeeID < out[j].EmployeeID
		}
		if out[i].JobCardID != out[j].JobCardID {
			return out[i].JobCardID < out[j].JobCardID
		}
		return out[i].OtherJobCardID < out[j].OtherJobCardID
	})
	return out
}

// Use the same current-operation material readiness as the workstation, without making it a scheduling gate.
func (s *Service) attachScheduleMaterialStatus(ctx context.Context, rows []ScheduleTask) error {
	coverageRepo, ok := s.repo.(workOrderWIPCoverageRepository)
	if !ok {
		return nil
	}
	byOrder := map[int64][]int{}
	for i, row := range rows {
		if row.ArrangementState != "history" {
			byOrder[row.WorkOrderID] = append(byOrder[row.WorkOrderID], i)
		}
	}
	for id, indexes := range byOrder {
		coverage, err := coverageRepo.GetWorkOrderWIPCoverage(ctx, id)
		if err != nil {
			return err
		}
		cards, err := s.repo.ListJobCards(ctx, JobCardQuery{WorkOrderID: id, Limit: 500})
		if err != nil {
			return err
		}
		tasks := []ProductionTask{}
		for _, card := range cards {
			if isActiveProductionTaskStatus(card.Status) {
				tasks = append(tasks, productionTaskFromJobCard(card, WorkOrderRow{ID: id}))
			}
		}
		applyTaskBatchAndMaterialReadiness(tasks, cards, map[int64]ProductionWIPStatus{id: coverage})
		for _, task := range tasks {
			if task.ReadinessLabel == "待领料" {
				for _, i := range indexes {
					if rows[i].ID == task.JobCardID {
						if !coverage.DataComplete {
							rows[i].MaterialStatus = "齐套资料待完善"
						} else {
							rows[i].MaterialStatus = "待领料 · 可提前排程"
						}
					}
				}
			}
		}
	}
	return nil
}
