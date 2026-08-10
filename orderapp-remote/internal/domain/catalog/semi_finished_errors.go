package catalog

import "errors"

var (
	ErrSemiFinishedMustBeParent       = errors.New("semi-finished product must be a parent product")
	ErrSemiFinishedRequiresWeightUnit = errors.New("semi-finished product must use a weight inventory unit (kg or g)")
	ErrSemiFinishedRequiresUnitTemplate = errors.New("semi-finished product must reference a sales spec template")
)
