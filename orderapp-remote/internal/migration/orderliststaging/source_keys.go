package orderliststaging

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func AssignSourceKeys(rows []RawOrder, previous map[string]SourceKeyAssignment) []Issue {
	issues := make([]Issue, 0)
	groups := map[string][]int{}
	originalsBySheet := map[string]map[string]struct{}{}
	for i := range rows {
		rows[i].SequenceEffective = ""
		rows[i].SourceOrderKey = ""
		rows[i].DuplicateSuffix = 0
		seq, ok := normalizeSequence(rows[i].SequenceOriginal)
		if !ok {
			code := "source_sequence_invalid"
			message := "A列序号不是整数，不能生成稳定来源键"
			if strings.TrimSpace(rows[i].SequenceOriginal) == "" {
				code = "source_sequence_missing"
				message = "A列序号为空，不能生成稳定来源键"
			}
			rows[i].ReviewStatus = ReviewNeedsReview
			issues = append(issues, newIssue("order", rowLocator(rows[i]), code, "error", message, rows[i]))
			continue
		}
		rows[i].SequenceOriginal = seq
		if _, ok := originalsBySheet[rows[i].SheetName]; !ok {
			originalsBySheet[rows[i].SheetName] = map[string]struct{}{}
		}
		originalsBySheet[rows[i].SheetName][seq] = struct{}{}
		groups[rows[i].SheetName+"\x00"+seq] = append(groups[rows[i].SheetName+"\x00"+seq], i)
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	usedBySheet := map[string]map[string]struct{}{}
	for _, groupKey := range groupKeys {
		indexes := groups[groupKey]
		sort.SliceStable(indexes, func(i, j int) bool {
			return rows[indexes[i]].SourceRowNumber < rows[indexes[j]].SourceRowNumber
		})
		sheet := rows[indexes[0]].SheetName
		original := rows[indexes[0]].SequenceOriginal
		if _, ok := usedBySheet[sheet]; !ok {
			usedBySheet[sheet] = map[string]struct{}{}
		}
		usedSuffixes := map[int]struct{}{}

		pending := make([]int, 0, len(indexes))
		for _, idx := range indexes {
			assignment, ok := previous[rows[idx].Fingerprint]
			if !ok || assignment.SheetName != sheet || assignment.OriginalSequence != original || assignment.DuplicateSuffix < 0 || assignment.DuplicateSuffix > 9 {
				pending = append(pending, idx)
				continue
			}
			if !assignEffectiveSequence(&rows[idx], assignment.EffectiveSequence, assignment.DuplicateSuffix, originalsBySheet[sheet], usedBySheet[sheet]) {
				pending = append(pending, idx)
				continue
			}
			usedSuffixes[assignment.DuplicateSuffix] = struct{}{}
		}

		for _, idx := range pending {
			suffix := -1
			for candidate := 0; candidate <= 9; candidate++ {
				if _, exists := usedSuffixes[candidate]; !exists {
					suffix = candidate
					break
				}
			}
			effective := original
			if suffix > 0 {
				effective = original + strconv.Itoa(suffix)
			}
			if suffix >= 0 && assignEffectiveSequence(&rows[idx], effective, suffix, originalsBySheet[sheet], usedBySheet[sheet]) {
				usedSuffixes[suffix] = struct{}{}
				continue
			}
			rows[idx].ReviewStatus = ReviewNeedsReview
			code := "duplicate_suffix_exhausted"
			message := "重复序号超过一位后缀容量，不能自动生成有效序号"
			if len(indexes) <= 10 {
				code = "duplicate_suffix_collision"
				message = "追加的一位重复后缀与工作表内真实序号冲突"
			}
			issues = append(issues, newIssue("order", rowLocator(rows[idx]), code, "error", message, rows[idx]))
		}
	}
	return issues
}

func assignEffectiveSequence(row *RawOrder, effective string, suffix int, originals, used map[string]struct{}) bool {
	if strings.TrimSpace(effective) == "" {
		return false
	}
	if effective != row.SequenceOriginal {
		if _, exists := originals[effective]; exists {
			return false
		}
	}
	if _, exists := used[effective]; exists {
		return false
	}
	row.SequenceEffective = effective
	row.DuplicateSuffix = suffix
	row.SourceOrderKey = row.SheetName + ":" + effective
	if row.ReviewStatus == "" {
		row.ReviewStatus = ReviewAutoReady
	}
	used[effective] = struct{}{}
	return true
}

func normalizeSequence(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if strings.HasSuffix(raw, ".0") {
		raw = strings.TrimSuffix(raw, ".0")
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return "", false
	}
	return strconv.FormatUint(n, 10), true
}

func rowLocator(row RawOrder) string {
	return fmt.Sprintf("row:%s:%d", row.SheetName, row.SourceRowNumber)
}

func newIssue(entityType, entityKey, code, severity, message string, row RawOrder) Issue {
	source := row.SourceOrderKey
	if source == "" {
		source = rowLocator(row)
	}
	keySum := sha256.Sum256([]byte(entityType + "\x00" + entityKey + "\x00" + code))
	return Issue{
		IssueKey:        hex.EncodeToString(keySum[:12]),
		EntityType:      entityType,
		EntityKey:       entityKey,
		Code:            code,
		Severity:        severity,
		Message:         message,
		SourceOrderKey:  source,
		SheetName:       row.SheetName,
		SourceRowNumber: row.SourceRowNumber,
		ReviewStatus:    ReviewNeedsReview,
	}
}
