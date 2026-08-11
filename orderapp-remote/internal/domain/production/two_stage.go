package production

const (
	ProductionStageSemiFinished = "semi_finished"
	ProductionStagePackaging    = "packaging"

	WorkOrderDependencySemiToPackaging = "semi_finished_to_packaging"
)

func IsTwoStageProductionStage(stage string) bool {
	return stage == ProductionStageSemiFinished || stage == ProductionStagePackaging
}

func IsPackagingStage(stage string) bool {
	return stage == ProductionStagePackaging
}
