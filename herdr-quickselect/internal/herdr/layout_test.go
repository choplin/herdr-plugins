package herdr

import "testing"

func TestDeriveLayoutTreePreservesNestedSplits(t *testing.T) {
	t.Parallel()

	layout := Layout{
		Area: Rect{Width: 100, Height: 40},
		Panes: []LayoutPane{
			{PaneID: "tl", Rect: Rect{Width: 50, Height: 20}},
			{PaneID: "bl", Rect: Rect{Y: 20, Width: 50, Height: 20}},
			{PaneID: "tr", Rect: Rect{X: 50, Width: 50, Height: 20}},
			{PaneID: "br", Rect: Rect{X: 50, Y: 20, Width: 50, Height: 20}},
		},
		Splits: []LayoutSplit{
			{Direction: "right", Ratio: 0.5, Rect: Rect{Width: 100, Height: 40}},
			{Direction: "down", Ratio: 0.5, Rect: Rect{Width: 50, Height: 40}},
			{Direction: "down", Ratio: 0.5, Rect: Rect{X: 50, Width: 50, Height: 40}},
		},
	}
	tree, err := DeriveLayoutTree(layout, "br")
	if err != nil {
		t.Fatalf("DeriveLayoutTree() error = %v", err)
	}
	if tree.Direction != "right" || tree.First.Direction != "down" || tree.Second.Second.SourcePaneID != "br" {
		t.Fatalf("tree = %#v", tree)
	}
}

func TestContentSizeAccountsForBordersAndGutter(t *testing.T) {
	t.Parallel()

	layout := Layout{
		Area:          Rect{Width: 100, Height: 40},
		FocusedPaneID: "p1",
		Panes: []LayoutPane{
			{PaneID: "p1", Rect: Rect{Width: 50, Height: 40}},
			{PaneID: "p2", Rect: Rect{X: 50, Width: 50, Height: 40}},
		},
	}
	width, height, err := ContentSize(layout, "p1")
	if err != nil || width != 47 || height != 38 {
		t.Fatalf("ContentSize() = %d, %d, %v", width, height, err)
	}
	layout.Zoomed = true
	width, height, err = ContentSize(layout, "p1")
	if err != nil || width != 97 || height != 38 {
		t.Fatalf("zoomed ContentSize() = %d, %d, %v", width, height, err)
	}
}
