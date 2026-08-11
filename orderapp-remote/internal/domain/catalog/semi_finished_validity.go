package catalog

type SalesSpecForPackagingCheck struct {
	SpecKey string
	Active  bool
}

type PackagingBomRefForCheck struct {
	SpecKey string
	IsValid bool
}

type SemiFinishedPackagingResult struct {
	Valid        bool
	MissingSpecs []string
}

func CheckSemiFinishedPackagingValidity(specs []SalesSpecForPackagingCheck, refs []PackagingBomRefForCheck) SemiFinishedPackagingResult {
	refMap := make(map[string]bool, len(refs))
	for _, ref := range refs {
		refMap[ref.SpecKey] = ref.IsValid
	}
	missing := []string{}
	for _, spec := range specs {
		if !spec.Active {
			continue
		}
		valid, ok := refMap[spec.SpecKey]
		if !ok || !valid {
			missing = append(missing, spec.SpecKey)
		}
	}
	return SemiFinishedPackagingResult{
		Valid:        len(missing) == 0,
		MissingSpecs: missing,
	}
}
