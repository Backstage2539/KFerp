package sales

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// OrderOperationNote identifies an operation in a specific manufacturing output
// and route; names alone cannot distinguish repeated or upstream operations.
type OrderOperationNote struct {
	OutputType    string `json:"output_type"`
	OutputID      int64  `json:"output_id"`
	BomSpecID     int64  `json:"bom_spec_id,omitempty"`
	BomVariantID  int64  `json:"bom_variant_id,omitempty"`
	RouteID       int64  `json:"route_id"`
	StepID        int64  `json:"step_id"`
	OperationID   int64  `json:"operation_id"`
	OperationName string `json:"operation_name"`
	Note          string `json:"note"`
}

func NormalizeOrderOperationNotes(input []OrderOperationNote) ([]OrderOperationNote, error) {
	if input == nil {
		return nil, nil
	}
	if len(input) > 100 {
		return nil, fmt.Errorf("工序备注过多")
	}
	out := make([]OrderOperationNote, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, item := range input {
		item.OutputType = strings.TrimSpace(item.OutputType)
		item.OperationName = strings.TrimSpace(item.OperationName)
		item.Note = strings.TrimSpace(item.Note)
		if (item.OutputType != "material" && item.OutputType != "product") || item.OutputID <= 0 || item.RouteID <= 0 || item.StepID <= 0 || item.OperationID <= 0 || item.OperationName == "" || item.Note == "" || utf8.RuneCountInString(item.Note) > 1000 {
			return nil, fmt.Errorf("工序备注信息不完整或超长")
		}
		if item.BomSpecID < 0 || item.BomVariantID < 0 {
			return nil, fmt.Errorf("工序规格无效")
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%d", item.OutputType, item.OutputID, item.BomSpecID, item.BomVariantID, item.RouteID, item.StepID)
		if seen[key] {
			return nil, fmt.Errorf("同一工序不能重复填写备注")
		}
		seen[key] = true
		out = append(out, item)
	}
	return out, nil
}

// MatchOrderOperationNotes requires each selected step to belong to the
// server-resolved manufacturing graph and replaces client-supplied names.
func MatchOrderOperationNotes(input []OrderOperationNote, options []OrderOperationOption) ([]OrderOperationNote, error) {
	notes, err := NormalizeOrderOperationNotes(input)
	if err != nil {
		return nil, err
	}
	for i := range notes {
		found := false
		for _, option := range options {
			if notes[i].OutputType == option.OutputType && notes[i].OutputID == option.OutputID && notes[i].BomSpecID == option.BomSpecID && notes[i].BomVariantID == option.BomVariantID && notes[i].RouteID == option.RouteID && notes[i].StepID == option.StepID && notes[i].OperationID == option.OperationID {
				notes[i].OperationName = option.OperationName
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("工序不属于该商品的制造方案，请重新选择")
		}
	}
	return notes, nil
}
