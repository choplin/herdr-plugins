package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

const tokenName = "repo"

// CommandResult contains the observable result of an argv command.
type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	StartErr error
}

// Runner executes argv commands without invoking a shell.
type Runner interface {
	Run(ctx context.Context, cwd string, argv []string) CommandResult
}

// OSRunner executes commands on the host operating system.
type OSRunner struct{}

// Run implements Runner.
func (OSRunner) Run(ctx context.Context, cwd string, argv []string) CommandResult {
	if len(argv) == 0 {
		return CommandResult{ExitCode: -1, StartErr: &CommandStartError{}}
	}

	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := CommandResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: 0,
	}
	if err == nil {
		return result
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result
	}
	result.ExitCode = -1
	result.StartErr = err
	return result
}

// CommandStartError indicates that an empty command was passed to a runner.
type CommandStartError struct{}

func (*CommandStartError) Error() string {
	return "cannot start an empty command"
}

// OperationError reports a failed Herdr operation.
type OperationError struct {
	Operation string
	Detail    string
}

func (err *OperationError) Error() string {
	return err.Operation + " failed: " + err.Detail
}

// ProtocolError reports malformed output from the Herdr CLI.
type ProtocolError struct {
	Operation string
}

func (err *ProtocolError) Error() string {
	return err.Operation + " returned an invalid response"
}

// RepositoryIdentity derives the shared repository name for a working directory.
func RepositoryIdentity(ctx context.Context, cwd string, runner Runner) string {
	result := runner.Run(
		ctx,
		cwd,
		[]string{"git", "rev-parse", "--path-format=absolute", "--git-common-dir"},
	)
	if result.StartErr != nil || result.ExitCode != 0 {
		return ""
	}

	output := bytes.TrimSuffix(result.Stdout, []byte("\n"))
	output = bytes.TrimSuffix(output, []byte("\r"))
	if len(output) == 0 {
		return ""
	}

	commonDir := filepath.Clean(string(output))
	base := filepath.Base(commonDir)
	switch {
	case base == ".git":
		return filepath.Base(filepath.Dir(commonDir))
	case strings.HasSuffix(base, ".git"):
		return strings.TrimSuffix(base, ".git")
	default:
		return base
	}
}

// Snapshot contains the Herdr resources needed for reconciliation.
type Snapshot struct {
	Workspaces []Workspace `json:"workspaces"`
	Panes      []Pane      `json:"panes"`
}

// Workspace is the workspace subset returned by Herdr's snapshot API.
type Workspace struct {
	ID          string `json:"workspace_id"`
	ActiveTabID string `json:"active_tab_id"`
	Label       string `json:"label"`
}

// Pane is the pane subset returned by Herdr's snapshot API.
type Pane struct {
	ID          string `json:"pane_id"`
	WorkspaceID string `json:"workspace_id"`
	TabID       string `json:"tab_id"`
	CWD         string `json:"cwd"`
}

// Target is the repository identity input and affected panes for one workspace.
type Target struct {
	WorkspaceID string
	Label       string
	CWD         string
	PaneIDs     []string
}

// WorkspaceTargets selects the active tab's CWD while retaining every pane ID.
func WorkspaceTargets(snapshot Snapshot) []Target {
	panesByWorkspace := make(map[string][]Pane)
	for _, pane := range snapshot.Panes {
		if pane.WorkspaceID == "" {
			continue
		}
		panesByWorkspace[pane.WorkspaceID] = append(panesByWorkspace[pane.WorkspaceID], pane)
	}

	targets := make([]Target, 0, len(snapshot.Workspaces))
	for _, workspace := range snapshot.Workspaces {
		if workspace.ID == "" {
			continue
		}
		panes := panesByWorkspace[workspace.ID]
		var selected Pane
		if len(panes) != 0 {
			selected = panes[0]
		}
		for _, pane := range panes {
			if pane.TabID == workspace.ActiveTabID {
				selected = pane
				break
			}
		}

		paneIDs := make([]string, 0, len(panes))
		for _, pane := range panes {
			if pane.ID != "" {
				paneIDs = append(paneIDs, pane.ID)
			}
		}
		targets = append(targets, Target{
			WorkspaceID: workspace.ID,
			Label:       workspace.Label,
			CWD:         selected.CWD,
			PaneIDs:     paneIDs,
		})
	}
	return targets
}

// Client is the Herdr API surface used during reconciliation.
type Client interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	ReportToken(ctx context.Context, resource, resourceID string, value *string) error
}

// HerdrClient calls the Herdr CLI through argv commands.
type HerdrClient struct {
	binary string
	source string
	runner Runner
}

// NewHerdrClient constructs a CLI-backed Herdr client.
func NewHerdrClient(binary, source string, runner Runner) *HerdrClient {
	return &HerdrClient{binary: binary, source: source, runner: runner}
}

type snapshotEnvelope struct {
	Result *snapshotResult `json:"result"`
}

type snapshotResult struct {
	Snapshot *Snapshot `json:"snapshot"`
}

// Snapshot returns the current Herdr workspace and pane state.
func (client *HerdrClient) Snapshot(ctx context.Context) (Snapshot, error) {
	result := client.runner.Run(ctx, "", []string{client.binary, "api", "snapshot"})
	if err := commandError("herdr api snapshot", result); err != nil {
		return Snapshot{}, err
	}

	var response snapshotEnvelope
	if err := json.Unmarshal(result.Stdout, &response); err != nil ||
		response.Result == nil || response.Result.Snapshot == nil {
		return Snapshot{}, &ProtocolError{Operation: "herdr api snapshot"}
	}
	return *response.Result.Snapshot, nil
}

// ReportToken reports or clears the repository token for a Herdr resource.
func (client *HerdrClient) ReportToken(
	ctx context.Context,
	resource string,
	resourceID string,
	value *string,
) error {
	arguments := []string{
		client.binary,
		resource,
		"report-metadata",
		resourceID,
		"--source",
		client.source,
	}
	if value == nil {
		arguments = append(arguments, "--clear-token", tokenName)
	} else {
		arguments = append(arguments, "--token", tokenName+"="+*value)
	}
	return commandError("metadata report for "+resourceID, client.runner.Run(ctx, "", arguments))
}

func commandError(operation string, result CommandResult) error {
	if result.StartErr == nil && result.ExitCode == 0 {
		return nil
	}
	detail := strings.TrimSpace(string(result.Stderr))
	if detail == "" && result.StartErr != nil {
		detail = result.StartErr.Error()
	}
	if detail == "" {
		detail = "unknown error"
	}
	return &OperationError{Operation: operation, Detail: detail}
}

// Reconcile reports the Git identity or workspace-label fallback to workspaces and panes.
func Reconcile(ctx context.Context, client Client, gitRunner Runner, stderr io.Writer) (int, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return 0, err
	}

	failures := 0
	for _, target := range WorkspaceTargets(snapshot) {
		value := ""
		if target.CWD != "" {
			value = RepositoryIdentity(ctx, target.CWD, gitRunner)
		}
		if value == "" {
			value = target.Label
		}

		resources := make([][2]string, 0, len(target.PaneIDs)+1)
		resources = append(resources, [2]string{"workspace", target.WorkspaceID})
		for _, paneID := range target.PaneIDs {
			resources = append(resources, [2]string{"pane", paneID})
		}
		for _, resource := range resources {
			var token *string
			if value != "" {
				token = &value
			}
			if err := client.ReportToken(ctx, resource[0], resource[1], token); err != nil {
				failures++
				_, _ = io.WriteString(stderr, err.Error()+"\n")
			}
		}
	}
	return failures, nil
}
