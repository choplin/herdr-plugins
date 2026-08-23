// Package config loads Quick Select commands, selectors, and actions.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/choplin/herdr-quickselect/internal/placeholder"
)

// FileName is the Quick Select configuration beside Herdr's config.toml.
const FileName = "quickselect.toml"

// Config describes the complete picker configuration.
type Config struct {
	Selectors []Selector `toml:"selectors"`
	Actions   []Action   `toml:"actions"`
	Commands  []Command  `toml:"commands"`
}

// Selector extracts values from pane text.
type Selector struct {
	ID       string `toml:"id"`
	Label    string `toml:"label"`
	Matcher  string `toml:"matcher"`
	Regex    string `toml:"regex"`
	Capture  string `toml:"capture"`
	Priority int    `toml:"priority"`
}

// Action describes what happens to a selected value.
type Action struct {
	ID    string   `toml:"id"`
	Label string   `toml:"label"`
	Type  string   `toml:"type"`
	Argv  []string `toml:"argv"`
	Stdin bool     `toml:"stdin"`
}

// Command is one externally invokable composition of selectors and an action.
type Command struct {
	ID        string   `toml:"id"`
	Label     string   `toml:"label"`
	Selectors []string `toml:"selectors"`
	Action    string   `toml:"action"`
}

// Error reports invalid user configuration.
type Error struct {
	Detail string
}

func (err *Error) Error() string {
	return "invalid Quick Select configuration: " + err.Detail
}

// Defaults returns the useful zero-configuration setup.
func Defaults() Config {
	selectors := []Selector{
		{ID: "url", Label: "URL", Matcher: "url", Priority: 10},
		{ID: "path", Label: "Path", Regex: `(~|\.{1,2}|/)?([A-Za-z0-9_.-]+/)+[A-Za-z0-9_.@:+-]+`, Priority: 20},
		{ID: "uuid", Label: "UUID", Regex: `\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}\b`, Priority: 40},
		{ID: "git-sha", Label: "Git SHA", Regex: `\b[0-9a-fA-F]{7,40}\b`, Priority: 50},
		{ID: "ipv4", Label: "IPv4", Regex: `\b([0-9]{1,3}\.){3}[0-9]{1,3}\b`, Priority: 60},
		{ID: "number", Label: "Number", Regex: `\b[0-9]{6,}\b`, Priority: 70},
	}
	selectorIDs := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		selectorIDs = append(selectorIDs, selector.ID)
	}

	return Config{
		Selectors: selectors,
		Actions: []Action{
			{ID: "clipboard", Label: "Copy", Type: "clipboard"},
			{ID: "browser", Label: "Open", Type: "open"},
		},
		Commands: []Command{
			{ID: "copy", Label: "Copy visible item", Selectors: selectorIDs, Action: "clipboard"},
			{ID: "open-url", Label: "Open visible URL", Selectors: []string{"url"}, Action: "browser"},
		},
	}
}

// Path resolves Quick Select configuration beside Herdr's main configuration.
func Path(herdrConfigPath, xdgConfigHome, home string) string {
	if herdrConfigPath != "" {
		return filepath.Join(filepath.Dir(herdrConfigPath), FileName)
	}
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "herdr", FileName)
	}
	if home != "" {
		return filepath.Join(home, ".config", "herdr", FileName)
	}
	return ""
}

// Load merges a Quick Select configuration file over the built-in entries by ID.
func Load(path string) (Config, error) {
	result := Defaults()
	if path == "" {
		return result, validate(result)
	}

	var overrides Config
	metadata, err := toml.DecodeFile(path, &overrides)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, validate(result)
		}
		return Config{}, &Error{Detail: fmt.Sprintf("read %s: %v", path, err)}
	}
	if err := validateDecodedKeys(metadata); err != nil {
		return Config{}, err
	}

	result.Selectors = mergeByID(result.Selectors, overrides.Selectors, func(value Selector) string { return value.ID })
	result.Actions = mergeByID(result.Actions, overrides.Actions, func(value Action) string { return value.ID })
	result.Commands = mergeByID(result.Commands, overrides.Commands, func(value Command) string { return value.ID })
	if err := validate(result); err != nil {
		return Config{}, err
	}
	return result, nil
}

// CommandByID returns a configured command.
func (cfg Config) CommandByID(id string) (Command, bool) {
	for _, command := range cfg.Commands {
		if command.ID == id {
			return command, true
		}
	}
	return Command{}, false
}

// ActionByID returns a configured action.
func (cfg Config) ActionByID(id string) (Action, bool) {
	for _, action := range cfg.Actions {
		if action.ID == id {
			return action, true
		}
	}
	return Action{}, false
}

// SelectorsFor returns selectors in command order.
func (cfg Config) SelectorsFor(command Command) []Selector {
	result := make([]Selector, 0, len(command.Selectors))
	for _, id := range command.Selectors {
		for _, selector := range cfg.Selectors {
			if selector.ID == id {
				result = append(result, selector)
				break
			}
		}
	}
	return result
}

func validateDecodedKeys(metadata toml.MetaData) error {
	for _, key := range metadata.Undecoded() {
		name := key.String()
		switch {
		case name == "profiles" || strings.HasPrefix(name, "profiles."):
			return &Error{Detail: "profiles was renamed to commands"}
		case name == "actions.command" || strings.HasPrefix(name, "actions.") && strings.HasSuffix(name, ".command"):
			return &Error{Detail: "action command was renamed to argv"}
		default:
			return &Error{Detail: "unknown configuration key " + name}
		}
	}
	return nil
}

func mergeByID[T any](base, overrides []T, id func(T) string) []T {
	result := slices.Clone(base)
	for _, override := range overrides {
		index := slices.IndexFunc(result, func(value T) bool { return id(value) == id(override) })
		if index >= 0 {
			result[index] = override
		} else {
			result = append(result, override)
		}
	}
	return result
}

func validate(cfg Config) error {
	selectorIDs := make(map[string]struct{}, len(cfg.Selectors))
	for _, selector := range cfg.Selectors {
		if err := validateID("selector", selector.ID, selectorIDs); err != nil {
			return err
		}
		if selector.Matcher == "url" {
			if selector.Regex != "" || selector.Capture != "" {
				return &Error{Detail: fmt.Sprintf("URL selector %q cannot define regex or capture", selector.ID)}
			}
			continue
		}
		if selector.Matcher != "" && selector.Matcher != "regex" {
			return &Error{Detail: fmt.Sprintf("selector %q has unsupported matcher %q", selector.ID, selector.Matcher)}
		}
		compiled, err := regexp.Compile(selector.Regex)
		if err != nil {
			return &Error{Detail: fmt.Sprintf("selector %q regex: %v", selector.ID, err)}
		}
		if selector.Capture != "" && compiled.SubexpIndex(selector.Capture) < 0 {
			return &Error{Detail: fmt.Sprintf("selector %q has no named capture %q", selector.ID, selector.Capture)}
		}
	}

	actionIDs := make(map[string]struct{}, len(cfg.Actions))
	for _, action := range cfg.Actions {
		if err := validateID("action", action.ID, actionIDs); err != nil {
			return err
		}
		switch action.Type {
		case "clipboard", "open":
			if len(action.Argv) != 0 {
				return &Error{Detail: fmt.Sprintf("action %q type %q cannot define argv", action.ID, action.Type)}
			}
		case "exec":
			if len(action.Argv) == 0 {
				return &Error{Detail: fmt.Sprintf("action %q argv is empty", action.ID)}
			}
			if slices.ContainsFunc(action.Argv, placeholder.ContainsLegacy) {
				return &Error{Detail: fmt.Sprintf("action %q uses legacy placeholders; use ${value}, ${pane_id}, or ${cwd}", action.ID)}
			}
			if !action.Stdin && !slices.ContainsFunc(action.Argv, func(arg string) bool {
				return placeholder.Contains(arg, placeholder.Value)
			}) {
				return &Error{Detail: fmt.Sprintf("action %q must use ${value} or stdin = true", action.ID)}
			}
		case "command":
			return &Error{Detail: fmt.Sprintf("action %q type command was renamed to exec", action.ID)}
		default:
			return &Error{Detail: fmt.Sprintf("action %q has unsupported type %q", action.ID, action.Type)}
		}
	}

	commandIDs := make(map[string]struct{}, len(cfg.Commands))
	for _, command := range cfg.Commands {
		if err := validateID("command", command.ID, commandIDs); err != nil {
			return err
		}
		if len(command.Selectors) == 0 {
			return &Error{Detail: fmt.Sprintf("command %q has no selectors", command.ID)}
		}
		for _, selectorID := range command.Selectors {
			if _, ok := selectorIDs[selectorID]; !ok {
				return &Error{Detail: fmt.Sprintf("command %q references unknown selector %q", command.ID, selectorID)}
			}
		}
		if _, ok := actionIDs[command.Action]; !ok {
			return &Error{Detail: fmt.Sprintf("command %q references unknown action %q", command.ID, command.Action)}
		}
	}
	return nil
}

func validateID(kind, id string, seen map[string]struct{}) error {
	if id == "" {
		return &Error{Detail: kind + " id is empty"}
	}
	if _, ok := seen[id]; ok {
		return &Error{Detail: fmt.Sprintf("duplicate %s id %q", kind, id)}
	}
	seen[id] = struct{}{}
	return nil
}
