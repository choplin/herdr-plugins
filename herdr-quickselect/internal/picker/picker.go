// Package picker renders inline hints over a captured Herdr pane viewport.
package picker

import (
	"io"
	"os"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/choplin/herdr-quickselect/internal/config"
	"github.com/choplin/herdr-quickselect/internal/matcher"
)

const (
	clearScreen = "\x1b[2J"
	hideCursor  = "\x1b[?25l"
	showCursor  = "\x1b[?25h"
	reset       = "\x1b[0m"
	dim         = "\x1b[2;90m"
	matched     = "\x1b[0;33m"
	hinted      = "\x1b[1;30;46m"
)

// CancelledError reports a deliberate Escape or Ctrl-C cancellation.
type CancelledError struct{}

func (*CancelledError) Error() string { return "selection cancelled" }

// ErrCancelled identifies deliberate picker cancellation.
var ErrCancelled = &CancelledError{}

// TerminalError reports a terminal setup or input failure.
type TerminalError struct {
	Operation string
	Cause     error
}

func (err *TerminalError) Error() string { return err.Operation + ": " + err.Cause.Error() }

// Unwrap returns the underlying terminal failure.
func (err *TerminalError) Unwrap() error { return err.Cause }

// Terminal supplies the interactive input and output surface.
type Terminal struct {
	Input  *os.File
	Output io.Writer
}

// NewTerminal returns the process terminal.
func NewTerminal() Terminal {
	return Terminal{Input: os.Stdin, Output: os.Stdout}
}

// Select renders the source viewport with destructive inline hints and returns one value.
func (terminal Terminal) Select(viewport Viewport, selectors []config.Selector, width, height int) (string, error) {
	if !term.IsTerminal(int(terminal.Input.Fd())) {
		return "", &TerminalError{Operation: "open picker", Cause: os.ErrInvalid}
	}
	matches := matcher.FindAll(viewport.LogicalLines, selectors)
	assignments := assignHints(matches)

	oldState, err := term.MakeRaw(int(terminal.Input.Fd()))
	if err != nil {
		return "", &TerminalError{Operation: "enable raw mode", Cause: err}
	}
	defer func() {
		_, _ = io.WriteString(terminal.Output, reset+showCursor)
		_ = term.Restore(int(terminal.Input.Fd()), oldState)
	}()

	_, _ = io.WriteString(terminal.Output, hideCursor+render(viewport, assignments, width, height))
	if len(assignments) == 0 {
		if _, err := readByte(terminal.Input); err != nil {
			return "", &TerminalError{Operation: "read picker input", Cause: err}
		}
		return "", ErrCancelled
	}

	buffer := ""
	hintWidth := len(assignments[0].Hint)
	for {
		value, err := readByte(terminal.Input)
		if err != nil {
			return "", &TerminalError{Operation: "read picker input", Cause: err}
		}
		switch value {
		case 3, 27:
			return "", ErrCancelled
		case 8, 127:
			if buffer != "" {
				buffer = buffer[:len(buffer)-1]
			}
		default:
			if strings.ContainsRune(hintAlphabet, rune(value)) {
				buffer += string(value)
			}
		}
		if len(buffer) != hintWidth {
			continue
		}
		for _, assignment := range assignments {
			if assignment.Hint == buffer {
				return assignment.Value, nil
			}
		}
		buffer = ""
	}
}

type style uint8

const (
	styleUnmatched style = iota
	styleMatch
	styleHint
)

type cell struct {
	text  string
	style style
}

func render(viewport Viewport, assignments []Assignment, width, height int) string {
	if width <= 0 || height <= 0 {
		return clearScreen + reset
	}
	cells := make([][]cell, 0, height)
	for _, row := range viewport.Rows[:min(len(viewport.Rows), height)] {
		cells = append(cells, rowCells(row, width))
	}
	for len(cells) < height {
		cells = append(cells, rowCells("", width))
	}
	for _, assignment := range assignments {
		for _, occurrence := range assignment.Occurrences {
			applyOccurrence(cells, viewport, occurrence, assignment.Hint)
		}
	}
	if len(assignments) == 0 && height > 0 {
		message := " No matches · press any key to close "
		for column, char := range []rune(message) {
			if column >= width {
				break
			}
			cells[height-1][column] = cell{text: string(char), style: styleHint}
		}
	}

	var output strings.Builder
	output.WriteString(clearScreen)
	for rowIndex, row := range cells {
		output.WriteString(cursorPosition(rowIndex, 0))
		current := style(255)
		for _, value := range row {
			if value.style != current {
				output.WriteString(escapeForStyle(value.style))
				current = value.style
			}
			output.WriteString(value.text)
		}
	}
	output.WriteString(cursorPosition(0, 0))
	output.WriteString(reset)
	return output.String()
}

func rowCells(row string, width int) []cell {
	result := make([]cell, 0, width)
	for _, char := range row {
		charWidth := runewidth.RuneWidth(char)
		if charWidth <= 0 {
			continue
		}
		if len(result)+charWidth > width {
			break
		}
		result = append(result, cell{text: string(char), style: styleUnmatched})
		for range charWidth - 1 {
			result = append(result, cell{style: styleUnmatched})
		}
	}
	for len(result) < width {
		result = append(result, cell{text: " ", style: styleUnmatched})
	}
	return result
}

func applyOccurrence(cells [][]cell, viewport Viewport, occurrence matcher.Match, hint string) {
	positions := make([][2]int, 0, occurrence.End-occurrence.Start)
	for _, segment := range viewport.Segments {
		if segment.LogicalLine != occurrence.Line {
			continue
		}
		start := max(occurrence.Start, segment.LogicalStart)
		end := min(occurrence.End, segment.LogicalEnd)
		if start >= end || segment.Row >= len(viewport.Rows) {
			continue
		}
		row := viewport.Rows[segment.Row]
		startColumn, startOK := displayWidthAt(row, start-segment.LogicalStart)
		endColumn, endOK := displayWidthAt(row, end-segment.LogicalStart)
		if !startOK || !endOK {
			continue
		}
		for column := segment.ColumnStart + startColumn; column < segment.ColumnStart+endColumn; column++ {
			positions = append(positions, [2]int{segment.Row, column})
		}
	}
	for _, position := range positions {
		if position[0] < len(cells) && position[1] < len(cells[position[0]]) {
			cells[position[0]][position[1]].style = styleMatch
		}
	}
	for index, char := range hint {
		if index >= len(positions) {
			break
		}
		position := positions[index]
		if position[0] < len(cells) && position[1] < len(cells[position[0]]) {
			cells[position[0]][position[1]] = cell{text: string(char), style: styleHint}
		}
	}
}

func displayWidthAt(text string, byteOffset int) (int, bool) {
	if byteOffset < 0 || byteOffset > len(text) || !utf8Boundary(text, byteOffset) {
		return 0, false
	}
	return runewidth.StringWidth(text[:byteOffset]), true
}

func utf8Boundary(text string, offset int) bool {
	return offset == 0 || offset == len(text) || (offset < len(text) && text[offset]&0xc0 != 0x80)
}

func cursorPosition(row, column int) string {
	return "\x1b[" + decimal(row+1) + ";" + decimal(column+1) + "H"
}

func decimal(value int) string {
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}

func escapeForStyle(value style) string {
	switch value {
	case styleMatch:
		return matched
	case styleHint:
		return hinted
	default:
		return dim
	}
}

func readByte(input io.Reader) (byte, error) {
	buffer := []byte{0}
	_, err := io.ReadFull(input, buffer)
	return buffer[0], err
}
