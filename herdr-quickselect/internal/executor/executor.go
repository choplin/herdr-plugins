// Package executor runs configured actions without a shell.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/choplin/herdr-quickselect/internal/config"
	"github.com/choplin/herdr-quickselect/internal/placeholder"
)

// CommandResult contains the observable result of an argv command.
type CommandResult struct {
	Stderr   string
	ExitCode int
	StartErr error
}

// Runner executes an argv command with optional standard input.
type Runner interface {
	Run(ctx context.Context, argv []string, stdin string) CommandResult
	LookPath(file string) bool
}

// OSRunner runs commands on the host operating system.
type OSRunner struct{}

// Run implements Runner.
func (OSRunner) Run(ctx context.Context, argv []string, stdin string) CommandResult {
	if len(argv) == 0 {
		return CommandResult{ExitCode: -1, StartErr: errors.New("empty command")}
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	command.Stdout = io.Discard
	command.Stderr = &stderr
	err := command.Run()
	result := CommandResult{Stderr: strings.TrimSpace(stderr.String())}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result
	}
	result.ExitCode = -1
	result.StartErr = err
	return result
}

// LookPath implements Runner.
func (OSRunner) LookPath(file string) bool {
	_, err := exec.LookPath(file)
	return err == nil
}

// Error reports an action execution failure.
type Error struct {
	Action string
	Detail string
}

func (err *Error) Error() string {
	return fmt.Sprintf("action %q failed: %s", err.Action, err.Detail)
}

// Execute applies an action to value.
func Execute(ctx context.Context, action config.Action, value, paneID, cwd string, runner Runner) error {
	var argv []string
	stdin := ""
	switch action.Type {
	case "clipboard":
		argv = clipboardCommand(runner)
		stdin = value
	case "open":
		argv = openCommand()
		if len(argv) != 0 {
			argv = append(argv, value)
		}
	case "exec":
		argv = expand(action.Argv, value, paneID, cwd)
		if action.Stdin {
			stdin = value
		}
	default:
		return &Error{Action: action.ID, Detail: "unsupported action type " + action.Type}
	}
	if len(argv) == 0 {
		return &Error{Action: action.ID, Detail: unavailableToolMessage(action.Type)}
	}

	result := runner.Run(ctx, argv, stdin)
	if result.StartErr != nil {
		return &Error{Action: action.ID, Detail: result.StartErr.Error()}
	}
	if result.ExitCode != 0 {
		detail := result.Stderr
		if detail == "" {
			detail = fmt.Sprintf("%s exited with status %d", argv[0], result.ExitCode)
		}
		return &Error{Action: action.ID, Detail: detail}
	}
	return nil
}

func clipboardCommand(runner Runner) []string {
	candidates := [][]string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, []string{"pbcopy"})
	case "linux":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			candidates = append(candidates, []string{"wl-copy"})
		}
		if os.Getenv("DISPLAY") != "" {
			candidates = append(
				candidates,
				[]string{"xclip", "-selection", "clipboard"},
				[]string{"xsel", "--clipboard", "--input"},
			)
		}
	}
	for _, candidate := range candidates {
		if runner.LookPath(candidate[0]) {
			return candidate
		}
	}
	return nil
}

func openCommand() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"open"}
	case "linux":
		return []string{"xdg-open"}
	default:
		return nil
	}
}

func expand(argv []string, value, paneID, cwd string) []string {
	values := placeholder.Values{Value: value, PaneID: paneID, CWD: cwd}
	result := make([]string, 0, len(argv))
	for _, arg := range argv {
		result = append(result, placeholder.Expand(arg, values))
	}
	return result
}

func unavailableToolMessage(actionType string) string {
	if actionType == "clipboard" {
		return "no supported clipboard command found"
	}
	return "no system opener is available"
}
