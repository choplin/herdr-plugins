package picker

import "testing"

func TestMapViewportJoinsSoftWrappedRows(t *testing.T) {
	t.Parallel()

	viewport := MapViewport("https://exa\nmple.com\n", 11, 2)
	if len(viewport.LogicalLines) != 1 || viewport.LogicalLines[0] != "https://example.com" {
		t.Fatalf("logical lines = %#v", viewport.LogicalLines)
	}
	if len(viewport.Segments) != 2 || viewport.Segments[1].LogicalStart != 11 {
		t.Fatalf("segments = %#v", viewport.Segments)
	}
}

func TestMapViewportPadsToHeightAndUsesDisplayWidth(t *testing.T) {
	t.Parallel()

	viewport := MapViewport("界x", 10, 3)
	if len(viewport.Rows) != 3 || viewport.Segments[0].ColumnEnd != 3 {
		t.Fatalf("viewport = %#v", viewport)
	}
}
