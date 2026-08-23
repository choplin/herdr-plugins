// Package placeholder expands command argument placeholders without invoking a shell.
package placeholder

import "strings"

const (
	// Value is the selected match placeholder.
	Value = "value"
	// PaneID is the source pane identifier placeholder.
	PaneID = "pane_id"
	// CWD is the source pane working directory placeholder.
	CWD = "cwd"
)

// Values contains values available to one command expansion.
type Values struct {
	Value  string
	PaneID string
	CWD    string
}

// Expand replaces supported ${name} expressions. $${name} emits a literal ${name}.
func Expand(input string, values Values) string {
	var output strings.Builder
	for index := 0; index < len(input); {
		escaped := strings.HasPrefix(input[index:], "$${")
		start := index
		if escaped {
			start++
		}
		if strings.HasPrefix(input[start:], "${") {
			if end := strings.IndexByte(input[start+2:], '}'); end >= 0 {
				end += start + 3
				name := input[start+2 : end-1]
				if escaped {
					output.WriteString(input[start:end])
					index = end
					continue
				}
				if value, ok := lookup(name, values); ok {
					output.WriteString(value)
					index = end
					continue
				}
			}
		}
		output.WriteByte(input[index])
		index++
	}
	return output.String()
}

// Contains reports whether input contains an unescaped supported placeholder.
func Contains(input, name string) bool {
	token := "${" + name + "}"
	for offset := 0; ; {
		index := strings.Index(input[offset:], token)
		if index < 0 {
			return false
		}
		index += offset
		if index == 0 || input[index-1] != '$' {
			return true
		}
		offset = index + len(token)
	}
}

// ContainsLegacy reports a pre-${name} placeholder that would otherwise pass through literally.
func ContainsLegacy(input string) bool {
	for _, name := range []string{Value, PaneID, CWD} {
		token := "{" + name + "}"
		for offset := 0; ; {
			index := strings.Index(input[offset:], token)
			if index < 0 {
				break
			}
			index += offset
			if index == 0 || input[index-1] != '$' {
				return true
			}
			offset = index + len(token)
		}
	}
	return false
}

func lookup(name string, values Values) (string, bool) {
	switch name {
	case Value:
		return values.Value, true
	case PaneID:
		return values.PaneID, true
	case CWD:
		return values.CWD, true
	default:
		return "", false
	}
}
