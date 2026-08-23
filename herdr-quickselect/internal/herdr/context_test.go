package herdr

import "testing"

func TestTargetPaneReadsSupportedContextShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		direct  string
		context string
		want    string
	}{
		{direct: "w1:p1", context: `{}`, want: "w1:p1"},
		{context: `{"focused_pane":{"pane_id":"w2:p3"}}`, want: "w2:p3"},
		{context: `{"focused_pane_id":"w3:p4"}`, want: "w3:p4"},
	}
	for _, test := range tests {
		got, err := TargetPane(test.direct, test.context)
		if err != nil || got != test.want {
			t.Errorf("TargetPane() = %q, %v, want %q", got, err, test.want)
		}
	}
}

func TestWorkspaceScopeUsesWorkspaceAcrossDifferentPanes(t *testing.T) {
	t.Parallel()

	if got := WorkspaceScope("", "w3:p1", `{}`); got != "workspace:w3" {
		t.Fatalf("WorkspaceScope() = %q", got)
	}
	if got := WorkspaceScope("", "w3:p9", `{"workspace_id":"w4"}`); got != "workspace:w4" {
		t.Fatalf("WorkspaceScope() from context = %q", got)
	}
}
