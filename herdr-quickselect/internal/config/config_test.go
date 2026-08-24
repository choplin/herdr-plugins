package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMergesEntriesByIDAndAddsCommand(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	contents := `
[[selectors]]
id = "ticket"
label = "Ticket"
regex = "TICKET-(?P<key>[0-9]+)"
capture = "key"
priority = 5

[[actions]]
id = "ticket-url"
label = "Open ticket"
type = "exec"
argv = ["open-ticket", "${value}"]

[[commands]]
id = "ticket"
label = "Open ticket"
selectors = ["ticket"]
action = "ticket-url"
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	command, ok := cfg.CommandByID("ticket")
	if !ok || command.Action != "ticket-url" {
		t.Fatalf("command = %#v, %t", command, ok)
	}
	if len(cfg.SelectorsFor(command)) != 1 || cfg.SelectorsFor(command)[0].Capture != "key" {
		t.Fatalf("selectors = %#v", cfg.SelectorsFor(command))
	}
	if _, ok := cfg.CommandByID("copy"); !ok {
		t.Fatal("built-in copy command was removed")
	}
}

func TestLoadRejectsUnknownReferences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	contents := `
[[commands]]
id = "broken"
label = "Broken"
selectors = ["missing"]
action = "clipboard"
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want unknown selector error")
	}
}

func TestDefaultsUseStructuredURLMatcher(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	selectors := cfg.SelectorsFor(cfg.Commands[1])
	if len(selectors) != 1 || selectors[0].Matcher != "url" || selectors[0].Regex != "" {
		t.Fatalf("open-url selectors = %#v", selectors)
	}
}

func TestLoadRejectsRegexOnURLMatcher(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	contents := `
[[selectors]]
id = "bad-url"
label = "URL"
matcher = "url"
regex = "https://.*"
priority = 5
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want conflicting URL matcher error")
	}
}

func TestLoadRejectsLegacyCommandPlaceholder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	contents := `
[[actions]]
id = "legacy"
label = "Legacy"
type = "exec"
argv = ["consume", "{value}"]
stdin = true
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want legacy placeholder error")
	}
}

func TestLoadDoesNotCountEscapedValueAsInput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	contents := `
[[actions]]
id = "escaped"
label = "Escaped"
type = "exec"
argv = ["consume", "$${value}"]
`
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want missing value input error")
	}
}

func TestPathUsesPluginConfigDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pluginConfigDirectory string
		want                  string
	}{
		{pluginConfigDirectory: "/plugins/config/choplin.quickselect", want: "/plugins/config/choplin.quickselect/config.toml"},
		{},
	}
	for _, test := range tests {
		if got := Path(test.pluginConfigDirectory); got != test.want {
			t.Errorf("Path() = %q, want %q", got, test.want)
		}
	}
}

func TestLoadReportsRenamedConfigurationKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "profiles", contents: `[[profiles]]
id = "old"
`, want: "profiles was renamed to commands"},
		{name: "action command", contents: `[[actions]]
id = "old"
label = "Old"
type = "exec"
command = ["consume", "${value}"]
`, want: "action command was renamed to argv"},
		{name: "command action type", contents: `[[actions]]
id = "old"
label = "Old"
type = "command"
argv = ["consume", "${value}"]
`, want: "type command was renamed to exec"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), FileName)
			if err := os.WriteFile(path, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}
