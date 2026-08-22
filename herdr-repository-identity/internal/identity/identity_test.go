package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMainAndLinkedWorktreesShareIdentityInPathWithSpaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mainWorktree := filepath.Join(root, "project with spaces 'quoted'")
	linkedWorktree := filepath.Join(root, "linked [worktree]")
	mustMkdir(t, mainWorktree)
	mustGit(t, mainWorktree, "init")
	mustGit(t, mainWorktree, "config", "user.email", "test@example.com")
	mustGit(t, mainWorktree, "config", "user.name", "Test User")
	mustWriteFile(t, filepath.Join(mainWorktree, "README"), "test\n")
	mustGit(t, mainWorktree, "add", "README")
	mustGit(t, mainWorktree, "commit", "-m", "initial")
	mustGit(t, mainWorktree, "worktree", "add", "-b", "linked", linkedWorktree)

	runner := OSRunner{}
	for _, worktree := range []string{mainWorktree, linkedWorktree} {
		if got := RepositoryIdentity(context.Background(), worktree, runner); got != filepath.Base(mainWorktree) {
			t.Errorf("RepositoryIdentity(%q) = %q, want %q", worktree, got, filepath.Base(mainWorktree))
		}
	}
}

func TestRepositoryIdentityFromGitDirectoryLayouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  string
	}{
		{
			name: "bare repository",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				bare := filepath.Join(root, "central.git")
				mustGit(t, root, "init", "--bare", bare)
				return bare
			},
			want: "central",
		},
		{
			name: "separate git directory",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				worktree := filepath.Join(root, "checkout")
				gitDir := filepath.Join(root, "shared metadata.git")
				mustMkdir(t, worktree)
				mustGit(t, worktree, "init", "--separate-git-dir="+gitDir)
				return worktree
			},
			want: "shared metadata",
		},
		{
			name: "non-git directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cwd := test.setup(t)
			if got := RepositoryIdentity(context.Background(), cwd, OSRunner{}); got != test.want {
				t.Fatalf("RepositoryIdentity() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWorkspaceTargetsPreferActiveTabAndIncludeAllPanes(t *testing.T) {
	t.Parallel()

	snapshot := Snapshot{
		Workspaces: []Workspace{{ID: "w1", ActiveTabID: "w1:t2", Label: "workspace"}},
		Panes: []Pane{
			{WorkspaceID: "w1", TabID: "w1:t1", ID: "w1:p1", CWD: "/old"},
			{WorkspaceID: "w1", TabID: "w1:t2", ID: "w1:p2", CWD: "/active"},
		},
	}

	want := []Target{{
		WorkspaceID: "w1",
		Label:       "workspace",
		CWD:         "/active",
		PaneIDs:     []string{"w1:p1", "w1:p2"},
	}}
	got := WorkspaceTargets(snapshot)
	if len(got) != len(want) || got[0].WorkspaceID != want[0].WorkspaceID ||
		got[0].Label != want[0].Label || got[0].CWD != want[0].CWD ||
		!slices.Equal(got[0].PaneIDs, want[0].PaneIDs) {
		t.Fatalf("WorkspaceTargets() = %#v, want %#v", got, want)
	}
}

type report struct {
	resource   string
	resourceID string
	value      *string
}

type fakeClient struct {
	snapshot Snapshot
	reports  []report
}

func (client *fakeClient) Snapshot(context.Context) (Snapshot, error) {
	return client.snapshot, nil
}

func (client *fakeClient) ReportToken(
	_ context.Context,
	resource string,
	resourceID string,
	value *string,
) error {
	var copiedValue *string
	if value != nil {
		copy := *value
		copiedValue = &copy
	}
	client.reports = append(client.reports, report{
		resource: resource, resourceID: resourceID, value: copiedValue,
	})
	return nil
}

func TestReconcileReportsGitIdentityFallbackAndClear(t *testing.T) {
	t.Parallel()

	repository := filepath.Join(t.TempDir(), "repo")
	mustMkdir(t, repository)
	mustGit(t, repository, "init")
	plain := t.TempDir()
	client := &fakeClient{snapshot: Snapshot{
		Workspaces: []Workspace{
			{ID: "w1", ActiveTabID: "w1:t1", Label: "git-workspace"},
			{ID: "w2", ActiveTabID: "w2:t1", Label: "plain-workspace"},
			{ID: "w3", ActiveTabID: "w3:t1"},
		},
		Panes: []Pane{
			{WorkspaceID: "w1", TabID: "w1:t1", ID: "w1:p1", CWD: repository},
			{WorkspaceID: "w2", TabID: "w2:t1", ID: "w2:p1", CWD: plain},
			{WorkspaceID: "w3", TabID: "w3:t1", ID: "w3:p1", CWD: plain},
		},
	}}

	failures, err := Reconcile(context.Background(), client, OSRunner{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if failures != 0 {
		t.Fatalf("Reconcile() failures = %d, want 0", failures)
	}

	want := []report{
		{resource: "workspace", resourceID: "w1", value: stringPointer("repo")},
		{resource: "pane", resourceID: "w1:p1", value: stringPointer("repo")},
		{resource: "workspace", resourceID: "w2", value: stringPointer("plain-workspace")},
		{resource: "pane", resourceID: "w2:p1", value: stringPointer("plain-workspace")},
		{resource: "workspace", resourceID: "w3", value: nil},
		{resource: "pane", resourceID: "w3:p1", value: nil},
	}
	if !reportsEqual(client.reports, want) {
		t.Fatalf("reports = %#v, want %#v", client.reports, want)
	}
}

type recordingRunner struct {
	calls [][]string
}

func (runner *recordingRunner) Run(_ context.Context, _ string, argv []string) CommandResult {
	call := slices.Clone(argv)
	runner.calls = append(runner.calls, call)
	if slices.Equal(call[1:], []string{"api", "snapshot"}) {
		snapshot := Snapshot{Workspaces: []Workspace{{ID: "w1"}}}
		stdout, _ := json.Marshal(map[string]any{
			"result": map[string]any{"snapshot": snapshot},
		})
		return CommandResult{Stdout: stdout, ExitCode: 0}
	}
	return CommandResult{Stdout: []byte("{}"), ExitCode: 0}
}

func TestHerdrClientUsesArgvAndPluginSource(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	client := NewHerdrClient("herdr-test", "plugin:test.repository-identity", runner)
	if _, err := client.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	value := "repository"
	if err := client.ReportToken(context.Background(), "workspace", "w1", &value); err != nil {
		t.Fatalf("ReportToken() error = %v", err)
	}

	want := []string{
		"herdr-test", "workspace", "report-metadata", "w1",
		"--source", "plugin:test.repository-identity", "--token", "repo=repository",
	}
	if len(runner.calls) != 2 || !slices.Equal(runner.calls[1], want) {
		t.Fatalf("calls = %#v, want second call %#v", runner.calls, want)
	}
}

func TestManifestBuildsGoDirectlyAndRunsBuiltBinary(t *testing.T) {
	t.Parallel()

	manifest, err := os.ReadFile(filepath.Join("..", "..", "herdr-plugin.toml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	contents := string(manifest)
	if !strings.Contains(contents, `command = ["go", "build", "-trimpath", "-o", "herdr-repository-identity", "./cmd/herdr-repository-identity"]`) {
		t.Error("manifest does not build the Go command directly")
	}
	if !strings.Contains(contents, `command = ["./herdr-repository-identity"]`) {
		t.Error("manifest does not run the built binary")
	}
	if strings.Contains(contents, "python") || strings.Contains(contents, `command = ["sh"`) {
		t.Error("manifest unexpectedly invokes Python or a shell")
	}
}

func mustGit(t *testing.T, cwd string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = cwd
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, output)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func stringPointer(value string) *string {
	return &value
}

func reportsEqual(left, right []report) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].resource != right[index].resource ||
			left[index].resourceID != right[index].resourceID {
			return false
		}
		if left[index].value == nil || right[index].value == nil {
			if left[index].value != nil || right[index].value != nil {
				return false
			}
			continue
		}
		if *left[index].value != *right[index].value {
			return false
		}
	}
	return true
}
