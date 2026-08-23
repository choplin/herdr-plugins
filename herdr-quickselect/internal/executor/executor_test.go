package executor

import (
	"context"
	"slices"
	"testing"

	"github.com/choplin/herdr-quickselect/internal/config"
)

type recordingRunner struct {
	argv  []string
	stdin string
}

func (runner *recordingRunner) Run(_ context.Context, argv []string, stdin string) CommandResult {
	runner.argv = slices.Clone(argv)
	runner.stdin = stdin
	return CommandResult{}
}

func (*recordingRunner) LookPath(string) bool { return true }

func TestExecuteExpandsCommandPlaceholdersWithoutShell(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	action := config.Action{
		ID:   "ticket",
		Type: "exec",
		Argv: []string{"ticket-open", "--pane", "${pane_id}", "https://tracker/${value}", "${cwd}"},
	}
	if err := Execute(context.Background(), action, "ABC-42", "w1:p2", "/repo", runner); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{"ticket-open", "--pane", "w1:p2", "https://tracker/ABC-42", "/repo"}
	if !slices.Equal(runner.argv, want) || runner.stdin != "" {
		t.Fatalf("command = %#v, stdin = %q, want %#v", runner.argv, runner.stdin, want)
	}
}

func TestExecutePassesValueOnStdin(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	action := config.Action{ID: "consume", Type: "exec", Argv: []string{"consume"}, Stdin: true}
	if err := Execute(context.Background(), action, "selected", "", "", runner); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.stdin != "selected" {
		t.Fatalf("stdin = %q, want selected", runner.stdin)
	}
}
