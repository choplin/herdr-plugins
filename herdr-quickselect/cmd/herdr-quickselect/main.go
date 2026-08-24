package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/choplin/herdr-quickselect/internal/config"
	"github.com/choplin/herdr-quickselect/internal/executor"
	"github.com/choplin/herdr-quickselect/internal/herdr"
	"github.com/choplin/herdr-quickselect/internal/picker"
)

func main() {
	os.Exit(run(context.Background(), os.Args, os.Environ()))
}

func run(ctx context.Context, argv, environment []string) int {
	if len(argv) < 2 {
		writeUsage()
		return 2
	}
	var err error
	switch argv[1] {
	case "launch":
		if len(argv) != 3 {
			writeUsage()
			return 2
		}
		err = launch(ctx, argv[2], environment)
	case "pick":
		err = pick(ctx, argv[2:], environment)
	case "idle":
		err = idle(ctx)
	default:
		writeUsage()
		return 2
	}
	if err == nil || errors.Is(err, picker.ErrCancelled) {
		return 0
	}
	_, _ = fmt.Fprintln(os.Stderr, err)
	return 1
}

func launch(ctx context.Context, commandID string, environment []string) (result error) {
	paneID, err := herdr.TargetPane(
		environmentOr(environment, "HERDR_PANE_ID", ""),
		environmentOr(environment, "HERDR_PLUGIN_CONTEXT_JSON", ""),
	)
	if err != nil {
		return err
	}
	pluginConfigDirectory := environmentOr(environment, "HERDR_PLUGIN_CONFIG_DIR", "")
	contextJSON := environmentOr(environment, "HERDR_PLUGIN_CONTEXT_JSON", "")
	workspaceScope := herdr.WorkspaceScope(
		environmentOr(environment, "HERDR_WORKSPACE_ID", ""), paneID, contextJSON,
	)
	lock, err := herdr.AcquireSessionLock(pluginConfigDirectory, workspaceScope)
	if err != nil {
		var active *herdr.AlreadyActiveError
		if errors.As(err, &active) {
			return nil
		}
		return err
	}
	defer func() { result = errors.Join(result, lock.Release()) }()
	configPath := config.Path(pluginConfigDirectory)
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	command, ok := cfg.CommandByID(commandID)
	if !ok {
		return &unknownCommandError{ID: commandID}
	}
	action, ok := cfg.ActionByID(command.Action)
	if !ok {
		return &unknownActionError{ID: command.Action}
	}
	socketPath := environmentOr(environment, "HERDR_SOCKET_PATH", "")
	if socketPath == "" {
		return &missingEnvironmentError{Name: "HERDR_SOCKET_PATH"}
	}
	binaryPath, err := os.Executable()
	if err != nil {
		return &executablePathError{Cause: err}
	}
	session, err := herdr.PrepareAndLaunch(
		ctx, herdr.SocketClient{Path: socketPath}, paneID, binaryPath,
		herdr.FocusedCWD(contextJSON),
		command, action, cfg.SelectorsFor(command),
	)
	if err != nil {
		return err
	}
	return session.Wait(ctx, herdr.SocketClient{Path: socketPath})
}

func pick(ctx context.Context, argv, environment []string) error {
	arguments := flag.NewFlagSet("pick", flag.ContinueOnError)
	arguments.SetOutput(os.Stderr)
	snapshotPath := arguments.String("snapshot", "", "picker snapshot path")
	readyPath := arguments.String("ready", "", "picker readiness path")
	if err := arguments.Parse(argv); err != nil {
		return &argumentError{Cause: err}
	}
	if *snapshotPath == "" || *readyPath == "" {
		return &argumentError{Detail: "--snapshot and --ready are required"}
	}
	socketPath := environmentOr(environment, "HERDR_SOCKET_PATH", "")
	temporaryTabID := environmentOr(environment, "HERDR_TAB_ID", "")
	if socketPath == "" || temporaryTabID == "" {
		return &missingEnvironmentError{Name: "HERDR_SOCKET_PATH or HERDR_TAB_ID"}
	}
	snapshot, files, err := herdr.ReadSnapshot(ctx, *snapshotPath, *readyPath)
	if err != nil {
		if snapshot.ReturnTabID != "" {
			client := herdr.SocketClient{Path: socketPath}
			return errors.Join(err, herdr.Cleanup(ctx, client, snapshot.ReturnTabID, temporaryTabID), files.Remove())
		}
		return errors.Join(err, files.Remove())
	}
	client := herdr.SocketClient{Path: socketPath}
	selected, primary := picker.NewTerminal().Select(
		snapshot.Viewport, snapshot.Selectors, snapshot.Width, snapshot.Height,
	)
	if primary == nil {
		primary = executor.Execute(
			ctx, snapshot.Action, selected, snapshot.TargetPaneID, snapshot.CWD, executor.OSRunner{},
		)
	}
	notifyActionSuccess(ctx, snapshot.Action, primary, client)
	cleanup := herdr.Cleanup(ctx, client, snapshot.ReturnTabID, temporaryTabID)
	return errors.Join(primary, cleanup, files.Remove())
}

type notifier interface {
	ShowNotification(context.Context, string) error
}

func notifyActionSuccess(ctx context.Context, action config.Action, actionErr error, client notifier) {
	if actionErr != nil {
		return
	}
	title := ""
	switch action.Type {
	case "clipboard":
		title = "Copied to clipboard"
	case "open":
		title = "Opened URL in browser"
	default:
		return
	}
	// A notification is useful feedback, but its failure must not turn a
	// successful action into a failed action.
	_ = client.ShowNotification(ctx, title)
}

func idle(ctx context.Context) error {
	_, _ = os.Stdout.WriteString("\x1b[2J\x1b[H")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-ctx.Done():
		return nil
	case <-signals:
		return nil
	}
}

type unknownCommandError struct{ ID string }

func (err *unknownCommandError) Error() string { return "unknown Quick Select command: " + err.ID }

type unknownActionError struct{ ID string }

func (err *unknownActionError) Error() string { return "unknown Quick Select action: " + err.ID }

type missingEnvironmentError struct{ Name string }

func (err *missingEnvironmentError) Error() string { return "missing environment variable " + err.Name }

type executablePathError struct{ Cause error }

func (err *executablePathError) Error() string {
	return "locate Quick Select executable: " + err.Cause.Error()
}
func (err *executablePathError) Unwrap() error { return err.Cause }

type argumentError struct {
	Cause  error
	Detail string
}

func (err *argumentError) Error() string {
	if err.Detail != "" {
		return "invalid picker arguments: " + err.Detail
	}
	return "invalid picker arguments: " + err.Cause.Error()
}
func (err *argumentError) Unwrap() error { return err.Cause }

func environmentOr(environment []string, name, fallback string) string {
	prefix := name + "="
	for _, entry := range environment {
		if len(entry) >= len(prefix) && entry[:len(prefix)] == prefix {
			return entry[len(prefix):]
		}
	}
	return fallback
}

func writeUsage() {
	_, _ = fmt.Fprintln(os.Stderr, "usage: herdr-quickselect launch <command> | pick --snapshot PATH --ready PATH | idle")
}
