package production

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const productionRosterTimezone = "Asia/Shanghai"

type WorkstationStaffCandidate struct {
	EmployeeID   int64  `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	Role         string `json:"role"`
	SortOrder    int    `json:"sort_order"`
	Active       bool   `json:"active"`
}

type WorkstationOwnerResolution struct {
	EmployeeID      int64  `json:"employee_id"`
	EmployeeName    string `json:"employee_name"`
	Source          string `json:"source"`
	Unattended      bool   `json:"unattended"`
	OverrideInvalid bool   `json:"override_invalid"`
	Reason          string `json:"reason"`
}

func ResolveWorkstationOwner(staff []WorkstationStaffCandidate, attendance map[int64]string, overrideEmployeeID int64) WorkstationOwnerResolution {
	byID := make(map[int64]WorkstationStaffCandidate, len(staff))
	for _, candidate := range staff {
		if candidate.EmployeeID > 0 && candidate.Active {
			byID[candidate.EmployeeID] = candidate
		}
	}
	working := func(id int64) bool { return strings.TrimSpace(attendance[id]) == "working" }
	if overrideEmployeeID > 0 {
		candidate, ok := byID[overrideEmployeeID]
		if !ok || !working(overrideEmployeeID) {
			return WorkstationOwnerResolution{Unattended: true, OverrideInvalid: true, Source: "override", Reason: "人工调整人员已失效或当天未上班"}
		}
		return WorkstationOwnerResolution{EmployeeID: candidate.EmployeeID, EmployeeName: candidate.EmployeeName, Source: "override"}
	}
	candidates := append([]WorkstationStaffCandidate(nil), staff...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Role != candidates[j].Role {
			return candidates[i].Role == "primary"
		}
		if candidates[i].SortOrder != candidates[j].SortOrder {
			return candidates[i].SortOrder < candidates[j].SortOrder
		}
		return candidates[i].EmployeeID < candidates[j].EmployeeID
	})
	for _, candidate := range candidates {
		if !candidate.Active || !working(candidate.EmployeeID) {
			continue
		}
		source := "backup"
		if candidate.Role == "primary" {
			source = "primary"
		}
		return WorkstationOwnerResolution{EmployeeID: candidate.EmployeeID, EmployeeName: candidate.EmployeeName, Source: source}
	}
	return WorkstationOwnerResolution{Unattended: true, Source: "automatic", Reason: "主负责人和替补当天均未上班"}
}

type ProductionRosterEmployee struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ProductionAttendanceEntry struct {
	EmployeeID int64  `json:"employee_id"`
	WorkDate   string `json:"work_date"`
	Status     string `json:"status"`
}

type ProductionWorkstationOverride struct {
	WorkstationID int64  `json:"workstation_id"`
	WorkDate      string `json:"work_date"`
	EmployeeID    int64  `json:"employee_id"`
	Reason        string `json:"reason,omitempty"`
}

type ProductionWorkstationDayAssignment struct {
	WorkstationID    int64  `json:"workstation_id"`
	Workstation      string `json:"workstation"`
	WorkDate         string `json:"work_date"`
	EmployeeID       int64  `json:"employee_id"`
	EmployeeName     string `json:"employee_name"`
	Source           string `json:"source"`
	Unattended       bool   `json:"unattended"`
	OverrideInvalid  bool   `json:"override_invalid"`
	Reason           string `json:"reason"`
	TaskCount        int    `json:"task_count"`
	RunningTaskCount int    `json:"running_task_count"`
	HandoverCount    int    `json:"handover_count"`
}

type ProductionRosterIssue struct {
	Code          string `json:"code"`
	WorkDate      string `json:"work_date,omitempty"`
	WorkstationID int64  `json:"workstation_id,omitempty"`
	Message       string `json:"message"`
}

type ProductionRosterWeek struct {
	WeekStart            string                               `json:"week_start"`
	WeekEnd              string                               `json:"week_end"`
	Version              int64                                `json:"version"`
	Days                 []string                             `json:"days"`
	Employees            []ProductionRosterEmployee           `json:"employees"`
	Entries              []ProductionAttendanceEntry          `json:"entries"`
	Overrides            []ProductionWorkstationOverride      `json:"overrides"`
	Assignments          []ProductionWorkstationDayAssignment `json:"assignments"`
	Issues               []ProductionRosterIssue              `json:"issues"`
	AffectedTaskCount    int                                  `json:"affected_task_count"`
	AffectedStationCount int                                  `json:"affected_workstation_count"`
	Saved                bool                                 `json:"saved"`
	Replayed             bool                                 `json:"replayed"`
}

type ProductionRosterQuery struct {
	WeekStart string
}

type SaveProductionRosterCommand struct {
	WeekStart       string                          `json:"week_start"`
	ExpectedVersion int64                           `json:"expected_version"`
	Entries         []ProductionAttendanceEntry     `json:"entries"`
	Overrides       []ProductionWorkstationOverride `json:"overrides"`
	RequestID       string                          `json:"request_id"`
	Operator        string                          `json:"-"`
}

type ProductionTodayRoster struct {
	Date          string                               `json:"date"`
	RosterVersion int64                                `json:"roster_version"`
	EmployeeID    int64                                `json:"employee_id"`
	EmployeeName  string                               `json:"employee_name"`
	Attendance    string                               `json:"attendance"`
	Assignments   []ProductionWorkstationDayAssignment `json:"assignments"`
}

type HandoverWorkstationCommand struct {
	WorkstationID   int64  `json:"workstation_id"`
	WorkDate        string `json:"work_date"`
	EmployeeID      int64  `json:"employee_id"`
	ExpectedVersion int64  `json:"expected_version"`
	RequestID       string `json:"request_id"`
	Operator        string `json:"-"`
}

type HandoverWorkstationResult struct {
	WorkstationID int64   `json:"workstation_id"`
	WorkDate      string  `json:"work_date"`
	EmployeeID    int64   `json:"employee_id"`
	EmployeeName  string  `json:"employee_name"`
	JobCardIDs    []int64 `json:"job_card_ids"`
	Replayed      bool    `json:"replayed"`
}

type productionRosterRepository interface {
	ProductionRosterWeek(context.Context, ProductionRosterQuery) (ProductionRosterWeek, error)
	SaveProductionRoster(context.Context, SaveProductionRosterCommand, bool) (ProductionRosterWeek, error)
	ProductionTodayRoster(context.Context, string, int64) (ProductionTodayRoster, error)
	HandoverWorkstation(context.Context, HandoverWorkstationCommand) (HandoverWorkstationResult, error)
	ResolveProductionWorkstationAssignments(context.Context, string) ([]ProductionWorkstationDayAssignment, error)
}

func productionLocation() *time.Location {
	location, err := time.LoadLocation(productionRosterTimezone)
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func NormalizeProductionRosterWeek(value string, location *time.Location) (string, []string, error) {
	if location == nil {
		location = productionLocation()
	}
	value = strings.TrimSpace(value)
	var day time.Time
	var err error
	if value == "" {
		day = time.Now().In(location)
	} else {
		day, err = time.ParseInLocation("2006-01-02", value, location)
		if err != nil {
			return "", nil, fmt.Errorf("请输入有效日期")
		}
	}
	offset := (int(day.Weekday()) + 6) % 7
	monday := day.AddDate(0, 0, -offset)
	days := make([]string, 7)
	for i := range days {
		days[i] = monday.AddDate(0, 0, i).Format("2006-01-02")
	}
	return days[0], days, nil
}

func ValidateProductionRosterEntries(weekStart string, entries []ProductionAttendanceEntry) error {
	week, days, err := NormalizeProductionRosterWeek(weekStart, productionLocation())
	if err != nil {
		return err
	}
	if strings.TrimSpace(weekStart) != week {
		return fmt.Errorf("week_start 必须是周一")
	}
	validDate := map[string]bool{}
	for _, day := range days {
		validDate[day] = true
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.EmployeeID <= 0 || !validDate[entry.WorkDate] {
			return fmt.Errorf("排班员工或日期无效")
		}
		status := strings.TrimSpace(entry.Status)
		if status != "working" && status != "off" && status != "unplanned" {
			return fmt.Errorf("排班状态必须是上班、休息或未排班")
		}
		key := fmt.Sprintf("%d:%s", entry.EmployeeID, entry.WorkDate)
		if seen[key] {
			return fmt.Errorf("同一员工同一天不能重复排班")
		}
		seen[key] = true
	}
	return nil
}

func (s *Service) ProductionRosterWeek(ctx context.Context, query ProductionRosterQuery) (ProductionRosterWeek, error) {
	r, ok := s.repo.(productionRosterRepository)
	if !ok {
		return ProductionRosterWeek{}, fmt.Errorf("生产排班暂不可用")
	}
	week, _, err := NormalizeProductionRosterWeek(query.WeekStart, productionLocation())
	if err != nil {
		return ProductionRosterWeek{}, err
	}
	query.WeekStart = week
	return r.ProductionRosterWeek(ctx, query)
}

func (s *Service) SaveProductionRoster(ctx context.Context, cmd SaveProductionRosterCommand, preview bool) (ProductionRosterWeek, error) {
	week, days, err := NormalizeProductionRosterWeek(cmd.WeekStart, productionLocation())
	if err != nil {
		return ProductionRosterWeek{}, err
	}
	cmd.WeekStart = week
	if err := ValidateProductionRosterEntries(week, cmd.Entries); err != nil {
		return ProductionRosterWeek{}, err
	}
	if strings.TrimSpace(cmd.Operator) == "" {
		return ProductionRosterWeek{}, fmt.Errorf("operator required")
	}
	if !preview && strings.TrimSpace(cmd.RequestID) == "" {
		return ProductionRosterWeek{}, fmt.Errorf("缺少有效请求编号，请重新预览")
	}
	seenOverrides := map[string]bool{}
	validDate := map[string]bool{}
	for _, day := range days {
		validDate[day] = true
	}
	for _, override := range cmd.Overrides {
		if override.WorkstationID <= 0 || override.EmployeeID <= 0 || !validDate[override.WorkDate] {
			return ProductionRosterWeek{}, fmt.Errorf("工位临时负责人无效")
		}
		key := fmt.Sprintf("%d:%s", override.WorkstationID, override.WorkDate)
		if seenOverrides[key] {
			return ProductionRosterWeek{}, fmt.Errorf("同一工位同一天不能重复调整")
		}
		seenOverrides[key] = true
	}
	r, ok := s.repo.(productionRosterRepository)
	if !ok {
		return ProductionRosterWeek{}, fmt.Errorf("生产排班暂不可用")
	}
	return r.SaveProductionRoster(ctx, cmd, preview)
}

func (s *Service) ProductionTodayRoster(ctx context.Context, date string, employeeID int64) (ProductionTodayRoster, error) {
	r, ok := s.repo.(productionRosterRepository)
	if !ok {
		return ProductionTodayRoster{}, fmt.Errorf("生产排班暂不可用")
	}
	if strings.TrimSpace(date) == "" {
		date = time.Now().In(productionLocation()).Format("2006-01-02")
	}
	if _, err := time.ParseInLocation("2006-01-02", date, productionLocation()); err != nil {
		return ProductionTodayRoster{}, fmt.Errorf("请输入有效日期")
	}
	return r.ProductionTodayRoster(ctx, date, employeeID)
}

func (s *Service) HandoverWorkstation(ctx context.Context, cmd HandoverWorkstationCommand) (HandoverWorkstationResult, error) {
	if cmd.WorkstationID <= 0 || cmd.EmployeeID <= 0 {
		return HandoverWorkstationResult{}, fmt.Errorf("工位或接班人无效")
	}
	if cmd.ExpectedVersion <= 0 {
		return HandoverWorkstationResult{}, fmt.Errorf("排班版本已失效，请重新读取")
	}
	if strings.TrimSpace(cmd.Operator) == "" || strings.TrimSpace(cmd.RequestID) == "" {
		return HandoverWorkstationResult{}, fmt.Errorf("交接请求信息不完整")
	}
	if strings.TrimSpace(cmd.WorkDate) == "" {
		cmd.WorkDate = time.Now().In(productionLocation()).Format("2006-01-02")
	}
	r, ok := s.repo.(productionRosterRepository)
	if !ok {
		return HandoverWorkstationResult{}, fmt.Errorf("工位交接暂不可用")
	}
	return r.HandoverWorkstation(ctx, cmd)
}
