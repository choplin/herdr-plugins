package picker

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// Segment maps one logical byte range to an exact visible row.
type Segment struct {
	LogicalLine  int `json:"logical_line"`
	LogicalStart int `json:"logical_start"`
	LogicalEnd   int `json:"logical_end"`
	Row          int `json:"row"`
	ColumnStart  int `json:"column_start"`
	ColumnEnd    int `json:"column_end"`
}

// Viewport preserves visible rows and reconstructed soft-wrapped logical lines.
type Viewport struct {
	Rows         []string  `json:"rows"`
	LogicalLines []string  `json:"logical_lines"`
	Segments     []Segment `json:"segments"`
}

// MapViewport reconstructs logical lines while preserving exact visible coordinates.
func MapViewport(text string, width, height int) Viewport {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	rows := strings.Split(text, "\n")
	if text == "" {
		rows = nil
	}
	if len(rows) > height {
		rows = rows[:height]
	}
	for len(rows) < height {
		rows = append(rows, "")
	}

	viewport := Viewport{Rows: rows}
	current := ""
	currentSegments := make([]Segment, 0)
	for rowIndex, row := range rows {
		start := len(current)
		current += row
		currentSegments = append(currentSegments, Segment{
			LogicalLine:  len(viewport.LogicalLines),
			LogicalStart: start,
			LogicalEnd:   len(current),
			Row:          rowIndex,
			ColumnEnd:    runewidth.StringWidth(row),
		})
		if width == 0 || runewidth.StringWidth(row) < width {
			viewport.LogicalLines = append(viewport.LogicalLines, current)
			viewport.Segments = append(viewport.Segments, currentSegments...)
			current = ""
			currentSegments = nil
		}
	}
	if len(currentSegments) != 0 {
		viewport.LogicalLines = append(viewport.LogicalLines, current)
		viewport.Segments = append(viewport.Segments, currentSegments...)
	}
	return viewport
}
