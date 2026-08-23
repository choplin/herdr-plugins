package picker

import (
	"fmt"
	"testing"

	"github.com/choplin/herdr-quickselect/internal/matcher"
)

func TestAssignHintsUsesFixedWidth(t *testing.T) {
	t.Parallel()

	matches := make([]matcher.Match, len(hintAlphabet)+1)
	for index := range matches {
		matches[index].Value = fmt.Sprintf("value-%d", index)
	}
	items := assignHints(matches)
	if items[0].Hint != "aa" || items[1].Hint != "as" || len(items[len(items)-1].Hint) != 2 {
		t.Fatalf("hints = first %q, second %q, last %q", items[0].Hint, items[1].Hint, items[len(items)-1].Hint)
	}
}

func TestAssignHintsUsesHomeRowOrder(t *testing.T) {
	t.Parallel()

	items := assignHints([]matcher.Match{{Value: "one"}, {Value: "two"}, {Value: "three"}})
	if items[0].Hint != "a" || items[1].Hint != "s" || items[2].Hint != "d" {
		t.Fatalf("hints = %#v", items)
	}
}

func TestAssignHintsSharesHintAcrossDuplicateOccurrences(t *testing.T) {
	t.Parallel()

	items := assignHints([]matcher.Match{{Value: "same", Line: 0}, {Value: "other"}, {Value: "same", Line: 2}})
	if len(items) != 2 || len(items[0].Occurrences) != 2 || items[0].Occurrences[1].Line != 2 {
		t.Fatalf("assignments = %#v", items)
	}
}
