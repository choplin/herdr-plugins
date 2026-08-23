package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"regexp"
	"slices"
	"strconv"
	"sync/atomic"
	"time"
)

const maxResponseBytes = 4 * 1024 * 1024

var (
	requestCounter atomic.Uint64
	ansiEscape     = regexp.MustCompile(`(\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07]*(\x07|\x1b\\))`)
)

// SocketClient calls the Herdr newline-delimited socket API.
type SocketClient struct{ Path string }

// APIError is an error returned by Herdr.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *APIError) Error() string { return "Herdr error " + err.Code + ": " + err.Message }

// TransportError reports socket framing, I/O, or response decoding failures.
type TransportError struct {
	Operation string
	Cause     error
	Detail    string
}

func (err *TransportError) Error() string {
	if err.Detail != "" {
		return err.Operation + ": " + err.Detail
	}
	return err.Operation + ": " + err.Cause.Error()
}

// Unwrap returns the transport failure when one exists.
func (err *TransportError) Unwrap() error { return err.Cause }

type envelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *APIError       `json:"error"`
}

// PaneLayout returns the source tab geometry.
func (client SocketClient) PaneLayout(ctx context.Context, paneID string) (Layout, error) {
	var result struct {
		Type   string `json:"type"`
		Layout Layout `json:"layout"`
	}
	if err := client.call(ctx, "pane.layout", map[string]string{"pane_id": paneID}, &result); err != nil {
		return Layout{}, err
	}
	if result.Type != "pane_layout" {
		return Layout{}, &TransportError{Operation: "pane.layout", Detail: "unexpected result type " + result.Type}
	}
	return result.Layout, nil
}

// ReadVisible returns exact visible terminal text without ANSI styling.
func (client SocketClient) ReadVisible(ctx context.Context, paneID string, lines int) (string, error) {
	var result struct {
		Type string `json:"type"`
		Read struct {
			Text string `json:"text"`
		} `json:"read"`
	}
	params := map[string]any{
		"pane_id": paneID, "source": "visible", "lines": lines,
		"format": "text", "strip_ansi": true,
	}
	if err := client.call(ctx, "pane.read", params, &result); err != nil {
		return "", err
	}
	if result.Type != "pane_read" {
		return "", &TransportError{Operation: "pane.read", Detail: "unexpected result type " + result.Type}
	}
	return ansiEscape.ReplaceAllString(result.Read.Text, ""), nil
}

// AppliedLayout identifies the temporary tab and picker pane.
type AppliedLayout struct {
	TabID        string
	PickerPaneID string
}

// ApplyLayout creates a temporary tab that mirrors the source layout.
func (client SocketClient) ApplyLayout(ctx context.Context, workspaceID, label string, root *LaunchNode) (AppliedLayout, error) {
	var result struct {
		Type   string `json:"type"`
		Layout struct {
			TabID string      `json:"tab_id"`
			Root  appliedNode `json:"root"`
		} `json:"layout"`
	}
	params := map[string]any{
		"workspace_id": workspaceID, "tab_label": label, "focus": true, "root": root,
	}
	if err := client.call(ctx, "layout.apply", params, &result); err != nil {
		return AppliedLayout{}, err
	}
	if result.Type != "layout_apply" || result.Layout.TabID == "" {
		return AppliedLayout{}, &TransportError{Operation: "layout.apply", Detail: "invalid layout result"}
	}
	pickerPaneID := findPickerPane(root, &result.Layout.Root)
	if pickerPaneID == "" {
		return AppliedLayout{}, &TransportError{Operation: "layout.apply", Detail: "picker pane is absent from result"}
	}
	return AppliedLayout{TabID: result.Layout.TabID, PickerPaneID: pickerPaneID}, nil
}

type appliedNode struct {
	Type   string       `json:"type"`
	PaneID string       `json:"pane_id"`
	First  *appliedNode `json:"first"`
	Second *appliedNode `json:"second"`
}

type tabInfo struct {
	TabID string `json:"tab_id"`
}

func findPickerPane(request *LaunchNode, result *appliedNode) string {
	if request == nil || result == nil {
		return ""
	}
	if request.Picker && request.Type == "pane" {
		return result.PaneID
	}
	if value := findPickerPane(request.First, result.First); value != "" {
		return value
	}
	return findPickerPane(request.Second, result.Second)
}

// FocusPane focuses a pane.
func (client SocketClient) FocusPane(ctx context.Context, paneID string) error {
	return client.call(ctx, "pane.focus", map[string]string{"pane_id": paneID}, nil)
}

// ZoomPane zooms a pane on.
func (client SocketClient) ZoomPane(ctx context.Context, paneID string) error {
	return client.call(ctx, "pane.zoom", map[string]string{"pane_id": paneID, "mode": "on"}, nil)
}

// FocusTab focuses a tab.
func (client SocketClient) FocusTab(ctx context.Context, tabID string) error {
	return client.call(ctx, "tab.focus", map[string]string{"tab_id": tabID}, nil)
}

// CloseTab closes a tab.
func (client SocketClient) CloseTab(ctx context.Context, tabID string) error {
	return client.call(ctx, "tab.close", map[string]string{"tab_id": tabID}, nil)
}

// ShowNotification displays a Herdr notification in the foreground client.
func (client SocketClient) ShowNotification(ctx context.Context, title string) error {
	var result struct {
		Type string `json:"type"`
	}
	if err := client.call(ctx, "notification.show", map[string]string{"title": title}, &result); err != nil {
		return err
	}
	if result.Type != "notification_show" {
		return &TransportError{Operation: "notification.show", Detail: "unexpected result type " + result.Type}
	}
	return nil
}

// TabExists reports whether a tab is still present in a workspace.
func (client SocketClient) TabExists(ctx context.Context, workspaceID, tabID string) (bool, error) {
	var result struct {
		Type string    `json:"type"`
		Tabs []tabInfo `json:"tabs"`
	}
	if err := client.call(ctx, "tab.list", map[string]string{"workspace_id": workspaceID}, &result); err != nil {
		return false, err
	}
	if result.Type != "tab_list" {
		return false, &TransportError{Operation: "tab.list", Detail: "unexpected result type " + result.Type}
	}
	return slices.ContainsFunc(result.Tabs, func(tab tabInfo) bool {
		return tab.TabID == tabID
	}), nil
}

func (client SocketClient) call(ctx context.Context, method string, params, result any) error {
	id := "quickselect-" + strconv.Itoa(os.Getpid()) + "-" + strconv.FormatUint(requestCounter.Add(1), 10)
	request := struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{ID: id, Method: method, Params: params}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "unix", client.Path)
	if err != nil {
		return &TransportError{Operation: method, Cause: err}
	}
	defer func() { _ = connection.Close() }()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return &TransportError{Operation: method, Cause: err}
	}
	reader := bufio.NewReader(io.LimitReader(connection, maxResponseBytes+1))
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return &TransportError{Operation: method, Cause: err}
	}
	if len(line) > maxResponseBytes {
		return &TransportError{Operation: method, Detail: "response exceeds size limit"}
	}
	var response envelope
	if err := json.Unmarshal(line, &response); err != nil {
		return &TransportError{Operation: method, Cause: err}
	}
	if response.ID != id {
		return &TransportError{Operation: method, Detail: "response id mismatch"}
	}
	if response.Error != nil {
		return response.Error
	}
	if len(response.Result) == 0 {
		return &TransportError{Operation: method, Detail: "response has no result"}
	}
	if result != nil {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return &TransportError{Operation: method, Cause: err}
		}
	}
	return nil
}
