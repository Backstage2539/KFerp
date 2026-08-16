package productspecmigration

import (
	"fmt"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"
)

type readinessCounts struct {
	ActiveSpecs                   int64
	PublishedSpecs                int64
	InvalidSpecTemplateProvenance int64
	InactiveMainInputMaterial     int64
	AmbiguousLegacySpecs          int64
	LegacyUnitMismatches          int64
	LegacyStock                   int64
	Reservations                  int64
	UnfinishedOrders              int64
	UnfinishedPlans               int64
	UnfinishedWorkOrders          int64
	UnfinishedFulfillment         int64
}

func finalizeReadiness(counts readinessCounts) productspecmigrationapp.Readiness {
	unpublished := counts.ActiveSpecs - counts.PublishedSpecs
	if unpublished < 0 {
		unpublished = 0
	}
	result := productspecmigrationapp.Readiness{
		ActiveSpecCount:                    counts.ActiveSpecs,
		PublishedSpecCount:                 counts.PublishedSpecs,
		UnpublishedSpecCount:               unpublished,
		InvalidSpecTemplateProvenanceCount: counts.InvalidSpecTemplateProvenance,
		InactiveMainInputMaterialCount:     counts.InactiveMainInputMaterial,
		AmbiguousLegacySpecCount:           counts.AmbiguousLegacySpecs,
		LegacyUnitMismatchCount:            counts.LegacyUnitMismatches,
		LegacyStockCount:                   counts.LegacyStock,
		LegacyReservationCount:             counts.Reservations,
		UnfinishedOrderCount:               counts.UnfinishedOrders,
		UnfinishedPlanCount:                counts.UnfinishedPlans,
		UnfinishedWorkOrderCount:           counts.UnfinishedWorkOrders,
		UnfinishedFulfillmentCount:         counts.UnfinishedFulfillment,
		CheckedAt:                          time.Now().UTC(),
		Blockers:                           make([]productspecmigrationapp.Blocker, 0, 12),
	}
	add := func(code string, count int64, message string) {
		if count > 0 {
			result.Blockers = append(result.Blockers, productspecmigrationapp.Blocker{Code: code, Count: count, Message: message})
		}
	}
	if counts.ActiveSpecs == 0 {
		add("no_active_specs", 1, "商品没有可迁移的有效规格")
	} else {
		add("unpublished_specs", unpublished, fmt.Sprintf("%d 个有效规格尚未出现在默认已发布 BOM 版本中", unpublished))
	}
	add("missing_published_spec_template_provenance", counts.InvalidSpecTemplateProvenance, "当前默认已发布 BOM 版本不是从曾发布的规格模板复制，不能切换")
	add("inactive_main_input_material", counts.InactiveMainInputMaterial, "当前默认已发布 BOM 版本未配置有效的主投入物料")
	add("ambiguous_legacy_specs", counts.AmbiguousLegacySpecs, "旧子商品存在同规格键但库存单位不一致，不能自动合并")
	add("legacy_unit_mismatch", counts.LegacyUnitMismatches, "旧规格库存单位为空或与 BOM 规格库存单位不一致")
	add("legacy_stock", counts.LegacyStock, "旧子商品仍有库存")
	add("legacy_reservations", counts.Reservations, "旧子商品仍有有效预留")
	add("unfinished_orders", counts.UnfinishedOrders, "旧子商品仍有未完成订单")
	add("unfinished_plans", counts.UnfinishedPlans, "旧子商品仍有未完成生产计划")
	add("unfinished_work_orders", counts.UnfinishedWorkOrders, "旧子商品仍有未完成工单")
	add("unfinished_fulfillment", counts.UnfinishedFulfillment, "旧子商品仍有未完成履约")
	result.Ready = len(result.Blockers) == 0
	return result
}
