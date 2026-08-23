// Package herdr integrates Quick Select with Herdr invocation context and socket APIs.
package herdr

import (
	"encoding/json"
	"strings"
)

// WorkspaceScope resolves a stable exclusion scope for one Herdr workspace.
func WorkspaceScope(direct, paneID, contextJSON string) string {
	if direct != "" {
		return "workspace:" + direct
	}
	var value any
	if json.Unmarshal([]byte(contextJSON), &value) == nil {
		for _, path := range [][]string{{"workspace", "workspace_id"}, {"workspace", "id"}, {"workspace_id"}} {
			if workspaceID, ok := stringAt(value, path); ok && workspaceID != "" {
				return "workspace:" + workspaceID
			}
		}
	}
	if separator := strings.IndexByte(paneID, ':'); separator > 0 {
		return "workspace:" + paneID[:separator]
	}
	return "global"
}

// TargetPane resolves the source pane from direct context or invocation JSON.
func TargetPane(direct, contextJSON string) (string, error) {
	if direct != "" {
		return direct, nil
	}
	if contextJSON == "" {
		return "", &ContextError{Detail: "Herdr did not provide a source pane"}
	}
	var value any
	if err := json.Unmarshal([]byte(contextJSON), &value); err != nil {
		return "", &ContextError{Detail: "parse HERDR_PLUGIN_CONTEXT_JSON: " + err.Error(), Cause: err}
	}
	for _, path := range [][]string{
		{"focused_pane", "pane_id"},
		{"focused_pane", "id"},
		{"pane", "pane_id"},
		{"pane", "id"},
		{"focused_pane_id"},
		{"pane_id"},
	} {
		if paneID, ok := stringAt(value, path); ok && paneID != "" {
			return paneID, nil
		}
	}
	return "", &ContextError{Detail: "Herdr invocation context has no focused pane"}
}

// ContextError reports missing or malformed Herdr invocation context.
type ContextError struct {
	Detail string
	Cause  error
}

func (err *ContextError) Error() string { return err.Detail }

// Unwrap returns the context decoding failure when one exists.
func (err *ContextError) Unwrap() error { return err.Cause }

// FocusedCWD returns the focused pane's cwd when present.
func FocusedCWD(contextJSON string) string {
	var value any
	if json.Unmarshal([]byte(contextJSON), &value) != nil {
		return ""
	}
	for _, path := range [][]string{{"focused_pane", "cwd"}, {"pane", "cwd"}, {"focused_pane_cwd"}} {
		if cwd, ok := stringAt(value, path); ok {
			return cwd
		}
	}
	return ""
}

func stringAt(value any, path []string) (string, bool) {
	current := value
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		current, ok = object[part]
		if !ok {
			return "", false
		}
	}
	result, ok := current.(string)
	return result, ok
}
