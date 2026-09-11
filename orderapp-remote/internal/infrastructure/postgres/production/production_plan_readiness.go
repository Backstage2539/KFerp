package production

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"sort"
	"strings"
)

func productionPlanReadiness(detail productionapp.ProductionPlanDetail) productionapp.ProductionPlanReadiness {
	result := productionapp.ProductionPlanReadiness{Issues: []productionapp.ProductionPlanReadinessIssue{}}
	if detail.Status != "draft" {
		return result
	}
	addBlocking := func(issue productionapp.ProductionPlanReadinessIssue) {
		issue.Severity = "blocking"
		result.Issues = append(result.Issues, issue)
		result.BlockingCount++
	}

	if len(detail.Items) == 0 {
		addBlocking(productionapp.ProductionPlanReadinessIssue{
			Code: "plan_item_missing", Category: "quantity", Message: "生产计划没有可执行任务", Action: "返回需求重新生成草稿",
		})
	}
	for _, item := range detail.Items {
		if productionPlanItemHasLegacyUpstreamLossSnapshot(item) {
			addBlocking(productionapp.ProductionPlanReadinessIssue{
				Code: "legacy_loss_snapshot", Category: "quantity", ProductionPlanItemID: item.ID,
				Message: fmt.Sprintf("%s 的旧草稿无法确认损耗是否已经计入计划投入", firstNonEmpty(item.OutputName, item.ProductName)), Action: "刷新供应",
			})
		}
		if _, err := productionPlanItemConsumptionNeeds(item); err != nil {
			addBlocking(productionapp.ProductionPlanReadinessIssue{
				Code: "quantity_invalid", Category: "quantity", ProductionPlanItemID: item.ID,
				Message: fmt.Sprintf("%s 的冻结数量无法计算：%v", firstNonEmpty(item.OutputName, item.ProductName), err), Action: "刷新供应或重新生成草稿",
			})
		}
	}

	sourceByGap := map[string]bool{}
	for _, source := range detail.ComponentSources {
		if source.Selected {
			sourceByGap[fmt.Sprintf("%d:%s:%d", source.ProductionPlanItemID, normalizedReadinessComponentType(source.ComponentType), source.ComponentID)] = true
		}
		if source.PickingVersion > 0 {
			sourceByGap[fmt.Sprintf("%d:%s:%d", source.ProductionPlanItemID, normalizedReadinessComponentType(source.ComponentType), source.ComponentID)] = true
			if source.ShortageG > 0 || source.ShortageUnits > 0 {
				addBlocking(productionapp.ProductionPlanReadinessIssue{Code: "component_source_shortage", Category: "source", ProductionPlanItemID: source.ProductionPlanItemID, ComponentSourceID: source.ID, Message: fmt.Sprintf("%s 供给未落实，缺少 %dg / %d件", source.ComponentName, source.ShortageG, source.ShortageUnits), Action: "补齐供给并刷新"})
			}
			continue
		}
		if !source.Selected {
			addBlocking(productionapp.ProductionPlanReadinessIssue{
				Code: "component_source_missing", Category: "source", ProductionPlanItemID: source.ProductionPlanItemID,
				ComponentSourceID: source.ID, Message: fmt.Sprintf("%s 尚未选择来源仓库", source.ComponentName), Action: "设置来源",
			})
		}
	}
	for _, gap := range detail.SupplyGaps {
		if gap.Status != "unresolved" {
			continue
		}
		key := fmt.Sprintf("%d:%s:%d", gap.ProductionPlanItemID, normalizedReadinessComponentType(gap.ItemType), gap.ItemID)
		if sourceByGap[key] {
			continue
		}
		addBlocking(productionapp.ProductionPlanReadinessIssue{
			Code: "supply_gap", Category: "supply", ProductionPlanItemID: gap.ProductionPlanItemID,
			Message: fmt.Sprintf("%s 存在未解决的备料缺口", firstNonEmpty(gap.ItemName, "组件")), Action: "刷新供应或完善采购条件",
		})
	}

	type sourceGroup struct {
		first                      productionapp.ProductionPlanComponentSource
		requiredG, requiredUnits   int64
		availableG, availableUnits int64
		availabilityFound          bool
	}
	groups := map[string]sourceGroup{}
	for _, source := range detail.ComponentSources {
		if !source.Selected || source.PickingVersion > 0 {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%s:%d", normalizedReadinessComponentType(source.ComponentType), source.ComponentID,
			source.ComponentBOMSpecID, source.ComponentSpecG, source.SourceWarehouse, source.SourceOwnerCustomerID)
		group := groups[key]
		if group.first.ID == 0 {
			group.first = source
		}
		group.requiredG += source.RequiredG
		group.requiredUnits += source.RequiredUnits
		for _, option := range source.Options {
			if option.Warehouse == source.SourceWarehouse && option.OwnerCustomerID == source.SourceOwnerCustomerID {
				group.availableG = option.AvailableG
				group.availableUnits = option.AvailableUnits
				group.availabilityFound = true
				break
			}
		}
		if !group.availabilityFound {
			group.availableG = source.AvailableGSnapshot
			group.availableUnits = source.AvailableUnitsSnapshot
		}
		groups[key] = group
	}
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	for _, key := range groupKeys {
		group := groups[key]
		if group.availableG >= group.requiredG && group.availableUnits >= group.requiredUnits {
			continue
		}
		addBlocking(productionapp.ProductionPlanReadinessIssue{
			Code: "component_source_shortage", Category: "source", ProductionPlanItemID: group.first.ProductionPlanItemID,
			ComponentSourceID: group.first.ID, Message: fmt.Sprintf("%s 所选来源的合计可用量不足", group.first.ComponentName), Action: "更换来源或刷新供应",
		})
	}

	splitsByItem := productionPlanSplitsByItem(detail.OperationSplits)
	for _, item := range detail.Items {
		if err := validateProductionPlanOperationSplitCoverage(item, splitsByItem[item.ID]); err != nil {
			code := "operation_split_invalid"
			if len(splitsByItem[item.ID]) == 0 {
				code = "operation_split_missing"
			}
			addBlocking(productionapp.ProductionPlanReadinessIssue{
				Code: code, Category: "operation", ProductionPlanItemID: item.ID, Message: err.Error(), Action: "查看拆分",
			})
		}
	}
	result.CanSubmit = result.BlockingCount == 0
	return result
}

func productionPlanItemHasLegacyUpstreamLossSnapshot(item productionapp.ProductionPlanItem) bool {
	if strings.TrimSpace(item.OutputType) != "material" || item.PlannedG <= 0 {
		return false
	}
	var rows []materialSnapshotRow
	if err := json.Unmarshal([]byte(strings.TrimSpace(item.MaterialSnapshot)), &rows); err != nil {
		return false
	}
	for _, row := range rows {
		if row.MaterialLossRate > 0 && !row.InputIncludesMaterialLoss {
			return true
		}
	}
	return false
}

func normalizedReadinessComponentType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "finished_product" {
		return "product"
	}
	return value
}

func productionPlanDraftToken(detail productionapp.ProductionPlanDetail) string {
	type itemToken struct {
		ID        int64  `json:"id"`
		Warehouse string `json:"warehouse"`
	}
	type appAllocationToken struct {
		Warehouse   string
		Owner, G, N int64
	}
	type sourceToken struct {
		ItemID, ComponentID, BOMSpecID, SpecG, OwnerID int64
		ComponentType, Warehouse                       string
		Mode                                           string
		Manual                                         []appAllocationToken
	}
	payload := struct {
		Status   string
		Revision int64
		Items    []itemToken
		Sources  []sourceToken
		Splits   []productionapp.ProductionPlanOperationSplit
	}{Status: detail.Status, Revision: detail.Revision}
	for _, item := range detail.Items {
		payload.Items = append(payload.Items, itemToken{ID: item.ID, Warehouse: item.TargetWarehouse})
	}
	for _, source := range detail.ComponentSources {
		manual := []appAllocationToken{}
		for _, a := range source.ManualAllocations {
			manual = append(manual, appAllocationToken{a.Warehouse, a.OwnerCustomerID, a.QtyG, a.QtyUnits})
		}
		payload.Sources = append(payload.Sources, sourceToken{Mode: source.AllocationMode, Manual: manual,
			ItemID: source.ProductionPlanItemID, ComponentID: source.ComponentID, BOMSpecID: source.ComponentBOMSpecID,
			SpecG: source.ComponentSpecG, OwnerID: source.SourceOwnerCustomerID, ComponentType: source.ComponentType, Warehouse: source.SourceWarehouse,
		})
	}
	payload.Splits = append(payload.Splits, detail.OperationSplits...)
	sort.Slice(payload.Items, func(i, j int) bool { return payload.Items[i].ID < payload.Items[j].ID })
	sort.Slice(payload.Sources, func(i, j int) bool {
		left := fmt.Sprintf("%d:%s:%d:%d:%d", payload.Sources[i].ItemID, payload.Sources[i].ComponentType, payload.Sources[i].ComponentID, payload.Sources[i].BOMSpecID, payload.Sources[i].SpecG)
		right := fmt.Sprintf("%d:%s:%d:%d:%d", payload.Sources[j].ItemID, payload.Sources[j].ComponentType, payload.Sources[j].ComponentID, payload.Sources[j].BOMSpecID, payload.Sources[j].SpecG)
		return left < right
	})
	sort.Slice(payload.Splits, func(i, j int) bool {
		if payload.Splits[i].ProductionPlanItemID != payload.Splits[j].ProductionPlanItemID {
			return payload.Splits[i].ProductionPlanItemID < payload.Splits[j].ProductionPlanItemID
		}
		if payload.Splits[i].OperationSeq != payload.Splits[j].OperationSeq {
			return payload.Splits[i].OperationSeq < payload.Splits[j].OperationSeq
		}
		return payload.Splits[i].ID < payload.Splits[j].ID
	})
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
