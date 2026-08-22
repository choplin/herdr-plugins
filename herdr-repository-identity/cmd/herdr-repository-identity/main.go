package main

import (
	"context"
	"os"

	"github.com/choplin/herdr-repository-identity/internal/identity"
)

const pluginID = "choplin.repository-identity"

func main() {
	os.Exit(run())
}

func run() int {
	binary := environmentOrDefault("HERDR_BIN_PATH", "herdr")
	source := "plugin:" + environmentOrDefault("HERDR_PLUGIN_ID", pluginID)
	stateDir := environmentOrDefault("HERDR_PLUGIN_STATE_DIR", ".herdr-plugin-state")
	runner := identity.OSRunner{}
	client := identity.NewHerdrClient(binary, source, runner)

	failures, err := identity.WithReconcileLock(stateDir, func() (int, error) {
		return identity.Reconcile(context.Background(), client, runner, os.Stderr)
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
