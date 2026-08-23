package picker

import "github.com/choplin/herdr-quickselect/internal/matcher"

const (
	hintAlphabet = "asdfghjklqwertyuiopzxcvbnm"
	maxHintWidth = 2
)

// Assignment gives one unique value a hint and retains every visible occurrence.
type Assignment struct {
	Hint        string
	Value       string
	Occurrences []matcher.Match
}

func assignHints(matches []matcher.Match) []Assignment {
	if len(matches) == 0 {
		return nil
	}
	byValue := make(map[string]int, len(matches))
	assignments := make([]Assignment, 0, len(matches))
	capacity := len(hintAlphabet) * len(hintAlphabet)
	for _, match := range matches {
		if index, ok := byValue[match.Value]; ok {
			assignments[index].Occurrences = append(assignments[index].Occurrences, match)
			continue
		}
		if len(assignments) >= capacity {
			continue
		}
		byValue[match.Value] = len(assignments)
		assignments = append(assignments, Assignment{Value: match.Value, Occurrences: []matcher.Match{match}})
	}
	width := 1
	if len(assignments) > len(hintAlphabet) {
		width = maxHintWidth
	}
	for index := range assignments {
		assignments[index].Hint = hintFor(index, width)
	}
	return assignments
}

func hintFor(index, width int) string {
	result := make([]byte, width)
	base := len(hintAlphabet)
	for position := width - 1; position >= 0; position-- {
		result[position] = hintAlphabet[index%base]
		index /= base
	}
	return string(result)
}
