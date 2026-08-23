// Package matcher extracts non-overlapping values from visible pane text.
package matcher

import (
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/choplin/herdr-quickselect/internal/config"
)

var uriCandidate = regexp.MustCompile(`(?i)(?:https?|file)://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)

// Match is one accepted occurrence in logical pane text.
type Match struct {
	Value    string
	Kind     string
	Line     int
	Column   int
	Priority int
	Start    int
	End      int
}

type occurrence struct {
	Match
}

// FindAll returns non-overlapping occurrences in visual order, retaining duplicates.
func FindAll(lines []string, selectors []config.Selector) []Match {
	accepted := make([]occurrence, 0)
	for lineIndex, line := range lines {
		matches := matchesForLine(line, lineIndex, selectors)
		sort.SliceStable(matches, func(left, right int) bool {
			if matches[left].Priority != matches[right].Priority {
				return matches[left].Priority < matches[right].Priority
			}
			leftLength := matches[left].End - matches[left].Start
			rightLength := matches[right].End - matches[right].Start
			if leftLength != rightLength {
				return leftLength > rightLength
			}
			return matches[left].Start < matches[right].Start
		})
		for _, match := range matches {
			if !slices.ContainsFunc(accepted, func(existing occurrence) bool {
				return existing.Line == match.Line && existing.Start < match.End && match.Start < existing.End
			}) {
				accepted = append(accepted, match)
			}
		}
	}

	sort.SliceStable(accepted, func(left, right int) bool {
		if accepted[left].Line != accepted[right].Line {
			return accepted[left].Line < accepted[right].Line
		}
		return accepted[left].Start < accepted[right].Start
	})

	result := make([]Match, 0, len(accepted))
	for _, match := range accepted {
		result = append(result, match.Match)
	}
	return result
}

// Find returns unique values in top-to-bottom, left-to-right order.
func Find(text string, selectors []config.Selector) []Match {
	matches := FindAll(splitLines(text), selectors)
	seen := make(map[string]struct{}, len(matches))
	result := make([]Match, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match.Value]; ok {
			continue
		}
		seen[match.Value] = struct{}{}
		result = append(result, match)
	}
	return result
}

func matchesForLine(line string, lineIndex int, selectors []config.Selector) []occurrence {
	result := make([]occurrence, 0)
	for _, selector := range selectors {
		if selector.Matcher == "url" {
			result = append(result, urlMatchesForLine(line, lineIndex, selector)...)
			continue
		}
		compiled := regexp.MustCompile(selector.Regex)
		captureIndex := 0
		if selector.Capture != "" {
			captureIndex = compiled.SubexpIndex(selector.Capture)
		}
		for _, indexes := range compiled.FindAllStringSubmatchIndex(line, -1) {
			startIndex := captureIndex * 2
			if startIndex+1 >= len(indexes) || indexes[startIndex] < 0 {
				continue
			}
			start, end := indexes[startIndex], indexes[startIndex+1]
			result = append(result, occurrence{
				Match: Match{
					Value:    line[start:end],
					Kind:     selector.Label,
					Line:     lineIndex,
					Column:   utf8.RuneCountInString(line[:start]),
					Priority: selector.Priority,
					Start:    start,
					End:      end,
				},
			})
		}
	}
	return result
}

func urlMatchesForLine(line string, lineIndex int, selector config.Selector) []occurrence {
	result := make([]occurrence, 0)
	for _, indexes := range uriCandidate.FindAllStringIndex(line, -1) {
		start, end := indexes[0], trimURLEnd(line, indexes[0], indexes[1])
		if start == end {
			continue
		}
		value := line[start:end]
		parsed, err := url.ParseRequestURI(value)
		if err != nil || !validURL(parsed) {
			continue
		}
		result = append(result, occurrence{Match: Match{
			Value: value, Kind: selector.Label, Line: lineIndex,
			Column: utf8.RuneCountInString(line[:start]), Priority: selector.Priority,
			Start: start, End: end,
		}})
	}
	return result
}

func trimURLEnd(line string, start, end int) int {
	for end > start {
		value := line[start:end]
		last, size := utf8.DecodeLastRuneInString(value)
		trim := strings.ContainsRune(".,;:!?", last)
		switch last {
		case ')':
			trim = strings.Count(value, ")") > strings.Count(value, "(")
		case ']':
			trim = strings.Count(value, "]") > strings.Count(value, "[")
		case '}':
			trim = strings.Count(value, "}") > strings.Count(value, "{")
		}
		if !trim {
			return end
		}
		end -= size
	}
	return end
}

func validURL(parsed *url.URL) bool {
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.Host != ""
	case "file":
		return parsed.Path != "" || parsed.Host != ""
	default:
		return false
	}
}

func splitLines(text string) []string {
	result := make([]string, 0)
	start := 0
	for index, char := range text {
		if char == '\n' {
			result = append(result, text[start:index])
			start = index + 1
		}
	}
	if start < len(text) {
		result = append(result, text[start:])
	}
	return result
}
