package matcher

import (
	"testing"

	"github.com/choplin/herdr-quickselect/internal/config"
)

func TestFindUsesPriorityForOverlapsAndDeduplicatesValues(t *testing.T) {
	t.Parallel()

	selectors := []config.Selector{
		{ID: "word", Label: "Word", Regex: `example\.com`, Priority: 20},
		{ID: "url", Label: "URL", Regex: `https://[^ ]+`, Priority: 10},
	}
	candidates := Find("https://example.com then https://example.com", selectors)
	if len(candidates) != 1 || candidates[0].Value != "https://example.com" || candidates[0].Kind != "URL" {
		t.Fatalf("Find() = %#v", candidates)
	}
}

func TestFindReturnsNamedCapture(t *testing.T) {
	t.Parallel()

	selectors := []config.Selector{{
		ID: "ticket", Label: "Ticket", Regex: `ticket=(?P<match>[A-Z]+-[0-9]+)`, Capture: "match",
	}}
	candidates := Find("prefix ticket=ABC-42 suffix", selectors)
	if len(candidates) != 1 || candidates[0].Value != "ABC-42" || candidates[0].Column != 14 {
		t.Fatalf("Find() = %#v", candidates)
	}
}

func TestFindAllRetainsDuplicateOccurrences(t *testing.T) {
	t.Parallel()

	selectors := []config.Selector{{ID: "url", Label: "URL", Regex: `https://[^ ]+`}}
	matches := FindAll([]string{"https://example.com", "again https://example.com"}, selectors)
	if len(matches) != 2 || matches[0].Line != 0 || matches[1].Line != 1 {
		t.Fatalf("FindAll() = %#v", matches)
	}
}

func TestURLMatcherStopsAtMarkdownAndJapaneseProse(t *testing.T) {
	t.Parallel()

	selector := config.Selector{ID: "url", Label: "URL", Matcher: "url"}
	matches := Find("参考: (https://github.com/rmarganti/herdr-pluck)、WezTerm", []config.Selector{selector})
	if len(matches) != 1 || matches[0].Value != "https://github.com/rmarganti/herdr-pluck" {
		t.Fatalf("Find() = %#v", matches)
	}
}

func TestURLMatcherRetainsValidReservedCharacters(t *testing.T) {
	t.Parallel()

	selector := config.Selector{ID: "url", Label: "URL", Matcher: "url"}
	text := "https://example.com/a_(b)?q=a,b&x=(y)#part"
	matches := Find(text, []config.Selector{selector})
	if len(matches) != 1 || matches[0].Value != text {
		t.Fatalf("Find() = %#v", matches)
	}
}

func TestURLMatcherTrimsSentencePunctuationAndRejectsMalformedURLs(t *testing.T) {
	t.Parallel()

	selector := config.Selector{ID: "url", Label: "URL", Matcher: "url"}
	matches := Find("see https://example.com/path., bad https:///path and https://example.com/%ZZ", []config.Selector{selector})
	if len(matches) != 1 || matches[0].Value != "https://example.com/path" {
		t.Fatalf("Find() = %#v", matches)
	}
}
