package herdr

import (
	"math"
	"slices"
)

// Rect is a terminal-cell rectangle in Herdr-global coordinates.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Layout is the source tab geometry returned by pane.layout.
type Layout struct {
	Area          Rect          `json:"area"`
	FocusedPaneID string        `json:"focused_pane_id"`
	Panes         []LayoutPane  `json:"panes"`
	Splits        []LayoutSplit `json:"splits"`
	TabID         string        `json:"tab_id"`
	WorkspaceID   string        `json:"workspace_id"`
	Zoomed        bool          `json:"zoomed"`
}

// LayoutPane is one pane leaf in a layout.
type LayoutPane struct {
	Focused bool   `json:"focused"`
	PaneID  string `json:"pane_id"`
	Rect    Rect   `json:"rect"`
}

// LayoutSplit describes one binary split region.
type LayoutSplit struct {
	Direction string  `json:"direction"`
	Ratio     float64 `json:"ratio"`
	Rect      Rect    `json:"rect"`
}

// LayoutNode is a replayable source layout tree.
type LayoutNode struct {
	SourcePaneID string
	Direction    string
	Ratio        float64
	First        *LayoutNode
	Second       *LayoutNode
}

// LaunchNode is the layout.apply request tree.
type LaunchNode struct {
	Type      string            `json:"type"`
	Command   []string          `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Direction string            `json:"direction,omitempty"`
	Ratio     float64           `json:"ratio,omitempty"`
	First     *LaunchNode       `json:"first,omitempty"`
	Second    *LaunchNode       `json:"second,omitempty"`
	Picker    bool              `json:"-"`
}

// LayoutError reports geometry that cannot be safely replayed.
type LayoutError struct{ Detail string }

func (err *LayoutError) Error() string { return "invalid Herdr layout: " + err.Detail }

// DeriveLayoutTree reconstructs the binary split tree from pane.layout geometry.
func DeriveLayoutTree(layout Layout, targetPaneID string) (*LayoutNode, error) {
	if !slices.ContainsFunc(layout.Panes, func(pane LayoutPane) bool { return pane.PaneID == targetPaneID }) {
		return nil, &LayoutError{Detail: "target pane " + targetPaneID + " is absent"}
	}
	return buildLayoutNode(layout.Area, layout.Panes, layout.Splits)
}

// ContentSize returns the target pane content dimensions used by its PTY.
func ContentSize(layout Layout, targetPaneID string) (int, int, error) {
	paneIndex := slices.IndexFunc(layout.Panes, func(pane LayoutPane) bool { return pane.PaneID == targetPaneID })
	if paneIndex < 0 {
		return 0, 0, &LayoutError{Detail: "target pane " + targetPaneID + " is absent"}
	}
	rect := layout.Panes[paneIndex].Rect
	if layout.Zoomed && layout.FocusedPaneID == targetPaneID {
		rect = layout.Area
	}
	inset := 0
	if len(layout.Panes) > 1 {
		inset = 1
	}
	width := max(0, rect.Width-inset*2)
	height := max(0, rect.Height-inset*2)
	if width > 1 {
		width--
	}
	if width == 0 || height == 0 {
		return 0, 0, &LayoutError{Detail: "target pane has no visible content area"}
	}
	return width, height, nil
}

func buildLayoutNode(region Rect, panes []LayoutPane, splits []LayoutSplit) (*LayoutNode, error) {
	inRegion := make([]LayoutPane, 0, len(panes))
	for _, pane := range panes {
		if contains(region, pane.Rect) {
			inRegion = append(inRegion, pane)
		}
	}
	if len(inRegion) == 0 {
		return nil, &LayoutError{Detail: "layout region contains no panes"}
	}
	if len(inRegion) == 1 {
		return &LayoutNode{SourcePaneID: inRegion[0].PaneID}, nil
	}
	splitIndex := slices.IndexFunc(splits, func(split LayoutSplit) bool { return split.Rect == region })
	if splitIndex < 0 {
		return nil, &LayoutError{Detail: "split tree is incomplete"}
	}
	split := splits[splitIndex]
	firstPanes, secondPanes, err := partition(region, split, inRegion)
	if err != nil {
		return nil, err
	}
	if len(firstPanes) == 0 || len(secondPanes) == 0 {
		return nil, &LayoutError{Detail: "split has an empty child"}
	}
	firstRect := boundingRect(firstPanes)
	secondRect := boundingRect(secondPanes)
	first, err := buildLayoutNode(firstRect, panes, splits)
	if err != nil {
		return nil, err
	}
	second, err := buildLayoutNode(secondRect, panes, splits)
	if err != nil {
		return nil, err
	}
	return &LayoutNode{Direction: split.Direction, Ratio: split.Ratio, First: first, Second: second}, nil
}

func partition(region Rect, split LayoutSplit, panes []LayoutPane) ([]LayoutPane, []LayoutPane, error) {
	first := make([]LayoutPane, 0)
	second := make([]LayoutPane, 0)
	if split.Direction == "right" {
		boundary := region.X + int(math.Round(float64(region.Width)*split.Ratio))
		for _, pane := range panes {
			if pane.Rect.X+pane.Rect.Width <= boundary {
				first = append(first, pane)
			} else if pane.Rect.X >= boundary {
				second = append(second, pane)
			} else {
				return nil, nil, &LayoutError{Detail: "pane crosses a right split boundary"}
			}
		}
	} else {
		boundary := region.Y + int(math.Round(float64(region.Height)*split.Ratio))
		for _, pane := range panes {
			if pane.Rect.Y+pane.Rect.Height <= boundary {
				first = append(first, pane)
			} else if pane.Rect.Y >= boundary {
				second = append(second, pane)
			} else {
				return nil, nil, &LayoutError{Detail: "pane crosses a down split boundary"}
			}
		}
	}
	return first, second, nil
}

func contains(outer, inner Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

func boundingRect(panes []LayoutPane) Rect {
	minX, minY := panes[0].Rect.X, panes[0].Rect.Y
	maxX, maxY := panes[0].Rect.X+panes[0].Rect.Width, panes[0].Rect.Y+panes[0].Rect.Height
	for _, pane := range panes[1:] {
		minX = min(minX, pane.Rect.X)
		minY = min(minY, pane.Rect.Y)
		maxX = max(maxX, pane.Rect.X+pane.Rect.Width)
		maxY = max(maxY, pane.Rect.Y+pane.Rect.Height)
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}
