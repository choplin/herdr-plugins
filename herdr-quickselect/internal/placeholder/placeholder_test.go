package placeholder

import "testing"

func TestExpandSupportedAndEscapedPlaceholders(t *testing.T) {
	t.Parallel()

	values := Values{Value: "ABC-42", PaneID: "w1:p2", CWD: "/repo"}
	tests := []struct {
		input string
		want  string
	}{
		{input: "https://tracker/${value}", want: "https://tracker/ABC-42"},
		{input: "${pane_id}:${cwd}", want: "w1:p2:/repo"},
		{input: "$${value}", want: "${value}"},
		{input: "${unknown}", want: "${unknown}"},
	}
	for _, test := range tests {
		if got := Expand(test.input, values); got != test.want {
			t.Errorf("Expand(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestContainsDistinguishesExpansionFromEscaping(t *testing.T) {
	t.Parallel()

	if !Contains("prefix ${value}", Value) {
		t.Fatal("Contains() = false for value placeholder")
	}
	if Contains("prefix $${value}", Value) {
		t.Fatal("Contains() = true for escaped value placeholder")
	}
	if !ContainsLegacy("prefix {value}") || ContainsLegacy("prefix ${value}") {
		t.Fatal("ContainsLegacy() did not distinguish placeholder syntaxes")
	}
}
