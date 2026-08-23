package picker

import (
	"strings"
	"testing"

	"github.com/choplin/herdr-quickselect/internal/config"
	"github.com/choplin/herdr-quickselect/internal/matcher"
)

func TestRenderPlacesDestructiveHintInline(t *testing.T) {
	t.Parallel()

	viewport := MapViewport("open https://example.com", 30, 1)
	matches := matcher.FindAll(viewport.LogicalLines, []config.Selector{{ID: "url", Matcher: "url"}})
	output := render(viewport, assignHints(matches), 30, 1)
	if !strings.Contains(output, hinted+"a") || !strings.Contains(output, matched+"t") {
		t.Fatalf("render output = %q", output)
	}
}

func TestRenderMapsHintAcrossSoftWrap(t *testing.T) {
	t.Parallel()

	viewport := MapViewport("https://exa\nmple.com", 11, 2)
	matches := matcher.FindAll(viewport.LogicalLines, []config.Selector{{ID: "url", Matcher: "url"}})
	assignments := assignHints(matches)
	output := render(viewport, assignments, 11, 2)
	if len(assignments) != 1 || !strings.Contains(output, hinted+"a") || !strings.Contains(output, "mple.com") {
		t.Fatalf("assignments = %#v, output = %q", assignments, output)
	}
}
