package updatetime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var tokyoTime = time.Date(2026, 8, 22, 14, 32, 0, 0, time.FixedZone("JST", 9*60*60))

func TestSemanticStateTransitionUpdatesTime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		before string
		after  string
	}{
		{before: "working", after: "blocked"},
		{before: "blocked", after: "working"},
		{before: "working", after: "idle"},
		{before: "working", after: "done"},
	}
	for _, test := range tests {
		t.Run(test.before+"_to_"+test.after, func(t *testing.T) {
			t.Parallel()

			state := newState()
			mustReconcile(t, &state, []LiveAgent{liveAgent(test.before, 1)}, "pane.agent_detected", "w1:p1", tokyoTime)
			later := tokyoTime.Add(7 * time.Minute)
			reports := mustReconcile(t, &state, []LiveAgent{liveAgent(test.after, 2)}, "pane.agent_status_changed", "w1:p1", later)

			if state.Agents[0].SemanticState != test.after {
				t.Fatalf("SemanticState = %q, want %q", state.Agents[0].SemanticState, test.after)
			}
			if state.Agents[0].UpdatedAt == nil || *state.Agents[0].UpdatedAt != later.Format(time.RFC3339) {
				t.Fatalf("UpdatedAt = %v, want %q", state.Agents[0].UpdatedAt, later.Format(time.RFC3339))
			}
			assertReport(t, reports, "w1:p1", "14:39")
		})
	}
}

func TestFirstDetectionSetsTime(t *testing.T) {
	t.Parallel()

	state := newState()
	reports := mustReconcile(t, &state, []LiveAgent{liveAgent("working", 1)}, "pane.agent_detected", "w1:p1", tokyoTime)

	if state.Agents[0].UpdatedAt == nil || *state.Agents[0].UpdatedAt != tokyoTime.Format(time.RFC3339) {
		t.Fatalf("UpdatedAt = %v, want %q", state.Agents[0].UpdatedAt, tokyoTime.Format(time.RFC3339))
	}
	assertReport(t, reports, "w1:p1", "14:32")
}

func TestDuplicateSameStateDoesNotUpdateTime(t *testing.T) {
	t.Parallel()

	state := newState()
	mustReconcile(t, &state, []LiveAgent{liveAgent("working", 1)}, "pane.agent_detected", "w1:p1", tokyoTime)
	original := *state.Agents[0].UpdatedAt
	mustReconcile(t, &state, []LiveAgent{liveAgent("working", 2)}, "pane.agent_status_changed", "w1:p1", tokyoTime.Add(10*time.Minute))

	if state.Agents[0].UpdatedAt == nil || *state.Agents[0].UpdatedAt != original {
		t.Fatalf("UpdatedAt = %v, want unchanged %q", state.Agents[0].UpdatedAt, original)
	}
	if state.Agents[0].StateChangeSeq != 2 {
		t.Fatalf("StateChangeSeq = %d, want 2", state.Agents[0].StateChangeSeq)
	}
}

func TestAgentsAreTrackedIndependentlyWithUnicodeAndSpaces(t *testing.T) {
	t.Parallel()

	state := newState()
	agents := []LiveAgent{
		liveAgentWith("w1:p1", "term 東京 1", "codex", "working", 1, nil),
		liveAgentWith("w2:p1", "term space 2", "claude", "working", 1, nil),
	}
	reports := mustReconcile(t, &state, agents, "pane.agent_detected", "w2:p1", tokyoTime)

	if len(reports) != 1 {
		t.Fatalf("reports = %#v, want one report", reports)
	}
	assertReport(t, reports, "w2:p1", "14:32")
	byPane := recordsByPane(state.Agents)
	if byPane["w1:p1"].UpdatedAt != nil {
		t.Fatalf("w1:p1 UpdatedAt = %v, want nil", byPane["w1:p1"].UpdatedAt)
	}
	if byPane["w2:p1"].UpdatedAt == nil {
		t.Fatal("w2:p1 UpdatedAt = nil, want timestamp")
	}
}

func TestPaneCloseRemovesState(t *testing.T) {
	t.Parallel()

	state := newState()
	mustReconcile(t, &state, []LiveAgent{liveAgent("working", 1)}, "pane.agent_detected", "w1:p1", tokyoTime)
	reports, err := Reconcile(&state, nil, map[string]bool{}, "pane.closed", "", tokyoTime, fixedID("unused"))
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(state.Agents) != 0 || len(reports) != 0 {
		t.Fatalf("state = %#v, reports = %#v, want empty", state, reports)
	}
}

func TestAgentReplacementInSamePaneGetsNewIdentity(t *testing.T) {
	t.Parallel()

	state := newState()
	mustReconcile(t, &state, []LiveAgent{liveAgentWith("w1:p1", "term shared", "codex", "working", 1, nil)}, "pane.agent_detected", "w1:p1", tokyoTime)
	oldID := state.Agents[0].TrackingID
	later := tokyoTime.Add(4 * time.Minute)
	reports, err := Reconcile(
		&state,
		[]LiveAgent{liveAgentWith("w1:p1", "term shared", "claude", "working", 2, nil)},
		map[string]bool{"w1:p1": true},
		"pane.agent_detected",
		"w1:p1",
		later,
		fixedID("replacement"),
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(state.Agents) != 1 || state.Agents[0].Agent != "claude" || state.Agents[0].TrackingID == oldID {
		t.Fatalf("Agents = %#v, want new claude identity", state.Agents)
	}
	assertReport(t, reports, "w1:p1", "14:36")
}

func TestSameKindReplacementUsesDetectionGeneration(t *testing.T) {
	t.Parallel()

	state := newState()
	mustReconcile(t, &state, []LiveAgent{liveAgent("working", 1)}, "pane.agent_detected", "w1:p1", tokyoTime)
	oldID := state.Agents[0].TrackingID
	_, err := Reconcile(
		&state,
		[]LiveAgent{liveAgent("idle", 2)},
		map[string]bool{"w1:p1": true},
		"pane.agent_detected",
		"w1:p1",
		tokyoTime.Add(5*time.Minute),
		fixedID("replacement"),
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if state.Agents[0].TrackingID == oldID || state.Agents[0].TrackingID != "replacement" {
		t.Fatalf("TrackingID = %q, want replacement", state.Agents[0].TrackingID)
	}
}

func TestOlderObservationDoesNotOverwriteNewerState(t *testing.T) {
	t.Parallel()

	state := newState()
	mustReconcile(t, &state, []LiveAgent{liveAgent("blocked", 5)}, "pane.agent_detected", "w1:p1", tokyoTime)
	saved := *state.Agents[0].UpdatedAt
	mustReconcile(t, &state, []LiveAgent{liveAgent("working", 4)}, "pane.agent_status_changed", "w1:p1", tokyoTime.Add(20*time.Minute))

	record := state.Agents[0]
	if record.SemanticState != "blocked" || record.StateChangeSeq != 5 || record.UpdatedAt == nil || *record.UpdatedAt != saved {
		t.Fatalf("record = %#v, want newer blocked state preserved", record)
	}
}

func TestStartupReconciliation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		initial    *LiveAgent
		startup    LiveAgent
		wantValue  string
		wantNoTime bool
	}{
		{
			name:      "matching native session restores metadata",
			initial:   agentPointer(liveAgentWithSession("working", 1, "セッション with spaces")),
			startup:   liveAgentWithSession("working", 1, "セッション with spaces"),
			wantValue: "14:32",
		},
		{
			name:       "unknown existing Agent gets no invented time",
			initial:    nil,
			startup:    liveAgent("working", 1),
			wantNoTime: true,
		},
		{
			name:       "state mismatch clears stale time",
			initial:    agentPointer(liveAgentWithSession("working", 1, "session-1")),
			startup:    liveAgentWithSession("idle", 2, "session-1"),
			wantNoTime: true,
		},
		{
			name:       "sequence mismatch clears stale time",
			initial:    agentPointer(liveAgentWithSession("working", 1, "session-1")),
			startup:    liveAgentWithSession("working", 3, "session-1"),
			wantNoTime: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			state := newState()
			if test.initial != nil {
				mustReconcile(t, &state, []LiveAgent{*test.initial}, "pane.agent_detected", "w1:p1", tokyoTime)
			}
			reports := mustReconcile(t, &state, []LiveAgent{test.startup}, "startup", "", tokyoTime.Add(24*time.Hour))
			if test.wantNoTime {
				if state.Agents[0].UpdatedAt != nil || reports["w1:p1"] != nil {
					t.Fatalf("state = %#v, reports = %#v, want no timestamp", state, reports)
				}
				return
			}
			assertReport(t, reports, "w1:p1", test.wantValue)
		})
	}
}

func TestFormatUpdatedUsesAsiaTokyo(t *testing.T) {
	t.Parallel()

	utc := "2026-08-22T05:32:00Z"
	got, err := formatUpdated(&utc)
	if err != nil {
		t.Fatalf("formatUpdated() error = %v", err)
	}
	if got == nil || *got != "14:32" {
		t.Fatalf("formatUpdated() = %v, want 14:32", got)
	}
}

func TestStateRoundTripReadsPythonSchemaWithUnicode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "状態", "state.json")
	pythonState := `{
  "version": 1,
  "next_report_seq": 9,
  "agents": [{
    "tracking_id": "tracking-1",
    "pane_id": "w1:p1",
    "terminal_id": "terminal 東京",
    "agent": "codex",
    "semantic_state": "working",
    "state_change_seq": 3,
    "updated_at": "2026-08-22T14:32:00+09:00",
    "session": {"source":"herdr:codex","agent":"codex","kind":"id","value":"会話 1"}
  }]
}`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(pythonState), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if state.Agents[0].Session == nil || state.Agents[0].Session.Value != "会話 1" {
		t.Fatalf("state = %#v, want Unicode session", state)
	}
	if err := SaveState(path, state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(contents, []byte("会話 1")) {
		t.Fatalf("state contents = %s, want Unicode preserved", contents)
	}
}

type recordingRunner struct {
	calls [][]string
}

func (runner *recordingRunner) Run(_ context.Context, _ string, argv []string) CommandResult {
	runner.calls = append(runner.calls, slices.Clone(argv))
	if slices.Equal(argv[1:], []string{"api", "snapshot"}) {
		stdout, _ := json.Marshal(map[string]any{
			"result": map[string]any{"snapshot": Snapshot{}},
		})
		return CommandResult{Stdout: stdout, ExitCode: 0}
	}
	return CommandResult{Stdout: []byte("{}"), ExitCode: 0}
}

func TestHerdrClientUsesArgvAndIsolatedMetadataSource(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	client := NewHerdrClient("herdr-test", "plugin:choplin.agent-update-time", runner)
	if _, err := client.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	value := "14:32"
	if err := client.ReportToken(context.Background(), "w1:p1", &value, 9); err != nil {
		t.Fatalf("ReportToken() error = %v", err)
	}
	want := []string{
		"herdr-test", "pane", "report-metadata", "w1:p1",
		"--source", "plugin:choplin.agent-update-time", "--seq", "9", "--token", "updated=14:32",
	}
	if len(runner.calls) != 2 || !slices.Equal(runner.calls[1], want) {
		t.Fatalf("calls = %#v, want second call %#v", runner.calls, want)
	}
	for _, argument := range runner.calls[1] {
		if strings.HasPrefix(argument, "repo=") {
			t.Fatalf("unexpected repository token argument %q", argument)
		}
	}
}

func TestSnapshotAcceptsExistingRepositoryTokenAndUnicodeSession(t *testing.T) {
	t.Parallel()

	runner := &fixtureRunner{stdout: []byte(`{"result":{"snapshot":{"agents":[{
"pane_id":"w1:p1","terminal_id":"terminal path with spaces/東京","agent":"codex",
"agent_status":"idle","state_change_seq":3,"tokens":{"repo":"my repo"},
"agent_session":{"source":"herdr:codex","agent":"codex","kind":"path","value":"/tmp/会話 with spaces.jsonl"}
}],"panes":[{"pane_id":"w1:p1"}]}}}`)}
	client := NewHerdrClient("herdr", "plugin:test", runner)
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Agents) != 1 || snapshot.Agents[0].Session == nil ||
		snapshot.Agents[0].Session.Value != "/tmp/会話 with spaces.jsonl" {
		t.Fatalf("snapshot = %#v, want Unicode Agent session", snapshot)
	}
}

type fixtureRunner struct {
	stdout []byte
}

func (runner *fixtureRunner) Run(context.Context, string, []string) CommandResult {
	return CommandResult{Stdout: runner.stdout, ExitCode: 0}
}

func TestEventContextPrefersEventJSONPaneID(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"HERDR_PLUGIN_EVENT":      "pane.agent_status_changed",
		"HERDR_PANE_ID":           "w1:p-old",
		"HERDR_PLUGIN_EVENT_JSON": `{"pane_id":"w1:p東京"}`,
	}
	event, paneID := EventContext(func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})
	if event != "pane.agent_status_changed" || paneID != "w1:p東京" {
		t.Fatalf("EventContext() = (%q, %q)", event, paneID)
	}
}

func TestManifestBuildsGoAndHasOnlyEventDrivenHooks(t *testing.T) {
	t.Parallel()

	manifest, err := os.ReadFile(filepath.Join("..", "..", "herdr-plugin.toml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	contents := string(manifest)
	if !strings.Contains(contents, `command = ["go", "build", "-trimpath", "-o", "herdr-agent-update-time", "./cmd/herdr-agent-update-time"]`) {
		t.Error("manifest does not build the Go command directly")
	}
	for _, event := range []string{"pane.agent_detected", "pane.agent_status_changed", "pane.closed"} {
		if !strings.Contains(contents, `on = "`+event+`"`) {
			t.Errorf("manifest does not subscribe to %s", event)
		}
	}
	if strings.Contains(contents, "pane.output_changed") || strings.Contains(contents, "python") ||
		strings.Contains(contents, `command = ["sh"`) {
		t.Error("manifest unexpectedly polls output or invokes Python or a shell")
	}
}

func newState() PluginState {
	return PluginState{Version: stateVersion, NextReportSeq: 1, Agents: []TrackedAgent{}}
}

func liveAgent(state string, seq uint64) LiveAgent {
	return liveAgentWith("w1:p1", "term-1", "codex", state, seq, nil)
}

func liveAgentWithSession(state string, seq uint64, value string) LiveAgent {
	return liveAgentWith(
		"w1:p1",
		"term-1",
		"codex",
		state,
		seq,
		&SessionIdentity{Source: "herdr:codex", Agent: "codex", Kind: "id", Value: value},
	)
}

func liveAgentWith(
	paneID string,
	terminalID string,
	agent string,
	state string,
	seq uint64,
	session *SessionIdentity,
) LiveAgent {
	return LiveAgent{
		PaneID:         paneID,
		TerminalID:     terminalID,
		Agent:          agent,
		SemanticState:  state,
		StateChangeSeq: seq,
		Session:        session,
	}
}

func agentPointer(agent LiveAgent) *LiveAgent {
	return &agent
}

func fixedID(value string) IDGenerator {
	return func() (string, error) {
		return value, nil
	}
}

func mustReconcile(
	t *testing.T,
	state *PluginState,
	agents []LiveAgent,
	event string,
	target string,
	at time.Time,
) map[string]*string {
	t.Helper()
	livePaneIDs := make(map[string]bool, len(agents))
	for _, agent := range agents {
		livePaneIDs[agent.PaneID] = true
	}
	reports, err := Reconcile(state, agents, livePaneIDs, event, target, at, fixedID("tracking"))
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	return reports
}

func assertReport(t *testing.T, reports map[string]*string, paneID, want string) {
	t.Helper()
	got, ok := reports[paneID]
	if !ok || got == nil || *got != want {
		t.Fatalf("reports[%q] = %v, want %q", paneID, got, want)
	}
}

func recordsByPane(records []TrackedAgent) map[string]TrackedAgent {
	result := make(map[string]TrackedAgent, len(records))
	for _, record := range records {
		result[record.PaneID] = record
	}
	return result
}
