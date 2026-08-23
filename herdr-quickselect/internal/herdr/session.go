package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/choplin/herdr-quickselect/internal/config"
	"github.com/choplin/herdr-quickselect/internal/picker"
)

// Snapshot is the immutable source state consumed by the temporary picker pane.
type Snapshot struct {
	TargetPaneID string            `json:"target_pane_id"`
	ReturnTabID  string            `json:"return_tab_id"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	ZoomPicker   bool              `json:"zoom_picker"`
	CWD          string            `json:"cwd"`
	Action       config.Action     `json:"action"`
	Selectors    []config.Selector `json:"selectors"`
	Viewport     picker.Viewport   `json:"viewport"`
}

// LaunchFiles transfer the snapshot and coordinate layout readiness.
type LaunchFiles struct {
	SnapshotPath string
	ReadyPath    string
}

// ActiveSession is a launched picker whose lifetime can be observed by its launcher.
type ActiveSession struct {
	TemporaryTabID string
	WorkspaceID    string
	Files          LaunchFiles
}

// SessionError reports picker orchestration failures.
type SessionError struct {
	Operation string
	Cause     error
	Detail    string
}

func (err *SessionError) Error() string {
	if err.Detail != "" {
		return err.Operation + ": " + err.Detail
	}
	return err.Operation + ": " + err.Cause.Error()
}

// Unwrap returns the underlying session failure.
func (err *SessionError) Unwrap() error { return err.Cause }

// PrepareAndLaunch captures the source and starts an inline picker in a mirrored tab.
func PrepareAndLaunch(
	ctx context.Context,
	client SocketClient,
	targetPaneID, binaryPath, cwd string,
	command config.Command,
	action config.Action,
	selectors []config.Selector,
) (ActiveSession, error) {
	layout, err := client.PaneLayout(ctx, targetPaneID)
	if err != nil {
		return ActiveSession{}, err
	}
	width, height, err := ContentSize(layout, targetPaneID)
	if err != nil {
		return ActiveSession{}, err
	}
	text, err := client.ReadVisible(ctx, targetPaneID, height)
	if err != nil {
		return ActiveSession{}, err
	}
	tree, err := DeriveLayoutTree(layout, targetPaneID)
	if err != nil {
		return ActiveSession{}, err
	}
	snapshot := Snapshot{
		TargetPaneID: targetPaneID,
		ReturnTabID:  layout.TabID,
		Width:        width,
		Height:       height,
		ZoomPicker:   layout.Zoomed && layout.FocusedPaneID == targetPaneID,
		CWD:          cwd,
		Action:       action,
		Selectors:    selectors,
		Viewport:     picker.MapViewport(text, width, height),
	}
	files, err := createLaunchFiles(snapshot)
	if err != nil {
		return ActiveSession{}, err
	}
	root := buildLaunchTree(tree, targetPaneID, binaryPath, files)
	applied, err := client.ApplyLayout(ctx, layout.WorkspaceID, command.Label, root)
	if err != nil {
		_ = files.Remove()
		return ActiveSession{}, err
	}
	cleanupOnFailure := func(primary error) error {
		cleanup := Cleanup(ctx, client, layout.TabID, applied.TabID)
		filesCleanup := files.Remove()
		return errors.Join(primary, cleanup, filesCleanup)
	}
	if err := client.FocusPane(ctx, applied.PickerPaneID); err != nil {
		return ActiveSession{}, cleanupOnFailure(err)
	}
	if snapshot.ZoomPicker {
		if err := client.ZoomPane(ctx, applied.PickerPaneID); err != nil {
			return ActiveSession{}, cleanupOnFailure(err)
		}
	}
	if err := files.SignalReady(); err != nil {
		return ActiveSession{}, cleanupOnFailure(err)
	}
	return ActiveSession{TemporaryTabID: applied.TabID, WorkspaceID: layout.WorkspaceID, Files: files}, nil
}

// Wait blocks until the picker removes its snapshot or its temporary tab disappears.
func (session ActiveSession) Wait(ctx context.Context, client SocketClient) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(session.Files.SnapshotPath); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return &SessionError{Operation: "watch picker snapshot", Cause: err}
		}
		exists, err := client.TabExists(ctx, session.WorkspaceID, session.TemporaryTabID)
		if err == nil && !exists {
			return session.Files.Remove()
		}
		select {
		case <-ctx.Done():
			return &SessionError{Operation: "wait for picker completion", Cause: ctx.Err()}
		case <-ticker.C:
		}
	}
}

// ReadSnapshot waits for layout readiness and loads the picker snapshot.
func ReadSnapshot(ctx context.Context, snapshotPath, readyPath string) (Snapshot, LaunchFiles, error) {
	files := LaunchFiles{SnapshotPath: snapshotPath, ReadyPath: readyPath}
	contents, err := os.ReadFile(snapshotPath)
	if err != nil {
		return Snapshot{}, files, &SessionError{Operation: "read picker snapshot", Cause: err}
	}
	var snapshot Snapshot
	if err := json.Unmarshal(contents, &snapshot); err != nil {
		return Snapshot{}, files, &SessionError{Operation: "decode picker snapshot", Cause: err}
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return snapshot, files, &SessionError{Operation: "wait for picker readiness", Cause: err}
		}
		select {
		case <-ctx.Done():
			return snapshot, files, &SessionError{Operation: "wait for picker readiness", Cause: ctx.Err()}
		case <-deadline.C:
			return snapshot, files, &SessionError{Operation: "wait for picker readiness", Detail: "timed out"}
		case <-ticker.C:
		}
	}
	return snapshot, files, nil
}

// Cleanup restores the source tab and closes only the explicit temporary tab.
func Cleanup(ctx context.Context, client SocketClient, returnTabID, temporaryTabID string) error {
	if returnTabID == "" || temporaryTabID == "" || returnTabID == temporaryTabID {
		return &SessionError{Operation: "cleanup picker", Detail: "unsafe tab identifiers"}
	}
	return errors.Join(client.FocusTab(ctx, returnTabID), client.CloseTab(ctx, temporaryTabID))
}

func createLaunchFiles(snapshot Snapshot) (LaunchFiles, error) {
	file, err := os.CreateTemp("", "herdr-quickselect-*.json")
	if err != nil {
		return LaunchFiles{}, &SessionError{Operation: "create picker snapshot", Cause: err}
	}
	files := LaunchFiles{SnapshotPath: file.Name(), ReadyPath: file.Name() + ".ready"}
	encoderErr := json.NewEncoder(file).Encode(snapshot)
	closeErr := file.Close()
	if err := errors.Join(encoderErr, closeErr); err != nil {
		_ = files.Remove()
		return LaunchFiles{}, &SessionError{Operation: "write picker snapshot", Cause: err}
	}
	return files, nil
}

// SignalReady atomically releases the picker process.
func (files LaunchFiles) SignalReady() error {
	temporary := files.ReadyPath + ".tmp"
	if err := os.WriteFile(temporary, []byte("ready"), 0o600); err != nil {
		return &SessionError{Operation: "write picker readiness", Cause: err}
	}
	if err := os.Rename(temporary, files.ReadyPath); err != nil {
		return &SessionError{Operation: "publish picker readiness", Cause: err}
	}
	return nil
}

// Remove deletes temporary launch files.
func (files LaunchFiles) Remove() error {
	var result error
	for _, path := range []string{files.SnapshotPath, files.ReadyPath, files.ReadyPath + ".tmp"} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}
	return result
}

func buildLaunchTree(node *LayoutNode, targetPaneID, binaryPath string, files LaunchFiles) *LaunchNode {
	if node.SourcePaneID != "" {
		if node.SourcePaneID == targetPaneID {
			return &LaunchNode{
				Type: "pane", Picker: true,
				Command: []string{binaryPath, "pick", "--snapshot", files.SnapshotPath, "--ready", files.ReadyPath},
			}
		}
		return &LaunchNode{Type: "pane", Command: []string{binaryPath, "idle"}}
	}
	return &LaunchNode{
		Type: "split", Direction: node.Direction, Ratio: node.Ratio,
		First:  buildLaunchTree(node.First, targetPaneID, binaryPath, files),
		Second: buildLaunchTree(node.Second, targetPaneID, binaryPath, files),
	}
}
