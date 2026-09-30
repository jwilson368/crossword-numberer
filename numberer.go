package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// parseRow converts a line of grid text into a slice of "is this cell
// open" flags. Any byte equal to block is a black square; everything
// else (a letter, a dot, whatever the caller uses for an empty cell)
// counts as open.
func parseRow(line string, block byte) []bool {
	row := make([]bool, len(line))
	for i := 0; i < len(line); i++ {
		row[i] = line[i] != block
	}
	return row
}

// needsNumber reports whether the cell at col in curr starts an across
// word, a down word, or both — exactly the condition standard crossword
// numbering uses to decide which cells get a number. prev and next are
// the rows above and below curr, and may be nil to mean "off the grid",
// which is treated the same as a black square.
func needsNumber(prev, curr, next []bool, col int) bool {
	if !curr[col] {
		return false
	}

	leftBlocked := col == 0 || !curr[col-1]
	rightOpen := col+1 < len(curr) && curr[col+1]
	startsAcross := leftBlocked && rightOpen

	aboveBlocked := prev == nil || col >= len(prev) || !prev[col]
	belowOpen := next != nil && col < len(next) && next[col]
	startsDown := aboveBlocked && belowOpen

	return startsAcross || startsDown
}

// StreamNumber reads one or more crossword grids and writes the numbered
// grid for each to w. Deciding whether a cell starts a down word needs
// the row below it, so this holds at most three rows in memory at once
// (above, current, below) rather than the whole grid — a grid with a
// million rows costs the same handful of bytes as one with ten.
//
// A blank line ends the current grid and starts a new one, with its own
// numbering starting back at 1. This lets a single stream carry a batch
// of puzzles — say, a week of dailies concatenated together — without
// the caller having to split them first. Runs of more than one blank
// line are treated as a single separator, and leading or trailing blank
// lines are ignored.
func StreamNumber(r io.Reader, w io.Writer, block byte) error {
	scanner := bufio.NewScanner(r)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	var prev, curr []bool
	haveCurr := false
	counter := 0
	lineNum := 0
	width := -1
	needSeparator := false

	emit := func(prevRow, currRow, nextRow []bool) {
		parts := make([]string, len(currRow))
		for col, open := range currRow {
			switch {
			case !open:
				parts[col] = "#"
			case needsNumber(prevRow, currRow, nextRow, col):
				counter++
				parts[col] = fmt.Sprintf("%d", counter)
			default:
				parts[col] = "."
			}
		}
		fmt.Fprintln(bw, strings.Join(parts, " "))
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if line == "" {
			if haveCurr {
				emit(prev, curr, nil)
				prev, curr = nil, nil
				haveCurr = false
				counter = 0
				width = -1
				needSeparator = true
			}
			continue
		}

		next := parseRow(line, block)
		if width == -1 {
			width = len(next)
		} else if len(next) != width {
			return fmt.Errorf("line %d: row width %d does not match grid width %d", lineNum, len(next), width)
		}

		if needSeparator {
			fmt.Fprintln(bw)
			needSeparator = false
		}

		if haveCurr {
			emit(prev, curr, next)
			prev = curr
		}
		curr = next
		haveCurr = true
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading grid: %w", err)
	}
	if haveCurr {
		emit(prev, curr, nil)
	}
	return nil
}

// Clue is one entry in a clue list: the number printed in the grid, and
// how many cells the word occupies.
type Clue struct {
	Number int
	Length int
}

// acrossLength counts the open cells starting at col and running right,
// stopping at the first black square or the edge of the row.
func acrossLength(row []bool, col int) int {
	n := 0
	for c := col; c < len(row) && row[c]; c++ {
		n++
	}
	return n
}

// ClueList holds the across and down clues of a single grid.
type ClueList struct {
	Across []Clue
	Down   []Clue
}

// ListClues reads crossword grids the same way StreamNumber does, but
// instead of printing the numbered grid it returns one clue list (number
// and word length) per grid. Blank lines separate grids, with the same
// rules as StreamNumber. An across word's length is known as soon as its
// row is read, but a down word's length isn't known until the row where
// it ends, so this tracks one length-in-progress per column rather than
// buffering the whole grid — memory stays proportional to the grid's
// width, not its area.
func ListClues(r io.Reader, block byte) ([]ClueList, error) {
	scanner := bufio.NewScanner(r)

	type downState struct {
		number int
		length int
	}
	active := map[int]*downState{}

	var grids []ClueList
	var across, down []Clue
	var prev, curr []bool
	haveCurr := false
	counter := 0
	lineNum := 0
	width := -1

	process := func(prevRow, currRow, nextRow []bool) {
		for col, open := range currRow {
			if !open {
				if ds, ok := active[col]; ok {
					down = append(down, Clue{ds.number, ds.length})
					delete(active, col)
				}
				continue
			}

			leftBlocked := col == 0 || !currRow[col-1]
			rightOpen := col+1 < len(currRow) && currRow[col+1]
			startsAcross := leftBlocked && rightOpen

			aboveBlocked := prevRow == nil || col >= len(prevRow) || !prevRow[col]
			belowOpen := nextRow != nil && col < len(nextRow) && nextRow[col]
			startsDown := aboveBlocked && belowOpen

			if startsAcross || startsDown {
				counter++
			}
			if startsAcross {
				across = append(across, Clue{counter, acrossLength(currRow, col)})
			}
			switch {
			case startsDown:
				active[col] = &downState{number: counter, length: 1}
			case active[col] != nil:
				active[col].length++
			}
		}
	}

	// finish closes out the current grid: process the last row, flush any
	// down word still open (no trailing black square to close it), and
	// reset all per-grid state. Columns are walked in order so the result
	// is deterministic rather than following map order.
	finish := func() {
		process(prev, curr, nil)
		for col := 0; col < width; col++ {
			if ds, ok := active[col]; ok {
				down = append(down, Clue{ds.number, ds.length})
				delete(active, col)
			}
		}
		sort.Slice(down, func(i, j int) bool { return down[i].Number < down[j].Number })
		grids = append(grids, ClueList{Across: across, Down: down})

		across, down = nil, nil
		prev, curr = nil, nil
		haveCurr = false
		counter = 0
		width = -1
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if line == "" {
			if haveCurr {
				finish()
			}
			continue
		}

		next := parseRow(line, block)
		if width == -1 {
			width = len(next)
		} else if len(next) != width {
			return nil, fmt.Errorf("line %d: row width %d does not match grid width %d", lineNum, len(next), width)
		}

		if haveCurr {
			process(prev, curr, next)
			prev = curr
		}
		curr = next
		haveCurr = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading grid: %w", err)
	}
	if haveCurr {
		finish()
	}

	return grids, nil
}

// WriteClueList prints each grid's across and down clues as "number.
// length" under headers, the format most crossword clue lists use before
// the clue text itself is filled in. Grids are separated by a blank line,
// matching the numbered-grid output.
func WriteClueList(w io.Writer, grids []ClueList) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for i, g := range grids {
		if i > 0 {
			fmt.Fprintln(bw)
		}
		fmt.Fprintln(bw, "Across")
		for _, c := range g.Across {
			fmt.Fprintf(bw, "%d. %d\n", c.Number, c.Length)
		}
		fmt.Fprintln(bw, "Down")
		for _, c := range g.Down {
			fmt.Fprintf(bw, "%d. %d\n", c.Number, c.Length)
		}
	}
}
