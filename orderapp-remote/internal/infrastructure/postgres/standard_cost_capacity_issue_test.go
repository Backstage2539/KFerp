package postgres

import (
	"strings"
	"testing"
)

func TestStandardCostCapacityIssueErrorNamesRouteOperationCapacityAndWorkstation(t *testing.T) {
	issue := StandardCostCapacityIssue{
		RouteID:               9,
		RouteName:             "PR-616 挂耳生产路线",
		Sequence:              1,
		OperationID:           5,
		OperationName:         "咖啡研磨",
		CapacityID:            11,
		CapacityName:          "咖啡研磨 1kg/批",
		CapacityStatus:        "inactive",
		WorkstationID:         6,
		WorkstationName:       "PR-616 挂耳生产工位",
		WorkstationStatus:     "active",
		WorkstationApplicable: false,
	}

	message := issue.Error()
	for _, want := range []string{
		"工艺路线「PR-616 挂耳生产路线」",
		"第1道工序「咖啡研磨」",
		"标准成本产能档「咖啡研磨 1kg/批」已停用",
		"工位「PR-616 挂耳生产工位」不适用工序「咖啡研磨」",
		"重新选择有效的标准成本产能档",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("detailed capacity error %q missing %q", message, want)
		}
	}
	if strings.Contains(message, "标准成本产能档必须来自启用工位且适用当前工序") {
		t.Fatalf("detailed capacity error must not fall back to the old ambiguous message: %q", message)
	}
}

func TestStandardCostCapacityIssueErrorDistinguishesInactiveWorkstationAndMissingCapacity(t *testing.T) {
	t.Run("inactive workstation", func(t *testing.T) {
		message := (StandardCostCapacityIssue{
			RouteID:           12,
			RouteName:         "盒装路线",
			Sequence:          2,
			OperationID:       8,
			OperationName:     "盒装包装",
			CapacityID:        21,
			CapacityName:      "盒装100盒",
			CapacityStatus:    "active",
			WorkstationID:     10,
			WorkstationName:   "盒装工位",
			WorkstationStatus: "inactive",
		}).Error()
		if !strings.Contains(message, "工位「盒装工位」已停用") {
			t.Fatalf("inactive workstation error = %q", message)
		}
	})

	t.Run("missing capacity selection", func(t *testing.T) {
		message := (StandardCostCapacityIssue{
			RouteID:       13,
			RouteName:     "烘焙路线",
			Sequence:      1,
			OperationID:   3,
			OperationName: "烘焙",
		}).Error()
		if !strings.Contains(message, "未选择标准成本产能档") {
			t.Fatalf("missing capacity error = %q", message)
		}
	})

	t.Run("deleted capacity", func(t *testing.T) {
		message := (StandardCostCapacityIssue{
			RouteID:       14,
			RouteName:     "研磨路线",
			Sequence:      1,
			OperationID:   5,
			OperationName: "咖啡研磨",
			CapacityID:    44,
		}).Error()
		if !strings.Contains(message, "标准成本产能档「#44」不存在") {
			t.Fatalf("deleted capacity error = %q", message)
		}
		if strings.Contains(message, "未关联有效工位") {
			t.Fatalf("deleted capacity must not add a misleading workstation reason: %q", message)
		}
	})

	t.Run("deleted workstation", func(t *testing.T) {
		message := (StandardCostCapacityIssue{
			RouteID:           15,
			RouteName:         "包装路线",
			Sequence:          1,
			OperationID:       8,
			OperationName:     "包装",
			CapacityID:        45,
			CapacityName:      "包装100件",
			CapacityStatus:    "active",
			WorkstationID:     55,
			WorkstationStatus: "",
		}).Error()
		if !strings.Contains(message, "工位「#55」不存在") {
			t.Fatalf("deleted workstation error = %q", message)
		}
	})
}
