package updatetime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const (
	tokenName    = "updated"
	stateVersion = 1
)

var semanticStates = map[string]bool{
	"working": true,
	"blocked": true,
	"done":    true,
	"idle":    true,
	"unknown": true,
}

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
	result := CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0}
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

// StateError reports invalid or inaccessible persistent plugin state.
type StateError struct {
	Operation string
	Path      string
	Err       error
}

func (err *StateError) Error() string {
	return err.Operation + " " + err.Path + ": " + err.Err.Error()
}

// Unwrap exposes the underlying state error.
func (err *StateError) Unwrap() error {
	return err.Err
}

// InvalidStateError reports incompatible or malformed state contents.
type InvalidStateError struct {
	Detail string
}

func (err *InvalidStateError) Error() string {
	return "invalid plugin state: " + err.Detail
}

// SessionIdentity is a native Agent session reference reported to Herdr.
type SessionIdentity struct {
	Source string `json:"source"`
	Agent  string `json:"agent"`
	Kind   string `json:"kind"`
	Value  string `json:"value"`
}

// LiveAgent is the identity and semantic state currently exposed by Herdr.
type LiveAgent struct {
	PaneID         string           `json:"pane_id"`
	TerminalID     string           `json:"terminal_id"`
	Agent          string           `json:"agent"`
	SemanticState  string           `json:"agent_status"`
	StateChangeSeq uint64           `json:"state_change_seq"`
	Session        *SessionIdentity `json:"agent_session"`
}

// Pane is the pane subset needed for cleanup.
type Pane struct {
	ID string `json:"pane_id"`
}

// Snapshot contains the Herdr resources needed for reconciliation.
type Snapshot struct {
	Agents []LiveAgent `json:"agents"`
	Panes  []Pane      `json:"panes"`
}

// TrackedAgent is persistent state owned by an Agent.
type TrackedAgent struct {
	TrackingID     string           `json:"tracking_id"`
	PaneID         string           `json:"pane_id"`
	TerminalID     string           `json:"terminal_id"`
	Agent          string           `json:"agent"`
	SemanticState  string           `json:"semantic_state"`
	StateChangeSeq uint64           `json:"state_change_seq"`
	UpdatedAt      *string          `json:"updated_at"`
	Session        *SessionIdentity `json:"session"`
}

// PluginState is the on-disk state schema shared with the original Python implementation.
type PluginState struct {
	Version       int            `json:"version"`
	NextReportSeq uint64         `json:"next_report_seq"`
	Agents        []TrackedAgent `json:"agents"`
}

// Report is a pane token projection produced by reconciliation.
type Report struct {
	PaneID string
	Value  *string
	Seq    uint64
}

// IDGenerator creates an opaque tracking generation.
type IDGenerator func() (string, error)

// Clock returns the local observation time.
type Clock func() time.Time

// Now returns the current time.
func Now() time.Time {
	return time.Now()
}

// NewTrackingID creates a cryptographically random tracking generation.
func NewTrackingID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", &OperationError{Operation: "generate tracking ID", Detail: err.Error()}
	}
	return hex.EncodeToString(value), nil
}

// Client is the Herdr API surface used during reconciliation.
type Client interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	ReportToken(ctx context.Context, paneID string, value *string, seq uint64) error
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

// Snapshot returns current Herdr Agent and pane state.
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

// ReportToken reports or clears the updated token for a pane.
func (client *HerdrClient) ReportToken(
	ctx context.Context,
	paneID string,
	value *string,
	seq uint64,
) error {
	arguments := []string{
		client.binary,
		"pane",
		"report-metadata",
		paneID,
		"--source",
		client.source,
		"--seq",
		strconv.FormatUint(seq, 10),
	}
	if value == nil {
		arguments = append(arguments, "--clear-token", tokenName)
	} else {
		arguments = append(arguments, "--token", tokenName+"="+*value)
	}
	return commandError("metadata report for "+paneID, client.runner.Run(ctx, "", arguments))
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

// EventContext reads the plugin event kind and target pane from Herdr's environment.
func EventContext(lookup func(string) (string, bool)) (string, string) {
	event, exists := lookup("HERDR_PLUGIN_EVENT")
	if !exists || event == "" {
		target, _ := lookup("HERDR_PANE_ID")
		return "startup", target
	}
	target, _ := lookup("HERDR_PANE_ID")
	if rawJSON, ok := lookup("HERDR_PLUGIN_EVENT_JSON"); ok && rawJSON != "" {
		var payload struct {
			PaneID string `json:"pane_id"`
		}
		if json.Unmarshal([]byte(rawJSON), &payload) == nil && payload.PaneID != "" {
			target = payload.PaneID
		}
	}
	return event, target
}

// Reconcile updates Agent-owned state and returns its pane metadata projections.
func Reconcile(
	state *PluginState,
	liveAgents []LiveAgent,
	livePaneIDs map[string]bool,
	event string,
	targetPaneID string,
	observedAt time.Time,
	newID IDGenerator,
) (map[string]*string, error) {
	oldRecords := append([]TrackedAgent(nil), state.Agents...)
	unmatched := make(map[int]bool, len(oldRecords))
	for index := range oldRecords {
		unmatched[index] = true
	}
	reconciled := make([]TrackedAgent, 0, len(liveAgents))
	reports := make(map[string]*string)
	occupiedPaneIDs := make(map[string]bool, len(liveAgents))
	for _, live := range liveAgents {
		if validLiveAgent(live) {
			occupiedPaneIDs[live.PaneID] = true
		}
	}
	fullReconcile := event == "startup" || event == "action" || event == "pane.closed" ||
		event == "pane.exited" || event == "pane.moved"

	for _, live := range liveAgents {
		if !validLiveAgent(live) {
			continue
		}
		shouldProcess := fullReconcile || live.PaneID == targetPaneID
		matchIndex := uniqueMatch(oldRecords, unmatched, live, event)

		var record TrackedAgent
		if matchIndex < 0 {
			trackingID, err := newID()
			if err != nil {
				return nil, err
			}
			record = TrackedAgent{
				TrackingID:     trackingID,
				PaneID:         live.PaneID,
				TerminalID:     live.TerminalID,
				Agent:          live.Agent,
				SemanticState:  live.SemanticState,
				StateChangeSeq: live.StateChangeSeq,
				Session:        cloneSession(live.Session),
			}
			if shouldProcess && strings.HasPrefix(event, "pane.agent_") {
				updatedAt := observedAt.Format(time.RFC3339)
				record.UpdatedAt = &updatedAt
			}
		} else {
			delete(unmatched, matchIndex)
			record = oldRecords[matchIndex]
			previousPaneID := record.PaneID
			record.PaneID = live.PaneID
			record.TerminalID = live.TerminalID
			record.Agent = live.Agent
			if live.Session != nil {
				record.Session = cloneSession(live.Session)
			}

			if (event == "startup" || event == "action") &&
				(record.SemanticState != live.SemanticState ||
					record.StateChangeSeq != live.StateChangeSeq) {
				record.SemanticState = live.SemanticState
				record.StateChangeSeq = live.StateChangeSeq
				record.UpdatedAt = nil
			} else if shouldProcess && live.StateChangeSeq > record.StateChangeSeq {
				if record.SemanticState != live.SemanticState {
					updatedAt := observedAt.Format(time.RFC3339)
					record.UpdatedAt = &updatedAt
				}
				record.SemanticState = live.SemanticState
				record.StateChangeSeq = live.StateChangeSeq
			}

			if previousPaneID != live.PaneID && livePaneIDs[previousPaneID] {
				reports[previousPaneID] = nil
			}
		}

		reconciled = append(reconciled, record)
		if shouldProcess {
			formatted, err := formatUpdated(record.UpdatedAt)
			if err != nil {
				return nil, err
			}
			reports[live.PaneID] = formatted
		}
	}

	for index := range unmatched {
		stale := oldRecords[index]
		if livePaneIDs[stale.PaneID] && !occupiedPaneIDs[stale.PaneID] {
			reports[stale.PaneID] = nil
		}
	}
	state.Agents = reconciled
	if targetPaneID != "" && livePaneIDs[targetPaneID] && !occupiedPaneIDs[targetPaneID] {
		reports[targetPaneID] = nil
	}
	return reports, nil
}

func uniqueMatch(
	records []TrackedAgent,
	unmatched map[int]bool,
	live LiveAgent,
	event string,
) int {
	matches := make([]int, 0, 1)
	for index := range unmatched {
		if sessionMatch(records[index], live) {
			matches = append(matches, index)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) != 0 || event == "pane.agent_detected" {
		return -1
	}

	allowMove := event == "startup" || event == "action" || event == "pane.moved"
	for index := range unmatched {
		if occupancyMatch(records[index], live, allowMove) {
			matches = append(matches, index)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return -1
}

func sessionMatch(record TrackedAgent, live LiveAgent) bool {
	return live.Session != nil && record.Session != nil && *record.Session == *live.Session
}

func occupancyMatch(record TrackedAgent, live LiveAgent, allowMove bool) bool {
	sameOccupant := record.Session == nil && record.TerminalID == live.TerminalID &&
		record.Agent == live.Agent
	return sameOccupant && (allowMove || record.PaneID == live.PaneID)
}

func cloneSession(session *SessionIdentity) *SessionIdentity {
	if session == nil {
		return nil
	}
	cloned := *session
	return &cloned
}

func validLiveAgent(agent LiveAgent) bool {
	return agent.PaneID != "" && agent.TerminalID != "" && agent.Agent != "" &&
		semanticStates[agent.SemanticState]
}

func formatUpdated(updatedAt *string) (*string, error) {
	if updatedAt == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *updatedAt)
	if err != nil {
		return nil, &InvalidStateError{Detail: "invalid updated_at timestamp"}
	}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return nil, &OperationError{Operation: "load Asia/Tokyo timezone", Detail: err.Error()}
	}
	formatted := parsed.In(location).Format("15:04")
	return &formatted, nil
}

// LoadState reads the persistent plugin state.
func LoadState(path string) (PluginState, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return PluginState{Version: stateVersion, NextReportSeq: 1, Agents: []TrackedAgent{}}, nil
	}
	if err != nil {
		return PluginState{}, &StateError{Operation: "read state", Path: path, Err: err}
	}

	var state PluginState
	if err := json.Unmarshal(contents, &state); err != nil {
		return PluginState{}, &StateError{Operation: "decode state", Path: path, Err: err}
	}
	if state.Version != stateVersion {
		return PluginState{}, &InvalidStateError{Detail: "unsupported state version"}
	}
	if state.NextReportSeq < 1 {
		return PluginState{}, &InvalidStateError{Detail: "invalid report sequence"}
	}
	if state.Agents == nil {
		state.Agents = []TrackedAgent{}
	}
	return state, nil
}

// SaveState atomically persists plugin state.
func SaveState(path string, state PluginState) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return &StateError{Operation: "create state directory", Path: directory, Err: err}
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return &StateError{Operation: "create temporary state", Path: path, Err: err}
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return &StateError{Operation: "set temporary state permissions", Path: temporaryPath, Err: err}
	}

	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		_ = temporary.Close()
		return &StateError{Operation: "encode state", Path: temporaryPath, Err: err}
	}
	if err := temporary.Close(); err != nil {
		return &StateError{Operation: "close temporary state", Path: temporaryPath, Err: err}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return &StateError{Operation: "replace state", Path: path, Err: err}
	}
	return nil
}

// Run snapshots, reconciles, persists, and projects Agent update metadata.
func Run(
	ctx context.Context,
	client Client,
	stateDir string,
	event string,
	targetPaneID string,
	clock Clock,
	newID IDGenerator,
	stderr io.Writer,
) (int, error) {
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		return 0, err
	}
	statePath := filepath.Join(stateDir, "state.json")
	state, err := LoadState(statePath)
	if err != nil {
		return 0, err
	}
	livePaneIDs := make(map[string]bool, len(snapshot.Panes))
	for _, pane := range snapshot.Panes {
		if pane.ID != "" {
			livePaneIDs[pane.ID] = true
		}
	}
	reportValues, err := Reconcile(
		&state,
		snapshot.Agents,
		livePaneIDs,
		event,
		targetPaneID,
		clock(),
		newID,
	)
	if err != nil {
		return 0, err
	}

	paneIDs := make([]string, 0, len(reportValues))
	for paneID := range reportValues {
		paneIDs = append(paneIDs, paneID)
	}
	sort.Strings(paneIDs)
	reports := make([]Report, 0, len(paneIDs))
	for _, paneID := range paneIDs {
		reports = append(reports, Report{
			PaneID: paneID,
			Value:  reportValues[paneID],
			Seq:    state.NextReportSeq,
		})
		state.NextReportSeq++
	}
	if err := SaveState(statePath, state); err != nil {
		return 0, err
	}

	failures := 0
	for _, report := range reports {
		if err := client.ReportToken(ctx, report.PaneID, report.Value, report.Seq); err != nil {
			failures++
			_, _ = io.WriteString(stderr, err.Error()+"\n")
		}
	}
	return failures, nil
}
