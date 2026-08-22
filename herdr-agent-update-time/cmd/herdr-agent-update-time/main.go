package main

import (
	"context"
	"os"

	"github.com/choplin/herdr-agent-update-time/internal/updatetime"
)

const pluginID = "choplin.agent-update-time"

func main() {
	os.Exit(run())
}

func run() int {
	binary := environmentOrDefault("HERDR_BIN_PATH", "herdr")
	source := "plugin:" + environmentOrDefault("HERDR_PLUGIN_ID", pluginID)
	stateDir := environmentOrDefault("HERDR_PLUGIN_STATE_DIR", ".herdr-plugin-state")
	runner := updatetime.OSRunner{}
	client := updatetime.NewHerdrClient(binary, source, runner)
	event, targetPaneID := updatetime.EventContext(os.LookupEnv)

	failures, err := updatetime.WithStateLock(stateDir, func() (int, error) {
		return updatetime.Run(
			context.Background(),
			client,
			stateDir,
			event,
			targetPaneID,
			timeNow,
			updatetime.NewTrackingID,
			os.Stderr,
		)
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	if failures != 0 {
		return 1
	}
	return 0
}

func environmentOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

var timeNow = updatetime.Now
